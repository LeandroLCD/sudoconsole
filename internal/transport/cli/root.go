// Package cli provides the cobra-based command tree for sudoconsole.
//
// This package is the composition root: it is the only place that
// knows about every layer (domain, usecase, infrastructure). It must
// not be imported by domain, usecase, or infrastructure packages.
package cli

import (
	"github.com/spf13/cobra"
)

// Options are the runtime knobs shared by every subcommand. They are
// populated from CLI flags and (later) from the user's config file.
//
// The zero value is a usable default; callers may construct Options
// directly in tests.
type Options struct {
	// ConfigPath overrides the location of the TOML config file. An
	// empty string means "use the platform default".
	ConfigPath string

	// CacheTimeoutSeconds overrides domain.CacheConfig.TimeoutSeconds
	// from the CLI. Zero means "use the config file / built-in default".
	CacheTimeoutSeconds int

	// Format overrides domain.OutputConfig.Format ("human" or "json").
	Format string

	// LogLevel overrides domain.OutputConfig.LogLevel.
	LogLevel string
}

// NewRootCmd builds the top-level cobra command. It accepts a version
// string so the binary can inject its build-time version without
// creating an import cycle on package main.
func NewRootCmd(version, commit, buildDate string) *cobra.Command {
	opts := &Options{}
	cmd := &cobra.Command{
		Use:   "sudoconsole",
		Short: "Secure sudo wrapper for CLI agents",
		Long: `sudoconsole lets any CLI coding agent run sudo commands
without exposing the password, and enforces a security policy that
blocks remote-access and credential-exposure commands by default.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.PersistentFlags().StringVar(&opts.ConfigPath, "config", "",
		"path to the TOML config file (default: platform XDG/Application Support location)")
	cmd.PersistentFlags().IntVar(&opts.CacheTimeoutSeconds, "cache-timeout", 0,
		"sudo cache timeout in seconds (60-3600); overrides config file")
	cmd.PersistentFlags().StringVar(&opts.Format, "format", "",
		"output format: human or json (overrides config file)")
	cmd.PersistentFlags().StringVar(&opts.LogLevel, "log-level", "",
		"log level: silent, error, warn, info, debug (overrides config file)")

	cmd.AddCommand(newVersionCmd(version, commit, buildDate))
	cmd.AddCommand(newConfigCmd(opts))
	return cmd
}
