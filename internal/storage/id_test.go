package storage

import (
	"fmt"
	"strings"
	"testing"
)

func TestIDGeneratorUsesDeterministicObjectSequences(t *testing.T) {
	g := NewIDGenerator(map[string]string{
		"Account":  "001",
		"Thing__c": "a00",
	})

	firstAccount, err := g.Next("Account")
	if err != nil {
		t.Fatal(err)
	}
	secondAccount, err := g.Next("Account")
	if err != nil {
		t.Fatal(err)
	}
	firstThing, err := g.Next("Thing__c")
	if err != nil {
		t.Fatal(err)
	}

	if firstAccount != "001000000000001" {
		t.Fatalf("first account id = %s", firstAccount)
	}
	if secondAccount != "001000000000002" {
		t.Fatalf("second account id = %s", secondAccount)
	}
	if firstThing != "a00000000000001" {
		t.Fatalf("first thing id = %s", firstThing)
	}
}

func TestStandardCollaborationGroupKeyPrefixMatchesSalesforce(t *testing.T) {
	if got := StandardKeyPrefix("CollaborationGroup"); got != "0F9" {
		t.Fatalf("CollaborationGroup key prefix = %q, want 0F9", got)
	}
	if got := StandardKeyPrefix("FeedItem"); got != "0D5" {
		t.Fatalf("FeedItem key prefix = %q, want 0D5", got)
	}
	if got := StandardKeyPrefix("FeedComment"); got != "0D7" {
		t.Fatalf("FeedComment key prefix = %q, want 0D7", got)
	}

	g := NewStandardIDGenerator()
	id, err := g.Next("CollaborationGroup")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(id), "0F9") {
		t.Fatalf("CollaborationGroup id = %q, want 0F9 prefix", id)
	}
	feedID, err := g.Next("FeedItem")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(feedID), "0D5") {
		t.Fatalf("FeedItem id = %q, want 0D5 prefix", feedID)
	}
	commentID, err := g.Next("FeedComment")
	if err != nil {
		t.Fatalf("FeedComment id: %v", err)
	}
	if !strings.HasPrefix(string(commentID), "0D7") {
		t.Fatalf("FeedComment id = %q, want 0D7 prefix", commentID)
	}
}

func TestRuntimeIDGeneratorKeepsLogicalSequencesButOffsetsIDBody(t *testing.T) {
	g := NewRuntimeIDGenerator(map[string]string{"Account": "001"})

	id, err := g.Next("Account")
	if err != nil {
		t.Fatal(err)
	}

	if id == "001000000000001" {
		t.Fatalf("runtime id collided with low fake-id sequence: %s", id)
	}
	if g.Sequences["Account"] != 1 {
		t.Fatalf("logical sequence = %d, want 1", g.Sequences["Account"])
	}
}

func TestIDsEqualKeepsFifteenCharacterCaseSignificant(t *testing.T) {
	if IDsEqual("aDa000000000001", "aDA000000000001") {
		t.Fatal("15 character ids that differ by case must not compare equal")
	}
	if !IDsEqual("aDa000000000001", "aDa000000000001AAA") {
		t.Fatal("15 and 18 character forms with the same first 15 chars should compare equal")
	}
}

func TestAssignDeterministicPrefixesKeepsStandardAndExplicitPrefixes(t *testing.T) {
	prefixes := AssignDeterministicPrefixes(
		[]string{"Widget__c", "Account", "Alpha__c"},
		map[string]string{"Widget__c": "a99"},
	)

	if prefixes["Account"] != "001" {
		t.Fatalf("Account prefix = %q", prefixes["Account"])
	}
	if prefixes["Widget__c"] != "a99" {
		t.Fatalf("Widget__c prefix = %q", prefixes["Widget__c"])
	}
	if prefixes["Alpha__c"] != "a00" {
		t.Fatalf("Alpha__c prefix = %q", prefixes["Alpha__c"])
	}
}

func TestEnsureUniqueKeyPrefixesReassignsDuplicateCustomPrefixes(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a00"}}
	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a00"}}
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001"}}

	EnsureUniqueKeyPrefixes(&org)

	alpha := org.Objects["Alpha__c"].Definition.KeyPrefix
	beta := org.Objects["Beta__c"].Definition.KeyPrefix
	if alpha == "" || beta == "" || alpha == beta {
		t.Fatalf("custom prefixes alpha=%q beta=%q, want unique", alpha, beta)
	}
	if got := org.Objects["Account"].Definition.KeyPrefix; got != "001" {
		t.Fatalf("Account prefix = %q", got)
	}
}

func TestEnsureUniqueKeyPrefixesUsesDedicatedPoolForImplicitCustomSettings(t *testing.T) {
	org := NewOrgState()
	org.Objects["Logger_Settings__c"] = ObjectState{Definition: ObjectDefinition{
		APIName:  "Logger_Settings__c",
		Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
	}}
	org.Objects["Widget__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Widget__c"}}

	EnsureUniqueKeyPrefixes(&org)
	if got := org.Objects["Logger_Settings__c"].Definition.KeyPrefix; got != "s00" {
		t.Fatalf("implicit custom-setting prefix = %q, want s00", got)
	}
	if got := org.Objects["Widget__c"].Definition.KeyPrefix; got != "a00" {
		t.Fatalf("regular custom-object prefix = %q, want a00", got)
	}
}

func TestPrefixesForOrgUsesDedicatedPoolForImplicitCustomSettingIDs(t *testing.T) {
	org := NewOrgState()
	org.Objects["Logger_Settings__c"] = ObjectState{Definition: ObjectDefinition{
		APIName:  "Logger_Settings__c",
		Metadata: map[string]string{"kind": "customSetting", "customSettingsType": "Hierarchy"},
	}}

	prefixes := prefixesForOrg(org)
	if got := prefixes["Logger_Settings__c"]; got != "s00" {
		t.Fatalf("fixture custom-setting prefix = %q, want s00", got)
	}
	generator := NewIDGenerator(prefixes)
	id, err := generator.Next("Logger_Settings__c")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(id), "s00") {
		t.Fatalf("fixture custom-setting id = %q, want s00 prefix", id)
	}
}

func TestEnsureUniqueKeyPrefixesFastPathAvoidsFullPrefixRebuild(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001"}}
	org.Objects["Contact"] = ObjectState{Definition: ObjectDefinition{APIName: "Contact", KeyPrefix: "003"}}
	for i := 0; i < 128; i++ {
		name := fmt.Sprintf("Ready_%03d__c", i)
		org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, KeyPrefix: customPrefix(i + 1000)}}
	}

	allocs := testing.AllocsPerRun(20, func() {
		EnsureUniqueKeyPrefixes(&org)
	})
	if allocs != 0 {
		t.Fatalf("allocs per ready-org EnsureUniqueKeyPrefixes = %.0f, want 0", allocs)
	}
}

func TestEnsureUniqueKeyPrefixesValidatedMarkerTracksObjectCount(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a00"}}

	EnsureUniqueKeyPrefixes(&org)
	if !org.keyPrefixesValidated || org.keyPrefixesValidatedObjectCount != 1 {
		t.Fatalf("validation marker = %v/%d, want true/1", org.keyPrefixesValidated, org.keyPrefixesValidatedObjectCount)
	}

	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a00"}}
	EnsureUniqueKeyPrefixes(&org)

	alpha := org.Objects["Alpha__c"].Definition.KeyPrefix
	beta := org.Objects["Beta__c"].Definition.KeyPrefix
	if alpha == "" || beta == "" || alpha == beta {
		t.Fatalf("custom prefixes alpha=%q beta=%q, want unique after object-count change", alpha, beta)
	}
	if !org.keyPrefixesValidated || org.keyPrefixesValidatedObjectCount != 2 {
		t.Fatalf("validation marker after repair = %v/%d, want true/2", org.keyPrefixesValidated, org.keyPrefixesValidatedObjectCount)
	}
}

func TestEnsureUniqueKeyPrefixesValidatedMarkerTracksPrefixChanges(t *testing.T) {
	org := NewOrgState()
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a00"}}
	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a01"}}

	EnsureUniqueKeyPrefixes(&org)
	beta := org.Objects["Beta__c"]
	beta.Definition.KeyPrefix = "a00"
	org.Objects["Beta__c"] = beta

	EnsureUniqueKeyPrefixes(&org)

	alphaPrefix := org.Objects["Alpha__c"].Definition.KeyPrefix
	betaPrefix := org.Objects["Beta__c"].Definition.KeyPrefix
	if alphaPrefix == "" || betaPrefix == "" || alphaPrefix == betaPrefix {
		t.Fatalf("custom prefixes alpha=%q beta=%q, want unique after same-count prefix change", alphaPrefix, betaPrefix)
	}
}

// referenceKeyPrefixSnapshotMatches is the former map-based check: walk every
// object and compare with the snapshot, reading absent names as "".
func referenceKeyPrefixSnapshotMatches(org *OrgState) bool {
	if org.keyPrefixesValidatedPrefixes == nil || len(org.keyPrefixesValidatedPrefixes) != len(org.Objects) {
		return false
	}
	prefixes := make(map[string]string, len(org.keyPrefixesValidatedPrefixes))
	for _, entry := range org.keyPrefixesValidatedPrefixes {
		prefixes[entry.name] = entry.prefix
	}
	for name, state := range org.Objects {
		if prefixes[name] != state.Definition.KeyPrefix {
			return false
		}
	}
	return true
}

func keyPrefixTestOrg() OrgState {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001"}}
	org.Objects["Alpha__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Alpha__c", KeyPrefix: "a00"}}
	org.Objects["Beta__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Beta__c", KeyPrefix: "a01"}}
	return org
}

func setKeyPrefix(org *OrgState, name, prefix string) {
	state := org.Objects[name]
	state.Definition.KeyPrefix = prefix
	org.Objects[name] = state
}

// Every mutation after validation must reach the same snapshot decision as the
// former check, and EnsureUniqueKeyPrefixes must leave the same prefixes.
func TestKeyPrefixSnapshotMatchesFormerCheckOnEveryMutation(t *testing.T) {
	mutations := map[string]func(*OrgState){
		"unchanged":                func(*OrgState) {},
		"record write":             func(org *OrgState) { org.Objects["Alpha__c"] = org.Objects["Alpha__c"] },
		"same-count prefix change": func(org *OrgState) { setKeyPrefix(org, "Beta__c", "a00") },
		"prefix whitespace":        func(org *OrgState) { setKeyPrefix(org, "Beta__c", "a01 ") },
		"prefix cleared":           func(org *OrgState) { setKeyPrefix(org, "Beta__c", "") },
		"standard prefix replaced": func(org *OrgState) { setKeyPrefix(org, "Account", "a02") },
		"object added": func(org *OrgState) {
			org.Objects["Gamma__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Gamma__c", KeyPrefix: "a00"}}
		},
		"object deleted": func(org *OrgState) { delete(org.Objects, "Beta__c") },
		"same-count swap with duplicate prefix": func(org *OrgState) {
			delete(org.Objects, "Alpha__c")
			org.Objects["Gamma__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Gamma__c", KeyPrefix: "a01"}}
		},
		"same-count swap with fresh prefix": func(org *OrgState) {
			delete(org.Objects, "Alpha__c")
			org.Objects["Gamma__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Gamma__c", KeyPrefix: "a05"}}
		},
		"same-count swap without prefix": func(org *OrgState) {
			delete(org.Objects, "Alpha__c")
			org.Objects["Gamma__c"] = ObjectState{Definition: ObjectDefinition{APIName: "Gamma__c"}}
		},
		"schema stamp cleared": func(org *OrgState) { org.ClearRuntimeSchemaStamp() },
		"mutable definition": func(org *OrgState) {
			definition, _ := EnsureMutableObjectDefinition(org, "Beta__c")
			definition.KeyPrefix = "a00"
		},
	}
	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			org := keyPrefixTestOrg()
			EnsureUniqueKeyPrefixes(&org)
			reference := keyPrefixTestOrg()
			EnsureUniqueKeyPrefixes(&reference)
			mutate(&org)
			mutate(&reference)
			if got, want := keyPrefixValidationSnapshotMatches(&org), referenceKeyPrefixSnapshotMatches(&reference); got != want {
				t.Fatalf("snapshot match = %v, former check = %v", got, want)
			}
			EnsureUniqueKeyPrefixes(&org)
			if !reference.keyPrefixesValidated || reference.keyPrefixesValidatedObjectCount != len(reference.Objects) || !referenceKeyPrefixSnapshotMatches(&reference) {
				repairKeyPrefixesWithFormerCheck(&reference)
			}
			for objectName, state := range reference.Objects {
				if got := org.Objects[objectName].Definition.KeyPrefix; got != state.Definition.KeyPrefix {
					t.Fatalf("%s prefix = %q, former path = %q", objectName, got, state.Definition.KeyPrefix)
				}
			}
			if !keyPrefixValidationSnapshotMatches(&org) {
				t.Fatal("snapshot does not match after EnsureUniqueKeyPrefixes")
			}
		})
	}
}

// repairKeyPrefixesWithFormerCheck runs the validation body that follows a
// failed snapshot check: it never depends on the snapshot itself.
func repairKeyPrefixesWithFormerCheck(org *OrgState) {
	org.keyPrefixesValidated = false
	EnsureUniqueKeyPrefixes(org)
}

// Clones copy the snapshot slice header. A clone's mutation and re-mark must
// not change what the source org validated.
func TestKeyPrefixSnapshotIsIsolatedAcrossClones(t *testing.T) {
	source := keyPrefixTestOrg()
	EnsureUniqueKeyPrefixes(&source)
	template := NewRuntimeTemplate(source)
	clones := map[string]OrgState{"Clone": source.Clone(), "CloneRuntimeOrg": template.CloneRuntimeOrg()}
	for name, clone := range clones {
		t.Run(name, func(t *testing.T) {
			setKeyPrefix(&clone, "Beta__c", "a00")
			EnsureUniqueKeyPrefixes(&clone)
			if got := clone.Objects["Beta__c"].Definition.KeyPrefix; got == "a00" {
				t.Fatalf("clone kept duplicate prefix %q", got)
			}
			if !keyPrefixValidationSnapshotMatches(&source) {
				t.Fatal("clone re-validation changed the source snapshot")
			}
			if got := source.Objects["Beta__c"].Definition.KeyPrefix; got != "a01" {
				t.Fatalf("source Beta__c prefix = %q, want a01", got)
			}
		})
	}
}

// The unchanged-org check must stay on the snapshot fast path: it reads one
// prefix per snapshot entry and never walks and copies every ObjectState.
func TestEnsureUniqueKeyPrefixesUnchangedOrgSkipsObjectWalk(t *testing.T) {
	org := NewOrgState()
	for i := 0; i < 256; i++ {
		name := fmt.Sprintf("Ready_%03d__c", i)
		org.Objects[name] = ObjectState{Definition: ObjectDefinition{APIName: name, KeyPrefix: customPrefix(i + 1000)}}
	}
	EnsureUniqueKeyPrefixes(&org)
	before := keyPrefixSnapshotObjectWalks.Load()
	for i := 0; i < 10; i++ {
		EnsureUniqueKeyPrefixes(&org)
		org.Objects["Ready_000__c"] = org.Objects["Ready_000__c"]
	}
	if walks := keyPrefixSnapshotObjectWalks.Load() - before; walks != 0 {
		t.Fatalf("unchanged org walked every object %d times, want 0", walks)
	}
	setKeyPrefix(&org, "Ready_001__c", customPrefix(1000))
	EnsureUniqueKeyPrefixes(&org)
	if walks := keyPrefixSnapshotObjectWalks.Load() - before; walks != 1 {
		t.Fatalf("changed org walked every object %d times, want 1", walks)
	}
}

func TestCustomPrefixDoesNotCycleAfterLeadingARange(t *testing.T) {
	const fullFirstCycle = 62*62 + 61*62*62
	seen := make(map[string]struct{}, fullFirstCycle)
	for i := 0; i < fullFirstCycle; i++ {
		prefix := customPrefix(i)
		if len(prefix) != 3 {
			t.Fatalf("customPrefix(%d) length = %d", i, len(prefix))
		}
		if _, ok := seen[prefix]; ok {
			t.Fatalf("customPrefix(%d) repeated %q", i, prefix)
		}
		seen[prefix] = struct{}{}
	}
}

func TestValidateIDAccepts15And18CharacterBase62IDs(t *testing.T) {
	for _, id := range []ID{"001000000000001", "001000000000001AAA"} {
		if err := ValidateID(id); err != nil {
			t.Fatalf("ValidateID(%q): %v", id, err)
		}
	}
	if err := ValidateID("00100000000000!"); err == nil {
		t.Fatal("ValidateID accepted non-base62 id")
	}
}

func TestSchemaGenerationAdvancesOnEveryDefinitionChangePath(t *testing.T) {
	org := keyPrefixTestOrg()
	start := org.SchemaGeneration()
	org.ClearRuntimeSchemaStamp()
	if got := org.SchemaGeneration(); got != start+1 {
		t.Fatalf("generation after ClearRuntimeSchemaStamp = %d, want %d", got, start+1)
	}
	if _, ok := EnsureMutableObjectDefinition(&org, "Beta__c"); !ok {
		t.Fatal("Beta__c definition missing")
	}
	if got := org.SchemaGeneration(); got != start+2 {
		t.Fatalf("generation after EnsureMutableObjectDefinition = %d, want %d", got, start+2)
	}
	org.Objects["Alpha__c"] = org.Objects["Alpha__c"]
	EnsureUniqueKeyPrefixes(&org)
	if got := org.SchemaGeneration(); got != start+2 {
		t.Fatalf("generation after a record write and prefix check = %d, want %d", got, start+2)
	}
	var missing *OrgState
	missing.ClearRuntimeSchemaStamp()
	if missing.SchemaGeneration() != 0 {
		t.Fatal("nil org generation")
	}
}
