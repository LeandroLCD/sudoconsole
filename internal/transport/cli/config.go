package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
	"github.com/LeandroLCD/sudoconsole/internal/infrastructure/config"
)

// newConfigCmd builds `sudoconsole config` with the following
// subcommands:
//   - show    : print the effective merged configuration
//   - path    : print the resolved default config file path
//   - validate: lint the config file and report errors
func newConfigCmd(opts *Options) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Inspect and validate the sudoconsole configuration",
	}
	cmd.AddCommand(newConfigShowCmd(opts))
	cmd.AddCommand(newConfigPathCmd(opts))
	cmd.AddCommand(newConfigValidateCmd(opts))
	return cmd
}

func newConfigPathCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "path",
		Short: "Print the default config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store := newStore(opts)
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), store.DefaultPath())
			return nil
		},
	}
}

func newConfigShowCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the effective configuration (defaults + file + flags)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadEffectiveConfig(cmd.Context(), opts)
			if err != nil {
				return err
			}
			return printConfig(cmd.OutOrStdout(), cfg)
		},
	}
}

func newConfigValidateCmd(opts *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the config file (or --config path) and exit non-zero on error",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := loadEffectiveConfig(cmd.Context(), opts)
			if err != nil {
				return err
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ok: %s\n", opts.ConfigPath)
			_ = cfg
			return nil
		},
	}
}

// newStore returns a config.Store bound to opts.ConfigPath (or the
// platform default when empty).
func newStore(opts *Options) *config.Store {
	if opts.ConfigPath != "" {
		abs, err := filepath.Abs(opts.ConfigPath)
		if err == nil {
			return config.NewAt(abs)
		}
		return config.NewAt(opts.ConfigPath)
	}
	return config.New()
}

// loadEffectiveConfig loads the config, applies CLI flag overrides
// and returns the result. The returned config is safe to use even if
// the user file is missing: defaults are merged in.
func loadEffectiveConfig(ctx context.Context, opts *Options) (domain.Config, error) {
	store := newStore(opts)
	cfg, err := store.Load(ctx)
	if err != nil {
		// Surface a clear error including the path so users know which
		// file failed to parse.
		if !errors.Is(err, domain.ErrConfigNotFound) {
			return domain.Config{}, fmt.Errorf("config: %w", err)
		}
		return domain.Config{}, fmt.Errorf("config: %w", err)
	}
	applyOverrides(&cfg, opts)
	if err := cfg.Cache.Validate(); err != nil {
		return domain.Config{}, fmt.Errorf("config cache: %w", err)
	}
	return cfg, nil
}

// applyOverrides layers the CLI flag values on top of the loaded
// configuration. A flag value of "" or 0 is treated as "not set" and
// leaves the underlying value untouched.
func applyOverrides(cfg *domain.Config, opts *Options) {
	if opts.CacheTimeoutSeconds > 0 {
		cfg.Cache.TimeoutSeconds = opts.CacheTimeoutSeconds
	}
	if opts.Format != "" {
		cfg.Output.Format = opts.Format
	}
	if opts.LogLevel != "" {
		cfg.Output.LogLevel = opts.LogLevel
	}
}

// printConfig writes a human-readable summary of the effective config.
//
// Errors from Fprintf/Fprintln against the output writer are ignored:
// the writer is owned by cobra (typically os.Stdout) and a write
// failure at this stage means the user pipe is gone, which the OS
// will report on the next syscall anyway.
func printConfig(w io.Writer, cfg domain.Config) error {
	bw := bufio.NewWriter(w)
	defer func() { _ = bw.Flush() }()
	_, _ = fmt.Fprintln(bw, "[cache]")
	_, _ = fmt.Fprintf(bw, "  timeout_seconds        = %d\n", cfg.Cache.TimeoutSeconds)
	_, _ = fmt.Fprintf(bw, "  refresh_before_seconds = %d\n", cfg.Cache.RefreshBeforeSeconds)
	_, _ = fmt.Fprintf(bw, "  disabled               = %t\n", cfg.Cache.Disabled)
	_, _ = fmt.Fprintln(bw, "[security]")
	_, _ = fmt.Fprintf(bw, "  purge_memory       = %t\n", cfg.Security.PurgeMemory)
	_, _ = fmt.Fprintf(bw, "  disable_core_dumps = %t\n", cfg.Security.DisableCoreDumps)
	_, _ = fmt.Fprintf(bw, "  require_tty        = %t\n", cfg.Security.RequireTTY)
	_, _ = fmt.Fprintln(bw, "[policy]")
	_, _ = fmt.Fprintf(bw, "  mode = %s\n", cfg.Policy.Mode)
	_, _ = fmt.Fprintf(bw, "  blocked.categories = %v\n", cfg.Policy.Blocked.Categories)
	_, _ = fmt.Fprintf(bw, "  allowed.categories = %v\n", cfg.Policy.Allowed.Categories)
	_, _ = fmt.Fprintf(bw, "  audit.log_blocked  = %t\n", cfg.Policy.Audit.LogBlocked)
	_, _ = fmt.Fprintln(bw, "[agent]")
	_, _ = fmt.Fprintf(bw, "  auto_detect = %t\n", cfg.Agent.AutoDetect)
	_, _ = fmt.Fprintf(bw, "  bin_dir     = %s\n", cfg.Agent.BinDir)
	_, _ = fmt.Fprintln(bw, "[output]")
	_, _ = fmt.Fprintf(bw, "  format    = %s\n", cfg.Output.Format)
	_, _ = fmt.Fprintf(bw, "  log_level = %s\n", cfg.Output.LogLevel)
	return nil
}
