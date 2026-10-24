package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Baseline-preservation controls for review findings on q363. Native rows
// exist only for null permission-set IDs and for the System/User constants
// with the feature disabled. Blank IDs and feature-enabled custom-scope JSON
// have no captured row, so they keep their pre-q363 behaviour.

func TestAccessLevelBlankPermissionSetIDPreservesBaseAcrossRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, id := range []string{"", "   "} {
				for _, form := range []string{"AccessLevel.withPermissionSetId('%s')", "AccessLevel.USER_MODE.withPermissionSetId('%s')"} {
					t.Run(fmt.Sprintf("%s/%s/%q/%s", api, route, id, form), func(t *testing.T) {
						machine := New(nil)
						if route == "isTest" {
							machine.EnableTestContext()
						}
						source := fmt.Sprintf("AccessLevel level = "+form+";System.assertNotEquals(null, level);", id)
						program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						if _, err = machine.Execute(program); err != nil {
							t.Fatalf("blank id must not hit the feature gate (no native row): %v", err)
						}
					})
				}
			}
		}
	}
}

func TestAccessLevelFeatureEnabledCustomScopeJSONPreservesBaseAcrossRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			for _, expr := range []string{
				"JSON.serialize(scoped)",
				"JSON.serialize(new List<AccessLevel>{scoped})",
				"JSON.serializePretty(scoped)",
				"JSON.createGenerator(false).writeObject(scoped)",
			} {
				t.Run(api+"/"+route+"/"+expr, func(t *testing.T) {
					machine := New(nil)
					machine.SetOrg(&storage.OrgState{Features: []string{"ApexUserModeWithPermset"}})
					if route == "isTest" {
						machine.EnableTestContext()
					}
					source := "AccessLevel scoped = AccessLevel.withPermissionSetId('0PS000000000001');String observed;try {" + expr + ";} catch(Exception issue) {observed=issue.getMessage();}System.assertEquals(null, observed);"
					program, err := CompileAnonymousWithOptions(source, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if _, err = machine.Execute(program); err != nil {
						t.Fatalf("feature-enabled custom scope JSON has no native row: %v", err)
					}
				})
			}
		}
	}
}
