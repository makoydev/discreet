package gateway

import (
	"regexp"
	"strings"
)

var nonWord = regexp.MustCompile(`[^a-z0-9]+`)

func normalise(s string) string {
	return strings.TrimSpace(nonWord.ReplaceAllString(strings.ToLower(s), " "))
}

// deniedPurpose reports whether a declared purpose is refused: it contains
// one of the policy's denied phrases, ignoring case and punctuation, or it
// pairs "eligibility" with deciding, determining, approving or rejecting.
// Discreet never takes part in decisions about people's eligibility.
func deniedPurpose(purpose string, denied []string) bool {
	p := " " + normalise(purpose) + " "
	for _, phrase := range denied {
		if n := normalise(phrase); n != "" && strings.Contains(p, " "+n+" ") {
			return true
		}
	}
	if strings.Contains(p, "eligib") {
		for _, verb := range []string{"decision", "decide", "determin", "approv", "reject", "deny", "denial"} {
			if strings.Contains(p, verb) {
				return true
			}
		}
	}
	return false
}
