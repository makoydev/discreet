package audit

import (
	"bytes"
	"encoding/csv"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var testKey = []byte("test-key-0123456789abcdefghijklmnop")

func writeLog(t *testing.T, n int) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := Open(path, testKey)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		if _, err := l.Append(Record{RequestID: "r", Tenant: "demo", Purpose: "claims summary", Decision: Allowed,
			Upstream: "mock", Model: "mock", Entities: map[string]int{"NRIC": i}, PromptHMAC: l.HMAC("p"), CostUSD: 0.001}); err != nil {
			t.Fatal(err)
		}
	}
	l.Close()
	return path
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Split(strings.TrimRight(string(data), "\n"), "\n")
}

func verifyLines(ls []string) (Summary, error) {
	return Verify(strings.NewReader(strings.Join(ls, "\n") + "\n"))
}

func wantBrokenAt(t *testing.T, err error, line int) {
	t.Helper()
	var ve *VerifyError
	if !errors.As(err, &ve) || ve.Line != line {
		t.Fatalf("err = %v, want a VerifyError on line %d", err, line)
	}
}

func TestCleanLogVerifies(t *testing.T) {
	path := writeLog(t, 5)
	s, err := verifyLines(lines(t, path))
	if err != nil || s.Records != 5 || s.HeadHash == Genesis {
		t.Fatalf("summary %+v, err %v", s, err)
	}
}

func TestOneChangedByteFails(t *testing.T) {
	ls := lines(t, writeLog(t, 5))
	ls[2] = strings.Replace(ls[2], `"NRIC":2`, `"NRIC":3`, 1)
	_, err := verifyLines(ls)
	wantBrokenAt(t, err, 3)
}

func TestDeletedRecordFails(t *testing.T) {
	ls := lines(t, writeLog(t, 5))
	_, err := verifyLines(append(ls[:1:1], ls[2:]...))
	wantBrokenAt(t, err, 2)
}

func TestSwappedRecordsFail(t *testing.T) {
	ls := lines(t, writeLog(t, 5))
	ls[1], ls[2] = ls[2], ls[1]
	_, err := verifyLines(ls)
	wantBrokenAt(t, err, 2)
}

func TestRewrittenRecordWithFixedSequenceFails(t *testing.T) {
	// Recomputing a changed record's own hash still breaks the next link.
	ls := lines(t, writeLog(t, 3))
	var r Record
	if err := jsonUnmarshal(ls[0], &r); err != nil {
		t.Fatal(err)
	}
	r.Tenant = "someone-else"
	r.Hash = computeHash(r)
	ls[0] = jsonMarshal(r)
	_, err := verifyLines(ls)
	wantBrokenAt(t, err, 2)
}

func TestReopenContinuesTheChain(t *testing.T) {
	path := writeLog(t, 2)
	l, err := Open(path, testKey)
	if err != nil {
		t.Fatal(err)
	}
	r, _ := l.Append(Record{Decision: RefusedPurpose})
	l.Close()
	if r.Seq != 3 {
		t.Errorf("seq = %d, want 3", r.Seq)
	}
	if s, err := verifyLines(lines(t, path)); err != nil || s.Records != 3 {
		t.Errorf("%+v %v", s, err)
	}
}

func TestOpenRefusesABrokenLog(t *testing.T) {
	path := writeLog(t, 3)
	ls := lines(t, path)
	ls[1] = strings.Replace(ls[1], "demo", "dem0", 1)
	os.WriteFile(path, []byte(strings.Join(ls, "\n")+"\n"), 0o600)
	if _, err := Open(path, testKey); err == nil {
		t.Error("opened a broken log for appending")
	}
}

func TestKeyRules(t *testing.T) {
	if _, err := Open(filepath.Join(t.TempDir(), "a"), []byte("short")); err == nil {
		t.Error("accepted a short key")
	}
	t.Setenv("DISCREET_HMAC_KEY", "")
	if _, err := KeyFromEnv(); err == nil {
		t.Error("accepted a missing key")
	}
	l, _ := Open(filepath.Join(t.TempDir(), "a"), testKey)
	other, _ := Open(filepath.Join(t.TempDir(), "b"), []byte("another-key-0123456789abcdefghijkl"))
	if l.HMAC("S1234567D") == other.HMAC("S1234567D") || l.HMAC("x") != l.HMAC("x") {
		t.Error("HMAC must be deterministic per key and differ between keys")
	}
}

func TestConcurrentAppendsKeepTheChain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, _ := Open(path, testKey)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); l.Append(Record{Decision: Allowed, CostUSD: 0.01}) }()
	}
	wg.Wait()
	l.Close()
	s, err := verifyLines(lines(t, path))
	if err != nil || s.Records != 50 {
		t.Fatalf("%+v %v", s, err)
	}
	l2, _ := Open(path, testKey)
	if got := l2.SpentToday(); got < 0.4999 || got > 0.5001 {
		t.Errorf("SpentToday = %v, want 0.50", got)
	}
}

func TestExportCSV(t *testing.T) {
	path := writeLog(t, 2)
	var out bytes.Buffer
	f, _ := os.Open(path)
	defer f.Close()
	if err := ExportCSV(f, &out); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil || len(rows) != 3 || rows[0][0] != "seq" || rows[2][0] != "2" || rows[2][9] != `"NRIC":1` {
		t.Fatalf("rows %v err %v", rows, err)
	}
}

func TestCSVNeutralisesFormulas(t *testing.T) {
	var out bytes.Buffer
	c := newCSV(&out)
	c.row("=HYPERLINK(1)", "ok")
	c.flush()
	if !strings.HasPrefix(out.String(), "'=HYPERLINK") {
		t.Errorf("formula not neutralised: %q", out.String())
	}
}
