package gladecli

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"
)

var caseFLSFields = []string{
	"EntitlementId", "BusinessHoursId", "MilestoneStatus", "SlaStartDate", "SlaExitDate", "IsStopped",
	"StopStartDate", "IsClosedOnCreate", "ProductId", "ServiceContractId", "AssetWarrantyId",
}

func caseFLSProject(t *testing.T, api string, granted, withClass bool) string {
	t.Helper()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
	if granted {
		var fields strings.Builder
		for _, field := range caseFLSFields {
			fields.WriteString(`<fieldPermissions><editable>true</editable><field>Case.` + field + `</field><readable>true</readable></fieldPermissions>`)
		}
		writeTestFile(t, filepath.Join(root, "force-app/main/default/permissionsets/CaseFls.permissionset-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><label>CaseFls</label>`+fields.String()+`</PermissionSet>`)
	}
	if withClass {
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CaseFlsProbe.cls"), `public class CaseFlsProbe {
  public static Boolean described() { return Schema.SObjectType.Case.fields.getMap().containsKey('EntitlementId') && Schema.SObjectType.Case.fields.getMap().containsKey('ServiceContractId'); }
  public static Integer staticQuery() { return [SELECT EntitlementId, BusinessHoursId, IsStopped, MilestoneStatus FROM Case].size(); }
  public static Integer dynamicQuery() { return Database.query('SELECT EntitlementId FROM Case').size(); }
}`)
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CaseFlsProbe.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
	}
	return root
}

func caseFLSExec(root, source string) (bool, string) {
	var stdout, stderr bytes.Buffer
	code := Run(context.Background(), []string{"exec", "--project", root, "--json", source}, &stdout, &stderr)
	return code == 0, stdout.String() + stderr.String()
}

// Native case_standard_field_fls rows (baseline orgs and the permission-set
// grant org, API 62.0 and 67.0): anonymous Apex does not see the 11 Case fields
// (describe map, token, static and dynamic query, put); assignment and constructor arguments are not checked (Glade accepts
// unknown fields on standard SObjects there; parked); a field grant restores every route; controls stay visible. A deployed
// permission set counts as assigned in exec.
func TestExecCaseStandardFieldFLSAnonymousRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	for _, api := range []string{"62.0", "67.0"} {
		for _, granted := range []bool{false, true} {
			t.Run(api+"/granted="+map[bool]string{false: "false", true: "true"}[granted], func(t *testing.T) {
				root := caseFLSProject(t, api, granted, false)
				for _, field := range caseFLSFields {
					for name, source := range map[string]string{
						"token":   `Schema.SObjectField f = Case.` + field + `;`,
						"static":  `List<Case> r = [SELECT ` + field + ` FROM Case LIMIT 1];`,
						"dynamic": `List<Case> r = Database.query('SELECT ` + field + ` FROM Case LIMIT 1');`,
						"map":     `if (!Schema.SObjectType.Case.fields.getMap().containsKey('` + field + `')) { throw new DmlException('missing'); }`,
						"put":     `Case c = new Case(); c.put('` + field + `', null);`,
					} {
						ok, output := caseFLSExec(root, source)
						if ok != granted {
							t.Errorf("%s %s granted=%v: ok=%v: %s", field, name, granted, ok, output)
						}
					}
				}
				for _, source := range []string{
					`Schema.SObjectField f = Case.ContactId; List<Case> r = [SELECT ContactId FROM Case LIMIT 1];`,
					`List<Entitlement> r = [SELECT Id FROM Entitlement LIMIT 1];`,
				} {
					if ok, output := caseFLSExec(root, source); !ok {
						t.Errorf("control failed: %s", output)
					}
				}
			})
		}
	}
}

// Native rows, route istest and anonymous-calls-named-class: a deployed class
// sees all 11 fields in describe at 62.0 and 67.0; its static and dynamic
// queries succeed at 62.0 and fail with "No such column" at 67.0 unless the
// field grant exists.
func TestExecCaseStandardFieldFLSNamedClassRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	for _, tc := range []struct {
		api     string
		granted bool
		failing bool
	}{{"62.0", false, false}, {"67.0", false, true}, {"62.0", true, false}, {"67.0", true, false}} {
		t.Run(tc.api+"/"+map[bool]string{false: "ungranted", true: "granted"}[tc.granted], func(t *testing.T) {
			root := caseFLSProject(t, tc.api, tc.granted, true)
			if ok, output := caseFLSExec(root, `if (!CaseFlsProbe.described()) { throw new DmlException('describe'); }`); !ok {
				t.Fatalf("named describe: %s", output)
			}
			for _, call := range []string{"staticQuery", "dynamicQuery"} {
				ok, output := caseFLSExec(root, `System.debug(CaseFlsProbe.`+call+`());`)
				if ok == tc.failing || (tc.failing && !strings.Contains(output, "No such column 'EntitlementId'")) {
					t.Errorf("%s: ok=%v: %s", call, ok, output)
				}
			}
		})
	}
}

// Callers of the shared field-permission function: stripInaccessible, user-mode
// DML and user-mode SOQL in a deployed class follow the same grant (Glade
// routes; the native rows cover describe and queries only).
func TestExecCaseStandardFieldFLSSecurityRoutes(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Setenv("GLADE_ROOT", "")
	for _, granted := range []bool{false, true} {
		root := caseFLSProject(t, "67.0", granted, false)
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CaseFlsSec.cls"), `public class CaseFlsSec {
  public static String strip() { Case c = new Case(Subject='a'); c.put('IsStopped', false); return String.valueOf(Security.stripInaccessible(AccessType.CREATABLE, new List<Case>{c}).getRemovedFields()); }
  public static void dml() { insert as user new Case(Subject='a', ProductId=null); }
  public static Integer soql() { return [SELECT Id, IsStopped FROM Case WITH USER_MODE].size(); }
}`)
		writeTestFile(t, filepath.Join(root, "force-app/main/default/classes/CaseFlsSec.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
		ok, output := caseFLSExec(root, `System.debug(CaseFlsSec.strip());`)
		if !ok || strings.Contains(output, "IsStopped") == granted {
			t.Errorf("granted=%v strip: ok=%v %s", granted, ok, output)
		}
		for _, call := range []string{"dml();", "soql();"} {
			ok, output := caseFLSExec(root, `CaseFlsSec.`+call)
			if ok != granted {
				t.Errorf("granted=%v %s ok=%v %s", granted, call, ok, output)
			}
		}
	}
}
