package config

import (
	"fmt"
	"regexp"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// validationError carries a field path and a human message.
//
// We attach the field name so the CLI can print e.g.
// `config: policy.mode: invalid value "blockklist"`. The underlying
// sentinel is domain.ErrConfigInvalid so callers can errors.Is it.
type validationError struct {
	Field   string
	Message string
}

func (e *validationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

func (e *validationError) Unwrap() error { return domain.ErrConfigInvalid }

// validate checks a fully-decoded fileSchema. It returns the first error
// found. Each branch is independent so a fix-and-retry workflow is
// natural: correct one field, re-run, see the next.
func validate(s fileSchema) error {
	if err := validateCache(s.Cache); err != nil {
		return err
	}
	if err := validateOutput(s.Output); err != nil {
		return err
	}
	if err := validatePolicy(s.Policy); err != nil {
		return err
	}
	if err := validateAgent(s.Agent); err != nil {
		return err
	}
	return nil
}

func validateCache(c cacheSection) error {
	if c.Disabled {
		return nil
	}
	if c.TimeoutSeconds < 60 || c.TimeoutSeconds > 3600 {
		return &validationError{
			Field:   "cache.timeout_seconds",
			Message: fmt.Sprintf("must be in [60, 3600], got %d", c.TimeoutSeconds),
		}
	}
	if c.RefreshBeforeSeconds < 0 || c.RefreshBeforeSeconds >= c.TimeoutSeconds {
		return &validationError{
			Field:   "cache.refresh_before_seconds",
			Message: fmt.Sprintf("must be in [0, %d), got %d", c.TimeoutSeconds, c.RefreshBeforeSeconds),
		}
	}
	return nil
}

func validateOutput(o outputSection) error {
	switch o.Format {
	case "", "human", "json":
	default:
		return &validationError{
			Field:   "output.format",
			Message: fmt.Sprintf("must be one of human, json; got %q", o.Format),
		}
	}
	switch o.LogLevel {
	case "", "silent", "error", "warn", "info", "debug":
	default:
		return &validationError{
			Field:   "output.log_level",
			Message: fmt.Sprintf("must be one of silent, error, warn, info, debug; got %q", o.LogLevel),
		}
	}
	return nil
}

func validatePolicy(p policySection) error {
	switch p.Mode {
	case "", "blocklist", "allowlist", "audit":
	default:
		return &validationError{
			Field:   "policy.mode",
			Message: fmt.Sprintf("must be one of blocklist, allowlist, audit; got %q", p.Mode),
		}
	}
	known := map[string]struct{}{
		"package_manager":     {},
		"service_control":     {},
		"filesystem":          {},
		"network_config":      {},
		"user_management":     {},
		"remote_access":       {},
		"credential_exposure": {},
		"shell_spawn":         {},
		"persistence":         {},
	}
	for _, c := range p.Blocked.Categories {
		if _, ok := known[c]; !ok {
			return &validationError{
				Field:   "policy.blocked.categories",
				Message: fmt.Sprintf("unknown category %q", c),
			}
		}
	}
	for _, c := range p.Allowed.Categories {
		if _, ok := known[c]; !ok {
			return &validationError{
				Field:   "policy.allowed.categories",
				Message: fmt.Sprintf("unknown category %q", c),
			}
		}
	}
	for i, pat := range p.ExtraPatterns {
		if _, err := regexp.Compile(pat); err != nil {
			return &validationError{
				Field:   "policy.extra_patterns",
				Message: fmt.Sprintf("entry %d: %v", i, err),
			}
		}
	}
	for i, pat := range p.Blocked.Patterns {
		if _, err := regexp.Compile(pat); err != nil {
			return &validationError{
				Field:   "policy.blocked.patterns",
				Message: fmt.Sprintf("entry %d: %v", i, err),
			}
		}
	}
	for i, pat := range p.Allowed.Patterns {
		if _, err := regexp.Compile(pat); err != nil {
			return &validationError{
				Field:   "policy.allowed.patterns",
				Message: fmt.Sprintf("entry %d: %v", i, err),
			}
		}
	}
	if p.Audit.MaxBytes < 0 {
		return &validationError{
			Field:   "policy.audit.max_bytes",
			Message: fmt.Sprintf("must be >= 0, got %d", p.Audit.MaxBytes),
		}
	}
	return nil
}

func validateAgent(a agentSection) error {
	known := map[string]struct{}{
		"kilo": {}, "claude": {}, "gemini": {}, "aider": {},
		"codex": {}, "copilot": {}, "generic": {},
	}
	for _, k := range a.Install {
		if _, ok := known[k]; !ok {
			return &validationError{
				Field:   "agent.install",
				Message: fmt.Sprintf("unknown agent kind %q", k),
			}
		}
	}
	return nil
}
