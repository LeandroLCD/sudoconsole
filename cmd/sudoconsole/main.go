// Package main is the entry point for the sudoconsole CLI.
//
// sudoconsole is a secure sudo wrapper for CLI agents that:
//   - Prompts for the sudo password on a real TTY (no echo, no log).
//   - Caches credentials via sudo's native timestamp mechanism.
//   - Evaluates a configurable policy before executing any command.
//   - Blocks remote-access and credential-exposure commands by default.
//   - Auto-installs adapters for detected CLI agents (Kilo, Claude Code, etc.).
//
// Architecture follows Clean Architecture:
//   - internal/domain: entities and ports (no external deps)
//   - internal/usecase: use cases (depends only on domain)
//   - internal/infrastructure: adapters implementing ports (PTY, sudo, cache, etc.)
//   - internal/transport/cli: cobra commands (composition root)
//   - cmd/sudoconsole: main wiring
package main

import (
	"fmt"
	"runtime"
)

// Version info. Injected at build time via -ldflags.
var (
	version   = "0.1.0-dev"
	commit    = "none"
	buildDate = "unknown"
)

func main() {
	fmt.Printf("sudoconsole %s\n", version)
	fmt.Printf("  commit:     %s\n", commit)
	fmt.Printf("  built:      %s\n", buildDate)
	fmt.Printf("  go version: %s\n", runtime.Version())
	fmt.Printf("  os/arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Println()
	fmt.Println("This is a bootstrap build. Subcommands will be wired in M5.")
}
