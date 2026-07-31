package agent

import (
	"context"
	"io/fs"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

var _ fstest.MapFS // keep the import

// memFS is an in-memory FS that satisfies the agent.FS contract and
// can be populated from a testing/fstest.MapFS.
type memFS struct {
	files map[string][]byte
	dirs  map[string]fs.FileMode
}

func newMemFS() *memFS {
	return &memFS{
		files: map[string][]byte{},
		dirs:  map[string]fs.FileMode{},
	}
}

func (m *memFS) populate(seed fstest.MapFS) {
	for k, v := range seed {
		if v.Mode&fs.ModeDir != 0 {
			m.dirs[k] = v.Mode
			continue
		}
		m.files[k] = v.Data
	}
}

var _ = (*memFS).populate // keep the helper for future tests

func (m *memFS) Stat(name string) (fs.FileInfo, error) {
	if mode, ok := m.dirs[name]; ok {
		return fakeInfo{name: filepath.Base(name), mode: mode}, nil
	}
	if data, ok := m.files[name]; ok {
		return fakeInfo{name: filepath.Base(name), size: int64(len(data))}, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (m *memFS) ReadFile(name string) ([]byte, error) {
	if data, ok := m.files[name]; ok {
		return append([]byte(nil), data...), nil
	}
	return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
}

func (m *memFS) WriteFile(name string, data []byte, _ fs.FileMode) error {
	m.files[name] = append([]byte(nil), data...)
	m.dirs[filepath.Dir(name)] = fs.ModeDir | 0o700
	return nil
}

func (m *memFS) MkdirAll(path string, perm fs.FileMode) error {
	for p := path; p != "" && p != "." && p != "/"; p = filepath.Dir(p) {
		m.dirs[p] = fs.ModeDir | perm
	}
	return nil
}

func (m *memFS) Remove(name string) error {
	delete(m.files, name)
	delete(m.dirs, name)
	return nil
}

func (m *memFS) Rename(oldp, newp string) error {
	if data, ok := m.files[oldp]; ok {
		m.files[newp] = data
		delete(m.files, oldp)
		return nil
	}
	return &fs.PathError{Op: "rename", Path: oldp, Err: fs.ErrNotExist}
}

func (m *memFS) get(name string) string { return string(m.files[name]) }
func (m *memFS) exists(name string) bool {
	_, ok := m.files[name]
	return ok
}

type fakeInfo struct {
	name string
	size int64
	mode fs.FileMode
}

func (f fakeInfo) Name() string       { return f.name }
func (f fakeInfo) Size() int64        { return f.size }
func (f fakeInfo) Mode() fs.FileMode  { return f.mode }
func (f fakeInfo) ModTime() time.Time { return time.Unix(0, 0) }
func (f fakeInfo) IsDir() bool        { return f.mode&fs.ModeDir != 0 }
func (f fakeInfo) Sys() any           { return nil }

// installInto wires a baseAdapter against the in-memory fs and a
// synthetic $HOME.
func installInto(t *testing.T, fsys *memFS, home string, kind domain.AgentKind, cfg domain.Config) domain.AgentInstaller {
	t.Helper()
	a, err := newForKind(kind, cfg)
	if err != nil {
		t.Fatal(err)
	}
	injectBase(a, baseAdapter{kind: kind, fs: fsys, home: home, config: cfg})
	return a
}

// injectBase mutates the unexported base field of an installer by
// switching on its concrete type.
func injectBase(a domain.AgentInstaller, b baseAdapter) {
	switch v := a.(type) {
	case *KiloAdapter:
		v.base = b
	case *ClaudeAdapter:
		v.base = b
	case *GeminiAdapter:
		v.base = b
	case *AiderAdapter:
		v.base = b
	case *CodexAdapter:
		v.base = b
	case *GenericAdapter:
		v.base = b
	}
}

// contract runs the canonical install/idempotent/force/uninstall
// sequence against the supplied adapter and verifies the
// post-conditions on the memFS.
func contract(t *testing.T, fsys *memFS, home string, kind domain.AgentKind, cfg domain.Config, target string) domain.AgentInstaller {
	t.Helper()
	a := installInto(t, fsys, home, kind, cfg)

	// 1. Install.
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatalf("Install: %v", err)
	}
	if !fsys.exists(target) {
		t.Fatalf("Install did not create %s", target)
	}

	// 2. Idempotent.
	if err := a.Install(context.Background(), domain.InstallOptions{}); err != nil {
		t.Fatalf("idempotent Install: %v", err)
	}
	if !fsys.exists(target) {
		t.Fatalf("idempotent Install removed %s", target)
	}

	// 3. Uninstall.
	if err := a.Uninstall(context.Background()); err != nil {
		t.Fatalf("Uninstall: %v", err)
	}
	if fsys.exists(target) {
		t.Fatalf("Uninstall did not remove %s", target)
	}

	// 4. Idempotent uninstall.
	if err := a.Uninstall(context.Background()); err != nil {
		t.Fatalf("idempotent Uninstall: %v", err)
	}
	return a
}
