package resource

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectReportDescriptionSnapshot(t *testing.T) {
	var old storage.ProjectReportMetadata
	if err := json.Unmarshal([]byte(`{"id":"00O000000000001","supported":true}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.Description != "" {
		t.Fatalf("old description %q", old.Description)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/reports/B6_Reports/B6_Accounts.report-meta.xml")
	report := func(desc string) string {
		return `<Report><name>B6 Accounts</name><description>` + desc + `</description><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns></Report>`
	}
	writeFile(t, path, report(`  Beta &amp; Gamma  `))
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
	if r := restored.Metadata.ProjectReports[0]; !r.Supported || r.Description != "Beta & Gamma" {
		t.Fatalf("restored = %#v", r)
	}
	writeFile(t, path, report(`  `))
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if r := org.Metadata.ProjectReports[0]; !r.Supported || r.Description != "" {
		t.Fatalf("reloaded = %#v", r)
	}
}
