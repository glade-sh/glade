package resource

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestProjectReportTimeFrameSnapshot(t *testing.T) {
	var old storage.ProjectReportMetadata
	if err := json.Unmarshal([]byte(`{"id":"00O000000000001","supported":true}`), &old); err != nil {
		t.Fatal(err)
	}
	if old.DateColumn != "" || old.StartDate != "" || old.EndDate != "" {
		t.Fatalf("old date fields = %#v", old)
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	path := filepath.Join(root, "force-app/main/default/reports/B6_Reports/B6_Accounts.report-meta.xml")
	report := func(filter string) string {
		return `<Report><name>B6 Accounts</name><format>Tabular</format><reportType>AccountList</reportType><columns><field>ACCOUNT.NAME</field></columns>` + filter + `</Report>`
	}
	writeFile(t, path, report(`<timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval></timeFrameFilter>`))
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
	if r := restored.Metadata.ProjectReports[0]; !r.Supported || r.DateColumn != "CREATED_DATE" {
		t.Fatalf("restored = %#v", r)
	}
	writeFile(t, path, report(`<timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>2026-11-02</startDate><endDate>2026-11-03</endDate></timeFrameFilter>`))
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	data, err = json.Marshal(org)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if r := restored.Metadata.ProjectReports[0]; !r.Supported || r.StartDate != "2026-11-02" || r.EndDate != "2026-11-03" {
		t.Fatalf("bounded restored = %#v", r)
	}
	writeFile(t, path, report(`<timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval></timeFrameFilter>`))
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if r := org.Metadata.ProjectReports[0]; !r.Supported || r.StartDate != "" || r.EndDate != "" {
		t.Fatalf("unbounded reapply = %#v", r)
	}
	for _, filter := range []string{`<timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>INTERVAL_CUSTOM</interval><startDate>invalid</startDate></timeFrameFilter>`, `<timeFrameFilter><dateColumn>CREATED_DATE</dateColumn><interval>OTHER</interval></timeFrameFilter>`, `<timeFrameFilter><dateColumn>UNKNOWN</dateColumn><interval>INTERVAL_CUSTOM</interval></timeFrameFilter>`} {
		writeFile(t, path, report(filter))
		if err = ApplyProject(&org, p); err != nil {
			t.Fatal(err)
		}
		if r := org.Metadata.ProjectReports[0]; r.Supported {
			t.Fatalf("unsupported date filter accepted: %#v", r)
		}
	}
}
