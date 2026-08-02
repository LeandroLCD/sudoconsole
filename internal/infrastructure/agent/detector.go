// Package agent implements domain.AgentDetector by scanning the host
// for installed CLI agents. Detection is parallelised with a hard
// 2-second budget so a slow PATH lookup never blocks the UI.
package agent

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// Spec describes how to detect a single CLI agent.
type Spec struct {
	Kind        domain.AgentKind
	BinaryName  string // basename to look up in PATH
	ConfigDir   string // path relative to $HOME; empty if N/A
	VersionFlag string // flag passed to the binary to get its version; "" if not supported
	ConfigFile  string // optional marker file inside ConfigDir (e.g. ".installed")
}

// Specs is the canonical table of supported CLI agents. Order is
// preserved by Detect so output is deterministic.
var Specs = []Spec{
	{Kind: domain.AgentKilo, BinaryName: "kilo", ConfigDir: ".config/kilo", VersionFlag: "--version"},
	{Kind: domain.AgentClaude, BinaryName: "claude", ConfigDir: ".claude", VersionFlag: "--version"},
	{Kind: domain.AgentGemini, BinaryName: "gemini", ConfigDir: ".config/gemini", VersionFlag: "--version"},
	{Kind: domain.AgentAider, BinaryName: "aider", ConfigDir: ".aider", VersionFlag: "--version"},
	{Kind: domain.AgentCodex, BinaryName: "codex", ConfigDir: ".codex", VersionFlag: "--version"},
	{Kind: domain.AgentCopilot, BinaryName: "gh", ConfigDir: ".config/gh", VersionFlag: "copilot -- --version"},
	{Kind: domain.AgentGeneric, BinaryName: "", ConfigDir: ""}, // always reported as "not detected"
}

// Detector implements domain.AgentDetector.
type Detector struct {
	// LookupEnv returns $PATH. Injected for tests.
	LookupEnv func(string) string
	// UserHomeDir resolves the user's home directory.
	UserHomeDir func() (string, error)
	// FS is the filesystem used for stat operations. Defaults to osFS.
	FS fs.FS
	// ExecutableLookup resolves a binary name to an absolute path.
	// Defaults to exec.LookPath but accepts injection for tests.
	ExecutableLookup func(name string) (string, error)
	// Timeout is the maximum time Detect may spend scanning. Default 2s.
	Timeout time.Duration
	// PathLayout overrides $PATH for ExecutableLookup. Used by tests.
	PathLayout func() []string
}

// NewDetector returns a Detector wired to the real OS environment.
func NewDetector() *Detector {
	return &Detector{
		LookupEnv:        os.Getenv,
		UserHomeDir:      os.UserHomeDir,
		FS:               os.DirFS("/"),
		ExecutableLookup: exec.LookPath,
		Timeout:          2 * time.Second,
	}
}

// Detect scans every known agent in parallel and returns the matching
// descriptors. The result is sorted by spec order. A single failing
// probe does not abort the scan; the result simply omits that kind.
func (d *Detector) Detect(ctx context.Context) ([]domain.AgentDescriptor, error) {
	if d.Timeout <= 0 {
		d.Timeout = 2 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, d.Timeout)
	defer cancel()

	home, _ := d.UserHomeDir()
	results := make([]domain.AgentDescriptor, len(Specs))
	var mu sync.Mutex
	g, gctx := errgroup.WithContext(ctx)
	for i, spec := range Specs {
		i, spec := i, spec
		g.Go(func() error {
			if gctx.Err() != nil {
				return gctx.Err()
			}
			desc, ok := d.detectOne(gctx, spec, home)
			if !ok {
				return nil
			}
			mu.Lock()
			results[i] = desc
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return nil, fmt.Errorf("detect: %w", err)
	}
	out := make([]domain.AgentDescriptor, 0, len(results))
	for _, r := range results {
		if !r.IsZero() {
			out = append(out, r)
		}
	}
	return out, nil
}

// DetectOne returns the descriptor for the requested kind, or
// domain.ErrAgentNotDetected.
func (d *Detector) DetectOne(ctx context.Context, kind domain.AgentKind) (domain.AgentDescriptor, error) {
	for _, s := range Specs {
		if s.Kind != kind {
			continue
		}
		home, _ := d.UserHomeDir()
		desc, ok := d.detectOne(ctx, s, home)
		if !ok {
			return domain.AgentDescriptor{}, domain.ErrAgentNotDetected
		}
		return desc, nil
	}
	return domain.AgentDescriptor{}, fmt.Errorf("%w: unknown kind %v", domain.ErrAgentNotDetected, kind)
}

// detectOne runs the per-agent probes. Returns ok=false when the
// agent is not installed.
func (d *Detector) detectOne(ctx context.Context, spec Spec, home string) (domain.AgentDescriptor, bool) {
	if spec.Kind == domain.AgentGeneric {
		// Generic is the fallback that the installer can target; the
		// detector never reports it as "found".
		return domain.AgentDescriptor{}, false
	}
	if ctx.Err() != nil {
		return domain.AgentDescriptor{}, false
	}

	binaryPath, hasBinary := d.lookupBinary(spec)
	configDir, hasConfig := d.lookupConfigDir(spec, home)

	// Agent is "detected" if either the binary is on PATH or the
	// expected config dir is present. This handles the case where an
	// agent was installed via npm/pip but is on a non-standard PATH.
	if !hasBinary && !hasConfig {
		return domain.AgentDescriptor{}, false
	}

	version := ""
	if hasBinary && spec.VersionFlag != "" {
		version = d.lookupVersion(ctx, binaryPath, spec.VersionFlag)
	}

	desc := domain.NewAgentDescriptor(spec.Kind, binaryPath, configDir)
	desc.Version = version
	desc.Available = desc.HasConfigDir() || binaryPath != ""
	return desc, true
}

// lookupBinary returns the absolute path of spec.BinaryName in PATH
// and whether it was found. The injected PathLayout is preferred over
// $PATH so tests can run without mutating the real environment.
func (d *Detector) lookupBinary(spec Spec) (string, bool) {
	if spec.BinaryName == "" {
		return "", false
	}
	lookup := d.ExecutableLookup
	if lookup == nil {
		lookup = exec.LookPath
	}
	// Allow tests to inject a fixed PATH layout.
	if d.PathLayout != nil {
		for _, dir := range d.PathLayout() {
			candidate := filepath.Join(dir, spec.BinaryName)
			if isExecutable(candidate) {
				return candidate, true
			}
		}
		return "", false
	}
	path, err := lookup(spec.BinaryName)
	if err != nil {
		return "", false
	}
	return path, true
}

// lookupConfigDir probes the spec's config dir relative to $HOME.
// Returns the absolute path and whether the directory exists.
func (d *Detector) lookupConfigDir(spec Spec, home string) (string, bool) {
	if spec.ConfigDir == "" || home == "" {
		return "", false
	}
	dir := filepath.Join(home, spec.ConfigDir)
	info, err := fsStat(d.FS, dir)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return dir, true
}

// fsStat wraps fs.Stat with a clean absolute-path prefix so callers
// using testing/fstest.MapFS can populate keys with full paths.
func fsStat(fsys fs.FS, path string) (os.FileInfo, error) {
	cleaned := strings.TrimPrefix(path, "/")
	return fs.Stat(fsys, cleaned)
}

// lookupVersion runs the binary with the version flag, capping the
// timeout at 500 ms per agent. Returns "" on any failure.
func (d *Detector) lookupVersion(ctx context.Context, binaryPath, versionFlag string) string {
	if versionFlag == "" {
		return ""
	}
	// Split multi-word flags like "copilot -- --version".
	args := strings.Fields(versionFlag)
	ctx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, binaryPath, args...) // #nosec G204 -- binaryPath was resolved by exec.LookPath from a fixed allow-list (Spec.BinaryName).
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	v := strings.TrimSpace(string(out))
	// Strip the binary name prefix some agents print ("kilo 0.1.0").
	if i := strings.IndexAny(v, " \t"); i > 0 {
		return v[i+1:]
	}
	return v
}

// installedAgentsMarker is the path (relative to home) of the file
// sudoconsole writes after installing an adapter. The detector treats
// its presence as evidence that the agent has been integrated before.
//
// Exported so the installer (M8) can share the same convention.
const InstalledAgentsMarker = ".config/sudoconsole/installed-agents.toml"

// isExecutable returns true if path exists and has the executable bit
// set. Used by the in-memory PATH probe in tests.
func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		return true
	}
	return info.Mode()&0o111 != 0
}
