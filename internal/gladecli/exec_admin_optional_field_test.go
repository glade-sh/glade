package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// glade exec runs anonymous Apex as the default administrator. Without a field
// grant an ordinary custom field is invisible to it: static references do not
// compile and describe omits the field (native admin_optional_field_api_matrix
// anonStatic*/anon* rows at 62.0-68.0). An Admin.profile field grant makes it
// visible (native profile row). Exec has no permission set assignment state
// (test.permissionSets is test-only), so a deployed permission set that grants
// the field counts as assigned: natively an assigned set makes the field
// visible, and the schema_reference_gate loaded rows run it that way.
func TestExecAdminOptionalFieldVisibility(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const family = "../apextest/testdata/conformance/admin_optional_field_api_matrix"
	read := func(rel string) string {
		b, err := os.ReadFile(filepath.Join(family, rel))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	staticSoql, probe := read("anon/static_soql.apex"), read("anon/probe.apex")
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, mode := range []string{"noGrant", "adminProfileGrant", "deployedPermissionSet"} {
			granted := mode != "noGrant"
			name := api + "/" + mode
			t.Run(name, func(t *testing.T) {
				root := t.TempDir()
				project := filepath.Join(family, "project")
				if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
					if err != nil || entry.IsDir() {
						return err
					}
					rel, err := filepath.Rel(project, path)
					if err != nil || strings.Contains(rel, "classes") || (mode != "deployedPermissionSet" && strings.Contains(rel, "permissionsets")) {
						return err
					}
					body, err := os.ReadFile(path)
					if err != nil {
						return err
					}
					writeTestFile(t, filepath.Join(root, rel), string(body))
					return nil
				}); err != nil {
					t.Fatal(err)
				}
				writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				if mode == "adminProfileGrant" {
					writeTestFile(t, filepath.Join(root, "force-app/main/default/profiles/Admin.profile-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><Profile xmlns="http://soap.sforce.com/2006/04/metadata"><custom>false</custom><fieldPermissions><editable>true</editable><field>CapObj__c.Optional__c</field><readable>true</readable></fieldPermissions></Profile>`)
				}
				exec := func(source string) (int, string) {
					var stdout, stderr bytes.Buffer
					code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
					return code, stdout.String() + stderr.String()
				}
				code, output := exec(staticSoql)
				if granted != (code == 0) {
					t.Fatalf("static SOQL exit=%d: %s", code, output)
				}
				if !granted && (!strings.Contains(output, "GLADESEMA") || !strings.Contains(output, "Optional__c")) {
					t.Fatalf("static SOQL is not a compile error naming the field: %s", output)
				}
				code, output = exec(probe)
				if code != 0 {
					t.Fatalf("probe exit=%d: %s", code, output)
				}
				var envelope struct {
					Data struct{ Debug []string }
				}
				if err := json.Unmarshal([]byte(output), &envelope); err != nil {
					t.Fatalf("%v: %s", err, output)
				}
				var observed map[string]any
				for _, line := range envelope.Data.Debug {
					if _, payload, ok := strings.Cut(line, "OBSERVED|anonymous|"); ok {
						if err := json.Unmarshal([]byte(payload), &observed); err != nil {
							t.Fatal(err)
						}
					}
				}
				if observed == nil {
					t.Fatalf("missing observation: %s", output)
				}
				if observed["fieldInMap"] != granted {
					t.Fatalf("fieldInMap=%v, want %v: %v", observed["fieldInMap"], granted, observed)
				}
				wantPut := "System.SObjectException:Invalid field Optional__c for CapObj__c"
				if granted {
					wantPut = "ok"
				}
				if observed["insert"] != wantPut {
					t.Fatalf("insert=%v, want %v", observed["insert"], wantPut)
				}
			})
		}
	}
}

// A named class called from anonymous Apex keeps the class API's field policy
// (native class rows: at 67.0 the unassigned administrator's static SELECT of
// an ungranted custom field fails with "No such column"; at 62.0 it returns
// rows). Behavior change versus the previous exec: the 67.0 class query now
// throws instead of returning 0 rows. A deployed permission set that grants the
// field counts as assigned in exec, so the query succeeds.
func TestExecNamedClassAdminOptionalFieldPolicy(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	const family = "../apextest/testdata/conformance/admin_optional_field_api_matrix"
	for _, tc := range []struct {
		api, mode string
		wantErr   bool
	}{
		{"62.0", "noGrant", false},
		{"67.0", "noGrant", true},
		{"68.0", "noGrant", true},
		{"67.0", "deployedPermissionSet", false},
		{"62.0", "deployedPermissionSet", false},
	} {
		t.Run(tc.api+"/"+tc.mode, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(family, "project")
			if err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(project, path)
				if err != nil || strings.Contains(rel, "classes") || (tc.mode == "noGrant" && strings.Contains(rel, "permissionsets")) {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeTestFile(t, filepath.Join(root, rel), string(body))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			api := strings.TrimSuffix(tc.api, ".0")
			writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+tc.api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Rd.cls"), `public class Rd { public static Integer n() { return [SELECT Id, Optional__c FROM CapObj__c].size(); } }`)
			writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/Rd.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`.0</apiVersion><status>Active</status></ApexClass>`)
			var stdout, stderr bytes.Buffer
			code := Run(context.Background(), []string{"exec", "--project", root, "--json", "System.debug(Rd.n());"}, &stdout, &stderr)
			output := stdout.String() + stderr.String()
			if tc.wantErr {
				if code == 0 || !strings.Contains(output, "No such column") {
					t.Fatalf("exit=%d, want QueryException No such column: %s", code, output)
				}
			} else if code != 0 {
				t.Fatalf("exit=%d: %s", code, output)
			}
		})
	}
}
