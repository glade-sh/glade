package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestSuggestOrgSetupNativeEvidenceRoutes(t *testing.T) {
	const dir = "testdata/conformance/org_setup_evidence"
	read := func(name string) [][]string {
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
	cases, native := read("cases.tsv"), read("org.tsv")
	wantNative := map[string]string{
		"IDENTIFIERS": "false/label", "LITERAL": "IsPortalEnabled",
		"USER_TYPE_NAME": "true", "TERRITORY_NAME": "label",
		"DYNAMIC_FIELD": "System.SObjectException", "ALIASED_RECORD_TYPE": "0",
		"DIRECT_WITH_RELATIONSHIP": "0",
	}
	if len(cases) != 8 || len(native) != 15 {
		t.Fatalf("incomplete matrix: %d cases, %d rows", len(cases)-1, len(native)-1)
	}
	observed := map[string]bool{}
	for _, row := range native[1:] {
		key := row[0] + "/" + row[1]
		if row[2] != "named-class" || row[3] != "OBSERVED" || row[4] != wantNative[row[0]] || observed[key] {
			t.Fatalf("invalid native row: %v", row)
		}
		observed[key] = true
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, row := range cases[1:] {
			t.Run("SuggestOrgSetup/"+api+"/"+row[0], func(t *testing.T) {
				if !observed[row[0]+"/"+api] {
					t.Fatal("missing native row")
				}
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app"}],"sourceApiVersion":"`+api+`"}`)
				source, err := os.ReadFile(filepath.Join(dir, "project/force-app/main/default/classes", row[1]+".cls"))
				if err != nil {
					t.Fatal(err)
				}
				writeFile(t, filepath.Join(root, "force-app/main/default/classes", row[1]+".cls"), string(source))
				got := SuggestOrgSetup(root)
				if row[2] == "none" {
					if len(got) != 0 {
						t.Fatalf("native=%s; names alone are not setup evidence: %+v", wantNative[row[0]], got)
					}
					return
				}
				if len(got) != 1 || got[0].Kind != row[2] {
					t.Fatalf("native=%s; expected %s %s, got %+v", wantNative[row[0]], row[2], row[3], got)
				}
				if row[2] == "feature" && (got[0].Feature != row[3] || got[0].YAML != "org:\n  features: [Communities]") {
					t.Fatalf("feature advice: %+v", got)
				}
				if row[2] == "recordType" && !reflect.DeepEqual(got[0].Names, []string{row[3]}) {
					t.Fatalf("direct name lost or relationship name included: %+v", got)
				}
			})
		}
	}
}

func TestRecordTypeReferencesQueryRoutes(t *testing.T) {
	for _, query := range []string{
		"SELECT Id FROM RecordType WHERE SObjectType = 'Account' AND DeveloperName = 'Synthetic_Client' AND CreatedBy.Name = 'Synthetic_Author'",
		"SELECT rt.Id FROM RecordType rt WHERE rt.SObjectType = 'Account' AND rt.DeveloperName = 'Synthetic_Client' AND rt.CreatedBy.Name = 'Synthetic_Author'",
	} {
		t.Run(query, func(t *testing.T) {
			want := []setupNameReference{{object: "Account", name: "Synthetic_Client"}}
			if got := recordTypeReferences("[" + query + "]"); !reflect.DeepEqual(got, want) {
				t.Fatalf("recordTypeReferences query route: got %+v, want %+v", got, want)
			}
		})
	}
}

func TestSuggestOrgSetupContextualFeatureRoutes(t *testing.T) {
	for _, tc := range []struct{ route, file, source, feature string }{
		{"schemaField", "SetupUse.cls", "class SetupUse { Object f = Schema.User.IsPortalEnabled; }", "Communities"},
		{"typedField", "SetupUse.cls", "class SetupUse { Boolean f(User u) { return u.IsPortalEnabled; } }", "Communities"},
		{"typedDynamicField", "SetupUse.cls", "class SetupUse { Object f(User u) { return u.get('IsPortalEnabled'); } }", "Communities"},
		{"typedUserType", "SetupUse.cls", "class SetupUse { Boolean f(User u) { return u.UserType == 'CustomerSuccess'; } }", "Communities"},
		{"unrelatedDynamicField", "SetupUse.cls", "class SetupUse { Object f(Map<String,Object> labels) { return labels.get('IsPortalEnabled'); } }", ""},
		{"wrongQueryObject", "SetupUse.cls", "class SetupUse { Object x = Database.query('SELECT IsPortalEnabled FROM SetupLabel__c'); }", ""},
		{"territoryType", "SetupUse.cls", "class SetupUse { List<Territory2> records; }", "Territory2"},
		{"territorySchema", "SetupUse.cls", "class SetupUse { Object f = Schema.Territory2.SObjectType; }", "Territory2"},
		{"territoryLabel", "SetupUse.cls", "class SetupUse { String label = 'Territory2 records'; }", ""},
		{"permissionMetadata", "Synthetic.permissionset-meta.xml", "<PermissionSet><fieldPermissions><field>User.IsPortalEnabled</field></fieldPermissions></PermissionSet>", "Communities"},
		{"metadataLabel", "Synthetic.permissionset-meta.xml", "<PermissionSet><label>IsPortalEnabled Territory2</label></PermissionSet>", ""},
		{"territoryField", "SetupUse.cls", "class SetupUse { Id f(Opportunity o) { return o.Territory2Id; } }", "Territory2"},
		{"territoryDynamicField", "SetupUse.cls", "class SetupUse { Object f(Opportunity o) { return o.get('Territory2Id'); } }", "Territory2"},
		{"territoryQueryField", "SetupUse.cls", "class SetupUse { Object x = Database.query('SELECT Territory2Id FROM Opportunity'); }", "Territory2"},
		{"territoryLongName", "SetupUse.cls", "class SetupUse { String Territory2Model = 'label'; }", ""},
		{"queryLiteralValue", "SetupUse.cls", "class SetupUse { Object x = [SELECT Id FROM User WHERE FirstName = 'IsPortalEnabled']; }", ""},
		{"territoryMethodName", "SetupUse.cls", "class SetupUse { String Territory2() { return 'label'; } }", ""},
		{"schemaDynamicField", "SetupUse.cls", "class SetupUse { Object f = Schema.SObjectType.User.fields.getMap().get('IsPortalEnabled'); }", "Communities"},
		{"schemaDynamicObject", "SetupUse.cls", "class SetupUse { Object f = Schema.getGlobalDescribe().get('User').getDescribe().fields.getMap().get('IsPortalEnabled'); }", "Communities"},
		{"splitDynamicQuery", "SetupUse.cls", "class SetupUse { Object x = Database.query('SELECT IsPortalEnabled ' + 'FROM User'); }", "Communities"},
		{"territoryConstructor", "SetupUse.cls", "class SetupUse { Object x = new Opportunity(Territory2Id = null); }", "Territory2"},
		{"userConstructor", "SetupUse.cls", "class SetupUse { Object x = new User(IsPortalEnabled = true); }", "Communities"},
		{"typedReceiverReuse", "SetupUse.cls", "class SetupUse { void a(User u) {} Object b(Map<String,Object> u) { return u.get('IsPortalEnabled'); } }", ""},
		{"territoryReceiverReuse", "SetupUse.cls", "class SetupUse { void a(Opportunity o) {} Object b(Map<String,Object> o) { return o.get('Territory2Id'); } }", ""},
		{"flowUserField", "Synthetic.flow-meta.xml", "<Flow><recordLookups><filters><field>IsPortalEnabled</field><operator>EqualTo</operator><value><booleanValue>true</booleanValue></value></filters><object>User</object></recordLookups></Flow>", "Communities"},
		{"flowTerritoryField", "Synthetic.flow-meta.xml", "<Flow><recordLookups><filters><field>Territory2Id</field></filters><object>Opportunity</object></recordLookups></Flow>", "Territory2"},
		{"lwcSchemaField", "setupUse.js", "import portalField from '@salesforce/schema/User.IsPortalEnabled';", "Communities"},
		{"lwcTerritoryField", "setupUse.js", "import territoryField from '@salesforce/schema/Opportunity.Territory2Id';", "Territory2"},
	} {
		t.Run(tc.route, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "force-app", tc.file), tc.source)
			got := SuggestOrgSetup(root)
			if tc.feature == "" {
				if len(got) != 0 {
					t.Fatalf("unrelated name generated advice: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Feature != tc.feature {
				t.Fatalf("expected %s, got %+v", tc.feature, got)
			}
		})
	}
}
