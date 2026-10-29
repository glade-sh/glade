package storage

import (
	"reflect"
	"testing"
)

var featureNoopShape = []string{"Chatter", "EnableSetPasswordInApi", "LightningExperience"}

// A run org with enabled features carries the feature signature in every
// prepared overlay marker. Re-ensuring a prepared object must be a no-op that
// keeps the shared schema generation, exactly as it is for a plain org.
func TestStandardEnsurePreparedFeatureOrgKeepsSchemaGeneration(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, featureNoopShape)
	names := KnownStandardObjectNames()
	for _, name := range names {
		EnsureStandardObject(&org, name)
	}
	// Each first ensure moves the generation and drops earlier record-type
	// proofs; the runtime template records them once at its freeze boundary.
	PrimeStandardObjectEnsureNoops(&org)
	org.RuntimeSchemaStamp = "trusted"
	generation := org.RuntimeGenerationID()
	for _, name := range names {
		before := org.Objects[name].Definition
		EnsureStandardObject(&org, name)
		after := org.Objects[name].Definition
		if !reflect.DeepEqual(before, after) {
			t.Fatalf("%s: re-ensure changed a prepared definition", name)
		}
		if org.RuntimeSchemaStamp != "trusted" || org.RuntimeGenerationID() != generation {
			t.Fatalf("%s: re-ensuring a prepared definition on a feature-enabled org discarded the schema generation", name)
		}
	}
}

// The relaxed predicate must still see every real change. Each route edits a
// prepared feature-org definition so that ensure has to write, and the
// generation must move.
func TestStandardEnsureFeatureOrgRealChangesStillInvalidate(t *testing.T) {
	for _, route := range []string{"missing_field", "missing_marker", "feature_added", "label_reset", "missing_prefix"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			ApplyOrgShape(&org, featureNoopShape)
			const name = "PermissionSetAssignment"
			EnsureStandardObject(&org, name)
			org.RuntimeSchemaStamp = "trusted"
			generation := org.RuntimeGenerationID()
			state := org.Objects[name]
			state.Definition = state.Definition.Clone()
			switch route {
			case "missing_field":
				delete(state.Definition.Fields, "AssigneeId")
			case "missing_marker":
				delete(state.Definition.Metadata, standardFieldsOverlayMarker)
			case "feature_added":
				org.setFeatureProfile(append(append([]string(nil), org.Features...), "PersonAccounts"))
			case "label_reset":
				state.Definition.Label = ""
			case "missing_prefix":
				state.Definition.KeyPrefix = ""
			}
			org.Objects[name] = state
			EnsureStandardObject(&org, name)
			if org.RuntimeSchemaStamp != "" || org.RuntimeGenerationID() == generation {
				t.Fatalf("%s kept the schema generation across a real schema write", route)
			}
			repaired := org.Objects[name].Definition
			if _, ok := repaired.Fields["AssigneeId"]; !ok || repaired.Label == "" || repaired.KeyPrefix == "" || !standardFieldsOverlayApplied(repaired, canonicalFeatureSignature(org.Features)) {
				t.Fatalf("%s left the definition unrepaired", route)
			}
		})
	}
}

// Priming records the record-type proof with the same signature the later
// ensure compares against, so a feature-enabled template's seeded record
// types are no-ops too.
func TestPrimeStandardObjectEnsureNoopsUsesFeatureSignature(t *testing.T) {
	org := NewOrgState()
	ApplyOrgShape(&org, featureNoopShape)
	EnsureStandardObject(&org, "Account")
	state := org.Objects["Account"]
	state.Definition = state.Definition.Clone()
	state.Definition.RecordTypes = []RecordTypeInfo{{DeveloperName: "Owned", Name: "Owned", Active: true, Available: true}}
	org.Objects["Account"] = state
	EnsureStandardObject(&org, "Account")
	// Drop the proof the ensure recorded; priming must rebuild it.
	org.InvalidateRuntimeGeneration()
	PrimeStandardObjectEnsureNoops(&org)
	org.RuntimeSchemaStamp = "trusted"
	generation := org.RuntimeGenerationID()
	EnsureStandardObject(&org, "Account")
	if org.RuntimeSchemaStamp != "trusted" || org.RuntimeGenerationID() != generation {
		t.Fatal("primed feature-org record types were not proven as a no-op")
	}
}
