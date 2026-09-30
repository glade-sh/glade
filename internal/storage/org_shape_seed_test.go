package storage

import "testing"

func TestApplyOrgShapeSeedsSchemaOnlyUser(t *testing.T) {
	for _, feature := range []string{"Sites", "Communities"} {
		t.Run(feature, func(t *testing.T) {
			org := NewOrgState()
			// Semantic analysis builds definitions without allocating record maps.
			org.Objects["User"] = ObjectState{Definition: ObjectDefinition{APIName: "User"}}
			ApplyOrgShape(&org, []string{feature})
			guest, ok := org.Objects["User"].Records[ID("005000000000G01")]
			if !ok || guest.Fields["UserType"].String != "Guest" {
				t.Fatalf("guest user = %#v, exists = %v", guest, ok)
			}
			guest.Fields["Alias"] = StringValue("custom")
			org.Objects["User"].Records[guest.ID] = guest
			ApplyOrgShape(&org, []string{feature})
			if got := org.Objects["User"].Records[guest.ID].Fields["Alias"].String; got != "custom" {
				t.Fatalf("repeated enrichment replaced existing alias: %q", got)
			}
		})
	}
}
