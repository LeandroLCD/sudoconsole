package agent

import (
	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// All returns one installer per supported kind, in the order they
// appear in the Specs table.
func All(cfg domain.Config) ([]domain.AgentInstaller, error) {
	out := make([]domain.AgentInstaller, 0, len(Specs))
	for _, s := range Specs {
		if s.Kind == domain.AgentGeneric {
			// Generic is opt-in only.
			continue
		}
		a, err := newForKind(s.Kind, cfg)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

// newForKind returns the installer that matches the supplied kind.
func newForKind(kind domain.AgentKind, cfg domain.Config) (domain.AgentInstaller, error) {
	return NewForKind(kind, cfg)
}

// NewForKind is the exported alias of newForKind so callers outside
// the package (e.g. the CLI install command) can construct a single
// installer by kind without going through the full registry.
func NewForKind(kind domain.AgentKind, cfg domain.Config) (domain.AgentInstaller, error) {
	switch kind {
	case domain.AgentKilo:
		return NewKiloAdapter(cfg)
	case domain.AgentClaude:
		return NewClaudeAdapter(cfg)
	case domain.AgentGemini:
		return NewGeminiAdapter(cfg)
	case domain.AgentAider:
		return NewAiderAdapter(cfg)
	case domain.AgentCodex:
		return NewCodexAdapter(cfg)
	case domain.AgentCopilot:
		return NewCopilotAdapter(cfg)
	case domain.AgentGeneric:
		return NewGenericAdapter(cfg)
	default:
		return nil, domain.ErrAgentNotDetected
	}
}
