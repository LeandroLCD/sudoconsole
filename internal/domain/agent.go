package domain

import (
	"path/filepath"
	"strings"
)

// AgentKind is the supported CLI agent type.
type AgentKind int

const (
	// AgentUnknown is the zero value.
	AgentUnknown AgentKind = iota
	// AgentKilo: Kilo CLI (github.com/Kilo-Org/kilo).
	AgentKilo
	// AgentClaude: Claude Code (claude.ai/code).
	AgentClaude
	// AgentGemini: Google Gemini CLI.
	AgentGemini
	// AgentAider: Aider (aider.chat).
	AgentAider
	// AgentCodex: OpenAI Codex CLI.
	AgentCodex
	// AgentCopilot: GitHub Copilot CLI (gh copilot).
	AgentCopilot
	// AgentGeneric: any shell (alias fallback).
	AgentGeneric
)

// String returns the canonical lower-case identifier.
func (k AgentKind) String() string {
	switch k {
	case AgentKilo:
		return "kilo"
	case AgentClaude:
		return "claude"
	case AgentGemini:
		return "gemini"
	case AgentAider:
		return "aider"
	case AgentCodex:
		return "codex"
	case AgentCopilot:
		return "copilot"
	case AgentGeneric:
		return "generic"
	default:
		return "unknown"
	}
}

// DisplayName returns the human-readable name.
func (k AgentKind) DisplayName() string {
	switch k {
	case AgentKilo:
		return "Kilo CLI"
	case AgentClaude:
		return "Claude Code"
	case AgentGemini:
		return "Gemini CLI"
	case AgentAider:
		return "Aider"
	case AgentCodex:
		return "OpenAI Codex"
	case AgentCopilot:
		return "GitHub Copilot"
	case AgentGeneric:
		return "Generic shell"
	default:
		return "Unknown"
	}
}

// AgentDescriptor describes a detected CLI agent on the host.
type AgentDescriptor struct {
	// Kind identifies the agent.
	Kind AgentKind
	// BinaryPath is the resolved absolute path to the agent executable.
	BinaryPath string
	// Version is the agent's self-reported version (may be empty).
	Version string
	// ConfigDir is the directory where the agent stores its configuration
	// (slash commands, aliases, etc.). Used by AgentInstaller.
	ConfigDir string
	// Available reports whether the agent is functional on this host.
	Available bool
}

// NewAgentDescriptor constructs an AgentDescriptor and normalizes paths.
//
// Empty paths are kept empty (filepath.Clean("") returns "." which is not
// useful for our purposes).
func NewAgentDescriptor(kind AgentKind, binary, config string) AgentDescriptor {
	bp := binary
	if bp != "" {
		bp = filepath.Clean(bp)
	}
	cd := config
	if cd != "" {
		cd = filepath.Clean(cd)
	}
	return AgentDescriptor{
		Kind:       kind,
		BinaryPath: bp,
		ConfigDir:  cd,
		Available:  bp != "" && cd != "",
	}
}

// AdapterCommand returns the command the agent should invoke to use
// sudoconsole (e.g. "sudoconsole auth && sudo ...").
//
// Implementations may override this via AgentInstaller.AdapterCommand.
func (a AgentDescriptor) AdapterCommand() string {
	return "sudoconsole"
}

// HasConfigDir reports whether the agent has a known config directory on
// this host.
func (a AgentDescriptor) HasConfigDir() bool {
	return strings.TrimSpace(a.ConfigDir) != ""
}

// IsZero reports whether the descriptor is the zero value (Kind unknown).
func (a AgentDescriptor) IsZero() bool {
	return a.Kind == AgentUnknown
}
