package apextest

import (
	"encoding/json"
	"os"
	"testing"
)

func TestStandardProfileInventoryCommunitiesFeatureCreationDefinition(t *testing.T) {
	body, err := os.ReadFile("testdata/conformance/standard_profile_inventory/configurations/communities_feature/project-scratch-def.json")
	if err != nil {
		t.Fatal(err)
	}
	var definition struct {
		Edition  string
		Features []string
		Settings struct {
			CommunitiesSettings struct {
				EnableNetworksEnabled bool
			}
		}
	}
	if err := json.Unmarshal(body, &definition); err != nil {
		t.Fatal(err)
	}
	if definition.Edition != "Developer" || len(definition.Features) != 1 || definition.Features[0] != "Communities" || !definition.Settings.CommunitiesSettings.EnableNetworksEnabled {
		t.Fatal("native profile capture requires a Developer org provisioned with the Communities feature and Networks enabled")
	}
}
