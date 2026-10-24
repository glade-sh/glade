package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSetupFeatureMatcherPlatformContextRoutes(t *testing.T) {
	data, err := os.ReadFile("testdata/org_setup_platform_context/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct{ Route, Source, Feature, File string }
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run("sourceMatcher/"+tc.Route, func(t *testing.T) {
			for _, rule := range featureEvidenceRules {
				if rule.feature != "PersonAccounts" && rule.feature != "MultiCurrency" {
					continue
				}
				if got, want := matchesSetupFeature(rule, tc.Source, strings.HasSuffix(tc.File, ".xml")), tc.Feature == rule.feature; got != want {
					t.Errorf("%s evidence = %v, want %v", rule.feature, got, want)
				}
			}
		})
		for _, api := range []string{"62.0", "67.0"} {
			t.Run("SuggestOrgSetup/"+api+"/"+tc.Route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app", tc.File), tc.Source)
				got := SuggestOrgSetup(root)
				if tc.Feature == "" {
					if len(got) != 0 {
						t.Fatalf("name-only source generated advice: %+v", got)
					}
				} else if len(got) != 1 || got[0].Feature != tc.Feature {
					t.Fatalf("want %s advice, got %+v", tc.Feature, got)
				}
			})
		}
	}
}
