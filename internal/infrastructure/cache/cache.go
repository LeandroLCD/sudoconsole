// Package cache implements domain.CacheRepository by reading the
// platform's sudo timestamp file.
package cache

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

// Repository is a domain.CacheRepository backed by the host's sudo
// timestamp file.
type Repository struct {
	username string
}

// NewRepository returns a Repository for the given username. If username
// is empty, it falls back to $USER / $LOGNAME.
func NewRepository(username string) *Repository {
	if username == "" {
		username = currentUsername()
	}
	return &Repository{username: username}
}

// IsActive reports whether the cache is currently valid.
func (r *Repository) IsActive(_ context.Context, cfg domain.CacheConfig) (domain.CacheStatus, error) {
	if cfg.Disabled {
		return domain.CacheExpired, nil
	}
	if r == nil || r.username == "" {
		return domain.CacheExpired, errors.New("cache: no username configured")
	}
	mtime, err := readTimestamp(r.username)
	if err != nil {
		return domain.CacheExpired, fmt.Errorf("read timestamp: %w", err)
	}
	if mtime == 0 {
		return domain.CacheExpired, nil
	}
	age := time.Since(time.Unix(mtime, 0)).Seconds()
	if cfg.IsActive(age) {
		return domain.CacheActive, nil
	}
	return domain.CacheExpired, nil
}

// TimeRemaining returns how long the cache is still valid.
func (r *Repository) TimeRemaining(ctx context.Context, cfg domain.CacheConfig) (time.Duration, error) {
	mtime, err := readTimestamp(r.username)
	if err != nil {
		return 0, err
	}
	if mtime == 0 {
		return 0, nil
	}
	age := time.Since(time.Unix(mtime, 0))
	remaining := cfg.EffectiveTimeout() - age
	if remaining < 0 {
		remaining = 0
	}
	return remaining, nil
}

// Refresh touches the timestamp file to extend the cache. This is a
// fallback used only when sudo -v is unavailable; the normal flow
// invokes `sudo -v` via SudoGateway.Execute.
func (r *Repository) Refresh(_ context.Context, cfg domain.CacheConfig) error {
	if cfg.Disabled {
		return domain.ErrCacheMiss
	}
	if r == nil || r.username == "" {
		return domain.ErrCacheMiss
	}
	// Touch the file (works only if it already exists).
	if _, err := os.Stat(timestampPath(r.username)); err != nil {
		return domain.ErrCacheMiss
	}
	if err := writeTimestampRefreshed(r.username); err != nil {
		return fmt.Errorf("refresh timestamp: %w", err)
	}
	return nil
}

// timeNow is a package-level variable so tests can override it.
var timeNow = func() time.Time { return time.Now() }
