package storage

import "testing"

// The plain-org rows reject these objects; enabled Knowledge captures use them.
func TestKnowledgeObjectAvailabilityB4(t *testing.T) {
	for _, name := range []string{"Knowledge__ka", "Knowledge__kav"} {
		t.Run(name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, name)
			ApplyOrgShape(&org, []string{})
			if org.AllowsStandardObject(name) {
				t.Fatalf("plain org unexpectedly allows %s", name)
			}
			// The license feature alone yields no objects (captured shape rule).
			ApplyOrgShape(&org, []string{"Knowledge"})
			if org.AllowsStandardObject(name) {
				t.Fatalf("feature-only org unexpectedly allows %s", name)
			}
			ApplyOrgShape(&org, []string{"Knowledge", "KnowledgeEnabled"})
			if !org.AllowsStandardObject(name) {
				t.Fatalf("enabled org rejects %s", name)
			}
		})
	}
}

// An unspecified local shape is ungated, so Knowledge objects are present
// (captured shape rule; matches the capi2 undeclared-shape rule).
func TestKnowledgeUnspecifiedShapeIsUngated(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, []string{"__glade_unspecified_standard_field_shape"})
	for _, name := range []string{"Knowledge__kav", "Quote", "WebStore", "AccountContactRelation"} {
		if !org.AllowsStandardObject(name) {
			t.Fatalf("unspecified defaults lost %s", name)
		}
	}
}
