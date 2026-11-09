package storage

import (
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// The static-gate marker never reaches the runtime org.
func TestApplyProjectOrgShapeDropsStaticOnlyFeatures(t *testing.T) {
	org := NewOrgState()
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionEnterprise, project.StaticOnlyShapeFeaturePrefix + "NetworksEnabled"})
	if org.HasFeature("NetworksEnabled") {
		t.Fatalf("static-only Networks must not enable the runtime feature: %v", org.Features)
	}
	for _, feature := range org.Features {
		if len(feature) >= len(project.StaticOnlyShapeFeaturePrefix) && feature[:len(project.StaticOnlyShapeFeaturePrefix)] == project.StaticOnlyShapeFeaturePrefix {
			t.Fatalf("marker leaked into org features: %v", org.Features)
		}
	}
}
