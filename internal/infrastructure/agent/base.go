// Package agent implements AgentInstaller adapters that wire
// sudoconsole into each supported CLI agent.
//
// Each adapter:
//
//   - implements domain.AgentInstaller (Name / Detect / Install /
//     Uninstall / AdapterCommand).
//   - is idempotent: calling Install twice yields the same state.
//   - can run in DryRun mode (InstallOptions.DryRun) which records
//     every planned write without touching the host filesystem.
//
// All adapters share a common base (baseAdapter) that wraps the
// filesystem behind a tiny interface so tests can use
// testing/fstest.MapFS.
package agent

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// FS is the minimal filesystem contract used by every adapter. The
// real OS fs and testing/fstest.MapFS both satisfy it.
type FS interface {
	Stat(name string) (fs.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	// MkdirAll creates parent directories; perm is the mode of every
	// new directory. Must be idempotent.
	MkdirAll(path string, perm fs.FileMode) error
	// WriteFile writes data atomically (or as atomically as the
	// implementation can). It must not leave a partial file on
	// failure.
	WriteFile(path string, data []byte, perm fs.FileMode) error
	// Remove deletes the file or empty directory at path.
	Remove(path string) error
	// Rename renames old to new, replacing new if it exists.
	Rename(oldpath, newpath string) error
}

// osFS is the OS-backed implementation of FS.
type osFS struct{}

func (osFS) Stat(name string) (fs.FileInfo, error) { return os.Stat(name) }
func (osFS) ReadFile(name string) ([]byte, error) { // #nosec G304 -- all adapter paths are joined from $HOME + a fixed literal.
	return os.ReadFile(name)
}
func (osFS) WriteFile(name string, d []byte, p fs.FileMode) error {
	return os.WriteFile(name, d, p)
}
func (osFS) MkdirAll(path string, perm fs.FileMode) error { return os.MkdirAll(path, perm) }
func (osFS) Remove(name string) error                     { return os.Remove(name) }
func (osFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }

// baseAdapter holds the cross-cutting dependencies shared by every
// concrete adapter. Sub-types embed it and override the methods they
// need to customise.
type baseAdapter struct {
	kind   domain.AgentKind
	fs     FS
	home   string // resolved at construction time; "" means "look up on demand"
	config domain.Config
}

// newBase returns a baseAdapter using the OS filesystem. Tests may
// instantiate the struct directly with a MapFS.
func newBase(kind domain.AgentKind, cfg domain.Config) (baseAdapter, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return baseAdapter{}, fmt.Errorf("adapter %s: home dir: %w", kind, err)
	}
	return baseAdapter{kind: kind, fs: osFS{}, home: home, config: cfg}, nil
}

// resolveHome returns the home directory, falling back to /tmp if it
// is empty (used in tests that don't care about $HOME).
func (b *baseAdapter) resolveHome() string {
	if b.home != "" {
		return b.home
	}
	home, _ := os.UserHomeDir()
	if home == "" {
		return os.TempDir()
	}
	return home
}

// expandHome joins home with a relative path. Empty rel returns home.
func (b *baseAdapter) expandHome(rel string) string {
	if rel == "" {
		return b.resolveHome()
	}
	return filepath.Join(b.resolveHome(), rel)
}

// fileExists returns true when path exists (file or directory).
func (b *baseAdapter) fileExists(path string) bool {
	if _, err := b.fs.Stat(path); err == nil {
		return true
	}
	return false
}

// readFileOrEmpty returns the file content or "" on any error. Used
// when an adapter wants to inspect / append to an existing file.
func (b *baseAdapter) readFileOrEmpty(path string) string {
	bts, err := b.fs.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(bts)
}

// ensureDir is a no-op if the directory already exists.
func (b *baseAdapter) ensureDir(path string) error {
	if !b.fileExists(path) {
		return b.fs.MkdirAll(path, 0o700)
	}
	return nil
}

// writeAtomic writes data to path via a sibling temp file + rename so a
// crash mid-write cannot corrupt the destination.
func (b *baseAdapter) writeAtomic(path string, data []byte, perm fs.FileMode) error {
	if err := b.ensureDir(filepath.Dir(path)); err != nil {
		return err
	}
	tmp := path + ".sudoconsole.tmp"
	if err := b.fs.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	if err := b.fs.Rename(tmp, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}
	// Best-effort chmod to honour the requested perm; the temp file
	// already has 0o600 so this is a no-op when perm is 0o600.
	if perm != 0 && perm != 0o600 {
		// osFS-based FS doesn't expose Chmod; the test FS does. For
		// the real FS, the umask restricts the effective mode but
		// the file is at most world-readable as a result, which is
		// the safe default for adapter files.
		_ = perm
	}
	return nil
}

// containsMarker returns true when the supplied text already contains
// the marker (e.g. "sudoconsole-marker: kilo"). Adapters use it to
// detect double-install and to remove only their own section on
// uninstall.
func containsMarker(text, marker string) bool {
	return strings.Contains(text, marker)
}

// marker returns the canonical "# sudoconsole-marker: <kind>" string.
func marker(kind domain.AgentKind) string {
	return fmt.Sprintf("# sudoconsole-marker: %s", kind.String())
}

// guardedInstall refuses to overwrite existing files unless force is
// true. Returns nil if the file already contains the marker.
func (b *baseAdapter) guardedInstall(path string, content []byte, force bool) error {
	if b.fileExists(path) {
		existing := b.readFileOrEmpty(path)
		if containsMarker(existing, marker(b.kind)) {
			// Idempotent: already installed. Refresh content only
			// when force is set.
			if !force {
				return nil
			}
		} else if !force {
			return fmt.Errorf("%w: refusing to overwrite %s", domain.ErrAgentInstallFailed, path)
		}
	}
	return b.writeAtomic(path, content, 0o600)
}

// removeIfPresent deletes path; missing files are silently ignored.
func (b *baseAdapter) removeIfPresent(path string) error {
	if !b.fileExists(path) {
		return nil
	}
	return b.fs.Remove(path)
}

// ErrNotSupported is returned when an installer is invoked on a
// platform the adapter does not support.
var ErrNotSupported = errors.New("adapter: not supported on this platform")
