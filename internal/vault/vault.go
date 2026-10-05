// Package vault keeps the real values behind placeholders for a short time,
// encrypted in memory with AES-256-GCM under a key that exists only in this
// process. Nothing in this package logs or formats a value.
package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"sync"
	"time"
)

// DefaultTTL is how long a value is kept: long enough for one request.
const DefaultTTL = 10 * time.Minute

type entry struct {
	nonce, sealed []byte
	expires       time.Time
}

// Vault maps keys to encrypted values. It is safe for concurrent use.
type Vault struct {
	mu      sync.Mutex
	aead    cipher.AEAD
	entries map[string]entry
	ttl     time.Duration
	now     func() time.Time
}

// New creates a vault with a fresh random 256-bit key.
func New(ttl time.Duration) (*Vault, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Vault{aead: aead, entries: map[string]entry{}, ttl: ttl, now: time.Now}, nil
}

// Put encrypts value under key. The key is bound to the ciphertext as
// additional data, so a sealed value can't be moved to another key.
func (v *Vault) Put(key, value string) {
	nonce := make([]byte, v.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		panic("vault: no randomness: " + err.Error())
	}
	sealed := v.aead.Seal(nil, nonce, []byte(value), []byte(key))
	v.mu.Lock()
	defer v.mu.Unlock()
	v.entries[key] = entry{nonce, sealed, v.now().Add(v.ttl)}
}

// Get decrypts the value under key, if it exists and hasn't expired.
func (v *Vault) Get(key string) (string, bool) {
	v.mu.Lock()
	e, ok := v.entries[key]
	if ok && !v.now().Before(e.expires) {
		delete(v.entries, key)
		ok = false
	}
	v.mu.Unlock()
	if !ok {
		return "", false
	}
	plain, err := v.aead.Open(nil, e.nonce, e.sealed, []byte(key))
	if err != nil {
		return "", false
	}
	return string(plain), true
}

// Delete removes keys immediately, for example when a request ends.
func (v *Vault) Delete(keys ...string) {
	v.mu.Lock()
	defer v.mu.Unlock()
	for _, k := range keys {
		delete(v.entries, k)
	}
}

// Sweep removes expired entries and returns how many it removed.
func (v *Vault) Sweep() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	n := 0
	for k, e := range v.entries {
		if !v.now().Before(e.expires) {
			delete(v.entries, k)
			n++
		}
	}
	return n
}

// Len reports how many entries are stored, expired or not.
func (v *Vault) Len() int {
	v.mu.Lock()
	defer v.mu.Unlock()
	return len(v.entries)
}

// String never shows contents, so a vault printed by mistake leaks nothing.
func (v *Vault) String() string { return "vault{redacted}" }

// GoString does the same for %#v.
func (v *Vault) GoString() string { return v.String() }
