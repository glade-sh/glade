package project

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

func TestDuplicateRuleDiscoverySourceAndMetadataAPIRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, suffix := range []string{"", "-meta.xml"} {
			route := "metadata"
			if suffix != "" {
				route = "source"
			}
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
				matching := filepath.Join(root, "force-app/main/default/matchingRules/Contact.matchingRule"+suffix)
				duplicate := filepath.Join(root, "force-app/main/default/duplicateRules/Contact.Grove_Email_Check.duplicateRule"+suffix)
				writeFile(t, matching, "<MatchingRules/>")
				writeFile(t, duplicate, "<DuplicateRule/>")
				p, err := Load(root)
				if err != nil {
					t.Fatal(err)
				}
				// Project JSON lets this regression compile before discovery exists.
				body, err := json.Marshal(p)
				if err != nil {
					t.Fatal(err)
				}
				var files struct {
					Matching  []string `json:"matchingRuleFiles"`
					Duplicate []string `json:"duplicateRuleFiles"`
				}
				if err := json.Unmarshal(body, &files); err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(files.Matching, []string{matching}) || !slices.Equal(files.Duplicate, []string{duplicate}) {
					t.Fatalf("rule discovery: matching=%v duplicate=%v; want %s and %s", files.Matching, files.Duplicate, matching, duplicate)
				}
			})
		}
	}
}
