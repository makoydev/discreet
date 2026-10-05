package protect

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/makoydev/discreet/internal/detect"
	"github.com/makoydev/discreet/internal/vault"
)

// BlockedError means the policy refuses a request containing an entity.
type BlockedError struct{ Entity string }

func (e *BlockedError) Error() string {
	return fmt.Sprintf("request contains %s, which the policy blocks", e.Entity)
}

// Session protects one request. Placeholders are numbered per entity and
// reused for the same value anywhere in the request, so the model can tell
// two people apart without seeing either. Values live only in the vault,
// encrypted; the session remembers them by a keyed hash.
type Session struct {
	id      string
	policy  *Policy
	vault   *vault.Vault
	hashKey []byte
	byHash  map[string]string // keyed hash of entity+value → placeholder
	next    map[string]int    // entity → last number issued
	issued  []string          // placeholders this session created
	found   map[string]int    // entity → matches seen, whatever the action
}

// NewSession starts protecting one request.
func NewSession(p *Policy, v *vault.Vault) *Session {
	id, key := make([]byte, 12), make([]byte, 32)
	if _, err := rand.Read(id); err != nil {
		panic(err)
	}
	if _, err := rand.Read(key); err != nil {
		panic(err)
	}
	return &Session{
		id: hex.EncodeToString(id), policy: p, vault: v, hashKey: key,
		byHash: map[string]string{}, next: map[string]int{}, found: map[string]int{},
	}
}

func (s *Session) vaultKey(placeholder string) string { return s.id + "/" + placeholder }

func (s *Session) hash(entity, value string) string {
	m := hmac.New(sha256.New, s.hashKey)
	m.Write([]byte(entity + "\x00" + value))
	return string(m.Sum(nil))
}

// Protect replaces each match in text according to the policy. Matches
// must come from detect.Engine.Detect on the same text.
func (s *Session) Protect(text string, matches []detect.Match) (string, error) {
	var b strings.Builder
	pos := 0
	for _, m := range matches {
		s.found[m.Entity]++
		var replacement string
		switch s.policy.ActionFor(m.Entity) {
		case Block:
			return "", &BlockedError{Entity: m.Entity}
		case Redact:
			replacement = "[REDACTED_" + m.Entity + "]"
		default:
			replacement = s.placeholder(m.Entity, m.Value)
		}
		b.WriteString(text[pos:m.Start])
		b.WriteString(replacement)
		pos = m.End
	}
	b.WriteString(text[pos:])
	return b.String(), nil
}

func (s *Session) placeholder(entity, value string) string {
	h := s.hash(entity, value)
	if p, ok := s.byHash[h]; ok {
		return p
	}
	s.next[entity]++
	p := fmt.Sprintf("<%s_%d>", entity, s.next[entity])
	s.vault.Put(s.vaultKey(p), value)
	s.byHash[h] = p
	s.issued = append(s.issued, p)
	return p
}

var placeholderPattern = regexp.MustCompile(`<[A-Z][A-Z_]*_\d+>`)

// Restore puts real values back in place of this session's placeholders.
// Placeholders it didn't issue, for example ones a model made up, are left
// as they are, so a model can't fish for values from other requests.
func (s *Session) Restore(text string) string {
	return placeholderPattern.ReplaceAllStringFunc(text, func(p string) string {
		if v, ok := s.vault.Get(s.vaultKey(p)); ok {
			return v
		}
		return p
	})
}

// Part is a piece of a restored answer: plain text, or a value put back in
// place of Placeholder. The demo page uses it to highlight restored values.
type Part struct {
	Text        string `json:"text"`
	Placeholder string `json:"placeholder,omitempty"`
}

// RestoreParts is Restore, split into parts.
func (s *Session) RestoreParts(text string) []Part {
	var parts []Part
	pos := 0
	for _, loc := range placeholderPattern.FindAllStringIndex(text, -1) {
		p := text[loc[0]:loc[1]]
		v, ok := s.vault.Get(s.vaultKey(p))
		if !ok {
			continue
		}
		if loc[0] > pos {
			parts = append(parts, Part{Text: text[pos:loc[0]]})
		}
		parts = append(parts, Part{Text: v, Placeholder: p})
		pos = loc[1]
	}
	if pos < len(text) {
		parts = append(parts, Part{Text: text[pos:]})
	}
	return parts
}

// Join puts parts back together as plain text.
func Join(parts []Part) string {
	var b strings.Builder
	for _, p := range parts {
		b.WriteString(p.Text)
	}
	return b.String()
}

// Counts returns how many matches of each entity were seen.
func (s *Session) Counts() map[string]int {
	out := make(map[string]int, len(s.found))
	for k, v := range s.found {
		out[k] = v
	}
	return out
}

// Placeholders lists the placeholders issued, in order.
func (s *Session) Placeholders() []string {
	out := append([]string(nil), s.issued...)
	sort.Strings(out)
	return out
}

// Close deletes this session's values from the vault straight away.
func (s *Session) Close() {
	keys := make([]string, len(s.issued))
	for i, p := range s.issued {
		keys[i] = s.vaultKey(p)
	}
	s.vault.Delete(keys...)
}
