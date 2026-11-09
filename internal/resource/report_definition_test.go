package resource

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectReportDefinitionSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports/B6_Reports")
	writeFile(t, filepath.Join(base, "B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
	writeFile(t, filepath.Join(base, "B6_Filtered.report-meta.xml"), `<Report><name>B6 Filtered</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns><filter><booleanFilter>1</booleanFilter></filter></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, r := range restored.Metadata.ProjectReports {
		found[r.DeveloperName] = true
		switch r.DeveloperName {
		case "B6_Accounts":
			if !r.Supported || len(r.Columns) != 1 || r.Columns[0] != "ACCOUNT.NAME" || r.Scope != "organization" {
				t.Fatalf("captured definition = %#v", r)
			}
		case "B6_Filtered":
			if r.Supported {
				t.Fatalf("unsupported filter was admitted: %#v", r)
			}
		}
	}
	if len(found) != 2 {
		t.Fatalf("definitions = %v", found)
	}
}

func TestProjectReportDefinitionReapply(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/reports/B6_Reports/B6_Accounts.report-meta.xml")
	writeFile(t, path, `<Report><name>Before</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, `<Report><name>After</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT_NUMBER</field></columns></Report>`)
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if len(org.Metadata.ProjectReports) != 1 {
		t.Fatalf("definitions after reapply: %d", len(org.Metadata.ProjectReports))
	}
	got := org.Metadata.ProjectReports[0]
	if got.Name != "After" || len(got.Columns) != 1 || got.Columns[0] != "ACCOUNT_NUMBER" {
		t.Fatalf("stale definition: %#v", got)
	}
}

func TestProjectReportShareSnapshot(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports")
	writeFile(t, filepath.Join(base, "B6_Reports.reportFolder-meta.xml"), `<ReportFolder><name>B6 Reports</name><folderShares><accessLevel>View</accessLevel><sharedTo>synthetic.user@example.invalid</sharedTo><sharedToType>User</sharedToType></folderShares></ReportFolder>`)
	writeFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><scope>organization</scope><columns><field>ACCOUNT.NAME</field></columns></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err = json.Unmarshal(raw, &restored); err != nil {
		t.Fatal(err)
	}
	if len(restored.Metadata.ProjectReports) != 1 || len(restored.Metadata.ReportFolderShares) != 1 {
		t.Fatalf("report/share metadata did not survive snapshot")
	}
	report, share := restored.Metadata.ProjectReports[0], restored.Metadata.ReportFolderShares[0]
	if report.FolderID == "" || report.FolderID != share.FolderID || share.SharedTo != "synthetic.user@example.invalid" || share.AccessLevel != "View" || share.SharedToType != "User" {
		t.Fatalf("restored share = %#v; report folder = %s", share, report.FolderID)
	}
}
