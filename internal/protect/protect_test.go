package protect

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/vault"
)

// Synthetic values only: S1234567D is the worked example in sg-pii-rules'
// VALIDATORS.md; T0000001E is built from patterned digits.
func setup(t *testing.T, policy *Policy) (*detect.Engine, *Session, *vault.Vault) {
	t.Helper()
	engine, err := detect.Default()
	if err != nil {
		t.Fatal(err)
	}
	v, err := vault.New(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return engine, NewSession(policy, v), v
}

func protect(t *testing.T, e *detect.Engine, s *Session, text string) string {
	t.Helper()
	out, err := s.Protect(text, e.Detect(text))
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestSameValueSamePlaceholder(t *testing.T) {
	e, s, _ := setup(t, DefaultPolicy())
	nric2 := "T0000001" + "E"
	got := protect(t, e, s, "Call S1234567D on 9123 4567. S1234567D and "+nric2+" are related.")
	want := "Call <NRIC_1> on <PHONE_1>. <NRIC_1> and <NRIC_2> are related."
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if c := s.Counts(); c["NRIC"] != 3 || c["PHONE"] != 1 {
		t.Errorf("counts = %v", c)
	}
}

func TestPlaceholdersAreConsistentAcrossMessages(t *testing.T) {
	e, s, _ := setup(t, DefaultPolicy())
	a := protect(t, e, s, "Member S1234567D")
	b := protect(t, e, s, "Remind S1234567D tomorrow")
	if a != "Member <NRIC_1>" || b != "Remind <NRIC_1> tomorrow" {
		t.Errorf("got %q and %q", a, b)
	}
}

func TestRestore(t *testing.T) {
	e, s, _ := setup(t, DefaultPolicy())
	protect(t, e, s, "Call S1234567D on 9123 4567")
	got := s.Restore("Draft: Hi <NRIC_1>, we'll call <PHONE_1>. Ref <NRIC_7>, <EMAIL_1>.")
	want := "Draft: Hi S1234567D, we'll call 9123 4567. Ref <NRIC_7>, <EMAIL_1>."
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

func TestRestoreIgnoresOtherSessions(t *testing.T) {
	e, s1, v := setup(t, DefaultPolicy())
	protect(t, e, s1, "Member S1234567D")
	s2 := NewSession(DefaultPolicy(), v)
	if got := s2.Restore("<NRIC_1>"); got != "<NRIC_1>" {
		t.Errorf("another session's value leaked: %q", got)
	}
}

func TestRedactIsNeverRestored(t *testing.T) {
	e, s, _ := setup(t, DefaultPolicy())
	card := "4111 1111 1111 1111" // a published test card number
	got := protect(t, e, s, "Card "+card+" was charged")
	if got != "Card [REDACTED_CARD] was charged" {
		t.Errorf("got %q", got)
	}
	if r := s.Restore(got); strings.Contains(r, "4111") {
		t.Errorf("redacted value came back: %q", r)
	}
}

func TestBlock(t *testing.T) {
	p, err := ParsePolicy([]byte("version: 1\nentities:\n  NRIC: block\n"))
	if err != nil {
		t.Fatal(err)
	}
	e, s, v := setup(t, p)
	_, err = s.Protect("Member S1234567D", e.Detect("Member S1234567D"))
	var blocked *BlockedError
	if !errors.As(err, &blocked) || blocked.Entity != "NRIC" {
		t.Fatalf("err = %v, want a BlockedError for NRIC", err)
	}
	if v.Len() != 0 {
		t.Error("a blocked request left values in the vault")
	}
}

func TestCloseClearsTheVault(t *testing.T) {
	e, s, v := setup(t, DefaultPolicy())
	protect(t, e, s, "S1234567D, 9123 4567")
	s.Close()
	if v.Len() != 0 || s.Restore("<NRIC_1>") != "<NRIC_1>" {
		t.Error("values still available after Close")
	}
}

func TestSessionNeverPrintsValues(t *testing.T) {
	e, s, _ := setup(t, DefaultPolicy())
	protect(t, e, s, "Member S1234567D")
	if out := fmt.Sprintf("%v %+v", s, s); strings.Contains(out, "S1234567D") {
		t.Errorf("printing a session leaked a value: %s", out)
	}
}

func TestParsePolicyRejectsMistakes(t *testing.T) {
	for _, bad := range []string{
		"version: 1\nentities:\n  NRIC: tokenize\n", // American spelling isn't an action
		"version: 1\nentitites:\n  NRIC: block\n",   // typo in a key
		"version: 2\n",
		"version: 1\ndefault_action: allow\n",
	} {
		if _, err := ParsePolicy([]byte(bad)); err == nil {
			t.Errorf("accepted %q", bad)
		}
	}
	if p := DefaultPolicy(); p.ActionFor("CARD") != Redact || p.ActionFor("SOMETHING_NEW") != Tokenise {
		t.Error("default policy actions are not as documented")
	}
}
