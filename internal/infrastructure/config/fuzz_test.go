package config

import (
	"context"
	"errors"
	"os"
	"testing"
)

// FuzzLoader hands random bytes to the TOML loader. The loader must
// never panic; it should return either (default config, nil) when
// the bytes don't form a valid file, or a wrapped error.
func FuzzLoader(f *testing.F) {
	f.Add([]byte("[cache]\ntimeout_seconds = 600\n"))
	f.Add([]byte("not = valid = toml =="))
	f.Add([]byte(""))
	f.Add([]byte("[policy]\nblocked.commands = [\"ssh\"]\n"))
	f.Add([]byte("[policy]\nextra_patterns = [\"re:(a+)+\"]\n"))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 4096 {
			return
		}
		store := New()
		store.FS = fuzzFS{data: raw}
		// Pin the path so the loader always tries to read it from
		// the fake FS.
		store.Path = "/etc/sudoconsole/config.toml"
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("loader panicked on input %q: %v", raw, r)
			}
		}()
		_, _ = store.Load(context.Background())
	})
}

// fuzzFS is a minimal FS that only serves ReadFile. Other methods
// return errors because the fuzzer never exercises them.
type fuzzFS struct{ data []byte }

func (f fuzzFS) ReadFile(string) ([]byte, error) { return f.data, nil }
func (fuzzFS) WriteFile(string, []byte, os.FileMode) error {
	return errors.New("fuzzFS: write not supported")
}
func (fuzzFS) MkdirAll(string, os.FileMode) error {
	return errors.New("fuzzFS: mkdir not supported")
}
func (fuzzFS) Rename(string, string) error { return errors.New("fuzzFS: rename not supported") }
func (fuzzFS) Remove(string) error         { return errors.New("fuzzFS: remove not supported") }
