package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// CheckInput configures CheckUseCase.
type CheckInput struct {
	Config domain.Config
}

// CheckOutput is the outcome of a cache inspection.
type CheckOutput struct {
	Status    domain.CacheStatus
	Remaining time.Duration
}

// CheckUseCase reports whether the sudo cache is currently active and
// how long it has left before expiry.
type CheckUseCase struct {
	Repository domain.CacheRepository
}

// NewCheckUseCase constructs the use case.
func NewCheckUseCase(repo domain.CacheRepository) *CheckUseCase {
	return &CheckUseCase{Repository: repo}
}

// Execute reads the cache timestamp and returns its status.
func (u *CheckUseCase) Execute(ctx context.Context, in CheckInput) (CheckOutput, error) {
	if u.Repository == nil {
		return CheckOutput{}, fmt.Errorf("check: repository not configured")
	}
	if in.Config.Cache.Disabled {
		return CheckOutput{Status: domain.CacheExpired}, nil
	}
	status, err := u.Repository.IsActive(ctx, in.Config.Cache)
	if err != nil {
		return CheckOutput{}, fmt.Errorf("check: %w", err)
	}
	remaining, err := u.Repository.TimeRemaining(ctx, in.Config.Cache)
	if err != nil {
		// Remaining is informational; do not fail the whole command.
		remaining = 0
	}
	return CheckOutput{Status: status, Remaining: remaining}, nil
}

// IsActive is a convenience for callers that only need a boolean.
func (u *CheckUseCase) IsActive(ctx context.Context, in CheckInput) bool {
	out, err := u.Execute(ctx, in)
	if err != nil {
		return false
	}
	return out.Status == domain.CacheActive
}
