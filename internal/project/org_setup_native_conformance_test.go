package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSuggestOrgSetupNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/org_setup"
	readRows := func(name string) [][]string {
		t.Helper()
		f, err := os.Open(filepath.Join(dir, name))
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
		return rows
	}
	cases, native := readRows("cases.tsv"), readRows("org.tsv")
	if len(cases) != 10 || len(native) != 19 {
		t.Fatalf("incomplete native matrix: cases=%d rows=%d", len(cases)-1, len(native)-1)
	}
	wantNative := map[string]string{
		"PERSON_FLAG": "System.QueryException", "PERSON_CONTACT": "System.QueryException",
		"PERSON_CUSTOM": "System.SObjectException", "CURRENCY": "System.QueryException",
		"EXPERIENCE_USER": "System.QueryException", "EXPERIENCE_TYPE": "0",
		"TERRITORY": "System.QueryException", "RECORD_TYPE_QUERY": "0", "RECORD_TYPE_DESCRIBE": "true",
	}
	observed := map[string]string{}
	for _, row := range native[1:] {
		key := row[0] + "/" + row[1]
		if row[2] != "named-class" || row[3] != "OBSERVED" || row[4] != wantNative[row[0]] || observed[key] != "" {
			t.Fatalf("invalid native row: %v", row)
		}
		observed[key] = row[4]
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range cases[1:] {
			t.Run(api+"/"+row[0], func(t *testing.T) {
				if observed[row[0]+"/"+api] == "" {
					t.Fatal("missing native observation")
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`)
				source, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes", row[1]+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				const relative = "force-app/main/default/classes/SetupUse.cls"
				writeFile(t, filepath.Join(root, relative), string(source))
				writeFile(t, filepath.Join(root, "glade.yml"), "org:\n  features: [Chatter]\n")
				got := SuggestOrgSetup(root)
				if len(got) != 1 || got[0].Kind != row[2] {
					t.Fatalf("native=%s; expected %s %s, got %+v", observed[row[0]+"/"+api], row[2], row[3], got)
				}
				s := got[0]
				if len(s.Files) != 1 || s.Files[0] != relative {
					t.Fatalf("evidence files: %v", s.Files)
				}
				switch s.Kind {
				case "feature":
					if s.Feature != row[3] || s.YAML != "org:\n  features: [Chatter, "+row[3]+"]" {
						t.Fatalf("feature configuration: %+v", s)
					}
					writeFile(t, filepath.Join(root, "glade.yml"), s.YAML+"\n")
				case "unsupportedFeature":
					if s.Feature != row[3] || s.YAML != "" || !strings.Contains(s.Reason, "not modelled") {
						t.Fatalf("unsupported setup must not suggest ineffective config: %+v", s)
					}
					return
				case "recordType":
					if strings.Join(s.Names, ",") != row[3] || s.YAML != "" {
						t.Fatalf("record type advice: %+v", s)
					}
					writeFile(t, filepath.Join(root, "force-app/main/default/objects/Account/recordTypes/Synthetic_Client.recordType-meta.xml"),
						"<RecordType><fullName>Synthetic_Client</fullName><label>Synthetic Client</label><active>true</active></RecordType>")
				}
				if got := SuggestOrgSetup(root); len(got) != 0 {
					t.Fatalf("declared setup still suggested: %+v", got)
				}
			})
		}
	}
}

func TestSuggestOrgSetupFeatureEvidenceBoundaries(t *testing.T) {
	for _, source := range []string{
		"class SetupUse { String Territory2Helper; String IsPortalEnabledFlag; }",
		"// IsPortalEnabled Territory2 CurrencyIsoCode\nclass SetupUse {}",
		"/* IsPersonAccount PersonContactId Preference__pc */ class SetupUse {}",
		"class SetupUse { Object x = [SELECT Id FROM User WHERE UserType = 'Standard']; }",
	} {
		root := t.TempDir()
		writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
		writeFile(t, filepath.Join(root, "force-app/classes/SetupUse.cls"), source)
		if got := SuggestOrgSetup(root); len(got) != 0 {
			t.Errorf("unexpected suggestions for %q: %+v", source, got)
		}
	}
}

func TestSuggestOrgSetupDirectRecordTypeFields(t *testing.T) {
	for _, field := range []string{"CreatedBy.Name", "CreatedBy . Name", "BusinessProcess.Name", "LastModifiedBy.DeveloperName"} {
		t.Run(field, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}]}`)
			writeFile(t, filepath.Join(root, "force-app/classes/SetupUse.cls"),
				"class SetupUse { Object x = [SELECT Id FROM RecordType WHERE SObjectType = 'Account' AND "+field+" = 'Example']; }")
			if got := SuggestOrgSetup(root); len(got) != 0 {
				t.Fatalf("relationship field is not a record-type name: %+v", got)
			}
		})
	}
}
