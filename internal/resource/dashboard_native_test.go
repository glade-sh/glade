package resource

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

func TestDashboardMetadataNativeRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	cases := []struct{ folder, label, name, title string }{
		{"B6_Dashboards", "B6 Dashboards", "B6_Dashboard", "B6 Dashboard"},
		{"B6_Another", "Another B6", "B6_Other_Dashboard", "B6 Other Dashboard"},
	}
	for _, c := range cases {
		base := filepath.Join(root, "force-app/main/default/dashboards")
		writeFile(t, filepath.Join(base, c.folder+".dashboardFolder-meta.xml"), `<DashboardFolder><name>`+c.label+`</name></DashboardFolder>`)
		writeFile(t, filepath.Join(base, c.folder, c.name+".dashboard-meta.xml"), `<Dashboard><title>`+c.title+`</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
	}
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["Dashboard"].Records
	for _, c := range cases {
		found := false
		for _, r := range rows {
			if r.Fields["DeveloperName"].String != c.name {
				continue
			}
			found = true
			for field, want := range map[string]string{"Title": c.title, "FolderName": c.label, "Type": "LoggedInUser"} {
				if got := r.Fields[field].String; got != want {
					t.Errorf("%s %s = %q, want %q", c.name, field, got, want)
				}
			}
			folder, ok := org.Objects["Folder"].Records[r.Fields["FolderId"].ID]
			if !ok || folder.Fields["DeveloperName"].String != c.folder {
				t.Errorf("%s folder reference = %#v", c.name, r.Fields["FolderId"])
			}
		}
		if !found {
			t.Errorf("dashboard %s not found", c.name)
		}
	}
}

func TestDashboardLegacyMetadataWarmIdentity(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"src","default":true}]}`)
	base := filepath.Join(root, "src/dashboards")
	writeFile(t, filepath.Join(base, "B6_Dashboards-meta.xml"), `<DashboardFolder><name>B6 Dashboards</name></DashboardFolder>`)
	writeFile(t, filepath.Join(base, "B6_Dashboards/B6_Dashboard.dashboard"), `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	var before storage.ID
	for id := range org.Objects["Dashboard"].Records {
		before = id
	}
	if before == "" {
		t.Fatal("dashboard missing")
	}
	// A new folder sorting earlier must not rebind existing project records.
	writeFile(t, filepath.Join(base, "B6_Aardvark-meta.xml"), `<DashboardFolder><name>B6 Aardvark</name></DashboardFolder>`)
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["Dashboard"].Records
	if len(rows) != 1 {
		t.Fatalf("dashboard count after reload = %d", len(rows))
	}
	if _, ok := rows[before]; !ok {
		t.Fatalf("dashboard identity changed after reload")
	}
}

func TestDashboardsWithoutFolderMetadataAreIndexed(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/dashboards/B6_Dashboards")
	writeFile(t, filepath.Join(base, "B6_DashboardOnly.dashboard-meta.xml"), `<Dashboard><title>B6 Dashboard Only</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
	writeFile(t, filepath.Join(base, "B6_Dashboard.dashboard-meta.xml"), `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"B6_DashboardOnly": "B6 Dashboard Only", "B6_Dashboard": "B6 Dashboard"}
	for _, d := range org.Objects["Dashboard"].Records {
		name := d.Fields["DeveloperName"].String
		if expected, ok := want[name]; ok {
			if got := d.Fields["Title"].String; got != expected {
				t.Errorf("%s title %q want %q", name, got, expected)
			}
			delete(want, name)
		}
	}
	if len(want) != 0 {
		t.Fatalf("missing dashboards: %v", want)
	}
}

func TestDashboardMoveBetweenFoldersNative(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	base := filepath.Join(root, "force-app/main/default/dashboards")
	for folder, label := range map[string]string{"B6_Dashboards": "B6 Dashboards", "B6_Another": "Another B6"} {
		writeFile(t, filepath.Join(base, folder+".dashboardFolder-meta.xml"), `<DashboardFolder><name>`+label+`</name></DashboardFolder>`)
	}
	original := filepath.Join(base, "B6_Dashboards/B6_Dashboard.dashboard-meta.xml")
	writeFile(t, original, `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(original); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(base, "B6_Another/B6_Dashboard.dashboard-meta.xml"), `<Dashboard><title>B6 Dashboard</title><dashboardType>LoggedInUser</dashboardType></Dashboard>`)
	p, err = project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if err = ApplyProject(&org, p); err != nil {
		t.Fatal(err)
	}
	rows := org.Objects["Dashboard"].Records
	if len(rows) != 1 {
		t.Fatalf("dashboards after move: %d", len(rows))
	}
	for _, r := range rows {
		if got := r.Fields["FolderName"].String; got != "Another B6" {
			t.Fatalf("folder after move: %q", got)
		}
	}
}
