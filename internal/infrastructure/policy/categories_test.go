package policy

import (
	"errors"
	"testing"

	"github.com/LeandroLCD/sudoconsole/internal/domain"
)

func TestDefaultCategoryRegistry(t *testing.T) {
	r := DefaultCategoryRegistry()
	if r.Len() < 50 {
		t.Fatalf("expected at least 50 built-in entries, got %d", r.Len())
	}
}

func TestCategoryRegistry_Lookup(t *testing.T) {
	r := DefaultCategoryRegistry()
	e, ok := r.Lookup("ssh")
	if !ok {
		t.Fatal("ssh should be registered")
	}
	if e.Category != domain.CategoryRemoteAccess {
		t.Fatalf("ssh category = %v, want remote_access", e.Category)
	}
	if e.Risk != domain.RiskCritical {
		t.Fatalf("ssh risk = %v, want critical", e.Risk)
	}
	if !e.BuiltIn {
		t.Fatal("ssh should be marked BuiltIn")
	}
}

func TestCategoryRegistry_LookupUnknown(t *testing.T) {
	r := DefaultCategoryRegistry()
	_, ok := r.Lookup("definitely-not-a-real-binary")
	if ok {
		t.Fatal("unknown binary should return false")
	}
}

func TestCategoryRegistry_NilSafe(t *testing.T) {
	var r *CategoryRegistry
	_, ok := r.Lookup("ssh")
	if ok {
		t.Fatal("nil registry should return false")
	}
	if r.Len() != 0 {
		t.Fatal("nil registry Len should be 0")
	}
	if len(r.All()) != 0 {
		t.Fatalf("nil registry All should be empty")
	}
}

func TestCategoryRegistry_Add(t *testing.T) {
	r := &CategoryRegistry{}
	r.Add("mytool", domain.CategoryPackageManager, domain.RiskMedium, "test")
	e, ok := r.Lookup("mytool")
	if !ok {
		t.Fatal("Add should register entry")
	}
	if e.BuiltIn {
		t.Fatal("user-added should not be BuiltIn")
	}
	if e.Overrides {
		t.Fatal("first add should not override anything")
	}
}

func TestCategoryRegistry_AddOverrides(t *testing.T) {
	r := DefaultCategoryRegistry()
	r.Add("ssh", domain.CategoryShellSpawn, domain.RiskLow, "override")
	e, _ := r.Lookup("ssh")
	if !e.Overrides {
		t.Fatal("Add should mark Overrides when replacing")
	}
	if e.BuiltIn {
		t.Fatal("override should not be BuiltIn")
	}
	if e.Category != domain.CategoryShellSpawn {
		t.Fatalf("override category = %v", e.Category)
	}
}

func TestCategoryRegistry_Remove(t *testing.T) {
	r := DefaultCategoryRegistry()
	if !r.Remove("ssh") {
		t.Fatal("Remove should return true for existing entry")
	}
	if _, ok := r.Lookup("ssh"); ok {
		t.Fatal("ssh should be gone after Remove")
	}
	if r.Remove("ssh") {
		t.Fatal("Remove on missing entry should return false")
	}
}

func TestCategoryRegistry_RemoveNilSafe(t *testing.T) {
	var r *CategoryRegistry
	if r.Remove("ssh") {
		t.Fatal("nil registry Remove should return false")
	}
}

func TestCategoryRegistry_All(t *testing.T) {
	r := DefaultCategoryRegistry()
	all := r.All()
	if len(all) != r.Len() {
		t.Fatalf("All length %d != Len %d", len(all), r.Len())
	}
}

func TestKeyCategories(t *testing.T) {
	// Spot-check the categories most likely to be involved in policy violations.
	r := DefaultCategoryRegistry()
	cases := []struct {
		bin  string
		cat  domain.Category
		risk domain.Risk
	}{
		{"ssh", domain.CategoryRemoteAccess, domain.RiskCritical},
		{"nc", domain.CategoryRemoteAccess, domain.RiskCritical},
		{"bash", domain.CategoryShellSpawn, domain.RiskCritical},
		{"passwd", domain.CategoryUserManagement, domain.RiskHigh},
		{"ssh-keygen", domain.CategoryCredentialExposure, domain.RiskCritical},
		{"visudo", domain.CategoryCredentialExposure, domain.RiskCritical},
		{"crontab", domain.CategoryPersistence, domain.RiskHigh},
		{"apt", domain.CategoryPackageManager, domain.RiskMedium},
		{"systemctl", domain.CategoryServiceControl, domain.RiskMedium},
	}
	for _, tc := range cases {
		e, ok := r.Lookup(tc.bin)
		if !ok {
			t.Errorf("%s: not registered", tc.bin)
			continue
		}
		if e.Category != tc.cat {
			t.Errorf("%s: category = %v, want %v", tc.bin, e.Category, tc.cat)
		}
		if e.Risk != tc.risk {
			t.Errorf("%s: risk = %v, want %v", tc.bin, e.Risk, tc.risk)
		}
	}
}

// Reference categories compile-time check.
var (
	_ = errors.New
)
