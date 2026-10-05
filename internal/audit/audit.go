// Package audit writes Discreet's tamper-evident audit log: an append-only
// file of JSON lines in which every record carries the SHA-256 of the one
// before it, so changing, deleting or reordering any record breaks the chain.
// Prompts and responses are stored only as HMAC-SHA-256 with a server secret,
// never as text: plain hashes of short identifiers such as NRICs could be
// reversed by trying every possible number.
package audit

import (
	"bufio"
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

// Genesis is the previous-hash of the first record.
const Genesis = "0000000000000000000000000000000000000000000000000000000000000000"

// Decisions recorded for a request.
const (
	Allowed        = "allowed"
	RefusedPurpose = "refused_purpose"
	BlockedEntity  = "blocked_entity"
	RefusedBudget  = "refused_budget"
	UpstreamError  = "upstream_error"
)

// Record is one line of the log. Field order is fixed, so the JSON
// encoding, and therefore the hash, is deterministic.
type Record struct {
	Seq          int64          `json:"seq"`
	Time         string         `json:"time"`
	RequestID    string         `json:"request_id"`
	Tenant       string         `json:"tenant"`
	Purpose      string         `json:"purpose"`
	Decision     string         `json:"decision"`
	Reason       string         `json:"reason,omitempty"`
	Upstream     string         `json:"upstream"`
	Model        string         `json:"model"`
	Entities     map[string]int `json:"entities"`
	PromptHMAC   string         `json:"prompt_hmac,omitempty"`
	ResponseHMAC string         `json:"response_hmac,omitempty"`
	CostUSD      float64        `json:"cost_usd"`
	LatencyMS    int64          `json:"latency_ms"`
	PrevHash     string         `json:"prev_hash"`
	Hash         string         `json:"hash"`
}

// computeHash is the SHA-256 of the record's JSON with the hash left empty.
func computeHash(r Record) string {
	r.Hash = ""
	data, _ := json.Marshal(r)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Log appends records. It is safe for concurrent use.
type Log struct {
	mu       sync.Mutex
	f        *os.File
	key      []byte
	seq      int64
	last     string
	spentDay map[string]float64 // UTC date → cost recorded that day
	now      func() time.Time
}

// MinKeyLength is the shortest HMAC key accepted, in bytes.
const MinKeyLength = 32

// KeyFromEnv reads the HMAC key from DISCREET_HMAC_KEY.
func KeyFromEnv() ([]byte, error) {
	k := os.Getenv("DISCREET_HMAC_KEY")
	if len(k) < MinKeyLength {
		return nil, fmt.Errorf("DISCREET_HMAC_KEY must be set to at least %d characters (for example: openssl rand -hex 32)", MinKeyLength)
	}
	return []byte(k), nil
}

// Open opens or creates the log at path. An existing log is verified
// first: Discreet refuses to append to a broken chain.
func Open(path string, key []byte) (*Log, error) {
	if len(key) < MinKeyLength {
		return nil, fmt.Errorf("audit: HMAC key shorter than %d bytes", MinKeyLength)
	}
	l := &Log{key: key, last: Genesis, spentDay: map[string]float64{}, now: time.Now}
	if existing, err := os.Open(path); err == nil {
		err := scan(existing, func(r Record) {
			l.seq, l.last = r.Seq, r.Hash
			l.spentDay[r.Time[:10]] += r.CostUSD
		})
		existing.Close()
		if err != nil {
			return nil, fmt.Errorf("audit: existing log %s is broken, refusing to append: %w", path, err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	l.f = f
	return l, nil
}

// HMAC returns the keyed hash of text, as stored for prompts and responses.
func (l *Log) HMAC(text string) string {
	m := hmac.New(sha256.New, l.key)
	m.Write([]byte(text))
	return hex.EncodeToString(m.Sum(nil))
}

// Append fills in the sequence number, time and hashes, writes the record
// and flushes it to disk before returning.
func (l *Log) Append(r Record) (Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	r.Seq = l.seq + 1
	r.Time = l.now().UTC().Format(time.RFC3339Nano)
	r.PrevHash = l.last
	if r.Entities == nil {
		r.Entities = map[string]int{}
	}
	r.Hash = computeHash(r)
	line, err := json.Marshal(r)
	if err != nil {
		return Record{}, err
	}
	if _, err := l.f.Write(append(line, '\n')); err != nil {
		return Record{}, err
	}
	if err := l.f.Sync(); err != nil {
		return Record{}, err
	}
	l.seq, l.last = r.Seq, r.Hash
	l.spentDay[r.Time[:10]] += r.CostUSD
	return r, nil
}

// SpentToday is the cost recorded since midnight UTC.
func (l *Log) SpentToday() float64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.spentDay[l.now().UTC().Format("2006-01-02")]
}

// Head returns the last record's hash and sequence number.
func (l *Log) Head() (string, int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.last, l.seq
}

// Close closes the file.
func (l *Log) Close() error { return l.f.Close() }

// VerifyError says which record broke the chain and how.
type VerifyError struct {
	Line    int
	Problem string
}

func (e *VerifyError) Error() string {
	return fmt.Sprintf("record on line %d: %s", e.Line, e.Problem)
}

// scan reads every record, checking the chain, and calls fn for each.
func scan(r io.Reader, fn func(Record)) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	prev, seq, line := Genesis, int64(0), 0
	for sc.Scan() {
		line++
		var rec Record
		dec := json.NewDecoder(bytes.NewReader(sc.Bytes()))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&rec); err != nil {
			return &VerifyError{line, "not a valid record: " + err.Error()}
		}
		switch {
		case rec.Seq != seq+1:
			return &VerifyError{line, fmt.Sprintf("sequence %d, expected %d: a record is missing or out of order", rec.Seq, seq+1)}
		case rec.PrevHash != prev:
			return &VerifyError{line, "previous-hash does not match the record before it"}
		case rec.Hash != computeHash(rec):
			return &VerifyError{line, "hash does not match the record's contents: it was changed"}
		case len(rec.Time) < 10:
			return &VerifyError{line, "missing time"}
		}
		fn(rec)
		prev, seq = rec.Hash, rec.Seq
	}
	return sc.Err()
}

// Summary is the result of a successful verification.
type Summary struct {
	Records  int64
	HeadHash string
}

// Verify checks a whole log. It needs no key: the chain uses plain SHA-256.
// It can't detect records cut off the end; compare the head hash with one
// recorded elsewhere for that.
func Verify(r io.Reader) (Summary, error) {
	var s Summary
	s.HeadHash = Genesis
	err := scan(r, func(rec Record) { s.Records, s.HeadHash = rec.Seq, rec.Hash })
	return s, err
}

// ExportCSV writes a verified log as CSV. A broken log is not exported.
func ExportCSV(r io.Reader, w io.Writer) error {
	var rows []Record
	if err := scan(r, func(rec Record) { rows = append(rows, rec) }); err != nil {
		return err
	}
	cw := newCSV(w)
	cw.row("seq", "time", "request_id", "tenant", "purpose", "decision", "reason", "upstream", "model", "entities", "prompt_hmac", "response_hmac", "cost_usd", "latency_ms", "prev_hash", "hash")
	for _, r := range rows {
		cw.row(fmt.Sprint(r.Seq), r.Time, r.RequestID, r.Tenant, r.Purpose, r.Decision, r.Reason, r.Upstream, r.Model,
			entitiesText(r.Entities), r.PromptHMAC, r.ResponseHMAC, fmt.Sprintf("%.6f", r.CostUSD), fmt.Sprint(r.LatencyMS), r.PrevHash, r.Hash)
	}
	return cw.flush()
}

func entitiesText(m map[string]int) string {
	data, _ := json.Marshal(m) // keys sorted, so stable
	return strings.Trim(string(data), "{}")
}
