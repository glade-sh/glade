package vm

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

func TestReportMetadataSnapshotCompatibility(t *testing.T) {
	root := t.TempDir()
	writeVMTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports")
	writeVMTestFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
	writeVMTestFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := testDataOrg()
	if err = resource.ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		t.Run(map[bool]string{false: "current", true: "legacy"}[legacy], func(t *testing.T) {
			raw, err := json.Marshal(org)
			if err != nil {
				t.Fatal(err)
			}
			if legacy {
				var envelope map[string]any
				if err = json.Unmarshal(raw, &envelope); err != nil {
					t.Fatal(err)
				}
				if metadata, ok := envelope["metadata"].(map[string]any); ok {
					delete(metadata, "projectReports")
				}
				raw, err = json.Marshal(envelope)
				if err != nil {
					t.Fatal(err)
				}
			}
			var restored storage.OrgState
			if err = json.Unmarshal(raw, &restored); err != nil {
				t.Fatal(err)
			}
			if legacy {
				if err = resource.ApplyProject(&restored, p); err != nil {
					t.Fatal(err)
				}
			}
			machine := New(nil)
			machine.SetOrg(&restored)
			program, err := CompileAnonymous(`Report r=[SELECT Id FROM Report WHERE DeveloperName='B6_Accounts' LIMIT 1];System.assertEquals('B6 Accounts',Reports.ReportManager.describeReport(r.Id).getReportMetadata().getName());`)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
