package vm

import (
	"fmt"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
	"path/filepath"
	"testing"
)

func TestReportScopeNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
			for name, scope := range map[string]string{"B6_NoScope": "", "B6_Accounts": "<scope>organization</scope>"} {
				writeVMTestFile(t, filepath.Join(base, "B6_Reports", name+".report-meta.xml"), `<Report><name>`+name+`</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns>`+scope+`</Report>`)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			code := `for(Report r:[SELECT Id,DeveloperName FROM Report]){Reports.ReportMetadata m=Reports.ReportManager.describeReport(r.Id).getReportMetadata();if(r.DeveloperName=='B6_NoScope')System.assertEquals('user',m.getScope());if(r.DeveloperName=='B6_Accounts')System.assertEquals('organization',m.getScope());}`
			program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportScopeOwnerVisibilityNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "test"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
				base := filepath.Join(root, "force-app/main/default/reports")
				writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
				for name, scope := range map[string]string{"B6_NoScope": "", "B6_Accounts": "<scope>organization</scope>"} {
					writeVMTestFile(t, filepath.Join(base, "B6_Reports", name+".report-meta.xml"), `<Report><name>`+name+`</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns>`+scope+`</Report>`)
				}
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				org := storage.NewOrgState()
				storage.EnsureStandardObject(&org, "Account")
				storage.EnsureStandardObject(&org, "User")
				users := org.Objects["User"]
				users.Records[storage.ID(storage.DefaultUserID)] = storage.Record{ID: storage.ID(storage.DefaultUserID), Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(storage.DefaultUserID)}}
				org.Objects["User"] = users
				accounts := org.Objects["Account"]
				for i, name := range []string{"B6 First", "B6 Second", "B6 Other Owner", "Sample Account for Entitlements"} {
					id := storage.ID(fmt.Sprintf("001%012d", i+1))
					owner := storage.ID(storage.DefaultUserID)
					if i >= 2 {
						owner = storage.ID("005000000000002")
					}
					accounts.Records[id] = storage.Record{ID: id, Object: "Account", Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(name), "OwnerId": storage.IDValue(owner)}}
				}
				org.Objects["Account"] = accounts
				if err = resource.ApplyProject(&org, p); err != nil {
					t.Fatal(err)
				}
				machine := New(nil)
				machine.SetOrg(&org)
				if route == "test" {
					machine.EnableTestContext()
					machine.SetTestSeeAllData(true)
				}
				code := `for(Report r:[SELECT Id,DeveloperName FROM Report]){Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);Decimal n=(Decimal)x.getFactMap().get('T!T').getAggregates()[0].getValue();if(r.DeveloperName=='B6_NoScope')System.assertEquals(2,n);if(r.DeveloperName=='B6_Accounts')System.assertEquals(4,n);}`
				program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err = machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestReportRestrictedSharingNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
			for name, scope := range map[string]string{"B6_NoScope": "", "B6_Accounts": "<scope>organization</scope>"} {
				writeVMTestFile(t, filepath.Join(base, "B6_Reports", name+".report-meta.xml"), `<Report><name>`+name+`</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns>`+scope+`</Report>`)
			}
			writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_Contacts.report-meta.xml"), `<Report><name>B6 Contacts</name><format>Tabular</format><reportType>ContactList</reportType><scope>organization</scope><columns><field>LAST_NAME</field></columns></Report>`)
			writeVMTestFile(t, filepath.Join(base, "B6_Reports", "B6_Contacts_User.report-meta.xml"), `<Report><name>B6 Contacts User</name><format>Tabular</format><reportType>ContactList</reportType><scope>user</scope><columns><field>LAST_NAME</field></columns></Report>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			account := org.Objects["Account"]
			account.Definition.SharingModel = "Private"
			account.Records = map[storage.ID]storage.Record{}
			for i := 0; i < 4; i++ {
				id := storage.ID(fmt.Sprintf("001%012d", i+1))
				owner := storage.ID("005000000000001")
				if i == 0 {
					owner = "005000000000002"
				}
				account.Records[id] = storage.Record{ID: id, Object: "Account", System: storage.SystemFields{OwnerID: owner}, Fields: map[string]storage.Value{"Id": storage.IDValue(id), "Name": storage.StringValue(fmt.Sprintf("B6 %d", i)), "OwnerId": storage.IDValue(owner)}}
			}
			org.Objects["Account"] = account
			storage.EnsureStandardObject(&org, "Contact")
			contacts := org.Objects["Contact"]
			contacts.Records = map[storage.ID]storage.Record{}
			for i := 1; i <= 2; i++ {
				id := storage.ID(fmt.Sprintf("003%012d", i))
				parent := storage.ID(fmt.Sprintf("001%012d", i))
				contacts.Records[id] = storage.Record{ID: id, Object: "Contact", System: storage.SystemFields{OwnerID: "005000000000001"}, Fields: map[string]storage.Value{"Id": storage.IDValue(id), "LastName": storage.StringValue("B6"), "AccountId": storage.IDValue(parent)}}
			}
			org.Objects["Contact"] = contacts
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			m := New(nil)
			m.SetOrg(&org)
			m.SetCurrentUser(storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000002"), "Username": storage.StringValue("synthetic.user@example.invalid")}})
			m.EnableTestContext()
			m.SetTestSeeAllData(true)
			code := `for(Report r:[SELECT Id,DeveloperName FROM Report]){Reports.ReportResults x=Reports.ReportManager.runReport(r.Id,false);System.assertEquals(1,x.getFactMap().get('T!T').getAggregates()[0].getValue());}`
			program, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestReportFolderUserSharingNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			base := filepath.Join(root, "force-app/main/default/reports")
			for folder, shares := range map[string]string{"B6_Shared": "<folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares>", "B6_Private": "", "B6_Edit": "<folderShares><accessLevel>EditAllContents</accessLevel><sharedTo>synthetic.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares>", "B6_Manage": "<folderShares><accessLevel>Manage</accessLevel><sharedTo>synthetic.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares>"} {
				writeVMTestFile(t, filepath.Join(base, folder+".reportFolder-meta.xml"), `<ReportFolder><name>`+folder+`</name>`+shares+`</ReportFolder>`)
				writeVMTestFile(t, filepath.Join(base, folder, folder+"_Report.report-meta.xml"), `<Report><name>`+folder+` Report</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			if err = resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			user := storage.Record{ID: "005000000000002", Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000002"), "Username": storage.StringValue("synthetic.user@example.invalid")}}
			storage.EnsureStandardObject(&org, "User")
			users := org.Objects["User"]
			users.Records[user.ID] = user
			org.Objects["User"] = users
			m := New(nil)
			m.SetOrg(&org)
			m.SetCurrentUser(user)
			m.EnableTestContext()
			m.SetTestSeeAllData(true)
			code := `for(Report r:[SELECT Id,DeveloperName FROM Report]){Boolean hidden=r.DeveloperName=='B6_Private_Report';Boolean d=false;Boolean x=false;try{Reports.ReportManager.describeReport(r.Id);}catch(System.NoDataFoundException e){d=true;}try{Reports.ReportManager.runReport(r.Id,false);}catch(System.NoDataFoundException e){x=true;}System.assertEquals(hidden,d);System.assertEquals(hidden,x);}`
			prog, err := CompileAnonymousWithOptions(code, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			if _, err = m.Execute(prog); err != nil {
				t.Fatal(err)
			}
		})
	}
}
