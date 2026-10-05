package bench

import (
	"reflect"
	"testing"

	"github.com/makoydev/discreet/internal/detect"
)

func TestGenerateIsDeterministicAndSpansAreExact(t *testing.T) {
	a, b := Generate(300, 7), Generate(300, 7)
	if !reflect.DeepEqual(a, b) {
		t.Fatal("the same seed gave different samples")
	}
	if reflect.DeepEqual(a, Generate(300, 8)) {
		t.Fatal("different seeds gave the same samples")
	}
	entities := map[string]int{}
	for _, s := range a {
		for _, sp := range s.Spans {
			if sp.Start < 0 || sp.End > len(s.Text) || sp.Start >= sp.End {
				t.Fatalf("bad span %+v in %q", sp, s.Text)
			}
			entities[sp.Entity]++
		}
	}
	for _, e := range entityOrder {
		if entities[e] == 0 {
			t.Errorf("no %s in 300 samples", e)
		}
	}
}

func TestGeneratedValuesAreValidWhereTheyShouldBe(t *testing.T) {
	engine, _ := detect.Default()
	// Standard NRICs from the independent generator must pass the engine's checksum.
	r := Generate(2000, 3)
	for _, s := range r {
		for _, sp := range s.Spans {
			if sp.Entity == "NRIC" && sp.Variant == "standard" {
				found := engine.Detect(s.Text[sp.Start:sp.End])
				if len(found) != 1 || found[0].Entity != "NRIC" {
					t.Fatalf("independently generated NRIC %q not valid to the engine: %v", s.Text[sp.Start:sp.End], found)
				}
			}
		}
	}
}

func TestEvaluateScoresCoverage(t *testing.T) {
	engine, _ := detect.Default()
	s := []Sample{{Text: "call 9123 4567 or 91 23 45 67", Spans: []Span{{"PHONE", 5, 14, "4-4"}, {"PHONE", 18, 29, "2-2-2-2"}}}}
	r := Evaluate(s, engine)["PHONE"]
	if r.Total != 2 || r.Caught != 1 || r.Detected != 1 || r.FalsePositives != 0 {
		t.Errorf("%+v", r)
	}
}
