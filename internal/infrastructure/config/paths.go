// Package config implements domain.ConfigStore by loading TOML
// configuration from the user's XDG (Linux) or Application Support
// (macOS) directory.
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// EnvXDGConfig is the XDG Base Directory variable. Overridable for tests.
var EnvXDGConfig = "XDG_CONFIG_HOME"

// EnvHome is the home-directory variable used as a fallback when
// XDG_CONFIG_HOME is unset. Overridable for tests.
var EnvHome = "HOME"

// AppDir is the sudoconsole subdirectory under the user's config root.
// Overridable for tests.
var AppDir = "sudoconsole"

// ConfigFile is the basename of the configuration file.
var ConfigFile = "config.toml"

// PathResolver computes the absolute path to the user's config file.
//
// The default constructor uses os.Getenv to honor XDG_CONFIG_HOME and HOME
// on all platforms. Tests may construct a PathResolver directly with
// fixed roots.
type PathResolver struct {
	// Getenv returns the value of an environment variable. It is a field
	// so tests can inject deterministic values without mutating the
	// process environment.
	Getenv func(string) string

	// GOOS is the operating system identifier ("linux", "darwin", ...).
	// It is a field so tests can pin the platform behavior without
	// depending on the actual host.
	GOOS string

	// UserHomeDir resolves $HOME. It is a field so tests can override it.
	UserHomeDir func() (string, error)
}

// NewPathResolver returns a PathResolver that reads from the current
// process environment and uses runtime.GOOS.
func NewPathResolver() PathResolver {
	return PathResolver{
		Getenv:      os.Getenv,
		GOOS:        runtime.GOOS,
		UserHomeDir: os.UserHomeDir,
	}
}

// DefaultPath returns the absolute path to the user config file.
//
// Resolution rules:
//   - macOS: $HOME/Library/Application Support/sudoconsole/config.toml
//   - Linux / other: $XDG_CONFIG_HOME/sudoconsole/config.toml, falling
//     back to $HOME/.config/sudoconsole/config.toml when XDG is unset.
func (r PathResolver) DefaultPath() string {
	root, err := r.configRoot()
	if err != nil {
		// Fall back to a relative path; downstream code will surface a
		// clearer error when the file is actually read.
		return ConfigFile
	}
	return filepath.Join(root, AppDir, ConfigFile)
}

func (r PathResolver) configRoot() (string, error) {
	getenv := r.Getenv
	if getenv == nil {
		getenv = os.Getenv
	}
	if r.GOOS == "darwin" {
		home := getenv(EnvHome)
		if home == "" && r.UserHomeDir != nil {
			h, err := r.UserHomeDir()
			if err != nil {
				return "", err
			}
			home = h
		}
		if home == "" {
			return "", os.ErrNotExist
		}
		return filepath.Join(home, "Library", "Application Support"), nil
	}
	if v := getenv(EnvXDGConfig); v != "" {
		return v, nil
	}
	home := getenv(EnvHome)
	if home == "" && r.UserHomeDir != nil {
		h, err := r.UserHomeDir()
		if err != nil {
			return "", err
		}
		home = h
	}
	if home == "" {
		return "", os.ErrNotExist
	}
	return filepath.Join(home, ".config"), nil
}
