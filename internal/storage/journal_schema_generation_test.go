package storage

import "testing"

func TestJournalStandardEnsurePreservesOnlyUnchangedSchema(t *testing.T) {
	for _, route := range []string{"noop", "definition_repair", "field_overlay", "new_object", "key_prefix_repair", "mutable_definition"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "PermissionSetAssignment")
			if route == "definition_repair" {
				object := org.Objects["PermissionSetAssignment"]
				object.Definition.PluralLabel = ""
				org.Objects["PermissionSetAssignment"] = object
			}
			if route == "field_overlay" {
				object := org.Objects["PermissionSetAssignment"]
				object.Definition.Metadata[standardFieldsOverlayMarker] = "PersonAccounts"
				org.Objects["PermissionSetAssignment"] = object
			}
			EnsureUniqueKeyPrefixes(&org)
			org.RuntimeSchemaStamp = "trusted"
			generation := org.RuntimeGenerationID()
			schemaGeneration := org.SchemaGeneration()
			sentinel := new(int)
			org.StoreRuntimeGenerationCache("owned-index", sentinel)
			journal := NewIsolationJournal(&org)
			outer := journal.Mark()
			inner := journal.Mark()
			switch route {
			case "new_object":
				EnsureStandardObject(&org, "Lead")
			case "key_prefix_repair":
				object := org.Objects["PermissionSetAssignment"]
				object.Definition.KeyPrefix = ""
				org.Objects["PermissionSetAssignment"] = object
				org.keyPrefixesValidated = false
				EnsureUniqueKeyPrefixes(&org)
			case "mutable_definition":
				definition, _ := EnsureMutableObjectDefinition(&org, "PermissionSetAssignment")
				definition.Label = "Changed"
				object := org.Objects["PermissionSetAssignment"]
				object.Definition = *definition
				org.Objects["PermissionSetAssignment"] = object
			default:
				EnsureStandardObject(&org, "PermissionSetAssignment")
			}
			for _, mark := range []IsolationMark{inner, outer} {
				if err := journal.Rollback(mark); err != nil {
					t.Fatal(err)
				}
			}
			if route == "noop" {
				if org.RuntimeSchemaStamp != "trusted" || org.RuntimeGenerationID() != generation || org.SchemaGeneration() != schemaGeneration {
					t.Errorf("no-op reset lost schema identity: stamp=%q generation=%d (want %d) schemaGeneration=%d (want %d)", org.RuntimeSchemaStamp, org.RuntimeGenerationID(), generation, org.SchemaGeneration(), schemaGeneration)
				}
				if got, ok := org.LoadRuntimeGenerationCache("owned-index"); !ok || got != sentinel {
					t.Error("no-op reset discarded shared index")
				}
			} else {
				if org.RuntimeSchemaStamp != "" || org.RuntimeGenerationID() == generation {
					t.Error("schema mutation retained trusted generation")
				}
				if _, ok := org.LoadRuntimeGenerationCache("owned-index"); ok {
					t.Error("schema mutation retained stale index")
				}
			}
		})
	}
}
