package gateway

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/makoydev/discreet/internal/audit"
	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/protect"
	"github.com/makoydev/discreet/internal/upstream"
	"github.com/makoydev/discreet/internal/vault"
)

// A fake OpenAI server: no test ever calls a paid API.
type fakeOpenAI struct {
	srv    *httptest.Server
	calls  atomic.Int32
	mu     sync.Mutex
	bodies []map[string]any
	auth   []string
	status int
}

func newFakeOpenAI(t *testing.T) *fakeOpenAI {
	f := &fakeOpenAI{status: 200}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.calls.Add(1)
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.bodies = append(f.bodies, body)
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		f.mu.Unlock()
		if f.status != 200 {
			w.WriteHeader(f.status)
			io.WriteString(w, `{"error":{"message":"nope"}}`)
			return
		}
		msgs := body["messages"].([]any)
		last := msgs[len(msgs)-1].(map[string]any)["content"].(string)
		json.NewEncoder(w).Encode(map[string]any{
			"model":   "gpt-6-luna-2026-09-01",
			"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": "Reply about " + last}, "finish_reason": "stop"}},
			"usage":   map[string]any{"prompt_tokens": 1000, "completion_tokens": 200},
		})
	}))
	t.Cleanup(f.srv.Close)
	return f
}

const goodToken = "tenant-a-token-0123456789abcdef"

type realHarness struct {
	*harness
	fake *fakeOpenAI
}

func newRealHarness(t *testing.T, budget float64, rate *RateLimiter) *realHarness {
	t.Helper()
	fake := newFakeOpenAI(t)
	engine, _ := detect.Default()
	v, _ := vault.New(time.Minute)
	h := &harness{logs: &strings.Builder{}, logPath: filepath.Join(t.TempDir(), "audit.jsonl")}
	var err error
	if h.audit, err = audit.Open(h.logPath, []byte("test-key-0123456789abcdefghijklmnop")); err != nil {
		t.Fatal(err)
	}
	tokens, err := ParseTokens("acme:" + goodToken)
	if err != nil {
		t.Fatal(err)
	}
	h.mock = &upstream.Mock{}
	s := New(Config{
		Engine: engine, Policy: protect.DefaultPolicy(), Vault: v, Audit: h.audit, Upstream: h.mock,
		Logger: slog.New(slog.NewJSONHandler(h.logs, nil)),
		Real:   &upstream.OpenAICompatible{BaseURL: fake.srv.URL, APIKey: "provider-key"},
		Tokens: tokens, Model: "gpt-6-luna", Price: KnownPrices["gpt-6-luna"],
		Budget: NewBudget(budget, h.audit.SpentToday), MaxOutputTokens: 1000, RateLimit: rate,
	})
	h.srv = httptest.NewServer(s.Handler())
	t.Cleanup(h.srv.Close)
	return &realHarness{h, fake}
}

func (h *realHarness) postAs(t *testing.T, token, model string) *http.Response {
	t.Helper()
	b, _ := json.Marshal(map[string]any{"model": model, "messages": []map[string]string{{"role": "user", "content": "Call " + nric + " on " + phone}}})
	req, _ := http.NewRequest("POST", h.srv.URL+"/v1/chat/completions", strings.NewReader(string(b)))
	req.Header.Set("X-Discreet-Purpose", "claims summary")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func TestValidTokenReachesTheRealModelWithPlaceholdersOnly(t *testing.T) {
	h := newRealHarness(t, 1.0, nil)
	resp := h.postAs(t, goodToken, "gpt-6-luna")
	if resp.StatusCode != 200 || resp.Header.Get("X-Discreet-Upstream") != "openai-compatible" {
		t.Fatalf("status %d, upstream %q", resp.StatusCode, resp.Header.Get("X-Discreet-Upstream"))
	}
	body := h.fake.bodies[0]
	sent, _ := json.Marshal(body["messages"])
	if strings.Contains(string(sent), nric) || strings.Contains(string(sent), phone) {
		t.Errorf("the provider received personal data: %s", sent)
	}
	if body["max_completion_tokens"] != float64(1000) || body["model"] != "gpt-6-luna" {
		t.Errorf("body %v", body)
	}
	if h.fake.auth[0] != "Bearer provider-key" {
		t.Errorf("provider got Authorization %q; the caller's token must never be forwarded", h.fake.auth[0])
	}
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	if a := answer(t, out); !strings.Contains(a, nric) {
		t.Errorf("answer not restored: %s", a)
	}
	rec := h.records(t)[0]
	if rec.Tenant != "acme" || rec.Upstream != "openai-compatible" || rec.CostUSD < 0.000199 || rec.CostUSD > 0.000201 {
		t.Errorf("record %+v (cost should be 1000×0.10 + 200×0.50 per million = US$0.0002)", rec)
	}
}

func TestWithoutAValidTokenEveryoneGetsTheMock(t *testing.T) {
	h := newRealHarness(t, 1.0, nil)
	for _, token := range []string{"", "not-needed-for-mock", goodToken + "x"} {
		resp := h.postAs(t, token, "gpt-6-luna")
		if resp.StatusCode != 200 || resp.Header.Get("X-Discreet-Upstream") != "mock" {
			t.Errorf("token %q: status %d upstream %q", token, resp.StatusCode, resp.Header.Get("X-Discreet-Upstream"))
		}
	}
	if n := h.fake.calls.Load(); n != 0 {
		t.Errorf("the paid model was called %d times without a valid token", n)
	}
}

func TestOnlyTheConfiguredModel(t *testing.T) {
	h := newRealHarness(t, 1.0, nil)
	if resp := h.postAs(t, goodToken, "gpt-6-sol"); resp.StatusCode != 400 {
		t.Errorf("status %d, want 400 for a model that isn't allowed", resp.StatusCode)
	}
}

func TestDailyBudgetIsEnforced(t *testing.T) {
	// Worst case here: prompt ≤ ~50 bytes + 16, output 1000 tokens → about
	// US$0.0005. A US$0.0006 budget allows one request, then refuses.
	h := newRealHarness(t, 0.0006, nil)
	if resp := h.postAs(t, goodToken, ""); resp.StatusCode != 200 {
		t.Fatalf("first request: %d", resp.StatusCode)
	}
	resp := h.postAs(t, goodToken, "")
	if resp.StatusCode != 429 {
		t.Fatalf("second request: %d, want 429", resp.StatusCode)
	}
	if n := h.fake.calls.Load(); n != 1 {
		t.Errorf("provider called %d times", n)
	}
	if recs := h.records(t); recs[1].Decision != audit.RefusedBudget {
		t.Errorf("records %+v", recs)
	}
	// Mock requests are free and never refused for budget.
	if resp := h.postAs(t, "", ""); resp.StatusCode != 200 {
		t.Errorf("mock request: %d", resp.StatusCode)
	}
}

func TestBudgetSurvivesARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	key := []byte("test-key-0123456789abcdefghijklmnop")
	l, _ := audit.Open(path, key)
	l.Append(audit.Record{Decision: audit.Allowed, CostUSD: 0.19})
	l.Close()
	l2, _ := audit.Open(path, key)
	if b := NewBudget(0.20, l2.SpentToday); b.Reserve(0.02) {
		t.Error("a restart reset today's spending")
	}
}

func TestParallelRequestsCantOverspend(t *testing.T) {
	b := NewBudget(1.0, func() float64 { return 0 })
	var granted atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if b.Reserve(0.1) {
				granted.Add(1)
			}
		}()
	}
	wg.Wait()
	if granted.Load() != 10 {
		t.Errorf("granted %d reservations of 0.1 within 1.0", granted.Load())
	}
}

func TestProviderErrorsAreRecordedAndNotLeaked(t *testing.T) {
	h := newRealHarness(t, 1.0, nil)
	h.fake.status = 500
	if resp := h.postAs(t, goodToken, ""); resp.StatusCode != 502 {
		t.Errorf("status %d", resp.StatusCode)
	}
	if recs := h.records(t); recs[0].Decision != audit.UpstreamError || recs[0].CostUSD != 0 {
		t.Errorf("record %+v", recs[0])
	}
}

func TestRateLimit(t *testing.T) {
	h := newRealHarness(t, 1.0, NewRateLimiter(60, 2, ""))
	codes := []int{}
	for i := 0; i < 3; i++ {
		codes = append(codes, h.postAs(t, "", "").StatusCode)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != 429 {
		t.Errorf("codes %v, want [200 200 429]", codes)
	}
}

func TestRateLimiterRefills(t *testing.T) {
	l := NewRateLimiter(60, 1, "Fly-Client-IP")
	now := time.Now()
	l.now = func() time.Time { return now }
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("Fly-Client-IP", "203.0.113.7")
	if !l.Allow(r) || l.Allow(r) {
		t.Fatal("burst of 1 not enforced")
	}
	other := httptest.NewRequest("POST", "/", nil)
	other.Header.Set("Fly-Client-IP", "203.0.113.8")
	if !l.Allow(other) {
		t.Error("one client's limit affected another")
	}
	now = now.Add(time.Second)
	if !l.Allow(r) {
		t.Error("did not refill after a second at 60 a minute")
	}
}

func TestParseTokensRejectsWeakTokens(t *testing.T) {
	for _, bad := range []string{"acme:short", "no-colon-0123456789abcdefghij", ":0123456789abcdefghijklmnop"} {
		if _, err := ParseTokens(bad); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
}
