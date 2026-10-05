package bench

import (
	"sort"

	"github.com/makoydev/discreet/internal/detect"
)

// sameKind maps a detected entity to the true entity it counts for: a
// mistyped NRIC found as NRIC_LIKE still protects an NRIC.
func sameKind(detected string) string {
	if detected == "NRIC_LIKE" {
		return "NRIC"
	}
	return detected
}

// Result counts one entity's outcomes.
type Result struct {
	Entity string
	// Recall side: true values of this entity.
	Total     int
	Caught    int // fully covered by a detection of the same kind
	Protected int // fully covered by any detection(s), whatever the kind
	// Precision side: detections reported as this entity.
	Detected       int
	FalsePositives int // overlap no true value of the same kind
	Variants       map[string][2]int
	FPExamples     []string
}

func (r *Result) Recall() float64    { return ratio(r.Caught, r.Total) }
func (r *Result) Precision() float64 { return ratio(r.Detected-r.FalsePositives, r.Detected) }

func ratio(a, b int) float64 {
	if b == 0 {
		return 0
	}
	return float64(a) / float64(b)
}

// Evaluate runs the engine over every sample and scores it.
func Evaluate(samples []Sample, engine *detect.Engine) map[string]*Result {
	results := map[string]*Result{}
	get := func(e string) *Result {
		if results[e] == nil {
			results[e] = &Result{Entity: e, Variants: map[string][2]int{}}
		}
		return results[e]
	}
	for _, s := range samples {
		found := engine.Detect(s.Text)
		for _, span := range s.Spans {
			r := get(span.Entity)
			r.Total++
			v := r.Variants[span.Variant]
			v[1]++
			caught := false
			for _, d := range found {
				if sameKind(d.Entity) == span.Entity && d.Start <= span.Start && d.End >= span.End {
					caught = true
				}
			}
			if caught {
				r.Caught++
				v[0]++
			}
			if covered(found, span) {
				r.Protected++
			}
			r.Variants[span.Variant] = v
		}
		for _, d := range found {
			r := get(sameKind(d.Entity))
			r.Detected++
			hit := false
			for _, span := range s.Spans {
				if span.Entity == sameKind(d.Entity) && d.Start < span.End && span.Start < d.End {
					hit = true
				}
			}
			if !hit {
				r.FalsePositives++
				if len(r.FPExamples) < 3 {
					r.FPExamples = append(r.FPExamples, s.Text[d.Start:d.End])
				}
			}
		}
	}
	return results
}

// covered reports whether every byte of span is inside some detection.
func covered(found []detect.Match, span Span) bool {
	pos := span.Start
	sorted := append([]detect.Match(nil), found...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start < sorted[j].Start })
	for _, d := range sorted {
		if d.Start <= pos && d.End > pos {
			pos = d.End
		}
	}
	return pos >= span.End
}
