package cli

import (
	"context"
	"errors"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// SetDefaultPrompt wires the default PTY-based password reader used by
// the `auth` command. It is called from main.go during composition.
func SetDefaultPrompt(fn func(ctx context.Context, prompt string) ([]byte, error)) {
	if fn == nil {
		return
	}
	defaultPrompt = fn
}

// ExitCode maps a use-case error to the documented exit codes:
//
//	0  OK
//	1  cache miss / runtime failure
//	2  authentication failed
//	64 policy blocked
//	65 policy warn
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	switch {
	case isPolicyBlock(err):
		return 64
	case isPolicyWarn(err):
		return 65
	}
	return exitCode(err)
}

func isPolicyBlock(err error) bool {
	var pve *domain.PolicyViolationError
	if errors.As(err, &pve) {
		return pve.Result.Decision == domain.DecisionBlock
	}
	return false
}

func isPolicyWarn(err error) bool {
	var pve *domain.PolicyViolationError
	if errors.As(err, &pve) {
		return pve.Result.Decision == domain.DecisionWarn
	}
	return false
}
