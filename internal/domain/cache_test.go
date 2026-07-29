package domain

import (
	"testing"
	"time"
)

func TestCacheStatus_String(t *testing.T) {
	if CacheUnknown.String() != "unknown" {
		t.Fatal("CacheUnknown.String wrong")
	}
	if CacheActive.String() != "active" {
		t.Fatal("CacheActive.String wrong")
	}
	if CacheExpired.String() != "expired" {
		t.Fatal("CacheExpired.String wrong")
	}
	if CacheStatus(99).String() != "unknown" {
		t.Fatal("invalid status should fall through to unknown")
	}
}

func TestDefaultCacheConfig(t *testing.T) {
	c := DefaultCacheConfig()
	if c.TimeoutSeconds != 900 || c.RefreshBeforeSeconds != 120 || c.Disabled {
		t.Fatalf("default config wrong: %+v", c)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestCacheConfig_EffectiveTimeout(t *testing.T) {
	c := CacheConfig{TimeoutSeconds: 300}
	if got := c.EffectiveTimeout(); got != 300*time.Second {
		t.Fatalf("EffectiveTimeout = %v, want 300s", got)
	}
	c = CacheConfig{TimeoutSeconds: 0}
	if c.EffectiveTimeout() != 0 {
		t.Fatal("zero TimeoutSeconds should yield 0")
	}
	c = CacheConfig{TimeoutSeconds: -1}
	if c.EffectiveTimeout() != 0 {
		t.Fatal("negative TimeoutSeconds should yield 0")
	}
}

func TestCacheConfig_EffectiveRefreshBefore(t *testing.T) {
	c := CacheConfig{RefreshBeforeSeconds: 60}
	if got := c.EffectiveRefreshBefore(); got != 60*time.Second {
		t.Fatalf("got %v", got)
	}
	c = CacheConfig{RefreshBeforeSeconds: 0}
	if c.EffectiveRefreshBefore() != 0 {
		t.Fatal("zero should yield 0")
	}
	c = CacheConfig{RefreshBeforeSeconds: -10}
	if c.EffectiveRefreshBefore() != 0 {
		t.Fatal("negative should yield 0")
	}
}

func TestCacheConfig_Validate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     CacheConfig
		wantErr bool
	}{
		{"valid default", DefaultCacheConfig(), false},
		{"disabled is valid", CacheConfig{Disabled: true}, false},
		{"too short", CacheConfig{TimeoutSeconds: 30, RefreshBeforeSeconds: 5}, true},
		{"too long", CacheConfig{TimeoutSeconds: 7200, RefreshBeforeSeconds: 60}, true},
		{"refresh >= timeout", CacheConfig{TimeoutSeconds: 600, RefreshBeforeSeconds: 600}, true},
		{"negative refresh", CacheConfig{TimeoutSeconds: 600, RefreshBeforeSeconds: -1}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestCacheConfig_IsActive(t *testing.T) {
	c := DefaultCacheConfig()
	if !c.IsActive(0) {
		t.Fatal("age 0 should be active")
	}
	if !c.IsActive(800) {
		t.Fatal("age 800 < 900 should be active")
	}
	if c.IsActive(901) {
		t.Fatal("age 901 > 900 should be inactive")
	}
	if (CacheConfig{Disabled: true}).IsActive(0) {
		t.Fatal("disabled config should never be active")
	}
}

func TestCacheConfig_ShouldRefresh(t *testing.T) {
	c := DefaultCacheConfig()
	if c.ShouldRefresh(0) {
		t.Fatal("age 0 should NOT refresh yet (full window left)")
	}
	if !c.ShouldRefresh(800) {
		t.Fatal("age 800 (100s remaining, < 120 refresh window) should refresh")
	}
	if !c.ShouldRefresh(850) {
		t.Fatal("age 850 (50s remaining) should refresh")
	}
	if c.ShouldRefresh(901) {
		t.Fatal("age 901 should NOT refresh (already expired)")
	}
	if (CacheConfig{Disabled: true}).ShouldRefresh(800) {
		t.Fatal("disabled config never refreshes")
	}
}
