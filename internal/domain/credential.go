package domain

import (
	"crypto/subtle"
	"fmt"
	"unsafe"
)

// Credential represents a sensitive secret such as a sudo password.
//
// A Credential MUST be created via NewCredential so the underlying buffer
// is allocated on the heap. Use Zeroize to overwrite the buffer before
// releasing the reference. Zeroize is safe to call multiple times.
//
// Credential is intentionally a struct (not a pointer to a string) so
// that the secret is not stored in the immutable backing array of a
// Go string, which cannot be reliably wiped.
type Credential struct {
	// bytes holds the password bytes. The slice header itself is on the
	// stack/in a struct field, but the underlying array lives on the heap
	// and is overwritten by Zeroize.
	bytes []byte
}

// NewCredential returns a Credential wrapping the given secret.
//
// The returned slice is a copy of the input — callers may overwrite the
// original buffer without affecting the Credential.
func NewCredential(secret []byte) Credential {
	if len(secret) == 0 {
		return Credential{}
	}
	cp := make([]byte, len(secret))
	copy(cp, secret)
	return Credential{bytes: cp}
}

// NewCredentialFromString is a convenience wrapper for string secrets.
func NewCredentialFromString(secret string) Credential {
	return NewCredential([]byte(secret))
}

// Bytes returns a constant-time-readable view of the credential bytes.
// Callers should NOT store the returned slice. Use it, then Zeroize.
//
// If Zeroize has already been called, Bytes returns nil.
func (c Credential) Bytes() []byte {
	if c.bytes == nil {
		return nil
	}
	out := make([]byte, len(c.bytes))
	copy(out, c.bytes)
	return out
}

// Len returns the length of the credential without exposing its content.
func (c Credential) Len() int {
	if c.bytes == nil {
		return 0
	}
	return len(c.bytes)
}

// IsEmpty reports whether the credential holds no bytes.
func (c Credential) IsEmpty() bool {
	return c.Len() == 0
}

// ConstantTimeEquals compares two credentials in constant time to avoid
// timing side channels. Returns true iff they have equal length and content.
func (c Credential) ConstantTimeEquals(other Credential) bool {
	return subtle.ConstantTimeCompare(c.Bytes(), other.Bytes()) == 1
}

// Zeroize overwrites the credential's underlying buffer with zeros and
// releases the reference. Safe to call multiple times.
//
// It is the caller's responsibility to also disable core dumps
// (RLIMIT_CORE=0) for the duration of the credential's lifetime if the
// threat model warrants it. See infra/sudo/auth.go.
func (c *Credential) Zeroize() {
	if c == nil || c.bytes == nil {
		return
	}
	// Write zeros across the entire backing array.
	for i := range c.bytes {
		c.bytes[i] = 0
	}
	// Hint to the GC. We use unsafe.Pointer + explicit clobber because
	// the Go runtime does not guarantee that runtime.KeepAlive alone
	// will prevent copies from lingering in registers.
	p := unsafe.Pointer(&c.bytes[0]) // #nosec G103 -- audited use for zeroing
	for i := 0; i < len(c.bytes); i++ {
		*(*byte)(unsafe.Add(p, i)) = 0
	}
	c.bytes = nil
}

// String returns a redacted representation. Implements fmt.Stringer.
//
// The credential value itself is never returned by String — this method
// exists only to prevent accidental logging of the password.
func (c Credential) String() string {
	return fmt.Sprintf("Credential(len=%d)", c.Len())
}

// GoString returns a redacted representation. Implements fmt.GoStringer.
func (c Credential) GoString() string {
	return fmt.Sprintf("Credential(len=%d)", c.Len())
}

// MarshalJSON returns a redacted JSON representation so the credential is
// never accidentally marshaled into a config file or log.
func (c Credential) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true,"len":` + itoa(c.Len()) + `}`), nil
}

// itoa is a tiny int-to-string helper to avoid importing strconv here.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
