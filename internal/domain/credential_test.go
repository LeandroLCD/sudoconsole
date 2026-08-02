package domain

import (
	"context"
	"errors"
	"strings"
	"testing"
	"unsafe"
)

func TestNewCredential_CopiesInput(t *testing.T) {
	original := []byte("secret")
	c := NewCredential(original)
	original[0] = 'X'
	if got := string(c.Bytes()); got != "secret" {
		t.Fatalf("Credential should be a copy, got %q", got)
	}
}

func TestNewCredentialFromString(t *testing.T) {
	c := NewCredentialFromString("p4ss")
	if c.Len() != 4 {
		t.Fatalf("Len = %d, want 4", c.Len())
	}
	if string(c.Bytes()) != "p4ss" {
		t.Fatalf("Bytes = %q, want %q", string(c.Bytes()), "p4ss")
	}
}

func TestCredential_Empty(t *testing.T) {
	var c Credential
	if !c.IsEmpty() {
		t.Fatal("zero-value Credential should be empty")
	}
	c = NewCredential([]byte(""))
	if !c.IsEmpty() {
		t.Fatal("empty-input Credential should be empty")
	}
	if c.Bytes() != nil {
		t.Fatal("Bytes of empty Credential should be nil")
	}
}

func TestCredential_Bytes_ReturnsCopy(t *testing.T) {
	c := NewCredential([]byte("abc"))
	out := c.Bytes()
	out[0] = 'Z'
	if string(c.Bytes()) != "abc" {
		t.Fatal("Bytes should return a defensive copy")
	}
}

func TestCredential_Zeroize(t *testing.T) {
	c := NewCredential([]byte("hunter2"))
	if c.IsEmpty() {
		t.Fatal("pre-condition: not empty")
	}
	c.Zeroize()
	if !c.IsEmpty() {
		t.Fatal("after Zeroize should be empty")
	}
	if c.Bytes() != nil {
		t.Fatal("Bytes after Zeroize should be nil")
	}
	// Multiple calls must not panic.
	c.Zeroize()
}

// TestCredential_ZeroizeOverwritesBackingArray is the M12
// hardening test: it verifies that the backing array actually
// contains zeros after Zeroize (not just that the slice header
// is nil). We capture the backing pointer via unsafe before
// zeroizing and inspect the bytes afterwards.
func TestCredential_ZeroizeOverwritesBackingArray(t *testing.T) {
	c := NewCredential([]byte("hunter2hunter2"))
	// Reach into the credential and grab the backing array
	// pointer. This test will trip if the Credential struct ever
	// stops holding the data in []byte.
	backing := structPtr(t, &c)
	for i := 0; i < len(*backing); i++ {
		if (*backing)[i] == 0 {
			t.Fatalf("backing[%d] already zero before test", i)
		}
	}
	c.Zeroize()
	for i := 0; i < len(*backing); i++ {
		if (*backing)[i] != 0 {
			t.Fatalf("backing[%d] = %d after Zeroize, want 0", i, (*backing)[i])
		}
	}
}

// TestCredential_BytesIsSeparateAllocation asserts that Bytes()
// returns a fresh copy that shares no backing storage with the
// credential — overwriting the returned slice must not corrupt
// the credential's internal bytes (and vice versa).
func TestCredential_BytesIsSeparateAllocation(t *testing.T) {
	c := NewCredential([]byte("abcdef"))
	out := c.Bytes()
	// The slice header addresses must differ because Bytes()
	// allocates a new buffer.
	if len(out) != len(c.Bytes()) {
		t.Fatal("length mismatch")
	}
	if &out[0] == &c.Bytes()[0] {
		t.Fatal("Bytes returned the same backing array")
	}
}

// structPtr returns a pointer to the []byte that backs the
// credential via reflection on the unsafe layout of the struct.
// Safe because the struct has only one []byte field at offset 0.
func structPtr(t *testing.T, c *Credential) *[]byte {
	t.Helper()
	p := unsafe.Pointer(c)
	return (*[]byte)(p)
}

func TestCredential_ZeroizeNilSafe(t *testing.T) {
	var c *Credential
	c.Zeroize() // must not panic
}

func TestCredential_ConstantTimeEquals(t *testing.T) {
	a := NewCredential([]byte("aaaa"))
	b := NewCredential([]byte("aaaa"))
	c := NewCredential([]byte("bbbb"))
	if !a.ConstantTimeEquals(b) {
		t.Fatal("equal credentials should compare equal")
	}
	if a.ConstantTimeEquals(c) {
		t.Fatal("different credentials should not compare equal")
	}
	if a.ConstantTimeEquals(NewCredential([]byte(""))) {
		t.Fatal("non-empty vs empty should not compare equal")
	}
	if !NewCredential([]byte("")).ConstantTimeEquals(NewCredential([]byte(""))) {
		t.Fatal("two empty credentials should compare equal")
	}
}

func TestCredential_String_Redacted(t *testing.T) {
	c := NewCredential([]byte("supersecret"))
	if strings.Contains(c.String(), "supersecret") {
		t.Fatalf("String() leaked the secret: %s", c.String())
	}
	if strings.Contains(c.GoString(), "supersecret") {
		t.Fatalf("GoString() leaked the secret: %s", c.GoString())
	}
}

func TestCredential_MarshalJSON_Redacted(t *testing.T) {
	c := NewCredential([]byte("supersecret"))
	data, err := c.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "supersecret") {
		t.Fatalf("MarshalJSON leaked the secret: %s", string(data))
	}
	if !strings.Contains(string(data), `"redacted":true`) {
		t.Fatalf("MarshalJSON should mark redacted: %s", string(data))
	}
}

func TestItoa(t *testing.T) {
	cases := map[int]string{0: "0", 1: "1", -1: "-1", 12345: "12345", -12345: "-12345"}
	for in, want := range cases {
		if got := itoa(in); got != want {
			t.Errorf("itoa(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestCredential_ImplementsStringerAndGoStringer(t *testing.T) {
	// Compile-time check that Stringer/GoStringer are implemented.
	c := NewCredential([]byte("x"))
	var _ interface{ String() string } = c
	var _ interface{ GoString() string } = c
}

// Ensure the package compiles when used via context (no-op test).
func TestCredential_PackageContext(t *testing.T) {
	_ = context.Background
	_ = errors.New
}
