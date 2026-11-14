package storage

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

func TestStandardSpreadUncapturedShapeSnapshot(t *testing.T) {
	org := sourceSpreadEagerOrg([]string{project.UnspecifiedStandardFieldShape})
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	// Serialized eager org from origin/main with the same test and inputs.
	const base = "d383e54f7e7db22e9f6bb4359abd18c859acda34bffc44826e02d0fff30843fb"
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
		{"direct", "84f28c5d923267cf47e63683d4098f1666152c540764c10cb31ba3a764f78a72", func() OrgState {
			return sourceSpreadEagerOrg([]string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
		}},
		{"project-transition", "1f46db4d3c4e4280d3e84d2562c16f0fc88c9672145e70fa251f7bb310309b97", func() OrgState {
			org := sourceSpreadEagerOrg(initial)
			ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper, project.PlainDeveloperObjectShape, "EnableSetPasswordInApi"})
			return org
		}},
		{"additive-transition", "1f46db4d3c4e4280d3e84d2562c16f0fc88c9672145e70fa251f7bb310309b97", func() OrgState {
			org := sourceSpreadEagerOrg(initial)
			ApplyOrgShape(&org, []string{"EnableSetPasswordInApi"})
			return org
		}},
		{"unmarked-transition", "f9c31d42954390cfd23d95927c346b705c25f7a2cd2c7c9f9e36e7deaa0712f1", func() OrgState {
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
