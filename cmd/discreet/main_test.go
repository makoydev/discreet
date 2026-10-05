package main

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/makoydev/discreet/internal/audit"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCode   int
		wantStdout string
		wantStderr string
	}{
		{"no arguments prints usage as an error", nil, 2, "", "Usage:"},
		{"help", []string{"help"}, 0, "discreet serve", ""},
		{"--help", []string{"--help"}, 0, "discreet audit verify", ""},
		{"version", []string{"version"}, 0, "discreet 0.0.0-dev", ""},
		{"serve help", []string{"serve", "--help"}, 0, "Usage: discreet serve", ""},
		{"audit help", []string{"audit", "--help"}, 0, "verify", ""},
		{"audit without a subcommand", []string{"audit"}, 2, "", "Usage: discreet audit"},
		{"unknown audit subcommand", []string{"audit", "delete"}, 2, "", `unknown subcommand "delete"`},
		{"unknown command", []string{"launch"}, 2, "", `unknown command "launch"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			check(t, "stdout", stdout.String(), tt.wantStdout)
			check(t, "stderr", stderr.String(), tt.wantStderr)
		})
	}
}

// check asserts that got contains want, or is empty when want is empty.
func check(t *testing.T, stream, got, want string) {
	t.Helper()
	if want == "" {
		if got != "" {
			t.Errorf("%s = %q, want empty", stream, got)
		}
		return
	}
	if !strings.Contains(got, want) {
		t.Errorf("%s = %q, want it to contain %q", stream, got, want)
	}
}

func TestAuditVerifyAndExport(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	l, err := audit.Open(path, []byte("test-key-0123456789abcdefghijklmnop"))
	if err != nil {
		t.Fatal(err)
	}
	l.Append(audit.Record{Tenant: "demo", Decision: audit.Allowed})
	l.Append(audit.Record{Tenant: "demo", Decision: audit.RefusedPurpose})
	l.Close()

	var out, errOut bytes.Buffer
	if code := run([]string{"audit", "verify", "-log", path}, &out, &errOut); code != 0 || !strings.Contains(out.String(), "OK: 2 records") {
		t.Fatalf("verify: code %d, out %q, err %q", code, out.String(), errOut.String())
	}
	out.Reset()
	if code := run([]string{"audit", "export", "-csv", "-log", path}, &out, &errOut); code != 0 || !strings.HasPrefix(out.String(), "seq,time,") {
		t.Fatalf("export: code %d, out %q", code, out.String())
	}

	data, _ := os.ReadFile(path)
	os.WriteFile(path, bytes.Replace(data, []byte("refused_purpose"), []byte("allowed"), 1), 0o600)
	out.Reset()
	errOut.Reset()
	if code := run([]string{"audit", "verify", "-log", path}, &out, &errOut); code != 1 || !strings.Contains(errOut.String(), "FAILED: record on line 2") {
		t.Errorf("tampered verify: code %d, err %q", code, errOut.String())
	}
	if code := run([]string{"audit", "export", "-log", path}, &out, &errOut); code != 1 {
		t.Errorf("a broken log was exported (code %d)", code)
	}
	if code := run([]string{"audit", "verify", "-log", filepath.Join(t.TempDir(), "missing")}, &out, &errOut); code != 1 {
		t.Errorf("missing log: code %d", code)
	}
}

func TestServeRefusesWithoutAKey(t *testing.T) {
	t.Setenv("DISCREET_HMAC_KEY", "")
	var errOut bytes.Buffer
	if code := run([]string{"serve", "-audit-log", filepath.Join(t.TempDir(), "a.jsonl")}, io.Discard, &errOut); code != 1 || !strings.Contains(errOut.String(), "DISCREET_HMAC_KEY") {
		t.Errorf("code %d, stderr %q", code, errOut.String())
	}
}

func TestServeAnswersAndStops(t *testing.T) {
	t.Setenv("DISCREET_HMAC_KEY", "test-key-0123456789abcdefghijklmnop")
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	addr := ln.Addr().String()
	ln.Close()
	ctx, cancel := context.WithCancel(context.Background())
	serveContext = ctx
	t.Cleanup(func() { serveContext = context.Background() })
	done := make(chan int)
	go func() {
		done <- run([]string{"serve", "-addr", addr, "-audit-log", filepath.Join(t.TempDir(), "a.jsonl")}, io.Discard, io.Discard)
	}()

	var resp *http.Response
	var err error
	for i := 0; i < 50; i++ {
		if resp, err = http.Get("http://" + addr + "/healthz"); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("healthz: %v %v", resp, err)
	}
	resp.Body.Close()
	cancel()
	if code := <-done; code != 0 {
		t.Errorf("exit code %d", code)
	}
}
