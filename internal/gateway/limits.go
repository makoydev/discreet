package gateway

import (
	"crypto/sha256"
	"crypto/subtle"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ----- Access tokens -----

// Tokens maps a SHA-256 of each access token to its tenant name. Only the
// hashes are kept in memory, and comparison takes constant time.
type Tokens struct{ hashes map[[32]byte]string }

// ParseTokens reads "tenant:token,tenant2:token2". Tokens must be at
// least 24 characters, so they can't be guessed.
func ParseTokens(spec string) (*Tokens, error) {
	t := &Tokens{hashes: map[[32]byte]string{}}
	for _, pair := range strings.Split(spec, ",") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		tenant, token, ok := strings.Cut(pair, ":")
		if !ok || tenant == "" || len(token) < 24 {
			return nil, fmt.Errorf("access tokens must look like tenant:token, with tokens of at least 24 characters")
		}
		t.hashes[sha256.Sum256([]byte(token))] = tenant
	}
	return t, nil
}

// Tenant returns the tenant for a request's bearer token, if it is valid.
func (t *Tokens) Tenant(r *http.Request) (string, bool) {
	if t == nil {
		return "", false
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || token == "" {
		return "", false
	}
	got := sha256.Sum256([]byte(token))
	var tenant string
	found := 0
	for h, name := range t.hashes {
		if subtle.ConstantTimeCompare(got[:], h[:]) == 1 {
			tenant, found = name, 1
		}
	}
	return tenant, found == 1
}

// Len reports how many tokens are configured.
func (t *Tokens) Len() int {
	if t == nil {
		return 0
	}
	return len(t.hashes)
}

// ----- Prices and the daily budget -----

// Price is US dollars per million tokens.
type Price struct{ Input, Output float64 }

// KnownPrices lists models whose standard prices were checked against the
// provider's own page (see EVALS.md and ADR 0007).
var KnownPrices = map[string]Price{
	"gpt-6-luna": {Input: 0.10, Output: 0.50}, // developers.openai.com, checked 2026-10-05
}

// Cost of a finished call.
func (p Price) Cost(inputTokens, outputTokens int) float64 {
	return (float64(inputTokens)*p.Input + float64(outputTokens)*p.Output) / 1e6
}

// Ceiling is the most a request can cost. A token is at least one byte, so
// the protected prompt's size in bytes, plus a small allowance per message,
// bounds the input tokens; the output cap bounds the rest.
func (p Price) Ceiling(promptBytes, messages, maxOutputTokens int) float64 {
	return p.Cost(promptBytes+16*messages, maxOutputTokens)
}

// Budget enforces a hard daily limit. Spending already recorded comes from
// the audit log, so a restart can't reset it; requests in flight reserve
// their worst case, so parallel requests can't overspend together.
type Budget struct {
	mu       sync.Mutex
	limit    float64
	reserved float64
	spent    func() float64
}

// NewBudget creates a budget of limit US dollars a day.
func NewBudget(limit float64, spentToday func() float64) *Budget {
	return &Budget{limit: limit, spent: spentToday}
}

// Reserve claims ceiling dollars, or reports that the day's budget would
// be exceeded. Every successful Reserve must be followed by Release.
func (b *Budget) Reserve(ceiling float64) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.spent()+b.reserved+ceiling > b.limit {
		return false
	}
	b.reserved += ceiling
	return true
}

// Release returns a reservation once the real cost has been recorded.
func (b *Budget) Release(ceiling float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reserved -= ceiling
}

// ----- Rate limit -----

// RateLimiter allows each client a burst of requests, refilled steadily.
type RateLimiter struct {
	mu       sync.Mutex
	perMin   float64
	burst    float64
	clients  map[string]*bucket
	now      func() time.Time
	ipHeader string
}

type bucket struct {
	tokens float64
	last   time.Time
}

// NewRateLimiter allows perMinute requests a minute per client, up to burst
// at once. ipHeader names a header set by a trusted proxy (for example
// Fly-Client-IP); leave it empty to use the connection's address.
func NewRateLimiter(perMinute, burst int, ipHeader string) *RateLimiter {
	return &RateLimiter{perMin: float64(perMinute), burst: float64(burst), clients: map[string]*bucket{}, now: time.Now, ipHeader: ipHeader}
}

// Allow reports whether the request's client may make a request now.
func (l *RateLimiter) Allow(r *http.Request) bool {
	if l == nil {
		return true
	}
	ip := ""
	if l.ipHeader != "" {
		ip = strings.TrimSpace(r.Header.Get(l.ipHeader))
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	b, ok := l.clients[ip]
	if !ok {
		b = &bucket{tokens: l.burst, last: now}
		l.clients[ip] = b
	}
	b.tokens = min(l.burst, b.tokens+now.Sub(b.last).Minutes()*l.perMin)
	b.last = now
	if len(l.clients) > 10000 { // forget idle clients now and then
		for k, c := range l.clients {
			if now.Sub(c.last) > 10*time.Minute {
				delete(l.clients, k)
			}
		}
	}
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
