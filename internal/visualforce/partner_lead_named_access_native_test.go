package visualforce_test

import (
	"encoding/csv"
	"html"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
	"github.com/glade-sh/glade/internal/vm"
)

// NATIVE testdata/partner_lead_named_access/org.tsv:2-3 visualforce 59.0 67.0.
func TestPartnerLeadNamedAccessNativeConformance(t *testing.T) {
	base := filepath.Join("testdata", "partner_lead_named_access")
	f, err := os.Open(filepath.Join(base, "org.tsv"))
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
	for _, row := range rows[1:] {
		row := row
		t.Run(row[1], func(t *testing.T) {
			root := t.TempDir()
			write := func(path string, data []byte) {
				t.Helper()
				path = filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, data, 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", []byte(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`))
			src := filepath.Join(base, "project")
			if err := filepath.WalkDir(src, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				write(filepath.Join("force-app/main/default", rel), data)
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			runtime, err := apextest.CompileProjectRuntimeForRequestWithSourceDigests(typesys.Build(p, sch), nil)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = "67.0"
			for _, name := range []string{"User", "Profile", "Lead", "LeadShare", "PermissionSet", "PermissionSetAssignment", "ObjectPermissions"} {
				storage.EnsureStandardObject(&org, name)
			}
			profiles := org.Objects["Profile"]
			profiles.Records = map[storage.ID]storage.Record{
				"00e000000000001": {ID: "00e000000000001", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("System Administrator")}},
				"00e000000000002": {ID: "00e000000000002", Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Partner Community User")}},
			}
			org.Objects["Profile"] = profiles
			users := org.Objects["User"]
			users.Records = map[storage.ID]storage.Record{
				"005000000000001": {ID: "005000000000001", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001"), "UserType": storage.StringValue("Standard")}},
				"005000000000002": {ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000002"), "Alias": storage.StringValue("rtprt"), "UserType": storage.StringValue("PowerPartner")}},
				"005000000000003": {ID: "005000000000003", Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue("00e000000000001"), "UserType": storage.StringValue("Standard")}},
			}
			org.Objects["User"] = users
			leads := org.Objects["Lead"]
			leads.Definition.SharingModel = "ReadWriteTransfer"
			leads.Records = map[storage.ID]storage.Record{
				"00Q000000000001": {ID: "00Q000000000001", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000003", CreatedByID: "005000000000001"}, Fields: map[string]storage.Value{"LastName": storage.StringValue("Route First")}},
				"00Q000000000002": {ID: "00Q000000000002", Object: "Lead", System: storage.SystemFields{OwnerID: "005000000000003", CreatedByID: "005000000000001"}, Fields: map[string]storage.Value{"LastName": storage.StringValue("Route Second")}},
			}
			org.Objects["Lead"] = leads
			shares := org.Objects["LeadShare"]
			shares.Records = map[storage.ID]storage.Record{"00T000000000001": {ID: "00T000000000001", Object: "LeadShare", Fields: map[string]storage.Value{"LeadId": storage.IDValue("00Q000000000001"), "UserOrGroupId": storage.IDValue("005000000000002"), "LeadAccessLevel": storage.StringValue("Read"), "RowCause": storage.StringValue("Manual")}}}
			org.Objects["LeadShare"] = shares
			sets := org.Objects["PermissionSet"]
			sets.Records = map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Object: "PermissionSet"}}
			org.Objects["PermissionSet"] = sets
			assign := org.Objects["PermissionSetAssignment"]
			assign.Records = map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000002"), "PermissionSetId": storage.IDValue("0PS000000000001")}}}
			org.Objects["PermissionSetAssignment"] = assign
			perm := org.Objects["ObjectPermissions"]
			perm.Records = map[storage.ID]storage.Record{"110000000000001": {ID: "110000000000001", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SObjectType": storage.StringValue("Lead"), "PermissionsRead": storage.BooleanValue(true)}}}
			org.Objects["ObjectPermissions"] = perm
			machine := vm.New(nil)
			machine.SetOrg(&org)
			machine.SetCurrentUser(users.Records["005000000000001"])
			if err := apextest.RegisterCompiledProjectRuntimeForRequest(machine, runtime); err != nil {
				t.Fatal(err)
			}
			visualforce.SetVMRenderEnvironment(machine, p)
			defer visualforce.ClearVMRenderEnvironment(machine)
			page := "RouteProjection" + strings.TrimSuffix(row[1], ".0")
			program, err := vm.CompileAnonymousWithOptions("System.debug(new PageReference('/apex/"+page+"').getContent().toString());", vm.CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatal(err)
			}
			result, err := machine.Execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Debug) != 1 {
				t.Fatalf("rendered debug rows=%d, want 1", len(result.Debug))
			}
			rendered := html.UnescapeString(result.Debug[0])
			index := strings.Index(rendered, "OBS|")
			if index < 0 {
				t.Fatal("rendered page contains no observation")
			}
			got := strings.SplitN(rendered[index+len("OBS|"):], "<", 2)[0]
			if got != row[5] {
				t.Fatalf("Glade=%q native=%q", got, row[5])
			}
		})
	}
}
