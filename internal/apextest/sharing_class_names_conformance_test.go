package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/testreport"
)

// A class's name never selects its sharing mode. Classes named WithSharing
// (inner and top-level) behave like neutrally named ones; only the keyword and
// the API-dependent default decide. Each native @IsTest row runs alone with its
// own companions, object and permission set. Only the terminal observation
// assertion is replaced for local execution.
func TestSharingClassNamesOrgConformance(t *testing.T) {
	type nativeRow struct {
		ID                                  string
		APIVersions, Twins                  []string
		DeniedPayload                       bool
		Classes, ClassAPIVersions, Expected map[string]string
	}
	var data struct {
		APIVersions, Routes      []string
		AbsentRoutes             []struct{ Route, Reason string }
		Object                   string
		Metadata, PermissionSets map[string]string
		Rows                     []nativeRow
		ProfileProjection        struct {
			NativeFullName, ProfileName, Path, Scope string
			APIVersions                              []string
		}
		Carried, ExcludedCaptures []struct {
			ID, Reason  string
			APIVersions []string
		}
	}
	raw, err := os.ReadFile("testdata/conformance/sharing_class_names.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,65.0,66.0,67.0,68.0" || strings.Join(data.Routes, ",") != "isTest" || len(data.Rows) != 12 ||
		data.Object != "ShareCase__c" || len(data.Metadata) != 2 || data.PermissionSets["granted"] == "" || data.PermissionSets["denied"] == "" {
		t.Fatalf("native matrix shape: APIs=%v routes=%v rows=%d", data.APIVersions, data.Routes, len(data.Rows))
	}
	if len(data.AbsentRoutes) != 1 || data.AbsentRoutes[0].Route != "anonymous" ||
		!strings.Contains(data.AbsentRoutes[0].Reason, "System.runAs") || !strings.Contains(data.AbsentRoutes[0].Reason, "compile-only") ||
		!strings.Contains(data.AbsentRoutes[0].Reason, "Inner types are not allowed to have inner types") {
		t.Fatal("missing anonymous route boundary")
	}
	// Native metadata fullName Standard resolves by Profile Id to the data Name
	// Standard User. The existing importer selects that data Name from this
	// fixture filepath. Import only the captured Payload false entry; this is
	// not a rule for Name, Result or unmentioned profile fields.
	profile := data.ProfileProjection
	const payloadProfile = "<?xml version='1.0' encoding='utf-8'?>\n<Profile xmlns=\"http://soap.sforce.com/2006/04/metadata\"><fieldPermissions><editable>false</editable><field>ShareCase__c.Payload__c</field><readable>false</readable></fieldPermissions></Profile>\n"
	if profile.NativeFullName != "Standard" || profile.ProfileName != "Standard User" ||
		profile.Path != "profiles/Standard User.profile" || profile.Scope == "" ||
		strings.Join(profile.APIVersions, ",") != "62.0,67.0,68.0" || data.Metadata[profile.Path] != payloadProfile {
		t.Fatal("invalid finite native Payload profile projection")
	}
	if len(data.Carried) != 0 || len(data.ExcludedCaptures) != 0 {
		t.Fatal("captured FLS rows must assert native answers without exemptions")
	}
	var ids []string
	pairs := 0
	for _, row := range data.Rows {
		ids = append(ids, row.ID)
		if len(row.Twins) == 0 || row.Classes["ShareCaseTest"] == "" || len(row.Expected) != len(row.APIVersions) {
			t.Fatalf("invalid native row: %s", row.ID)
		}
		// The fresh capture adds only selected APIs; no missing pair is inferred.
		wantAPIs := "62.0,65.0,66.0,67.0"
		switch row.ID {
		case "R011", "R012", "R018", "R036":
			wantAPIs += ",68.0"
		case "F001", "R015", "R019", "R020":
			wantAPIs = "62.0,67.0,68.0"
		}
		if strings.Join(row.APIVersions, ",") != wantAPIs {
			t.Fatalf("captured APIs for %s: %v", row.ID, row.APIVersions)
		}
		for _, api := range row.APIVersions {
			if expected := row.Expected[api]; expected == "" || expected == "?" || !strings.HasSuffix(expected, ";with=0;without=2;user=0") {
				t.Fatalf("uncaptured or uncontrolled %s at %s: %q", row.ID, api, expected)
			}
			if row.ID == "F001" && row.Expected[api] != "observed=default=1;userMode=1;systemMode=1;with=0;without=2;user=0" {
				t.Fatalf("invalid captured granted-Payload control at %s", api)
			}
			pairs++
		}
	}
	// R034-R037 decide the name rule; R006, R011 and R012 are their twins.
	// R018-R020 retain denied-FLS preconditions. F001 checks the same profile
	// before and after an additive Payload grant with all three query modes.
	if strings.Join(ids, " ") != "F001 R006 R011 R012 R015 R018 R019 R020 R034 R035 R036 R037" {
		t.Fatalf("native rows: %v", ids)
	}
	if pairs != 48 {
		t.Fatalf("captured row/API pairs: %d", pairs)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			for _, row := range data.Rows {
				expected, captured := row.Expected[api]
				if !captured {
					continue
				}
				t.Run("isTest/"+row.ID, func(t *testing.T) {
					root := t.TempDir()
					writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
					base := filepath.Join(root, "force-app/main/default")
					for path, source := range data.Metadata {
						writeFile(t, filepath.Join(base, path), source)
					}
					permission := data.PermissionSets["granted"]
					if row.DeniedPayload {
						permission = data.PermissionSets["denied"]
					}
					writeFile(t, filepath.Join(base, "permissionsets/ShareCaseAccess.permissionset"), permission)
					terminal := "System.assert(false,'P|" + row.ID + "|observed='+value+';with='+visibleWith+';without='+visibleWithout+';user='+visibleUser);"
					if strings.Count(row.Classes["ShareCaseTest"], terminal) != 1 {
						t.Fatal("missing captured @IsTest observation assertion")
					}
					observed := "'observed='+value+';with='+visibleWith+';without='+visibleWithout+';user='+visibleUser"
					assertion := "System.assert(" + conformanceApexString(expected) + ".equals(" + observed + "),'" + row.ID + " actual <'+" + observed + "+'>');"
					var names []string
					for name := range row.Classes {
						names = append(names, name)
					}
					sort.Strings(names)
					for _, name := range names {
						source := row.Classes[name]
						if name == "ShareCaseTest" {
							source = strings.Replace(source, terminal, assertion, 1)
						}
						version := api
						if fixed := row.ClassAPIVersions[name]; fixed != "" {
							version = fixed
						}
						path := filepath.Join(base, "classes", name+".cls")
						writeFile(t, path, source)
						writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+version+"</apiVersion></ApexClass>")
					}
					run := Run(loadTestIndex(t, root), Options{NoDiskCache: true, Parallelism: 1})
					got := run.Summary()
					if got.Total != 1 {
						t.Fatalf("summary = %#v", got)
					}
					if got.Passed != 1 {
						for _, suite := range run.Suites {
							for _, result := range suite.Cases {
								if result.Status != testreport.StatusPass {
									t.Errorf("%s.%s native <%s>: status %s problem %#v", result.ClassName, result.MethodName, expected, result.Status, result.Problem)
								}
							}
						}
						t.Fatalf("summary = %#v", got)
					}
				})
			}
		})
	}
}
