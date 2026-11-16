package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// Digests that touch standard objects with a native relationship order include
// those orders (relationship_order_standard/org.tsv). Apart from the order
// entries they equal the origin/main serialization;
// TestStandardRelationshipOrderIsTheOnlySpreadDifference checks that.
func TestStandardSpreadUncapturedShapeSnapshot(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.UnspecifiedStandardFieldShape})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized eager org from origin/main with the same test and inputs.
	const base = "c86920c2e0e6c4e16b894cc73c9f6266812a1bb2adc016a7914acc5bfdda6d30"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != base {
		t.Fatalf("uncaptured shape sha256 %s, want origin/main %s", got, base)
	}
}

func TestPlainDeveloperUncapturedNeighborMatchesBaseline(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	object, ok := org.Objects["CareProgram"]
	if !ok {
		t.Fatal("neighbor missing")
	}
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	// Object serialized from origin/main with the same eager plain Developer shape.
	const baseline = "87c49a5c0609dc4add0f7d87a7a5484623b80fc99a0ba63e05ff5831877e7fe6"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("neighbor digest %s, want origin/main %s", got, baseline)
	}
}

func TestPlainDeveloperUncapturedFeatureRetainsInventory(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	object, ok := org.Objects["AIFeatureExtractor"]
	if !ok {
		t.Fatal("uncaptured feature shape removed baseline object")
	}
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized object from origin/main under the same uncaptured feature shape.
	const baseline = "aaa91bc763df14a858f38c8f52457661ece5bea9d1f3acf0e9ddf37797d578d3"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("object digest %s, want origin/main %s", got, baseline)
	}
}

func TestPlainDeveloperShapeTransitionRestoresUncapturedFeatureInventory(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := org.Objects["AIFeatureExtractor"]; ok {
		t.Fatal("captured default object unexpectedly present")
	}
	storageShape := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"}
	ApplyProjectOrgShape(&org, storageShape)
	object, ok := org.Objects["AIFeatureExtractor"]
	if !ok {
		t.Fatal("uncaptured feature shape did not restore baseline object")
	}
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	const baseline = "c0483914b506fbf4fcd04767e8253fb1a5f1884ced2a4f1c566d79070dc1e407"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("object digest %s, want origin/main %s", got, baseline)
	}
}

func TestPlainDeveloperAdditiveShapeRestoresUncapturedFeatureInventory(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := org.Objects["AIFeatureExtractor"]; ok {
		t.Fatal("captured default object unexpectedly present")
	}
	ApplyOrgShape(&org, []string{"EnableSetPasswordInApi"})
	object, ok := org.Objects["AIFeatureExtractor"]
	if !ok {
		t.Fatal("additive feature did not restore baseline object")
	}
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized object from origin/main after the same additive transition.
	const baseline = "c0483914b506fbf4fcd04767e8253fb1a5f1884ced2a4f1c566d79070dc1e407"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("object digest %s, want origin/main %s", got, baseline)
	}
}

func TestPlainDeveloperUnmarkedShapeTransitionMatchesBaseline(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
	object, ok := org.Objects["AIFeatureExtractor"]
	if !ok {
		t.Fatal("unmarked shape did not restore baseline object")
	}
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized object from origin/main after the same transition.
	const baseline = "2fb31d1636fcf45af25141d6f23c2e1bbbf1550846de52cda02135998d36d80c"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("object digest %s, want origin/main %s", got, baseline)
	}
}

func TestUncapturedPlainDeveloperShapesMatchBaseline(t *testing.T) {
	initial := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	tests := []struct {
		name, digest string
		build        func() OrgState
	}{
		{"direct", "caad0e8e6d59f4ec3fafbe1423d9050f97542e2696774a26c629374577720762", func() OrgState {
			return sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
		}},
		{"project-transition", "b0e2c1f826d978069f6829aa61c95491d1ac34ec72d29b09f97568bf60908d42", func() OrgState {
			org := sourceSpreadEagerOrg(initial)
			ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
			return org
		}},
		{"additive-transition", "b0e2c1f826d978069f6829aa61c95491d1ac34ec72d29b09f97568bf60908d42", func() OrgState {
			org := sourceSpreadEagerOrg(initial)
			ApplyOrgShape(&org, []string{"EnableSetPasswordInApi"})
			return org
		}},
		{"unmarked-transition", "d1dde048f86ccfbaab6d243d739d4e65e02005da3e030396e4a9e88022f22f45", func() OrgState {
			org := sourceSpreadEagerOrg(initial)
			ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
			return org
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			org := tc.build()
			data, err := json.Marshal(org)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.digest {
				t.Fatalf("org digest %s, want origin/main %s", got, tc.digest)
			}
		})
	}
}

func TestUncapturedSourceSpreadTransitionsMatchBaseline(t *testing.T) {
	initial := []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape}
	tests := []struct {
		name, digest string
		apply        func(*OrgState)
	}{
		{"project", "3b6a584b7ed6c44a3937d335257b1efa94985f0317747a06eca95065a709a9fc", func(org *OrgState) {
			ApplyProjectOrgShape(org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
		}},
		{"additive", "3b6a584b7ed6c44a3937d335257b1efa94985f0317747a06eca95065a709a9fc", func(org *OrgState) { ApplyOrgShape(org, []string{"EnableSetPasswordInApi"}) }},
		{"unmarked", "55d66ee681f8c6d209c52b5e39009a77609e29b66f7ad28d74465f92f3cb653e", func(org *OrgState) {
			ApplyProjectOrgShape(org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape})
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			org := sourceSpreadLazyOrg(initial)
			MaterializeStandardObjectSpread(&org)
			tc.apply(&org)
			data, err := json.Marshal(org)
			if err != nil {
				t.Fatal(err)
			}
			if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != tc.digest {
				t.Fatalf("org digest %s, want origin/main %s", got, tc.digest)
			}
		})
	}
}

func TestSourceSpreadRestoresPreviouslyMaterializedObjectOnShapeTransition(t *testing.T) {
	org := NewOrgState()
	org.EnableStandardSpreadSource()
	if _, ok := EnsureLazyStandardObject(&org, "AIFeatureExtractor"); !ok {
		t.Fatal("source object not installed before shape")
	}
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	MaterializeStandardObjectSpread(&org)
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized source-backed org from origin/main with the same operations.
	const baseline = "6caf2090c1f93da673baa1dfa96c4dff7e37d43672e6125abed3479f43d1aae3"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("org digest %s, want origin/main %s", got, baseline)
	}
}

func TestSourceSpreadRestoresRemovedObjectAfterSerialization(t *testing.T) {
	org := NewOrgState()
	org.EnableStandardSpreadSource()
	if _, ok := EnsureLazyStandardObject(&org, "AIFeatureExtractor"); !ok {
		t.Fatal("source object not installed before shape")
	}
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	MaterializeStandardObjectSpread(&org)
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	ApplyProjectOrgShape(&restored, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	data, err = json.Marshal(restored)
	if err != nil {
		t.Fatal(err)
	}
	const baseline = "6caf2090c1f93da673baa1dfa96c4dff7e37d43672e6125abed3479f43d1aae3"
	if got := fmt.Sprintf("%x", sha256.Sum256(data)); got != baseline {
		t.Fatalf("org digest %s, want origin/main %s", got, baseline)
	}
}

func TestSourceSpreadTransitionDropsUntouchedAddedInventory(t *testing.T) {
	org := sourceSpreadLazyOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	if _, exists := org.Objects["CareProgram"]; exists {
		t.Fatal("uncaptured shape retained a source-only inventory addition")
	}
}

func TestSourceSpreadTransitionDropsUntouchedAddedInventoryAfterSerialization(t *testing.T) {
	org := sourceSpreadLazyOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored OrgState
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	ApplyProjectOrgShape(&restored, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	if _, exists := restored.Objects["CareProgram"]; exists {
		t.Fatal("serialized uncaptured shape retained a source-only inventory addition")
	}
}

func TestSourceSpreadTransitionRetainsEditedInventory(t *testing.T) {
	org := sourceSpreadLazyOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	object := org.Objects["CareProgram"]
	original := object.Definition.Fields["Id"]
	defer func() { object.Definition.Fields["Id"] = original }()
	edited := original
	edited.Label = "edited"
	object.Definition.Fields["Id"] = edited
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	if retained, exists := org.Objects["CareProgram"]; !exists || retained.Definition.Fields["Id"].Label != "edited" {
		t.Fatal("edited inventory definition was removed")
	}
}

func TestSourceSpreadTransitionRetainsInventoryRecords(t *testing.T) {
	org := sourceSpreadLazyOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, project.PlainDeveloperDescribeShape})
	if _, ok := EnsureLazyStandardObject(&org, "CareProgram"); !ok {
		t.Fatal("inventory object was not materialized")
	}
	object := org.Objects["CareProgram"]
	if object.Records == nil {
		object.SetRecords(make(map[ID]Record))
	}
	id := ID("a00000000000001")
	object.PutRecord(id, Record{ID: id, Object: "CareProgram"})
	org.Objects["CareProgram"] = object
	ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
	retained, exists := org.Objects["CareProgram"]
	if !exists || len(retained.Records) != 1 {
		t.Fatal("record-bearing inventory was removed")
	}
}
