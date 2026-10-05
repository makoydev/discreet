// Package detect finds Singapore personal data in text, implementing the
// sg-pii-rules specification (SPEC.md) with Go's regexp package, which is RE2.
// The rules themselves are data, vendored in third_party/sgpiirules.
package detect

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/makoydev/discreet/third_party/sgpiirules"
)

// Match is one piece of personal data found in a text. Start and End are
// byte offsets; they are not part of conformance (SPEC.md §5).
type Match struct {
	Entity string
	Value  string
	Start  int
	End    int
}

type rulesFile struct {
	Version   string `json:"version"`
	Detectors []struct {
		ID        string `json:"id"`
		Entity    string `json:"entity"`
		Pattern   string `json:"pattern"`
		Validator string `json:"validator"`
	} `json:"detectors"`
}

type detector struct {
	id       string
	entity   string
	order    int
	pattern  *regexp.Regexp
	valueIdx int // index of the `value` group, or -1 to use the whole match
	validate func(string) bool
}

// Engine holds compiled detectors. It is safe for concurrent use.
type Engine struct {
	Version   string
	detectors []detector
}

// New compiles a detectors.json file. It fails on an invalid pattern or a
// validator this package doesn't implement, never silently (SPEC.md §2, §3).
func New(rulesJSON []byte) (*Engine, error) {
	var rules rulesFile
	if err := json.Unmarshal(rulesJSON, &rules); err != nil {
		return nil, fmt.Errorf("detectors.json: %w", err)
	}
	e := &Engine{Version: rules.Version}
	for i, r := range rules.Detectors {
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return nil, fmt.Errorf("detector %s: %w", r.ID, err)
		}
		d := detector{id: r.ID, entity: r.Entity, order: i, pattern: re, valueIdx: re.SubexpIndex("value")}
		if r.Validator != "" {
			if d.validate = validators[r.Validator]; d.validate == nil {
				return nil, fmt.Errorf("detector %s names unknown validator %s", r.ID, r.Validator)
			}
		}
		e.detectors = append(e.detectors, d)
	}
	return e, nil
}

// Default compiles the vendored sg-pii-rules release.
func Default() (*Engine, error) {
	data, err := sgpiirules.Files.ReadFile("detectors.json")
	if err != nil {
		return nil, err
	}
	return New(data)
}

type candidate struct {
	Match
	order int
}

// Detect runs the algorithm in SPEC.md §4 and returns matches in order of
// position, with no two overlapping.
func (e *Engine) Detect(text string) []Match {
	var cands []candidate
	for _, d := range e.detectors {
		for _, loc := range d.pattern.FindAllStringSubmatchIndex(text, -1) {
			start, end := loc[0], loc[1]
			if d.valueIdx >= 0 { // report only the value group; the rest is context
				start, end = loc[2*d.valueIdx], loc[2*d.valueIdx+1]
			}
			if start < 0 || start == end {
				continue
			}
			value := text[start:end]
			if d.validate != nil && !d.validate(value) {
				continue
			}
			cands = append(cands, candidate{Match{d.entity, value, start, end}, d.order})
		}
	}

	// Overlaps: sort by start, then longest first, then detector order. Kept
	// matches never overlap, so a candidate can only clash with the last kept.
	sort.SliceStable(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Start != b.Start {
			return a.Start < b.Start
		}
		if la, lb := a.End-a.Start, b.End-b.Start; la != lb {
			return la > lb
		}
		return a.order < b.order
	})
	var kept []candidate
	for _, c := range cands {
		if len(kept) == 0 || c.Start >= kept[len(kept)-1].End {
			kept = append(kept, c)
			continue
		}
		last := kept[len(kept)-1]
		cl, ll := c.End-c.Start, last.End-last.Start
		if cl > ll || (cl == ll && c.order < last.order) {
			kept[len(kept)-1] = c
		}
	}

	out := make([]Match, len(kept))
	for i, c := range kept {
		out[i] = c.Match
	}
	return out
}
