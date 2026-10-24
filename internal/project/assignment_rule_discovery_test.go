package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestAssignmentRuleDiscoverySourceAndMetadataAPIRoutes(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", "platform_transaction_paths", "project", "force-app", "main", "default", "assignmentRules", "Lead.assignmentRules-meta.xml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, suffix := range []string{".assignmentRules", ".assignmentRules-meta.xml"} {
			t.Run(api+"/"+suffix, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				path := filepath.Join(root, "force-app", "main", "default", "assignmentRules", "Lead"+suffix)
				writeFile(t, path, string(body))
				p, err := Load(root)
				if err != nil {
					t.Fatal(err)
				}
				raw, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				var discovered struct {
					Files []string `json:"assignmentRuleFiles"`
				}
				if err := json.Unmarshal(raw, &discovered); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(discovered.Files, []string{path}) {
					t.Fatalf("assignment metadata discovery=%v; want [%s]", discovered.Files, path)
				}
			})
		}
	}
}
