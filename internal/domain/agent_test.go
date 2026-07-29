package domain

import (
	"path/filepath"
	"testing"
)

func TestAgentKind_String(t *testing.T) {
	for k, want := range map[AgentKind]string{
		AgentKilo:     "kilo",
		AgentClaude:   "claude",
		AgentGemini:   "gemini",
		AgentAider:    "aider",
		AgentCodex:    "codex",
		AgentCopilot:  "copilot",
		AgentGeneric:  "generic",
		AgentUnknown:  "unknown",
		AgentKind(99): "unknown",
	} {
		if got := k.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", k, got, want)
		}
	}
}

func TestAgentKind_DisplayName(t *testing.T) {
	for k, want := range map[AgentKind]string{
		AgentKilo:    "Kilo CLI",
		AgentClaude:  "Claude Code",
		AgentGemini:  "Gemini CLI",
		AgentAider:   "Aider",
		AgentCodex:   "OpenAI Codex",
		AgentCopilot: "GitHub Copilot",
		AgentGeneric: "Generic shell",
		AgentUnknown: "Unknown",
	} {
		if got := k.DisplayName(); got != want {
			t.Errorf("%d.DisplayName() = %q, want %q", k, got, want)
		}
	}
}

func TestNewAgentDescriptor(t *testing.T) {
	a := NewAgentDescriptor(AgentKilo, "/usr/local/bin/kilo", "/home/u/.config/kilo")
	if a.Kind != AgentKilo {
		t.Fatal("Kind wrong")
	}
	if filepath.Base(a.BinaryPath) != "kilo" {
		t.Fatalf("BinaryPath = %q", a.BinaryPath)
	}
	if !a.Available {
		t.Fatal("Available should be true when both paths set")
	}
	if !a.HasConfigDir() {
		t.Fatal("HasConfigDir should be true")
	}
	if a.IsZero() {
		t.Fatal("IsZero should be false for known kind")
	}
}

func TestNewAgentDescriptor_Empty(t *testing.T) {
	a := NewAgentDescriptor(AgentUnknown, "", "")
	if a.Available {
		t.Fatal("Available should be false when both paths empty")
	}
	if a.HasConfigDir() {
		t.Fatal("HasConfigDir should be false")
	}
	if !a.IsZero() {
		t.Fatal("IsZero should be true for AgentUnknown")
	}
}

func TestAgentDescriptor_AdapterCommand(t *testing.T) {
	a := AgentDescriptor{}
	if a.AdapterCommand() != "sudoconsole" {
		t.Fatalf("AdapterCommand = %q", a.AdapterCommand())
	}
}
