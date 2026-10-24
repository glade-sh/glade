package project

import (
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const customPersonQueryDir = "testdata/conformance/org_setup_custom_person_query"

type customPersonQueryCase struct {
	Route, Class, Want string
	Feature            bool
}

func customPersonQueryCases(t *testing.T) []customPersonQueryCase {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(customPersonQueryDir, "cases.json"))
	if err != nil {
		t.Fatal(err)
	}
	var cases []customPersonQueryCase
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	return cases
}

func customPersonQuerySource(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(customPersonQueryDir, "project/force-app/main/default", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertCustomPersonQueryAdvice(t *testing.T, root string, want bool) {
	t.Helper()
	got := SuggestOrgSetup(root)
	found := false
	for _, s := range got {
		if s.Feature != "PersonAccounts" {
			continue
		}
		found = true
		if len(s.Files) != 1 || !strings.HasSuffix(s.Files[0], ".cls") || !strings.Contains(s.YAML, "PersonAccounts") {
			t.Errorf("custom query advice lost source or configuration: %+v", s)
		}
	}
	if found != want {
		t.Fatalf("custom Person Account query advice=%v, want %v: %+v", found, want, got)
	}
}

// Native rows establish field identity and executable query shapes; advice is
// local tooling policy. Each isolated class must supply its own field evidence.
func TestCustomPersonQueryNativeSuggestionRoutes(t *testing.T) {
	cases := customPersonQueryCases(t)
	f, err := os.Open(filepath.Join(customPersonQueryDir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	observed := map[string]string{}
	for _, row := range rows[1:] {
		if len(row) != 5 || row[3] != "OBSERVED" {
			t.Fatalf("invalid native row: %v", row)
		}
		key := strings.Join(row[:3], "/")
		if _, ok := observed[key]; ok {
			t.Fatalf("duplicate native row: %s", key)
		}
		observed[key] = row[4]
	}
	if len(observed) != (len(cases)+1)*4 {
		t.Fatalf("incomplete native matrix: %d rows", len(observed))
	}
	field := customPersonQuerySource(t, "objects/Contact/fields/Loyalty__c.field-meta.xml")
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			if got := observed["fieldMetadata/"+api+"/"+route]; got != "true/true" {
				t.Fatalf("native field metadata: %q", got)
			}
		}
		for _, tc := range cases {
			for _, route := range []string{"anonymous", "isTest"} {
				if got := observed[tc.Route+"/"+api+"/"+route]; got != tc.Want {
					t.Fatalf("native %s/%s/%s = %q, want %q", tc.Route, api, route, got, tc.Want)
				}
			}
			t.Run("SuggestOrgSetup/"+api+"/"+tc.Route, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`)
				writeFile(t, filepath.Join(root, "force-app/classes", tc.Class+".cls"), customPersonQuerySource(t, "classes/"+tc.Class+".cls"))
				writeFile(t, filepath.Join(root, "force-app/objects/Contact/fields/Loyalty__c.field-meta.xml"), field)
				assertCustomPersonQueryAdvice(t, root, tc.Feature)
			})
		}
	}
}

func TestCustomPersonQueryMetadataIsolationRoutes(t *testing.T) {
	field := customPersonQuerySource(t, "objects/Contact/fields/Loyalty__c.field-meta.xml")
	source := customPersonQuerySource(t, "classes/PersonQueryStaticSelect.cls")
	for _, route := range []string{"splitContactField", "legacyContactObject", "missingMetadata", "otherObject", "otherField", "metadataOnly", "declaredFeature", "caseInsensitive"} {
		t.Run(route, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"67.0"}`)
			if route != "metadataOnly" {
				text := source
				if route == "caseInsensitive" {
					text = strings.ReplaceAll(text, "Loyalty__pc FROM Account", "loyalty__PC FROM ACCOUNT")
				}
				writeFile(t, filepath.Join(root, "force-app/classes/PersonQueryStaticSelect.cls"), text)
			}
			switch route {
			case "missingMetadata":
			case "legacyContactObject":
				writeFile(t, filepath.Join(root, "force-app/objects/Contact.object"), `<CustomObject><fields><fullName>Loyalty__c</fullName><type>Text</type><length>20</length></fields></CustomObject>`)
			case "otherObject":
				writeFile(t, filepath.Join(root, "force-app/objects/Account/fields/Loyalty__c.field-meta.xml"), field)
			case "otherField":
				writeFile(t, filepath.Join(root, "force-app/objects/Contact/fields/Other__c.field-meta.xml"), strings.ReplaceAll(field, "Loyalty__c", "Other__c"))
			default:
				writeFile(t, filepath.Join(root, "force-app/objects/Contact/fields/Loyalty__c.field-meta.xml"), field)
			}
			if route == "declaredFeature" {
				writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [PersonAccounts]\n")
			}
			want := route == "splitContactField" || route == "legacyContactObject" || route == "caseInsensitive"
			assertCustomPersonQueryAdvice(t, root, want)
			assertCustomPersonQueryAdvice(t, root, want)
		})
	}
}
