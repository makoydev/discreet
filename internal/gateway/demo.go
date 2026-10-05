package gateway

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/makoydev/discreet/internal/protect"
)

//go:embed web
var webFiles embed.FS

// The demo page loads nothing but its own files and can only call back to
// this server.
const demoCSP = "default-src 'none'; script-src 'self'; style-src 'self' https://fonts.googleapis.com; " +
	"font-src https://fonts.gstatic.com; connect-src 'self'; img-src 'self' data:; " +
	"base-uri 'none'; form-action 'none'; frame-ancestors 'none'"

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", demoCSP)
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) demoRoutes(mux *http.ServeMux) {
	static, _ := fs.Sub(webFiles, "web")
	files := securityHeaders(http.FileServerFS(static))
	mux.Handle("GET /{$}", files)
	mux.Handle("GET /demo.js", files)
	mux.Handle("GET /demo.css", files)
	mux.HandleFunc("POST /demo/run", s.demoRun)
}

type demoPart struct {
	Text        string `json:"text"`
	Placeholder string `json:"placeholder,omitempty"`
}

// demoRun runs one message through exactly the same pipeline as the API,
// always with the mock model, and returns every stage for the four panes.
func (s *Server) demoRun(w http.ResponseWriter, r *http.Request) {
	id := newID()
	if !s.allow(w, r, id) {
		return
	}
	var in struct {
		Text    string `json:"text"`
		Purpose string `json:"purpose"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10)).Decode(&in); err != nil || strings.TrimSpace(in.Text) == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "send some text")
		return
	}
	if utf8.RuneCountInString(in.Text) > 4000 {
		writeError(w, http.StatusBadRequest, "invalid_request", "the demo takes up to 4,000 characters")
		return
	}
	o := s.process(r.Context(), call{
		id: id, tenant: "demo", purpose: strings.TrimSpace(in.Purpose),
		roles: []string{"user"}, texts: []string{in.Text},
		up: s.cfg.Upstream, // the demo never uses the real model
	})
	out := map[string]any{"request_id": id, "original": in.Text}
	if o.status != http.StatusOK {
		out["status"], out["code"], out["message"] = "refused", o.code, o.message
		if o.record.Seq > 0 {
			out["audit_record"] = o.record
		}
	} else {
		parts := make([]demoPart, 0, len(o.parts)+1)
		for _, p := range o.parts {
			parts = append(parts, demoPart(p))
		}
		parts = append(parts, demoPart{Text: strings.TrimPrefix(o.answer, protect.Join(o.parts))})
		out["status"] = "answered"
		out["sent_to_model"] = o.sent[0].Content
		out["model_answer"] = o.response.Content
		out["returned"] = parts
		out["audit_record"] = o.record
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.Encode(out)
}
