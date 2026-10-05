package detect

import (
	"encoding/json"
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/makoydev/discreet/third_party/sgpiirules"
)

type fixtureFile struct {
	Entity string `json:"entity"`
	Cases  []struct {
		ID     string `json:"id"`
		Kind   string `json:"kind"`
		Text   string `json:"text"`
		Expect []struct {
			Entity string `json:"entity"`
			Value  string `json:"value"`
		} `json:"expect"`
	} `json:"cases"`
}

// TestConformance runs every case in the vendored sg-pii-rules fixtures.
// Passing all of them is what conforming to SPEC.md means.
func TestConformance(t *testing.T) {
	engine, err := Default()
	if err != nil {
		t.Fatal(err)
	}
	files, _ := fs.Glob(sgpiirules.Files, "fixtures/*.json")
	total := 0
	for _, name := range files {
		data, _ := sgpiirules.Files.ReadFile(name)
		var f fixtureFile
		if err := json.Unmarshal(data, &f); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		for _, c := range f.Cases {
			total++
			t.Run(c.ID, func(t *testing.T) {
				got := []string{}
				for _, m := range engine.Detect(c.Text) {
					got = append(got, m.Entity+"="+m.Value)
				}
				want := []string{}
				for _, e := range c.Expect {
					want = append(want, e.Entity+"="+e.Value)
				}
				if !reflect.DeepEqual(got, want) {
					t.Errorf("%s (%s)\n text: %q\n got:  %v\n want: %v", c.ID, c.Kind, c.Text, got, want)
				}
			})
		}
	}
	if total < 369 {
		t.Errorf("ran %d cases; the vendored v0.2.0 release has 369", total)
	}
}

// The overlap rules in SPEC.md §4, with toy detectors. The shared fixtures
// never have two detectors matching the same span, so the tie-break is only
// tested here (as in the reference implementation's own detect tests).
func TestOverlapRules(t *testing.T) {
	tests := []struct {
		name, rules, text string
		want              []string
	}{
		{"same span: the earlier detector wins",
			`[{"id":"a","entity":"A","pattern":"ab"},{"id":"b","entity":"B","pattern":"ab"}]`, "ab", []string{"A=ab"}},
		{"same span, other order",
			`[{"id":"b","entity":"B","pattern":"ab"},{"id":"a","entity":"A","pattern":"ab"}]`, "ab", []string{"B=ab"}},
		{"the longer match wins whatever the order",
			`[{"id":"a","entity":"A","pattern":"a"},{"id":"b","entity":"B","pattern":"ab"}]`, "ab", []string{"B=ab"}},
		{"a later, longer overlap replaces the kept match",
			`[{"id":"a","entity":"A","pattern":"ab"},{"id":"b","entity":"B","pattern":"bcd"}]`, "abcd", []string{"B=bcd"}},
		{"overlaps are judged on the value group, not the whole match",
			`[{"id":"num","entity":"NUM","pattern":"\\d+"},{"id":"id","entity":"ID","pattern":"id:(?P<value>\\d+)"}]`, "id:123", []string{"NUM=123"}},
		{"matches come back in order of position",
			`[{"id":"b","entity":"B","pattern":"b+"},{"id":"a","entity":"A","pattern":"a+"}]`, "aa bb a", []string{"A=aa", "B=bb", "A=a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine, err := New([]byte(`{"version":"0.0.0","detectors":` + tt.rules + `}`))
			if err != nil {
				t.Fatal(err)
			}
			got := []string{}
			for _, m := range engine.Detect(tt.text) {
				got = append(got, m.Entity+"="+m.Value)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNewRejectsUnknownValidator(t *testing.T) {
	_, err := New([]byte(`{"version":"0.0.0","detectors":[{"id":"x","entity":"X","pattern":"x","validator":"nope"}]}`))
	if err == nil || !strings.Contains(err.Error(), "unknown validator nope") {
		t.Errorf("err = %v, want an unknown-validator error", err)
	}
}

func TestNewRejectsNonRE2Pattern(t *testing.T) {
	_, err := New([]byte(`{"version":"0.0.0","detectors":[{"id":"x","entity":"X","pattern":"(?<=a)b"}]}`))
	if err == nil {
		t.Error("a lookbehind pattern compiled; RE2 must reject it")
	}
}

func TestValueGroupOffsetsAreBytes(t *testing.T) {
	engine, _ := Default()
	text := "地址 Singapore 520123"
	got := engine.Detect(text)
	if len(got) != 1 || got[0].Value != "520123" || text[got[0].Start:got[0].End] != "520123" {
		t.Errorf("got %+v, want one POSTAL match whose byte offsets slice to 520123", got)
	}
}

func TestValidators(t *testing.T) {
	cases := []struct {
		name  string
		fn    func(string) bool
		value string
		want  bool
	}{
		// Synthetic example from sg-pii-rules VALIDATORS.md, and published illustrative numbers that fail.
		{"nric valid", isValidNRIC, "S1234567D", true},
		{"nric lowercase", isValidNRIC, "s1234567d", true},
		{"ICA example fails", isValidNRIC, "M1234567B", false},
		{"nric-like", hasNRICShapeButInvalidChecksum, "S1234567A", true},
		{"nric-like rejects valid", hasNRICShapeButInvalidChecksum, "S1234567D", false},
		// Published test card numbers, not issued cards.
		{"visa", isPaymentCard, "4111 1111 1111 1111", true},
		{"visa bad luhn", isPaymentCard, "4111111111111112", false},
		{"amex", isPaymentCard, "378282246310005", true},
		{"amex wrong length", isPaymentCard, "3782822463100050", false},
		{"mastercard 2-series", isPaymentCard, "2221000000000009", true},
		{"no network", isPaymentCard, "9111111111111111", false},
		{"postal", isPostalCode, "520123", true},
		{"postal sector 74", isPostalCode, "740123", false},
		{"postal sector 83", isPostalCode, "830000", false},
		{"date day first", isPlausibleDate, "12/03/1988", true},
		{"date month first", isPlausibleDate, "03/31/1988", true},
		{"leap day", isPlausibleDate, "29/02/1992", true},
		{"not a leap year", isPlausibleDate, "29/02/1990", false},
		{"century not leap", isPlausibleDate, "29/02/1900", false},
		{"iso", isPlausibleDate, "1988-03-12", true},
		{"written", isPlausibleDate, "12 Sept. 1988", true},
		{"us written", isPlausibleDate, "March 12, 1988", true},
		{"31 April", isPlausibleDate, "31 April 1990", false},
		{"unknown month", isPlausibleDate, "12 Foo 1988", false},
	}
	for _, c := range cases {
		if got := c.fn(c.value); got != c.want {
			t.Errorf("%s: %q = %v, want %v", c.name, c.value, got, c.want)
		}
	}
}

func BenchmarkDetect(b *testing.B) {
	engine, _ := Default()
	text := strings.Repeat("Please call S1234567D on +65 9123 4567 about card 4111 1111 1111 1111, Blk 123 #05-123 Singapore 520123, DOB: 12/03/1988. ", 20)
	b.SetBytes(int64(len(text)))
	for b.Loop() {
		engine.Detect(text)
	}
}
