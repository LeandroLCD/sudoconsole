package config

import (
	"strings"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// toConfig converts a parsed fileSchema into a domain.Config, filling
// any zero-valued field with the per-area default.
//
// We deliberately merge instead of relying on iszero/reflection so the
// rules are explicit and testable.
func (s fileSchema) toConfig() domain.Config {
	def := domain.DefaultConfig()
	out := domain.Config{
		Cache:    mergeCache(s.Cache, def.Cache),
		Security: mergeSecurity(s.Security, def.Security),
		Policy:   mergePolicy(s.Policy, def.Policy),
		Agent:    mergeAgent(s.Agent, def.Agent),
		Output:   mergeOutput(s.Output, def.Output),
	}
	return out
}

func mergeCache(f cacheSection, d domain.CacheConfig) domain.CacheConfig {
	out := d
	if f.TimeoutSeconds != 0 {
		out.TimeoutSeconds = f.TimeoutSeconds
	}
	if f.RefreshBeforeSeconds != 0 {
		out.RefreshBeforeSeconds = f.RefreshBeforeSeconds
	}
	out.Disabled = f.Disabled
	return out
}

func mergeSecurity(f securitySection, d domain.SecurityConfig) domain.SecurityConfig {
	return domain.SecurityConfig{
		PurgeMemory:      f.PurgeMemory || d.PurgeMemory,
		DisableCoreDumps: f.DisableCoreDumps || d.DisableCoreDumps,
		RequireTTY:       f.RequireTTY || d.RequireTTY,
	}
}

func mergeOutput(f outputSection, d domain.OutputConfig) domain.OutputConfig {
	out := d
	if f.Format != "" {
		out.Format = f.Format
	}
	if f.LogLevel != "" {
		out.LogLevel = f.LogLevel
	}
	return out
}

func mergePolicy(f policySection, d domain.Policy) domain.Policy {
	out := d
	if m, ok := parsePolicyMode(f.Mode); ok {
		out.Mode = m
	}
	out.Blocked.Categories = mergeCategories(f.Blocked.Categories, d.Blocked.Categories)
	out.Blocked.Commands = mergeStringSlice(f.Blocked.Commands, d.Blocked.Commands)
	out.Blocked.Patterns = mergeStringSlice(f.Blocked.Patterns, d.Blocked.Patterns)
	out.Allowed.Categories = mergeCategories(f.Allowed.Categories, d.Allowed.Categories)
	out.Allowed.Commands = mergeStringSlice(f.Allowed.Commands, d.Allowed.Commands)
	out.Allowed.Patterns = mergeStringSlice(f.Allowed.Patterns, d.Allowed.Patterns)
	if f.ExtraPatterns != nil {
		out.ExtraPatterns = append([]string(nil), f.ExtraPatterns...)
	}
	out.Audit = mergeAudit(f.Audit, d.Audit)
	if f.RemoteAccess != (remoteAccessSection{}) {
		out.RemoteAccess = domain.RemoteAccessConfig{
			BlockReverseTunnels: f.RemoteAccess.BlockReverseTunnels,
			BlockPortForward:    f.RemoteAccess.BlockPortForward,
			BlockShellSpawn:     f.RemoteAccess.BlockShellSpawn,
			BlockTunnels:        f.RemoteAccess.BlockTunnels,
		}
	}
	if f.CredentialExposure != (credExposureSection{}) {
		out.CredentialExposure = domain.CredentialExposureConfig{
			BlockPasswdChange: f.CredentialExposure.BlockPasswdChange,
			BlockShadowEdit:   f.CredentialExposure.BlockShadowEdit,
			BlockSudoersEdit:  f.CredentialExposure.BlockSudoersEdit,
			BlockSSHKeyExport: f.CredentialExposure.BlockSSHKeyExport,
			BlockSecretExport: f.CredentialExposure.BlockSecretExport,
		}
	}
	return out
}

func mergeAudit(f auditSection, d domain.AuditConfig) domain.AuditConfig {
	out := d
	if f.LogFile != "" {
		out.LogFile = f.LogFile
	}
	if f.LogBlocked {
		out.LogBlocked = true
	}
	if f.LogAllowed {
		out.LogAllowed = true
	}
	if f.LogWarned {
		out.LogWarned = true
	}
	if f.MaxBytes != 0 {
		out.MaxBytes = f.MaxBytes
	}
	return out
}

func mergeAgent(f agentSection, d domain.AgentConfig) domain.AgentConfig {
	out := d
	if f.AutoDetect {
		out.AutoDetect = true
	}
	if f.Install != nil {
		out.Install = make([]domain.AgentKind, 0, len(f.Install))
		for _, k := range f.Install {
			if ak, ok := parseAgentKind(k); ok {
				out.Install = append(out.Install, ak)
			}
		}
	}
	if f.BinDir != "" {
		out.BinDir = f.BinDir
	}
	return out
}

// parsePolicyMode maps a TOML string to a domain.PolicyMode. Unknown
// values are rejected earlier in validate(); here we only return false
// for the empty string (the caller wants the default).
func parsePolicyMode(s string) (domain.PolicyMode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "blocklist":
		return domain.PolicyModeBlocklist, true
	case "allowlist":
		return domain.PolicyModeAllowlist, true
	case "audit":
		return domain.PolicyModeAudit, true
	default:
		return domain.PolicyModeUnknown, false
	}
}

func parseAgentKind(s string) (domain.AgentKind, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "kilo":
		return domain.AgentKilo, true
	case "claude":
		return domain.AgentClaude, true
	case "gemini":
		return domain.AgentGemini, true
	case "aider":
		return domain.AgentAider, true
	case "codex":
		return domain.AgentCodex, true
	case "copilot":
		return domain.AgentCopilot, true
	case "generic":
		return domain.AgentGeneric, true
	default:
		return domain.AgentUnknown, false
	}
}

func mergeCategories(fromFile []string, fromDefault []domain.Category) []domain.Category {
	if fromFile == nil {
		out := make([]domain.Category, len(fromDefault))
		copy(out, fromDefault)
		return out
	}
	out := make([]domain.Category, 0, len(fromFile))
	for _, c := range fromFile {
		out = append(out, domain.Category(c))
	}
	return out
}

func mergeStringSlice(fromFile, fromDefault []string) []string {
	if fromFile == nil {
		return append([]string(nil), fromDefault...)
	}
	return append([]string(nil), fromFile...)
}
