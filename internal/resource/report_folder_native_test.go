package resource

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestReportFolderDisplayNameNative(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	for _, c := range []struct{ folder, label, report string }{
		{"B6_Reports", "B6 Reports", "B6_Accounts"},
		{"B6_Other", "Other B6", "B6_Summary"},
	} {
		base := filepath.Join(root, "force-app/main/default/reports")
		writeFile(t, filepath.Join(base, c.folder+".reportFolder-meta.xml"), `<ReportFolder><name>`+c.label+`</name></ReportFolder>`)
		writeFile(t, filepath.Join(base, c.folder, c.report+".report-meta.xml"), `<Report><name>Example</name><format>Tabular</format></Report>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err := ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"B6_Accounts": "B6 Reports", "B6_Summary": "Other B6"}
	for _, report := range org.Objects["Report"].Records {
		name := report.Fields["DeveloperName"].String
		if got := report.Fields["FolderName"].String; got != want[name] {
			t.Errorf("%s FolderName = %q, want %q", name, got, want[name])
		}
		delete(want, name)
	}
	if len(want) != 0 {
		t.Fatalf("missing reports: %v", want)
	}
}

func TestReportIdentitySurvivesFolderLabelChange(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports")
	folder := filepath.Join(base, "B6_Reports.reportFolder-meta.xml")
	writeFile(t, folder, `<ReportFolder><name>B6 Reports</name></ReportFolder>`)
	writeFile(t, filepath.Join(base, "B6_Reports/B6_Accounts.report-meta.xml"), `<Report><name>B6 Accounts</name><format>Tabular</format></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	var before storage.ID
	for id := range org.Objects["Report"].Records {
		before = id
	}
	if before == "" {
		t.Fatal("missing report")
	}
	writeFile(t, folder, `<ReportFolder><name>B6 Renamed</name></ReportFolder>`)
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["Report"].Records
	if len(rows) != 1 {
		t.Fatalf("reports after rename: %d", len(rows))
	}
	r, ok := rows[before]
	if !ok {
		t.Fatal("report ID changed")
	}
	if got := r.Fields["FolderName"].String; got != "B6 Renamed" {
		t.Fatalf("folder name = %q", got)
	}
}

func TestReportsWithoutFolderMetadataAreIndexed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/reports/B6_Reports")
	writeFile(t, filepath.Join(base, "B6_ReportOnly.report-meta.xml"), `<Report><name>B6 Report Only</name><format>Tabular</format><reportType>AccountList</reportType></Report>`)
	writeFile(t, filepath.Join(base, "B6_Contacts.report-meta.xml"), `<Report><name>B6 Contacts</name><format>Tabular</format><reportType>ContactList</reportType></Report>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"B6_ReportOnly": "B6 Report Only", "B6_Contacts": "B6 Contacts"}
	for _, r := range org.Objects["Report"].Records {
		name := r.Fields["DeveloperName"].String
		if expected, ok := want[name]; ok {
			if got := r.Fields["Name"].String; got != expected {
				t.Errorf("%s name %q want %q", name, got, expected)
			}
			delete(want, name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing reports: %v", want)
	}
}
