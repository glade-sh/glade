package apextest

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// TestStandardProfileAccountNative compares the owned permission inventory and
// effective runAs describes against observations from a fresh Developer org.
func TestStandardProfileAccountNative(t *testing.T) {
	dir := filepath.Join("testdata", "conformance", "standard_profile_account")
	f, err := os.Open(filepath.Join(dir, "org.tsv"))
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
	source, err := os.ReadFile(filepath.Join(dir, "ProfileProbe.cls"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			want := map[string]any{}
			var matches []string
			for _, row := range rows[1:] {
				if row[0] != api {
					continue
				}
				var v map[string]any
				if err := json.Unmarshal([]byte(row[3]), &v); err != nil {
					t.Fatal(err)
				}
				if row[2] == "profile" {
					want[v["name"].(string)] = v
				} else {
					for _, n := range v["matches"].([]any) {
						matches = append(matches, n.(string))
					}
				}
			}
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			classes := filepath.Join(root, "force-app", "main", "default", "classes")
			writeFile(t, filepath.Join(classes, "ProfileProbe.cls"), string(source))
			writeFile(t, filepath.Join(classes, "ProfileProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"ProfileProbe"}, NoDiskCache: true, Parallelism: 1})
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("expected observation: %+v", run.Summary())
			}
			message := run.Suites[0].Cases[0].Problem.Message
			_, payload, found := strings.Cut(message, "PROFILE_CAPTURE:")
			if !found {
				t.Fatalf("missing observation: %s", message)
			}
			var got struct {
				Profiles []map[string]any `json:"profiles"`
				Matches  []string         `json:"matches"`
				First    *string          `json:"first"`
			}
			if err := json.Unmarshal([]byte(payload), &got); err != nil {
				t.Fatalf("%v: %s", err, message)
			}
			seen := map[string]bool{}
			for _, profile := range got.Profiles {
				name, ok := profile["name"].(string)
				if !ok {
					t.Fatalf("missing profile name: %v", profile)
				}
				expected, ok := want[name]
				if !ok {
					t.Errorf("unexpected Standard profile %s", name)
					continue
				}
				if seen[name] {
					t.Errorf("duplicate Standard profile %s", name)
				}
				seen[name] = true
				if !reflect.DeepEqual(profile, expected) {
					t.Errorf("%s: got %v; native %v", name, profile, expected)
				}
			}
			for name := range want {
				if !seen[name] {
					t.Errorf("missing native profile %s", name)
				}
			}
			// The capture records one observed order, but SOQL without ORDER BY
			// does not guarantee it; assert the match set and eligible LIMIT row.
			sort.Strings(matches)
			sort.Strings(got.Matches)
			if !reflect.DeepEqual(got.Matches, matches) {
				t.Errorf("query matches %v; native %v", got.Matches, matches)
			}
			if got.First == nil {
				t.Error("LIMIT 1 returned no profile")
			} else {
				found := false
				for _, n := range matches {
					if n == *got.First {
						found = true
					}
				}
				if !found {
					t.Errorf("LIMIT 1 returned %q outside native match set", *got.First)
				}
			}
		})
	}
}
