package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectReportStandardUserCatalogNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/catalog-standard-user"
	b, e := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	flagsRaw, e := os.ReadFile(filepath.Join(dir, "effective-plain.json"))
	if e != nil {
		t.Fatal(e)
	}
	var flags []string
	if e = json.Unmarshal(flagsRaw, &flags); e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n")[1:] {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatal("invalid native row")
		}
		parts := strings.SplitN(p[2], "-", 2)
		if len(parts) != 2 {
			t.Fatal("invalid case")
		}
		actor, name := parts[0], parts[1]
		n++
		modes := []string{"explicit"}
		if actor == "standard" {
			modes = append(modes, "default-seed")
		}
		for _, mode := range modes {
			t.Run(strings.Join(p[:3], "/")+"/"+mode, func(t *testing.T) {
				root := t.TempDir()
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
				reportName := strings.ToUpper(name[:1]) + name[1:]
				raw, e := os.ReadFile(filepath.Join(dir, "reports/B6_Reports/B6_"+reportName+".report-meta.xml"))
				if e != nil {
					t.Fatal(e)
				}
				writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_"+reportName+".report-meta.xml"), string(raw))
				def, e := project.Load(root)
				if e != nil {
					t.Fatal(e)
				}
				org := storage.NewOrgState()
				for _, object := range []string{"Account", "Contact", "Opportunity", "User", "Profile", "PermissionSet", "PermissionSetAssignment"} {
					storage.EnsureStandardObject(&org, object)
				}
				if mode == "default-seed" {
					storage.EnsureDeterministicPlatformData(&org)
				}
				profiles := org.Objects["Profile"]
				for id, name := range map[storage.ID]string{storage.ID(storage.DefaultProfileID): "System Administrator", storage.ID("00e000000000005"): "Standard User"} {
					profiles.Records[id] = storage.Record{ID: id, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue(name)}}
				}
				org.Objects["Profile"] = profiles
				if e = resource.ApplyProject(&org, def); e != nil {
					t.Fatal(e)
				}
				uid := storage.ID("005000000000099")
				profileID := storage.ID("00e000000000005")
				if actor == "admin" {
					uid = storage.ID(storage.DefaultUserID)
					profileID = storage.ID(storage.DefaultProfileID)
				}
				user := storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(uid), "ProfileId": storage.IDValue(profileID), "Username": storage.StringValue("synthetic.user@example.invalid"), "TimeZoneSidKey": storage.StringValue("America/Los_Angeles"), "LocaleSidKey": storage.StringValue("en_US"), "LanguageLocaleKey": storage.StringValue("en_US")}}
				if actor == "standard" {
					users := org.Objects["User"]
					users.Records[uid] = user
					org.Objects["User"] = users
					if mode == "explicit" {
						psID := storage.ID("0PS000000000099")
						fields := map[string]storage.Value{"Name": storage.StringValue("SyntheticEffectivePermissions")}
						for _, flag := range flags {
							fields[flag] = storage.BooleanValue(true)
						}
						sets := org.Objects["PermissionSet"]
						sets.Records[psID] = storage.Record{ID: psID, Object: "PermissionSet", Fields: fields}
						org.Objects["PermissionSet"] = sets
						assigns := org.Objects["PermissionSetAssignment"]
						aid := storage.ID("0Pa000000000099")
						assigns.Records[aid] = storage.Record{ID: aid, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(psID)}}
						org.Objects["PermissionSetAssignment"] = assigns
					}
				}
				m := New(nil)
				m.SetOrg(&org)
				m.SetCurrentUser(user)
				m.fakeNow = time.Date(2026, 11, 4, 4, 0, 0, 0, time.UTC)
				if p[1] == "seealldata" || p[1] == "isolated" {
					m.EnableTestContext()
					m.SetTestSeeAllData(p[1] == "seealldata")
				}
				rid := ""
				for _, r := range org.Metadata.ProjectReports {
					if r.DeveloperName == "B6_"+reportName {
						rid = string(r.ID)
						break
					}
				}
				if rid == "" {
					t.Fatal("missing synthetic report")
				}
				getID := `Id rid=[SELECT Id FROM Report WHERE DeveloperName='B6_` + reportName + `' LIMIT 1].Id;`
				if p[1] == "isolated" {
					getID = `Id rid=Id.valueOf('` + rid + `');`
				}
				code := getID + `Reports.ReportTypeMetadata m=Reports.ReportManager.describeReport(rid).getReportTypeMetadata();System.assertNotEquals(null,m);System.debug('B6CAT|'+JSON.serialize(m));`
				if actor == "standard" && p[1] == "anonymous" {
					code = getID + `try{Reports.ReportManager.describeReport(rid);System.debug('B6REJ|ok');}catch(Exception e){System.debug('B6REJ|'+e.getTypeName()+'~'+e.getMessage());}`
				}
				program, e := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: p[0]})
				if e != nil {
					t.Fatal(e)
				}
				out, e := m.Execute(program)
				if e != nil {
					t.Fatal(e)
				}
				if actor == "standard" && p[1] == "anonymous" {
					if len(out.Debug) != 1 || out.Debug[0] != "B6REJ|"+p[3] {
						t.Fatalf("got %v, native %s", out.Debug, p[3])
					}
					return
				}
				if len(out.Debug) != 1 || !strings.HasPrefix(out.Debug[0], "B6CAT|") {
					t.Fatal("missing catalog output")
				}
				fileActor := actor
				if p[1] == "isolated" {
					fileActor = "isolated"
				}
				expected, e := os.ReadFile(filepath.Join(dir, fileActor+"-"+name+"-"+strings.TrimSuffix(p[0], ".0")+".json"))
				if e != nil {
					t.Fatal(e)
				}
				var got, want any
				if e = json.Unmarshal([]byte(strings.TrimPrefix(out.Debug[0], "B6CAT|")), &got); e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(expected, &want); e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("catalog differs from native %s %s", p[1], name)
				}
			})
		}
	}
	if n != 24 {
		t.Fatalf("got %d native rows", n)
	}
}

// An explicit object-permission overlay is not the captured untouched profile.
func TestReportDefaultStandardProfileOverrideGuard(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	user := storage.Record{ID: "005000000000099", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000099"), "ProfileId": storage.IDValue("00e000000000005")}}
	m := New(nil)
	m.SetOrg(&org)
	m.SetCurrentUser(user)
	m.EnableTestContext()
	if !m.projectReportCapturedStandardCatalog(m.testContext.CurrentUser) {
		t.Fatal("untouched standard profile did not match its captured baseline")
	}
	profileSet := storage.ID(m.profileOwnedPermissionSetID("00e000000000005"))
	if profileSet == "" {
		t.Fatal("Standard User has no profile-owned permission set")
	}
	// A project override of a seeded row (Account.Site read revoked) is not
	// the untouched profile either.
	for id, record := range org.Objects["FieldPermissions"].Records {
		parent, _ := record.GetField("ParentId")
		if field, _ := record.GetField("Field"); storage.IDsEqual(parent.ID, profileSet) && field.String == "Account.Site" {
			record.Fields["PermissionsRead"] = storage.BooleanValue(false)
			org.Objects["FieldPermissions"].Records[id] = record
			if m.projectReportCapturedStandardCatalog(m.testContext.CurrentUser) {
				t.Fatal("field-permission override of a seeded row was treated as the captured untouched profile")
			}
			record.Fields["PermissionsRead"] = storage.BooleanValue(true)
			org.Objects["FieldPermissions"].Records[id] = record
		}
	}
	if !m.projectReportCapturedStandardCatalog(m.testContext.CurrentUser) {
		t.Fatal("restored seeded row did not match the captured baseline again")
	}
	state := org.Objects["ObjectPermissions"]
	state.Records["110000000000099"] = storage.Record{ID: "110000000000099", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(profileSet), "SObjectType": storage.StringValue("Lead"), "PermissionsRead": storage.BooleanValue(false)}}
	org.Objects["ObjectPermissions"] = state
	if m.projectReportCapturedStandardCatalog(m.testContext.CurrentUser) {
		t.Fatal("object-permission override was treated as the captured untouched profile")
	}
	for _, kind := range []string{"field", "object"} {
		t.Run("explicit-"+kind, func(t *testing.T) {
			state := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&state)
			raw, err := os.ReadFile("../resource/testdata/report_rows_capture/catalog-standard-user/effective-plain.json")
			if err != nil {
				t.Fatal(err)
			}
			var flags []string
			if err = json.Unmarshal(raw, &flags); err != nil {
				t.Fatal(err)
			}
			ps := storage.ID("0PS000000000099")
			values := map[string]storage.Value{"Name": storage.StringValue("SyntheticEffectivePermissions")}
			for _, flag := range flags {
				values[flag] = storage.BooleanValue(true)
			}
			sets := state.Objects["PermissionSet"]
			sets.Records[ps] = storage.Record{ID: ps, Object: "PermissionSet", Fields: values}
			state.Objects["PermissionSet"] = sets
			assignments := state.Objects["PermissionSetAssignment"]
			aid := storage.ID("0Pa000000000099")
			assignments.Records[aid] = storage.Record{ID: aid, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(user.ID), "PermissionSetId": storage.IDValue(ps)}}
			state.Objects["PermissionSetAssignment"] = assignments
			machine := New(nil)
			machine.SetOrg(&state)
			machine.SetCurrentUser(user)
			machine.EnableTestContext()
			if !machine.projectReportCapturedStandardCatalog(machine.testContext.CurrentUser) {
				t.Fatal("captured plain permissions did not match")
			}
			anonymous := New(nil)
			anonymous.SetOrg(&state)
			anonymous.SetCurrentUser(user)
			if !anonymous.projectReportCapturedPermissionDenied(anonymous.executionUser) {
				t.Fatal("captured anonymous rejection did not match")
			}
			if kind == "field" {
				o := state.Objects["FieldPermissions"]
				id := storage.ID("0FP000000000099")
				o.Records[id] = storage.Record{ID: id, Object: "FieldPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(ps), "SObjectType": storage.StringValue("Account"), "Field": storage.StringValue("Account.Name"), "PermissionsRead": storage.BooleanValue(false)}}
				state.Objects["FieldPermissions"] = o
			} else {
				o := state.Objects["ObjectPermissions"]
				id := storage.ID("110000000000098")
				o.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(ps), "SObjectType": storage.StringValue("Contact"), "PermissionsRead": storage.BooleanValue(false)}}
				state.Objects["ObjectPermissions"] = o
			}
			if machine.projectReportCapturedStandardCatalog(machine.testContext.CurrentUser) {
				t.Errorf("%s permission override was treated as captured plain state in tests", kind)
			}
			if anonymous.projectReportCapturedPermissionDenied(anonymous.executionUser) {
				t.Errorf("%s permission override was treated as captured plain state anonymously", kind)
			}
		})
	}
}
