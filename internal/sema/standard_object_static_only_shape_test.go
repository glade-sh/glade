package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A feature that only an unreferenced definition enables ungates the static
// object-absence check for the objects it provides, and for nothing else.
func TestStandardObjectShapeGateUngatedByStaticOnlyFeature(t *testing.T) {
	declared := []string{project.StandardFieldEditionEnterprise}
	if !semaStandardObjectAbsentFromShape(declared, "AccountBrand") {
		t.Fatal("AccountBrand must be absent from a declared shape without Networks")
	}
	withStatic := append(append([]string(nil), declared...), project.StaticOnlyShapeFeaturePrefix+"NetworksEnabled")
	if semaStandardObjectAbsentFromShape(withStatic, "AccountBrand") {
		t.Fatal("a static-only Networks feature must ungate AccountBrand")
	}
	if !semaStandardObjectAbsentFromShape(withStatic, "OrderItem") && semaStandardObjectAbsentFromShape(declared, "OrderItem") {
		t.Fatal("a static-only Networks feature must not ungate unrelated objects")
	}
}
