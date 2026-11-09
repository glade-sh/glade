package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestFolderItemsUseCapturedMetadata(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default")
	folder := filepath.Join(base, "reports/B6_Reports.reportFolder-meta.xml")
	report := filepath.Join(base, "reports/B6_Reports/B6_Accounts.report-meta.xml")
	dashboardFolder := filepath.Join(base, "dashboards/B6_Dashboards.dashboardFolder-meta.xml")
	dashboard := filepath.Join(base, "dashboards/B6_Dashboards/B6_Dashboard.dashboard-meta.xml")
	sources := map[string]string{folder: `<ReportFolder><name>B6 Reports</name></ReportFolder>`, report: `<Report><name>B6 Accounts</name><format>Tabular</format></Report>`, dashboardFolder: `<DashboardFolder><name>B6 Dashboards</name></DashboardFolder>`, dashboard: `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`}
	for path, source := range sources {
		writeFile(t, path, source)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for path := range sources {
		writeFile(t, path, `<broken`)
	}
	read := func(path string) ([]byte, error) {
		if source, ok := sources[path]; ok {
			return []byte(source), nil
		}
		return os.ReadFile(path)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p, read); err != nil {
		t.Fatal(err)
	}
	for _, r := range org.Objects["Report"].Records {
		if got := r.Fields["Name"].String; got != "B6 Accounts" {
			t.Fatalf("report name %q", got)
		}
	}
	for _, d := range org.Objects["Dashboard"].Records {
		if got := d.Fields["Title"].String; got != "B6 Dashboard" {
			t.Fatalf("dashboard title %q", got)
		}
	}
	if len(org.Objects["Report"].Records) != 1 || len(org.Objects["Dashboard"].Records) != 1 {
		t.Fatal("captured records missing")
	}
}
