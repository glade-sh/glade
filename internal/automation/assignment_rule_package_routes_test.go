package automation

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestAssignmentRuleLoadedPackageAndShimMetadataRoutes(t *testing.T) {
	path := filepath.Join("..", "apextest", "testdata", "conformance", "platform_transaction_paths", "project", "force-app", "main", "default", "assignmentRules", "Lead.assignmentRules-meta.xml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"root", "dependency", "shim", "duplicate_dependency"} {
		t.Run(route, func(t *testing.T) {
			child := map[string]any{"assignmentRuleFiles": []string{path}}
			shape := child
			if route != "root" {
				field := "managedPackageDependencies"
				if route == "shim" {
					field = "packageShims"
				}
				entries := []any{map[string]any{"status": "loaded", "project": child}}
				if route == "duplicate_dependency" {
					entries = append(entries, entries[0])
				}
				shape = map[string]any{field: entries}
			}
			data, err := json.Marshal(shape)
			if err != nil {
				t.Fatal(err)
			}
			var p project.Project
			if err := json.Unmarshal(data, &p); err != nil {
				t.Fatal(err)
			}
			read := func(request string) ([]byte, error) {
				if request != path {
					t.Fatalf("unexpected read %s", request)
				}
				return body, nil
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Lead")
			if err := resource.ApplyProject(&org, p, read); err != nil {
				t.Fatal(err)
			}
			idx, err := LoadProject(p, read)
			if err != nil {
				t.Fatal(err)
			}
			if len(idx.Diagnostics) != 0 {
				t.Fatal(idx.Diagnostics)
			}
			ApplyToOrg(&org, idx)
			raw, err := json.Marshal(org.Objects["Lead"].Definition)
			if err != nil {
				t.Fatal(err)
			}
			var definition struct {
				Rules []struct {
					ID   storage.ID `json:"id"`
					Name string     `json:"name"`
				} `json:"assignmentRules"`
			}
			if err := json.Unmarshal(raw, &definition); err != nil {
				t.Fatal(err)
			}
			if len(definition.Rules) != 1 {
				t.Fatalf("loaded %d executable assignment rules, want one", len(definition.Rules))
			}
			rule := definition.Rules[0]
			_, row, ok := storage.LookupRecordByID(org.Objects["AssignmentRule"].Records, rule.ID)
			if !ok || rule.Name != "CaptureLeadRouting" || row.Fields["Name"].String != rule.Name {
				t.Fatalf("executable rule and setup row disagree: %+v %+v", rule, row)
			}
		})
	}
}
