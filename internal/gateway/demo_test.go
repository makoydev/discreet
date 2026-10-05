package gateway

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

func demoPost(t *testing.T, base, token string, in map[string]string) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(in)
	req, _ := http.NewRequest("POST", base+"/demo/run", strings.NewReader(string(b)))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out map[string]any
	json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func TestDemoPageIsServedWithAStrictPolicy(t *testing.T) {
	h := newHarness(t, nil, nil)
	for _, path := range []string{"/", "/demo.js", "/demo.css"} {
		resp, err := http.Get(h.srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 || len(body) == 0 {
			t.Errorf("%s: status %d", path, resp.StatusCode)
		}
		csp := resp.Header.Get("Content-Security-Policy")
		if !strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "script-src 'self'") || strings.Contains(csp, "unsafe") {
			t.Errorf("%s: CSP %q", path, csp)
		}
	}
	if resp, _ := http.Get(h.srv.URL + "/nope"); resp.StatusCode != 404 {
		t.Errorf("unknown path: %d", resp.StatusCode)
	}
}

func TestDemoShowsEveryStage(t *testing.T) {
	h := newHarness(t, nil, nil)
	code, out := demoPost(t, h.srv.URL, "", map[string]string{"purpose": "claims summary", "text": "Member " + nric + " on " + phone + ", card " + card})
	if code != 200 || out["status"] != "answered" {
		t.Fatalf("%d %v", code, out)
	}
	sent := out["sent_to_model"].(string)
	for _, v := range []string{nric, phone, "4111"} {
		if strings.Contains(sent, v) || strings.Contains(out["model_answer"].(string), v) {
			t.Errorf("the model saw %q", v)
		}
	}
	var restored []string
	for _, p := range out["returned"].([]any) {
		part := p.(map[string]any)
		if part["placeholder"] != nil {
			restored = append(restored, part["text"].(string))
		}
	}
	// The mock repeats placeholders in its echo and its draft, so each value
	// comes back twice; only the NRIC and phone may be restored, never the card.
	if len(restored) != 4 || restored[0] != nric || restored[1] != phone || restored[2] != nric || restored[3] != phone {
		t.Errorf("restored %v", restored)
	}
	rec := out["audit_record"].(map[string]any)
	if rec["tenant"] != "demo" || rec["upstream"] != "mock" || rec["decision"] != "allowed" {
		t.Errorf("record %v", rec)
	}
}

func TestDemoNeverUsesTheRealModel(t *testing.T) {
	h := newRealHarness(t, 1.0, nil)
	code, out := demoPost(t, h.srv.URL, goodToken, map[string]string{"purpose": "claims summary", "text": "hello " + nric})
	if code != 200 || h.fake.calls.Load() != 0 {
		t.Errorf("status %d, paid model called %d times (%v)", code, h.fake.calls.Load(), out)
	}
}

func TestDemoRefusalIncludesTheAuditRecord(t *testing.T) {
	h := newHarness(t, nil, nil)
	code, out := demoPost(t, h.srv.URL, "", map[string]string{"purpose": "automated eligibility decision", "text": "Is " + nric + " eligible?"})
	if code != 200 || out["status"] != "refused" || out["code"] != "purpose_refused" {
		t.Fatalf("%d %v", code, out)
	}
	if rec := out["audit_record"].(map[string]any); rec["decision"] != "refused_purpose" {
		t.Errorf("record %v", rec)
	}
}

func TestDemoRejectsBadInput(t *testing.T) {
	h := newHarness(t, nil, nil)
	for name, text := range map[string]string{"empty": "  ", "too long": strings.Repeat("a", 4001)} {
		if code, _ := demoPost(t, h.srv.URL, "", map[string]string{"purpose": "x", "text": text}); code != 400 {
			t.Errorf("%s: status %d", name, code)
		}
	}
}
