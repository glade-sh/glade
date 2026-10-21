package apextest

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
)

// A packageDirectories entry (from sfdx-project.json or glade.yml
// project.packageDirs) may sit outside the project root. Its metadata is
// loaded and indexed, so runtime metadata generation must read it too.
func TestMetadataGenerationReadsPackageDirectoryOutsideProjectRoot(t *testing.T) {
	for _, route := range []string{"sfdx", "glade-yml"} {
		for _, mode := range []string{"classonly", "cmdt", "listview", "layout", "permsetgroup"} {
			t.Run(route+"/"+mode, func(t *testing.T) {
				ws := t.TempDir()
				app := filepath.Join(ws, "app")
				shared := filepath.Join(ws, "shared")
				if route == "sfdx" {
					writeFile(t, filepath.Join(app, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true},{"path":"../shared"}]}`)
				} else {
					writeFile(t, filepath.Join(app, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
					writeFile(t, filepath.Join(app, "glade.yml"), "project:\n  packageDirs: [force-app, ../shared]\n")
				}
				writeFile(t, filepath.Join(app, "force-app/main/default/classes/AppTest.cls"), `@IsTest private class AppTest { @IsTest static void check() { System.assertEquals(1, SharedThing.one()); } }`)
				writeFile(t, filepath.Join(shared, "main/default/classes/SharedThing.cls"), `public class SharedThing { public static Integer one() { return 1; } }`)
				def := filepath.Join(shared, "main/default")
				switch mode {
				case "cmdt":
					writeFile(t, filepath.Join(def, "objects/Widget__mdt/Widget__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel></CustomObject>`)
					writeFile(t, filepath.Join(def, "objects/Widget__mdt/fields/Flag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
					writeFile(t, filepath.Join(def, "customMetadata/Widget__mdt.Seed.md-meta.xml"), workspaceMetadataXML)
				case "listview":
					writeFile(t, filepath.Join(def, "objects/Account/listViews/All_Shared.listView-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ListView xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>All_Shared</fullName><filterScope>Everything</filterScope><label>All Shared</label></ListView>`)
				case "layout":
					writeFile(t, filepath.Join(def, "layouts/Account-Shared Layout.layout-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><Layout xmlns="http://soap.sforce.com/2006/04/metadata"><layoutSections><label>Information</label><style>TwoColumnsTopToBottom</style><layoutColumns><layoutItems><behavior>Edit</behavior><field>Name</field></layoutItems></layoutColumns></layoutSections></Layout>`)
				case "permsetgroup":
					writeFile(t, filepath.Join(def, "permissionsetgroups/Shared_Group.permissionsetgroup-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><PermissionSetGroup xmlns="http://soap.sforce.com/2006/04/metadata"><label>Shared Group</label><description>Shared</description></PermissionSetGroup>`)
				}
				index := loadTestIndex(t, app)
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				if problem := firstRunProblem(run); strings.Contains(problem, "outside source root") || strings.Contains(problem, "CompileError") {
					t.Fatalf("package directory outside project root rejected: %q", problem)
				}
				if s := run.Summary(); s.Failed != 0 || s.Passed == 0 {
					t.Fatalf("summary = %#v problem=%q", s, firstRunProblem(run))
				}
			})
		}
	}
}

// Paths outside every loaded root, including ../ traversal out of a package
// root, stay rejected.
func TestMetadataPathOutsideEveryPackageRootRejected(t *testing.T) {
	ws := t.TempDir()
	app := filepath.Join(ws, "app")
	writeFile(t, filepath.Join(app, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true},{"path":"../shared"}]}`)
	writeFile(t, filepath.Join(app, "force-app/main/default/classes/AppTest.cls"), `@IsTest private class AppTest { @IsTest static void check() { System.assertEquals(1, 1); } }`)
	writeFile(t, filepath.Join(ws, "shared/main/default/classes/SharedThing.cls"), `public class SharedThing {}`)
	writeFile(t, filepath.Join(ws, "other/secret.xml"), `<x/>`)
	p, err := project.Load(app)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		filepath.Join(ws, "other/secret.xml"),
		filepath.Join(ws, "shared/../other/secret.xml"),
		filepath.Join(ws, "shared2/x.xml"),
	} {
		if _, err := readProjectMetadataFile(p, path); err == nil {
			t.Fatalf("expected %q to be rejected", path)
		}
	}
}
