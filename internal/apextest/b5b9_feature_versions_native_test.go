package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The b5b9_feature_objects table contains anonymous and named 62-67
// membership observations for every field whose endpoint shapes differ.
func TestB5B9FeatureObjectVersionNativeRoutes(t *testing.T) {
	const dir = "testdata/conformance/standard_describe_profiles/b5b9_feature_objects/"
	f, err := os.Open(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	groups := map[string][]string{}
	for _, row := range rows[1:] {
		if row[2] != "anonymous" || (row[1] != "gift-bisect" && row[1] != "doc-bisect") {
			continue
		}
		pair := strings.Split(row[3], ".")
		if len(pair) != 2 {
			t.Fatalf("bad field %q", row[3])
		}
		key := row[0] + "/" + row[1]
		groups[key] = append(groups[key], `System.assertEquals(`+row[4]+`,m.get('`+pair[0]+`').getDescribe().fields.getMap().containsKey('`+pair[1]+`'),'`+row[3]+`');`)
	}
	for _, api := range []string{"62.0", "63.0", "64.0", "65.0", "66.0", "67.0"} {
		for _, group := range []string{"gift-bisect", "doc-bisect"} {
			t.Run(api+"/"+group, func(t *testing.T) {
				assertions := groups[api+"/"+group]
				if len(assertions) == 0 {
					t.Fatal("native rows missing")
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				file := "npc-minimal.json"
				if group == "doc-bisect" {
					file = "docgen-minimal.json"
				}
				data, err := os.ReadFile(dir + file)
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(data))
				body := `Map<String,Schema.SObjectType> m=Schema.getGlobalDescribe();` + strings.Join(assertions, "")
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}
