package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestProjectOrgShapeReplacesEditionIdentity(t *testing.T) {
	org := NewOrgState()
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, "MultiCurrency"})
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionEnterprise})
	if org.HasFeature(project.StandardFieldEditionDeveloper) || !org.HasFeature(project.StandardFieldEditionEnterprise) {
		t.Fatalf("edition identity was not replaced: %v", org.Features)
	}
	if !org.HasFeature("MultiCurrency") {
		t.Fatal("replacing edition discarded an additive capability")
	}
	ApplyOrgShape(&org, []string{"Entitlements"})
	if !org.HasFeature(project.StandardFieldEditionEnterprise) {
		t.Fatal("adding a capability lost the declared edition")
	}
	ApplyProjectOrgShape(&org, []string{project.UnspecifiedStandardFieldShape})
	if org.HasFeature(project.StandardFieldEditionEnterprise) {
		t.Fatal("unspecified shape retained an old edition")
	}
}
