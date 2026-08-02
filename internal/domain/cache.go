package domain

import "time"

// CacheStatus describes whether sudo credentials are currently cached.
type CacheStatus int

const (
	// CacheUnknown is the zero value. CacheRepository implementations must
	// return either CacheActive or CacheExpired; CacheUnknown is only valid
	// before any check has been performed.
	CacheUnknown CacheStatus = iota
	// CacheActive means credentials are valid and may be used without prompt.
	CacheActive
	// CacheExpired means credentials are no longer valid; a prompt is required.
	CacheExpired
)

// String returns the human-readable name of the status.
func (s CacheStatus) String() string {
	switch s {
	case CacheActive:
		return "active"
	case CacheExpired:
		return "expired"
	default:
		return "unknown"
	}
}

// CacheConfig holds the tunable parameters of the credentials cache.
//
// All durations are absolute; CacheRepository implementations compute the
// effective expiry from a timestamp file (or equivalent) at the OS level.
type CacheConfig struct {
	// TimeoutSeconds is the total lifetime of the cache after a successful
	// authentication. Default: 900 (15 min, matches sudo's default).
	//
	// Valid range: [60, 3600]. Values outside this range are rejected by
	// the config validator.
	TimeoutSeconds int

	// RefreshBeforeSeconds is the window before TimeoutSeconds at which the
	// cache is proactively refreshed silently (no prompt). Default: 120.
	//
	// Must be < TimeoutSeconds.
	RefreshBeforeSeconds int

	// Disabled disables the cache entirely; every command requires a fresh
	// authentication. Default: false.
	Disabled bool
}

// DefaultCacheConfig returns a safe default configuration.
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		TimeoutSeconds:       900,
		RefreshBeforeSeconds: 120,
		Disabled:             false,
	}
}

// EffectiveTimeout returns the absolute duration of the cache.
func (c CacheConfig) EffectiveTimeout() time.Duration {
	if c.TimeoutSeconds <= 0 {
		return 0
	}
	return time.Duration(c.TimeoutSeconds) * time.Second
}

// EffectiveRefreshBefore returns the duration before expiry to refresh.
func (c CacheConfig) EffectiveRefreshBefore() time.Duration {
	if c.RefreshBeforeSeconds <= 0 {
		return 0
	}
	return time.Duration(c.RefreshBeforeSeconds) * time.Second
}

// Validate checks that the configuration is internally consistent and
// within the documented ranges.
func (c CacheConfig) Validate() error {
	if c.Disabled {
		return nil
	}
	if c.TimeoutSeconds < 60 || c.TimeoutSeconds > 3600 {
		return ErrInvalidCacheConfig
	}
	if c.RefreshBeforeSeconds < 0 || c.RefreshBeforeSeconds >= c.TimeoutSeconds {
		return ErrInvalidCacheConfig
	}
	return nil
}

// IsActive reports whether a cache with this configuration should currently
// be treated as active given the age of the timestamp file.
//
// This is a pure helper used by CacheRepository implementations; the
// repository is still the source of truth for CacheStatus.
func (c CacheConfig) IsActive(ageSeconds float64) bool {
	if c.Disabled {
		return false
	}
	return ageSeconds < float64(c.TimeoutSeconds)
}

// ShouldRefresh reports whether the cache should be proactively refreshed
// (i.e. is within the refresh window but not yet expired).
func (c CacheConfig) ShouldRefresh(ageSeconds float64) bool {
	if c.Disabled {
		return false
	}
	remaining := float64(c.TimeoutSeconds) - ageSeconds
	return remaining > 0 && remaining <= float64(c.RefreshBeforeSeconds)
}
