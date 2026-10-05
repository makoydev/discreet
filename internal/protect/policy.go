// Package protect replaces personal data with placeholders before a request
// reaches the model and puts it back in the answer, following a policy.
package protect

import (
	"bytes"
	_ "embed"
	"fmt"

	"go.yaml.in/yaml/v3"
)

// Action says what happens to one kind of personal data.
type Action string

const (
	Tokenise Action = "tokenise" // placeholder, restored in the answer
	Redact   Action = "redact"   // fixed marker, never restored
	Block    Action = "block"    // the whole request is refused
)

// Policy is the parsed policy file.
type Policy struct {
	Version        int               `yaml:"version"`
	DefaultAction  Action            `yaml:"default_action"`
	Entities       map[string]Action `yaml:"entities"`
	DeniedPurposes []string          `yaml:"denied_purposes"`
}

//go:embed default-policy.yaml
var defaultPolicy []byte

// DefaultPolicy returns the built-in policy.
func DefaultPolicy() *Policy {
	p, err := ParsePolicy(defaultPolicy)
	if err != nil {
		panic("protect: built-in policy is invalid: " + err.Error())
	}
	return p
}

// ParsePolicy reads a policy file. Unknown keys and unknown actions are
// errors, so a typo can't silently weaken protection.
func ParsePolicy(data []byte) (*Policy, error) {
	var p Policy
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("policy: %w", err)
	}
	if p.Version != 1 {
		return nil, fmt.Errorf("policy: unsupported version %d", p.Version)
	}
	if p.DefaultAction == "" {
		p.DefaultAction = Tokenise
	}
	for entity, a := range p.Entities {
		if !a.valid() {
			return nil, fmt.Errorf("policy: entity %s has unknown action %q", entity, a)
		}
	}
	if !p.DefaultAction.valid() {
		return nil, fmt.Errorf("policy: unknown default_action %q", p.DefaultAction)
	}
	return &p, nil
}

func (a Action) valid() bool { return a == Tokenise || a == Redact || a == Block }

// ActionFor returns the action for an entity, or the default.
func (p *Policy) ActionFor(entity string) Action {
	if a, ok := p.Entities[entity]; ok {
		return a
	}
	return p.DefaultAction
}
