package apextest

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

const workspaceMetadataXML = `<CustomMetadata xmlns="http://soap.sforce.com/2006/04/metadata" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xmlns:xsd="http://www.w3.org/2001/XMLSchema"><label>Seed</label><protected>false</protected><values><field>Flag__c</field><value xsi:type="xsd:boolean">true</value></values></CustomMetadata>`

// A project that loads a sibling project as a package dependency must read the
// sibling's custom metadata, including when both projects declare a record
// with the same name and source-unit discovery runs.
//
// This is a local graph-routing regression: named-disk-project and
// named-project-snapshot are twins, with anonymous registration and all request
// registration/construction/compilation routes below, at source API 62.0/67.0.
// The snapshot route retains the indexed graph after disk discovery changes.
// Native generated-metadata results are independently pinned by existing rows:
// internal/apextest/testdata/conformance/metadata_field_names/org.tsv,
// cases custom-column, standard-column, entity, filtered-groups at source API
// 62.0/67.0, anonymous/istest (TestMetadataFieldNamesNativeConformance); and
// internal/apextest/testdata/conformance/setup_metadata/org.tsv,
// cases businessHours, duplicateRule, personDuplicateRule at source API
// 62.0/67.0, named @IsTest (TestSetupMetadataNativeConformance).
// ORG-RUN.txt in each directory records those native captures. These rows pin
// metadata results; they do not assert a Salesforce sibling-directory layout.
func TestMetadataGenerationReadsSiblingProjectCustomMetadata(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		for _, duplicate := range []bool{false, true} {
			name := "unique"
			if duplicate {
				name = "duplicate-record"
			}
			t.Run(version+"/"+name, func(t *testing.T) {
				workspace := t.TempDir()
				alpha := filepath.Join(workspace, "ProjAlpha")
				beta := filepath.Join(workspace, "ProjBeta")
				manifest := fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true,"package":"ProjAlpha","dependencies":[{"package":"ProjBeta"}]}]}`, version)
				manifestPath := filepath.Join(alpha, "sfdx-project.json")
				writeFile(t, manifestPath, manifest)
				writeFile(t, filepath.Join(beta, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"sfdx-source/beta/main","default":true,"package":"ProjBeta"}]}`, version))
				writeFile(t, filepath.Join(alpha, "force-app/main/default/classes/AlphaTest.cls"), `@IsTest private class AlphaTest { @IsTest static void check() { System.assertEquals(1, 1); } }`)
				betaSchema := filepath.Join(beta, "sfdx-source/beta/main/schema")
				writeFile(t, filepath.Join(betaSchema, "objects/Widget__mdt/Widget__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel></CustomObject>`)
				writeFile(t, filepath.Join(betaSchema, "objects/Widget__mdt/fields/Flag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
				writeFile(t, filepath.Join(betaSchema, "customMetadata/Widget__mdt.Seed.md-meta.xml"), workspaceMetadataXML)
				if duplicate {
					writeFile(t, filepath.Join(alpha, "force-app/main/default/objects/Widget__mdt/Widget__mdt.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel></CustomObject>`)
					writeFile(t, filepath.Join(alpha, "force-app/main/default/objects/Widget__mdt/fields/Flag__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Flag__c</fullName><label>Flag</label><type>Checkbox</type><defaultValue>false</defaultValue></CustomField>`)
					writeFile(t, filepath.Join(alpha, "force-app/main/default/customMetadata/Widget__mdt.Seed.md-meta.xml"), workspaceMetadataXML)
				}
				p, err := project.Load(alpha)
				if err != nil {
					t.Fatal(err)
				}
				loaded := false
				for _, dep := range p.ManagedPackageDependencies {
					loaded = loaded || (dep.Status == "loaded" && dep.Project != nil)
				}
				if !loaded {
					t.Fatalf("sibling dependency was not loaded: %#v %#v", p.ManagedPackageDependencies, p.DependencyDiagnostics)
				}
				index := loadTestIndex(t, alpha)
				for _, route := range []string{"named-disk-project", "named-project-snapshot"} {
					t.Run(route, func(t *testing.T) {
						opts := Options{NoDiskCache: true, Parallelism: 1}
						if route == "named-project-snapshot" {
							opts = workspaceProjectOptions(&p)
							// The supplied graph must survive dependency discovery changing
							// on disk after indexing, including duplicate record markers.
							writeFile(t, manifestPath, strings.ReplaceAll(manifest, `,"dependencies":[{"package":"ProjBeta"}]`, ""))
							defer writeFile(t, manifestPath, manifest)
						}
						run := Run(index, opts)
						if problem := firstRunProblem(run); strings.Contains(problem, "outside source root") || strings.Contains(problem, "CompileError") {
							t.Fatalf("sibling project metadata rejected: %q", problem)
						}
						if s := run.Summary(); s.Failed != 0 || s.Passed == 0 {
							t.Fatalf("summary = %#v", s)
						}
					})
				}
				for _, route := range []struct {
					name string
					run  func() error
				}{
					{"anonymous-registration", func() error { return RegisterProjectRuntime(vm.New(nil), index) }},
					{"request-registration", func() error { return RegisterProjectRuntimeForRequest(vm.New(nil), index) }},
					{"new-request-runtime", func() error { _, err := NewProjectRuntimeForRequest(index); return err }},
					{"uncached-request-registration", func() error { return RegisterUncachedProjectRuntimeForRequest(vm.New(nil), index) }},
					{"request-compilation", func() error { _, err := CompileProjectRuntimeForRequestWithSourceDigests(index, nil); return err }},
				} {
					t.Run(route.name, func(t *testing.T) {
						if err := route.run(); err != nil {
							t.Fatalf("sibling project metadata rejected: %v", err)
						}
					})
				}
			})
		}
	}
}

func workspaceProjectOptions(p *project.Project) Options {
	return Options{ProjectSnapshot: p, NoDiskCache: true, Parallelism: 1}
}

func TestMetadataGenerationProjectSnapshotOptionRoute(t *testing.T) {
	p := project.Project{Root: t.TempDir()}
	opts := workspaceProjectOptions(&p)
	field := reflect.ValueOf(opts).FieldByName("ProjectSnapshot")
	if !field.IsValid() || field.Type() != reflect.TypeOf(&p) {
		t.Fatal("Options must expose ProjectSnapshot *project.Project")
	}
	if field.Interface() != &p {
		t.Fatal("Options lost the supplied project snapshot")
	}
}

func TestSourceUnitGenerationLoadedWorkspaceRootRoutes(t *testing.T) {
	workspace := t.TempDir()
	primary := project.Project{Root: filepath.Join(workspace, "consumer")}
	dependency := project.Project{Root: filepath.Join(workspace, "dependency")}
	nested := project.Project{Root: filepath.Join(dependency.Root, "nested")}
	shim := project.Project{Root: filepath.Join(workspace, "shim")}
	unloaded := project.Project{Root: filepath.Join(workspace, "unloaded")}
	primary.ManagedPackageDependencies = []project.ManagedPackageDependency{
		{Status: "loaded", Project: &dependency}, {Status: "unavailable", Project: &unloaded},
	}
	dependency.PackageShims = []project.PackageShim{{Status: "loaded", Project: &nested}}
	nested.ManagedPackageDependencies = []project.ManagedPackageDependency{{Status: "loaded", Project: &primary}}
	primary.PackageShims = []project.PackageShim{{Status: "loaded", Project: &shim}}
	roots := loadedProjectRoots(primary)
	index := typesys.Index{Project: typesys.ProjectInfo{Root: primary.Root}}
	for _, p := range []*project.Project{&primary, &dependency, &nested, &shim} {
		file := filepath.Join(p.Root, "customMetadata", "Widget.Seed.md-meta.xml")
		writeFile(t, filepath.Join(p.Root, "sfdx-project.json"), `{}`)
		index.CustomMetadataRecords = append(index.CustomMetadataRecords, schema.CustomMetadataRecord{
			ObjectName: "Widget__mdt", DeveloperName: "Seed", File: file,
		})
		t.Run(filepath.Base(p.Root), func(t *testing.T) {
			if got := containingSourceRoot(file, primary.Root, roots); got != p.Root {
				t.Fatalf("containing root = %q, want deepest loaded root %q", got, p.Root)
			}
		})
	}
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	units, err := captureRuntimeSourceUnitsInRoots(index, &generation, map[string]string{}, roots)
	if err != nil {
		t.Fatal(err)
	}
	for _, record := range index.CustomMetadataRecords {
		root := filepath.Dir(filepath.Dir(record.File))
		if got, want := units[record.File], runtimeSourceTreeKey(root, record.File); got != want {
			t.Fatalf("source unit = %q, want %q", got, want)
		}
	}
	for _, file := range []string{
		filepath.Join(unloaded.Root, "record.xml"),
		dependency.Root + "/../outside/record.xml",
		dependency.Root + "-other/record.xml",
	} {
		root := containingSourceRoot(file, primary.Root, roots)
		if _, err := sourceRootPath(root, file); err == nil {
			t.Fatalf("unloaded or escaped source accepted: %q", file)
		}
	}
}
