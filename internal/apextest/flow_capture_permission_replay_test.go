package apextest

import (
	"encoding/json"
	"os"
	"testing"
)

// The native setup assigned one permission set, not a schema-derived blanket
// grant. Check two FlowCore objects and both independent FlowRefs captures.
func TestFlowCapturedPermissionAssignments(t *testing.T) {
	for _, family := range []struct{ directory, cluster, name string }{
		{"flow_core", "ordering", "FlowCoreCaptureAccess"},
		{"flow_core", "lifecycle", "FlowCoreCaptureAccess"},
		{"flow_refs", "record", ""},
		{"flow_refs", "prior_record", ""},
	} {
		t.Run(family.cluster, func(t *testing.T) {
			raw, err := os.ReadFile("testdata/conformance/" + family.directory + "/" + family.cluster + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var data struct{ Rows []flowRefsRow }
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatal(err)
			}
			for _, api := range []string{"62.0", "67.0"} {
				t.Run(api, func(t *testing.T) {
					files := data.Rows[0].ByAPI[api].Files
					var index = flowCoreIndex
					if family.directory == "flow_refs" {
						index = flowRefsIndex
					}
					loaded := index(t, api, nil, files)
					org := orgFromIndex(loaded)
					org.APIVersion = api
					if err := applyConfiguredTestPermissions(&org, loadProjectForTest(t, loaded.Project.Root)); err != nil {
						t.Fatal(err)
					}
					name := family.name
					if name == "" {
						var grants map[string]struct{ Name string }
						raw, err := os.ReadFile("testdata/conformance/flow_capture_permissions/refs.json")
						if err != nil {
							t.Fatal(err)
						}
						if err := json.Unmarshal(raw, &grants); err != nil {
							t.Fatal(err)
						}
						for obj, g := range grants {
							if _, ok := org.Objects[obj]; ok {
								name = g.Name
							}
						}
						if name == "" {
							t.Fatal("missing original capture object")
						}
					}
					assertCapturedPermissionAssignments(t, &org, name)
				})
			}
		})
	}
}

// The native API-67 catalog copy capture was denied for inaccessible fields.
// A general metadata factory must not require or invent a fixture grant.
func TestFlowCatalogCaptureHasNoPermissionAssignment(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/flow_trigger_type_projection.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Fixtures map[string]struct{ Files map[string]string }
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			index := flowCoreIndex(t, api, nil, data.Fixtures[api].Files)
			p := loadProjectForTest(t, index.Project.Root)
			if len(p.Test.PermissionSets) != 0 {
				t.Fatalf("ungranted native catalog fixture acquired grants: %v", p.Test.PermissionSets)
			}
		})
	}
}
