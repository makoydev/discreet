package main

import (
	"bytes"
	"strings"
	"testing"
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
		{"serve is not built yet", []string{"serve"}, 1, "", "not built yet (issue D5)"},
		{"audit help", []string{"audit", "--help"}, 0, "verify", ""},
		{"audit without a subcommand", []string{"audit"}, 2, "", "Usage: discreet audit"},
		{"audit verify is not built yet", []string{"audit", "verify"}, 1, "", "audit verify: not built yet (issue D7)"},
		{"audit export is not built yet", []string{"audit", "export"}, 1, "", "audit export: not built yet (issue D7)"},
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
