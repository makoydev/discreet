package vault

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func newTest(t *testing.T) *Vault {
	t.Helper()
	v, err := New(time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	return v
}

func TestPutGet(t *testing.T) {
	v := newTest(t)
	v.Put("r1/<NRIC_1>", "S1234567D") // synthetic example NRIC
	got, ok := v.Get("r1/<NRIC_1>")
	if !ok || got != "S1234567D" {
		t.Fatalf("Get = %q, %v", got, ok)
	}
	if _, ok := v.Get("r2/<NRIC_1>"); ok {
		t.Error("a key that was never stored was found")
	}
}

func TestValuesAreEncryptedAndNeverPrinted(t *testing.T) {
	v := newTest(t)
	v.Put("k", "S1234567D")
	for _, e := range v.entries {
		if strings.Contains(string(e.sealed), "S1234567D") {
			t.Error("value stored in plain text")
		}
	}
	for _, s := range []string{fmt.Sprint(v), fmt.Sprintf("%v %+v %#v", v, v, v)} {
		if strings.Contains(s, "S1234567D") {
			t.Errorf("formatting leaked the value: %s", s)
		}
	}
}

func TestSealedValueIsBoundToItsKey(t *testing.T) {
	v := newTest(t)
	v.Put("a", "secret-a")
	v.entries["b"] = v.entries["a"] // move the ciphertext to another key
	if _, ok := v.Get("b"); ok {
		t.Error("a sealed value opened under a different key")
	}
}

func TestExpiry(t *testing.T) {
	v := newTest(t)
	now := time.Now()
	v.now = func() time.Time { return now }
	v.Put("k", "value")
	now = now.Add(59 * time.Second)
	if _, ok := v.Get("k"); !ok {
		t.Fatal("expired too early")
	}
	now = now.Add(time.Second)
	if _, ok := v.Get("k"); ok {
		t.Error("value still readable after its TTL")
	}
	v.Put("x", "1")
	v.Put("y", "2")
	now = now.Add(time.Minute)
	if n := v.Sweep(); n != 2 || v.Len() != 0 {
		t.Errorf("Sweep removed %d, %d left", n, v.Len())
	}
}

func TestDelete(t *testing.T) {
	v := newTest(t)
	v.Put("a", "1")
	v.Put("b", "2")
	v.Delete("a", "b")
	if v.Len() != 0 {
		t.Errorf("%d entries left", v.Len())
	}
}
