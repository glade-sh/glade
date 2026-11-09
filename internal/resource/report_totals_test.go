package resource

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectReportTotalsSnapshot(t *testing.T) {
	var old storage.ProjectReportMetadata
	if err := json.Unmarshal([]byte(`{"id":"00O000000000001","supported":true}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.HideGrandTotal || old.HideSubTotals {
		t.Fatalf("old totals = %#v", old)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/reports/B6_Reports/B6_Accounts.report-meta.xml")
	report := func(flags string) string {
		return `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns>` + flags + `</Report>`
	}
	writeFile(t, path, report(`<showGrandTotal>false</showGrandTotal><showSubTotals>true</showSubTotals>`))
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	var restored storage.OrgState
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if r := restored.Metadata.ProjectReports[0]; !r.Supported || !r.HideGrandTotal || r.HideSubTotals {
		t.Fatalf("restored = %#v", r)
	}
	writeFile(t, path, report(`<showGrandTotal>true</showGrandTotal><showSubTotals>false</showSubTotals>`))
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if r := org.Metadata.ProjectReports[0]; !r.Supported || r.HideGrandTotal || !r.HideSubTotals {
		t.Fatalf("reloaded = %#v", r)
	}
	writeFile(t, path, report(`<showGrandTotal>invalid</showGrandTotal>`))
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if r := org.Metadata.ProjectReports[0]; r.Supported {
		t.Fatalf("invalid flag accepted = %#v", r)
	}
}
