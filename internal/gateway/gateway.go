// Package gateway is Discreet's HTTP server: an OpenAI-compatible
// /v1/chat/completions endpoint that protects personal data on the way to
// the model and restores it on the way back, recording every decision in
// the audit log.
package gateway

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/makoydev/discreet/internal/audit"
	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/protect"
	"github.com/makoydev/discreet/internal/upstream"
	"github.com/makoydev/discreet/internal/vault"
)

// Footer is appended to every answer, so people reading it know.
const Footer = "\n\n---\nPersonal data was replaced before this request reached the AI model, and put back afterwards (Discreet request %s)."

// Config wires the gateway together.
type Config struct {
	Engine       *detect.Engine
	Policy       *protect.Policy
	Vault        *vault.Vault
	Audit        *audit.Log
	Upstream     upstream.Client // the free mock everyone gets by default
	Logger       *slog.Logger
	Tenant       string // tenant name for callers without a valid token
	MaxBodyBytes int64

	// The real model, used only for callers with a valid access token
	// (ADR 0007). Nil means everyone gets the mock.
	Real            upstream.Client
	Tokens          *Tokens
	Model           string // the one model Real may be asked for
	Price           Price
	Budget          *Budget
	MaxOutputTokens int
	RateLimit       *RateLimiter
}

// Server handles requests. Create it with New.
type Server struct{ cfg Config }

// New returns a server, filling in defaults.
func New(cfg Config) *Server {
	if cfg.Upstream == nil {
		cfg.Upstream = &upstream.Mock{}
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.Default()
	}
	if cfg.Tenant == "" {
		cfg.Tenant = "public"
	}
	if cfg.MaxOutputTokens == 0 {
		cfg.MaxOutputTokens = 2048
	}
	if cfg.MaxBodyBytes == 0 {
		cfg.MaxBodyBytes = 64 << 10
	}
	return &Server{cfg}
}

// Handler returns the HTTP routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v1/chat/completions", s.chatCompletions)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok\n")) })
	s.demoRoutes(mux)
	return mux
}

// chatRequest is the subset of OpenAI's request Discreet understands.
// Anything else in the body is ignored and never forwarded.
type chatRequest struct {
	Model               string          `json:"model"`
	Messages            []chatMessageIn `json:"messages"`
	MaxTokens           *int            `json:"max_tokens"`
	MaxCompletionTokens *int            `json:"max_completion_tokens"`
	Temperature         *float64        `json:"temperature"`
	Stream              bool            `json:"stream"`
}

type chatMessageIn struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

// text returns a message's text: a string, or an array of text parts.
func (m chatMessageIn) text() (string, error) {
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return s, nil
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(m.Content, &parts); err != nil {
		return "", errors.New("message content must be a string or an array of text parts")
	}
	var b strings.Builder
	for _, p := range parts {
		if p.Type != "text" {
			return "", fmt.Errorf("content part type %q is not supported; only text", p.Type)
		}
		b.WriteString(p.Text)
	}
	return b.String(), nil
}

var allowedRoles = map[string]bool{"system": true, "developer": true, "user": true, "assistant": true}

type apiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var e apiError
	e.Error.Message, e.Error.Type, e.Error.Code = message, "discreet_error", code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(e)
}

func newID() string {
	b := make([]byte, 12)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Server) chatCompletions(w http.ResponseWriter, r *http.Request) {
	id := newID()
	w.Header().Set("X-Discreet-Request-Id", id)
	if !s.allow(w, r, id) {
		return
	}
	var req chatRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, s.cfg.MaxBodyBytes)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_request", "request body is not valid JSON or is too large")
		return
	}
	if req.Stream {
		writeError(w, http.StatusBadRequest, "streaming_unsupported", "streaming is not supported in Discreet v0.1; send stream: false")
		return
	}
	if len(req.Messages) == 0 {
		writeError(w, http.StatusBadRequest, "invalid_request", "messages must not be empty")
		return
	}
	c := call{id: id, purpose: strings.TrimSpace(r.Header.Get("X-Discreet-Purpose")), model: req.Model, temperature: req.Temperature}
	for _, m := range req.Messages {
		if !allowedRoles[m.Role] {
			writeError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("message role %q is not supported", m.Role))
			return
		}
		t, err := m.text()
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
			return
		}
		c.roles, c.texts = append(c.roles, m.Role), append(c.texts, t)
	}
	if req.MaxCompletionTokens != nil {
		c.maxTokens = *req.MaxCompletionTokens
	} else if req.MaxTokens != nil {
		c.maxTokens = *req.MaxTokens
	}

	// Only a valid access token reaches the real model; anyone else gets
	// the free mock, so the public endpoint can never spend money.
	c.up, c.tenant = s.cfg.Upstream, s.cfg.Tenant
	if t, ok := s.cfg.Tokens.Tenant(r); ok {
		c.tenant = t
		if s.cfg.Real != nil {
			c.up, c.real = s.cfg.Real, true
		}
	}
	w.Header().Set("X-Discreet-Upstream", c.up.Name())

	o := s.process(r.Context(), c)
	if o.status != http.StatusOK {
		writeError(w, o.status, o.code, o.message)
		return
	}
	w.Header().Set("X-Discreet-Entities", entityHeader(o.record.Entities))
	w.Header().Set("X-Discreet-Audit-Seq", fmt.Sprint(o.record.Seq))
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false) // keep <NRIC_1> readable in raw output
	enc.Encode(map[string]any{
		"id":      "chatcmpl-" + id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   o.response.Model,
		"choices": []map[string]any{{
			"index":         0,
			"message":       map[string]string{"role": "assistant", "content": o.answer},
			"finish_reason": o.response.FinishReason,
		}},
		"usage": map[string]int{
			"prompt_tokens":     o.response.PromptTokens,
			"completion_tokens": o.response.OutputTokens,
			"total_tokens":      o.response.PromptTokens + o.response.OutputTokens,
		},
	})
}

// allow applies the per-address rate limit.
func (s *Server) allow(w http.ResponseWriter, r *http.Request, id string) bool {
	if s.cfg.RateLimit.Allow(r) {
		return true
	}
	s.cfg.Logger.Info("rate limited", "request_id", id)
	w.Header().Set("Retry-After", "10")
	writeError(w, http.StatusTooManyRequests, "rate_limited", "too many requests from this address; try again shortly")
	return false
}

// finish writes the audit record. If it can't, the caller must fail the
// request: Discreet never answers a request it couldn't record.
func (s *Server) finish(log *slog.Logger, rec audit.Record, start time.Time) (audit.Record, bool) {
	rec.LatencyMS = time.Since(start).Milliseconds()
	saved, err := s.cfg.Audit.Append(rec)
	if err != nil {
		log.Error("audit write failed", "error", err.Error())
		return saved, false
	}
	log.Info("request", "decision", rec.Decision, "entities", entityHeader(rec.Entities), "upstream", rec.Upstream, "latency_ms", rec.LatencyMS, "audit_seq", saved.Seq)
	return saved, true
}

// scrub replaces personal data in short metadata (the purpose) with the
// entity name, and caps its length.
func (s *Server) scrub(text string) string {
	if len(text) > 200 {
		text = text[:200]
	}
	var b strings.Builder
	pos := 0
	for _, m := range s.cfg.Engine.Detect(text) {
		b.WriteString(text[pos:m.Start] + "[" + m.Entity + "]")
		pos = m.End
	}
	return b.String() + text[pos:]
}

// entityHeader summarises counts, never values: "NRIC=2,PHONE=1".
func entityHeader(counts map[string]int) string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%d", k, counts[k])
	}
	return strings.Join(parts, ",")
}
