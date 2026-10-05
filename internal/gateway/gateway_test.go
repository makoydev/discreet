package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/makoydev/discreet/internal/audit"
	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/protect"
	"github.com/makoydev/discreet/internal/upstream"
	"github.com/makoydev/discreet/internal/vault"
)

// Synthetic values only. The NRIC is sg-pii-rules' worked example; the
// card is a published test number; the email uses a reserved domain.
const (
	nric  = "S1234567D"
	phone = "9123 4567"
	email = "tan.ah.kow@example.com"
	card  = "4111 1111 1111 1111"
)

var planted = []string{nric, phone, email, card, "4111111111111111"}

type harness struct {
	srv  *httptest.Server
	mock *upstream.Mock
	logs interface {
		io.Writer
		String() string
	}
	logPath string
	audit   *audit.Log
}

func newHarness(t *testing.T, policy *protect.Policy, up upstream.Client) *harness {
	t.Helper()
	engine, err := detect.Default()
	if err != nil {
		t.Fatal(err)
	}
	v, _ := vault.New(time.Minute)
	h := &harness{logs: &bytes.Buffer{}, logPath: filepath.Join(t.TempDir(), "audit.jsonl")}
	h.audit, err = audit.Open(h.logPath, []byte("test-key-0123456789abcdefghijklmnop"))
	if err != nil {
		t.Fatal(err)
	}
	if up == nil {
		h.mock = &upstream.Mock{}
		up = h.mock
	}
	logger := slog.New(slog.NewJSONHandler(h.logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := New(Config{Engine: engine, Policy: policy, Vault: v, Audit: h.audit, Upstream: up, Logger: logger})
	h.srv = httptest.NewServer(s.Handler())
	t.Cleanup(h.srv.Close)
	return h
}

func (h *harness) post(t *testing.T, purpose, body string) (*http.Response, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest("POST", h.srv.URL+"/v1/chat/completions", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if purpose != "" {
		req.Header.Set("X-Discreet-Purpose", purpose)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func (h *harness) records(t *testing.T) []audit.Record {
	t.Helper()
	data, _ := os.ReadFile(h.logPath)
	var out []audit.Record
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var r audit.Record
		json.Unmarshal([]byte(line), &r)
		out = append(out, r)
	}
	return out
}

func body(messages ...string) string {
	type msg struct{ Role, Content string }
	var ms []map[string]string
	for i, m := range messages {
		role := "user"
		if i == 0 && len(messages) > 1 {
			role = "system"
		}
		ms = append(ms, map[string]string{"role": role, "content": m})
	}
	data, _ := json.Marshal(map[string]any{"model": "gpt-6-luna", "messages": ms})
	return string(data)
}

func answer(t *testing.T, out map[string]any) string {
	t.Helper()
	choices, ok := out["choices"].([]any)
	if !ok || len(choices) == 0 {
		t.Fatalf("no choices in %v", out)
	}
	return choices[0].(map[string]any)["message"].(map[string]any)["content"].(string)
}

func TestTheModelNeverSeesPersonalData(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), nil)
	resp, out := h.post(t, "claims summary", body(
		"You help with insurance claims.",
		"Member "+nric+" (phone "+phone+", "+email+") disputes a charge on card "+card+". Draft a reply to "+nric+"."))
	if resp.StatusCode != 200 {
		t.Fatalf("status %d: %v", resp.StatusCode, out)
	}

	var all []string
	for _, m := range h.mock.Last.Messages {
		all = append(all, m.Content)
	}
	seen := []byte(strings.Join(all, "\n"))
	for _, v := range planted {
		if strings.Contains(string(seen), v) {
			t.Errorf("the model received %q", v)
		}
	}
	for _, p := range []string{"<NRIC_1>", "<PHONE_1>", "<EMAIL_1>", "[REDACTED_CARD]"} {
		if !strings.Contains(string(seen), p) {
			t.Errorf("the model did not receive %s", p)
		}
	}
	if strings.Count(string(seen), "<NRIC_1>") != 2 {
		t.Error("the same NRIC should be the same placeholder both times")
	}

	got := answer(t, out)
	for _, v := range []string{nric, phone, email} {
		if !strings.Contains(got, v) {
			t.Errorf("answer is missing restored %q: %s", v, got)
		}
	}
	if strings.Contains(got, "4111") || !strings.Contains(got, "[REDACTED_CARD]") {
		t.Error("a redacted card must never be restored")
	}
	if !strings.Contains(got, "Personal data was replaced") {
		t.Error("disclosure footer missing")
	}
	if e := resp.Header.Get("X-Discreet-Entities"); e != "CARD=1,EMAIL=1,NRIC=2,PHONE=1" {
		t.Errorf("X-Discreet-Entities = %q", e)
	}

	recs := h.records(t)
	if len(recs) != 1 || recs[0].Decision != audit.Allowed || recs[0].Entities["NRIC"] != 2 || recs[0].PromptHMAC == "" || recs[0].ResponseHMAC == "" {
		t.Errorf("audit records %+v", recs)
	}
	logFile, _ := os.ReadFile(h.logPath)
	for _, v := range planted {
		if strings.Contains(h.logs.String(), v) {
			t.Errorf("server logs contain %q", v)
		}
		if strings.Contains(string(logFile), v) {
			t.Errorf("audit log contains %q", v)
		}
	}
}

func TestPurposeIsRequired(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), nil)
	resp, out := h.post(t, "", body("hi"))
	if resp.StatusCode != 400 || !strings.Contains(out["error"].(map[string]any)["code"].(string), "missing_purpose") {
		t.Errorf("status %d %v", resp.StatusCode, out)
	}
}

func TestEligibilityDecisionsAreRefused(t *testing.T) {
	for _, purpose := range []string{
		"automated eligibility decision",
		"Automated-Eligibility Decision!",
		"decide eligibility for the subsidy",
		"benefits decision",
		"eligibility determination",
		"approve or reject eligibility",
	} {
		t.Run(purpose, func(t *testing.T) {
			h := newHarness(t, protect.DefaultPolicy(), nil)
			resp, _ := h.post(t, purpose, body("Is "+nric+" eligible?"))
			if resp.StatusCode != 403 {
				t.Fatalf("status %d", resp.StatusCode)
			}
			if h.mock.Last != nil {
				t.Error("the model was called for a refused purpose")
			}
			if recs := h.records(t); len(recs) != 1 || recs[0].Decision != audit.RefusedPurpose {
				t.Errorf("audit %+v", recs)
			}
		})
	}
}

func TestOtherPurposesAreAllowed(t *testing.T) {
	for _, purpose := range []string{"claims summary", "explain the eligibility criteria", "ticket triage"} {
		h := newHarness(t, protect.DefaultPolicy(), nil)
		if resp, out := h.post(t, purpose, body("hello")); resp.StatusCode != 200 {
			t.Errorf("%q: status %d %v", purpose, resp.StatusCode, out)
		}
	}
}

func TestPurposeIsScrubbedBeforeRecording(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), nil)
	h.post(t, "follow up with "+nric, body("hello"))
	if recs := h.records(t); recs[0].Purpose != "follow up with [NRIC]" {
		t.Errorf("purpose recorded as %q", recs[0].Purpose)
	}
}

func TestBlockedEntityRefusesTheRequest(t *testing.T) {
	p, _ := protect.ParsePolicy([]byte("version: 1\nentities:\n  NRIC: block\n"))
	h := newHarness(t, p, nil)
	resp, out := h.post(t, "claims summary", body("Phone "+phone+", member "+nric))
	if resp.StatusCode != 403 || h.mock.Last != nil {
		t.Fatalf("status %d, model called: %v (%v)", resp.StatusCode, h.mock.Last != nil, out)
	}
	if recs := h.records(t); recs[0].Decision != audit.BlockedEntity || recs[0].Entities["NRIC"] != 1 {
		t.Errorf("audit %+v", recs)
	}
}

func TestOnlyKnownFieldsAreForwarded(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), nil)
	resp, _ := h.post(t, "claims summary", `{"model":"m","user":"`+nric+`","tools":[{"type":"function","function":{"name":"lookup","description":"`+email+`"}}],"messages":[{"role":"user","content":[{"type":"text","text":"hi "},{"type":"text","text":"there"}]}]}`)
	if resp.StatusCode != 200 || h.mock.Last.Messages[0].Content != "hi there" {
		t.Fatalf("status %d, content %q", resp.StatusCode, h.mock.Last.Messages[0].Content)
	}
	if strings.Contains(h.logs.String(), nric) || strings.Contains(h.logs.String(), email) {
		t.Error("unforwarded fields reached the logs")
	}
}

func TestBadRequests(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), nil)
	for name, b := range map[string]string{
		"not json":   "{",
		"streaming":  `{"model":"m","stream":true,"messages":[{"role":"user","content":"hi"}]}`,
		"no message": `{"model":"m","messages":[]}`,
		"tool role":  `{"model":"m","messages":[{"role":"tool","content":"hi"}]}`,
		"image part": `{"model":"m","messages":[{"role":"user","content":[{"type":"image_url","image_url":{"url":"x"}}]}]}`,
	} {
		if resp, _ := h.post(t, "claims summary", b); resp.StatusCode != 400 {
			t.Errorf("%s: status %d", name, resp.StatusCode)
		}
	}
}

type failing struct{}

func (failing) Name() string { return "failing" }
func (failing) Complete(context.Context, upstream.Request) (upstream.Response, error) {
	return upstream.Response{}, errors.New("boom")
}

func TestUpstreamErrorIsRecorded(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), failing{})
	resp, _ := h.post(t, "claims summary", body("hi "+nric))
	if resp.StatusCode != 502 {
		t.Fatalf("status %d", resp.StatusCode)
	}
	if recs := h.records(t); recs[0].Decision != audit.UpstreamError {
		t.Errorf("audit %+v", recs)
	}
}

func TestNoAnswerWithoutAnAuditRecord(t *testing.T) {
	h := newHarness(t, protect.DefaultPolicy(), nil)
	h.audit.Close()
	resp, _ := h.post(t, "claims summary", body("hi"))
	if resp.StatusCode != 500 {
		t.Errorf("status %d, want 500 when the audit log can't be written", resp.StatusCode)
	}
}
