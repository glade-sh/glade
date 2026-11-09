package vm

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestReportSecurityPermissionsNative(t *testing.T) {
	dir := "../resource/testdata/report_rows_capture/security-effective-permissions"
	b, e := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	boundary := "../resource/testdata/report_rows_capture/security-permission-boundaries"
	extra, e := os.ReadFile(filepath.Join(boundary, "org.tsv"))
	if e != nil {
		t.Fatal(e)
	}
	lines := append(strings.Split(strings.TrimSpace(string(b)), "\n")[1:], strings.Split(strings.TrimSpace(string(extra)), "\n")[1:]...)
	source, e := os.ReadFile(filepath.Join(dir, "security.apex"))
	if e != nil {
		t.Fatal(e)
	}
	n := 0
	for _, line := range lines {
		p := strings.SplitN(line, "\t", 4)
		if len(p) != 4 {
			t.Fatalf("invalid native row %q", line)
		}
		n++
		t.Run(strings.Join(p[:3], "/"), func(t *testing.T) {
			state := "probe"
			if parts := strings.SplitN(p[2], "-", 2); len(parts) == 2 && parts[0] != "security" {
				state = parts[0]
			}
			var flags []string
			raw, e := os.ReadFile(filepath.Join(boundary, "effective-"+state+".json"))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(raw, &flags); e != nil {
				t.Fatal(e)
			}
			root := t.TempDir()
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.standard@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
			report, e := os.ReadFile(filepath.Join(dir, "reports/B6_Reports/B6_Account.report-meta.xml"))
			if e != nil {
				t.Fatal(e)
			}
			writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Account.report-meta.xml"), string(report))
			def, e := project.Load(root)
			if e != nil {
				t.Fatal(e)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			if e = resource.ApplyProject(&org, def); e != nil {
				t.Fatal(e)
			}
			userID := storage.ID("005000000000099")
			profileID := storage.ID("00e000000000005")
			user := storage.Record{ID: userID, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(userID), "ProfileId": storage.IDValue(profileID), "Username": storage.StringValue("synthetic.standard@example.invalid")}}
			users := org.Objects["User"]
			users.Records[userID] = user
			org.Objects["User"] = users
			psID := storage.ID("0PS000000000099")
			fields := map[string]storage.Value{"Name": storage.StringValue("SyntheticEffectivePermissions")}
			for _, flag := range flags {
				fields[flag] = storage.BooleanValue(true)
			}
			sets := org.Objects["PermissionSet"]
			sets.Records[psID] = storage.Record{ID: psID, Object: "PermissionSet", Fields: fields}
			org.Objects["PermissionSet"] = sets
			assignments := org.Objects["PermissionSetAssignment"]
			aid := storage.ID("0Pa000000000099")
			assignments.Records[aid] = storage.Record{ID: aid, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(userID), "PermissionSetId": storage.IDValue(psID)}}
			org.Objects["PermissionSetAssignment"] = assignments
			m := New(nil)
			m.SetOrg(&org)
			if strings.HasSuffix(p[2], "-standard") {
				m.SetCurrentUser(user)
			}
			if p[1] == "seealldata" {
				m.EnableTestContext()
				m.SetTestSeeAllData(true)
			}
			program, e := CompileAnonymousWithOptions(string(source), CompileOptions{APIVersion: p[0]})
			if e != nil {
				t.Fatal(e)
			}
			out, e := m.Execute(program)
			if e != nil {
				t.Fatal(e)
			}
			if len(out.Debug) != 1 || out.Debug[0] != "B6SEC|"+p[3] {
				t.Fatalf("got %v; native %s", out.Debug, p[3])
			}
		})
	}
	if n != 40 {
		t.Fatalf("got %d native rows", n)
	}
}
