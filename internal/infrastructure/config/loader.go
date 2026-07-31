package config

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// Store is a file-backed domain.ConfigStore.
type Store struct {
	// Path is the absolute path of the user config file. Empty means
	// "resolve on demand via PathResolver".
	Path string

	// Resolver computes the default path when Path is empty.
	Resolver PathResolver

	// FS is the filesystem used for read/write. Defaults to the OS fs.
	FS FS
}

// FS is the minimal subset of the standard library fs needed by Store.
// Tests can supply an in-memory implementation.
type FS interface {
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm os.FileMode) error
	MkdirAll(path string, perm os.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(name string) error
}

// osFS implements FS against the real filesystem.
type osFS struct{}

func (osFS) ReadFile(name string) ([]byte, error) { // #nosec G304 -- path is supplied by the user (config path), validated by the caller.
	return os.ReadFile(name)
}
func (osFS) WriteFile(name string, d []byte, p os.FileMode) error {
	return os.WriteFile(name, d, p)
}
func (osFS) MkdirAll(path string, perm os.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }
func (osFS) Remove(name string) error                     { return os.Remove(name) }

// New returns a Store using the OS filesystem and the platform's
// default config path resolution.
func New() *Store {
	r := NewPathResolver()
	return &Store{Path: r.DefaultPath(), Resolver: r, FS: osFS{}}
}

// NewAt returns a Store that reads/writes a specific file. It is the
// constructor the CLI and tests should use.
func NewAt(path string) *Store {
	r := NewPathResolver()
	return &Store{Path: path, Resolver: r, FS: osFS{}}
}

// DefaultPath returns the resolved default path, regardless of whether
// the store is currently bound to a specific file.
func (s *Store) DefaultPath() string {
	if s.Path != "" {
		return s.Path
	}
	return s.Resolver.DefaultPath()
}

// Load reads, parses and validates the configuration file, applying the
// per-area defaults for any section the user did not set.
//
// If the file does not exist, Load returns domain.DefaultConfig() with
// no error. Callers can detect "no file" by checking the returned
// Source field on the loader's diagnostic struct if needed; for the
// domain port we keep the contract simple.
func (s *Store) Load(ctx context.Context) (domain.Config, error) {
	if err := ctx.Err(); err != nil {
		return domain.Config{}, err
	}
	path := s.DefaultPath()
	raw, err := s.FS.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return domain.DefaultConfig(), nil
		}
		return domain.Config{}, fmt.Errorf("read %s: %w", path, err)
	}
	schema, err := decode(raw)
	if err != nil {
		return domain.Config{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := validate(schema); err != nil {
		return domain.Config{}, fmt.Errorf("validate %s: %w", path, err)
	}
	return schema.toConfig(), nil
}

// Save writes the configuration to the file at s.Path, creating any
// missing parent directories. The file is written atomically by
// staging to a sibling temp file and renaming.
func (s *Store) Save(ctx context.Context, cfg domain.Config) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	path := s.DefaultPath()
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := s.FS.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("mkdir %s: %w", dir, err)
		}
	}
	schema := newFileSchemaFromConfig(cfg)
	buf := bytes.NewBuffer(nil)
	enc := toml.NewEncoder(buf)
	enc.SetIndentTables(false)
	if err := enc.Encode(schema); err != nil {
		return fmt.Errorf("encode toml: %w", err)
	}
	// Stage via a temp file to avoid leaving a half-written config on
	// disk if the process is killed mid-write. We use a sibling path
	// under the same directory so the rename is atomic on the same
	// filesystem.
	dir := filepath.Dir(path)
	tmpName := filepath.Join(dir, ".sudoconsole.toml.tmp")
	defer func() { _ = s.FS.Remove(tmpName) }()
	if err := s.FS.WriteFile(tmpName, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := s.FS.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename %s -> %s: %w", tmpName, path, err)
	}
	return nil
}

func decode(b []byte) (fileSchema, error) {
	var s fileSchema
	if err := toml.Unmarshal(b, &s); err != nil {
		return fileSchema{}, err
	}
	return s, nil
}
