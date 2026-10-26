package automation

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestAssignmentRuleLoadAndApplyNativeMetadataAPIRoutes(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "platform_transaction_paths", "project", "force-app", "main", "default", "assignmentRules", "Lead.assignmentRules-meta.xml")
	body, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, suffix := range []string{".assignmentRules", ".assignmentRules-meta.xml"} {
			for _, reader := range []string{"disk", "captured"} {
				t.Run(api+"/"+suffix+"/"+reader, func(t *testing.T) {
					root := t.TempDir()
					writeWorkflowTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					path := filepath.Join(root, "force-app", "main", "default", "assignmentRules", "Lead"+suffix)
					writeWorkflowTestFile(t, path, string(body))
					p, err := project.Load(root)
					if err != nil {
						t.Fatal(err)
					}
					var idx Index
					if reader == "captured" {
						if err := os.Remove(path); err != nil {
							t.Fatal(err)
						}
						idx, err = LoadProject(p, func(request string) ([]byte, error) {
							if request != path {
								return nil, fmt.Errorf("unexpected metadata read: %s", request)
							}
							return append([]byte(nil), body...), nil
						})
					} else {
						idx, err = LoadProject(p)
					}
					if err != nil {
						t.Fatal(err)
					}
					if len(idx.Diagnostics) != 0 {
						t.Fatalf("native-deployed assignment metadata diagnostics: %v", idx.Diagnostics)
					}
					org := storage.OrgState{}
					storage.EnsureStandardObject(&org, "Lead")
					ApplyToOrg(&org, idx)
					ApplyToOrg(&org, idx)
					raw, err := json.Marshal(org.Objects["Lead"].Definition)
					if err != nil {
						t.Fatal(err)
					}
					var definition struct {
						Rules []struct {
							Name    string `json:"name"`
							Active  bool   `json:"active"`
							Entries []struct {
								AssignedTo     string `json:"assignedTo"`
								AssignedToType string `json:"assignedToType"`
								Criteria       []struct {
									Field     string `json:"field"`
									Operation string `json:"operation"`
									Value     string `json:"value"`
								} `json:"criteria"`
							} `json:"entries"`
						} `json:"assignmentRules"`
					}
					if err := json.Unmarshal(raw, &definition); err != nil {
						t.Fatal(err)
					}
					if len(definition.Rules) != 1 {
						t.Fatalf("Lead assignment definitions=%d, want 1 from native-deployed metadata", len(definition.Rules))
					}
					rule := definition.Rules[0]
					if rule.Name != "CaptureLeadRouting" || !rule.Active || len(rule.Entries) != 1 {
						t.Fatalf("assignment definition lost deployed values: %+v", rule)
					}
					entry := rule.Entries[0]
					if entry.AssignedTo != "CaptureLeadQueue" || entry.AssignedToType != "Queue" || len(entry.Criteria) != 1 {
						t.Fatalf("assignment entry lost deployed values: %+v", entry)
					}
					criterion := entry.Criteria[0]
					if criterion.Field != "Company" || criterion.Operation != "equals" || criterion.Value != "route-match" {
						t.Fatalf("assignment criterion lost deployed values: %+v", criterion)
					}
				})
			}
		}
	}
}
