package apextest

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func projectMetadataGenerationRoutes() []struct {
	name string
	set  func(*project.Project, string)
} {
	return []struct {
		name string
		set  func(*project.Project, string)
	}{
		{"list-view", func(p *project.Project, path string) { p.ListViewFiles = []string{path} }},
		{"layout", func(p *project.Project, path string) { p.LayoutFiles = []string{path} }},
		{"flow", func(p *project.Project, path string) { p.FlowFiles = []string{path} }},
		{"permission-set-group", func(p *project.Project, path string) { p.PermissionSetGroupFiles = []string{path} }},
		{"business-hours", func(p *project.Project, path string) { p.BusinessHoursSettingsFiles = []string{path} }},
		{"duplicate-rule", func(p *project.Project, path string) { p.DuplicateRuleFiles = []string{path} }},
	}
}

func TestProjectMetadataGenerationCaptureRoutesRejectTraversal(t *testing.T) {
	for _, route := range projectMetadataGenerationRoutes() {
		t.Run(route.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			writeFile(t, filepath.Join(root, ".keep"), "")
			path := root + string(filepath.Separator) + "../outside.xml"
			writeFile(t, filepath.Clean(path), "outside")
			p := project.Project{Root: root}
			route.set(&p, path)
			sources := newSourceCache()
			sources.seedProjectSnapshot(&p)
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if err := sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: p.Root}}, &generation); err == nil {
				t.Fatal("project metadata capture accepted ../ traversal")
			}
		})
	}
}

func TestProjectMetadataGenerationFallbackRouteRejectsTraversal(t *testing.T) {
	root := filepath.Join(t.TempDir(), "project")
	writeFile(t, filepath.Join(root, ".keep"), "")
	path := root + string(filepath.Separator) + "../outside.xml"
	writeFile(t, filepath.Clean(path), "outside")
	sources := newSourceCache()
	sources.seedProjectSnapshot(&project.Project{Root: root})
	if _, err := sources.readCapturedProjectMetadata(path); err == nil {
		t.Fatal("uncaptured metadata reader accepted ../ traversal")
	}
}

func TestProjectMetadataGenerationCaptureAndFallbackRoutesRetainValidBytes(t *testing.T) {
	for _, route := range projectMetadataGenerationRoutes() {
		t.Run(route.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"list-view": "Owned.listView-meta.xml", "layout": "Owned.layout-meta.xml",
				"flow": "Owned.flow-meta.xml", "permission-set-group": "Owned.permissionsetgroup-meta.xml",
				"business-hours": "BusinessHours.settings-meta.xml", "duplicate-rule": "Owned.duplicateRule-meta.xml",
			}
			path := filepath.Join(root, files[route.name])
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{}`)
			const body = "<metadata>owned</metadata>\n"
			writeFile(t, path, body)
			p := project.Project{Root: root}
			route.set(&p, path)
			sources := newSourceCache()
			sources.seedProjectSnapshot(&p)
			if data, err := sources.readCapturedProjectMetadata(path); err != nil || string(data) != body {
				t.Fatalf("fallback bytes = %q, %v", data, err)
			}
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if err := sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: p.Root}}, &generation); err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, "changed")
			if data, err := sources.readCapturedProjectMetadata(path); err != nil || string(data) != body {
				t.Fatalf("snapshot bytes = %q, %v", data, err)
			}
			if input := generation.inputs[path]; !input.Present || input.Digest != sha256.Sum256([]byte(body)) {
				t.Fatalf("snapshot digest = %#v", input)
			}
		})
	}
}

func TestSourceUnitGenerationClassAndCustomMetadataRoutesRejectTraversal(t *testing.T) {
	for _, route := range []string{"class", "custom-metadata"} {
		t.Run(route, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			writeFile(t, filepath.Join(root, ".keep"), "")
			outside := root + string(filepath.Separator) + "../outside/Owned.cls"
			writeFile(t, filepath.Join(filepath.Dir(filepath.Clean(outside)), "sfdx-project.json"), `{}`)
			index := typesys.Index{Project: typesys.ProjectInfo{Root: root}}
			if route == "class" {
				writeFile(t, filepath.Clean(outside), "public class Owned {}")
				index = typesys.Build(project.Project{Root: root, ApexFiles: []string{outside, outside}}, schema.Schema{})
			} else {
				index.CustomMetadataRecords = []schema.CustomMetadataRecord{
					{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: outside},
					{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: outside},
				}
			}
			generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
			if _, err := captureRuntimeSourceUnits(index, &generation, map[string]string{}); err == nil {
				t.Fatal("source-unit marker discovery accepted ../ traversal")
			}
		})
	}
}

func TestSourceUnitGenerationMarkerRouteStopsAtIndexedRoot(t *testing.T) {
	parent := t.TempDir()
	writeFile(t, filepath.Join(parent, "sfdx-project.json"), `{}`)
	root := filepath.Join(parent, "source")
	file := filepath.Join(root, "Owned.cls")
	writeFile(t, file, "public class Owned {}")
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{file, file}}, schema.Schema{})
	generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
	units, err := captureRuntimeSourceUnits(index, &generation, map[string]string{})
	if err != nil {
		t.Fatal(err)
	}
	if units[file] != "file:"+file || len(generation.inputs) != 1 {
		t.Fatalf("marker discovery crossed indexed root: units=%v inputs=%v", units, generation.inputs)
	}
}

func TestProjectMetadataGenerationRunRoutesRejectTraversal(t *testing.T) {
	for _, route := range projectMetadataGenerationRoutes() {
		t.Run(route.name, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			writeFile(t, filepath.Join(root, ".keep"), "")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app", "classes", "OwnedBoundaryTest.cls"), `@IsTest private class OwnedBoundaryTest { @IsTest static void check() { System.assertEquals(1,1); } }`)
			index := loadTestIndex(t, root)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			path := root + string(filepath.Separator) + "../outside.xml"
			writeFile(t, filepath.Clean(path), "<metadata/>")
			route.set(&p, path)
			run := Run(index, Options{ProjectSnapshot: &p, NoDiskCache: true, Parallelism: 1})
			if problem := firstRunProblem(run); !strings.Contains(problem, "outside source root") {
				t.Fatalf("Run must reject escaped metadata before execution: summary=%#v problem=%q", run.Summary(), problem)
			}
		})
	}
}

func TestProjectMetadataGenerationDependencyAndShimRoutes(t *testing.T) {
	for _, route := range projectMetadataGenerationRoutes() {
		if route.name != "layout" && route.name != "business-hours" && route.name != "duplicate-rule" {
			continue
		}
		for _, kind := range []string{"dependency", "shim"} {
			if kind == "shim" && route.name == "layout" {
				continue // LayoutMetadataFiles includes loaded dependencies only.
			}
			for _, escape := range []bool{false, true} {
				name := "valid"
				if escape {
					name = "traversal"
				}
				t.Run(kind+"/"+route.name+"/"+name, func(t *testing.T) {
					base := t.TempDir()
					root, childRoot := filepath.Join(base, "project"), filepath.Join(base, "dependency")
					writeFile(t, filepath.Join(childRoot, ".keep"), "")
					path := filepath.Join(childRoot, "metadata.xml")
					if escape {
						path = childRoot + string(filepath.Separator) + "../outside.xml"
					}
					writeFile(t, filepath.Clean(path), "owned")
					child := project.Project{Root: childRoot}
					route.set(&child, path)
					p := project.Project{Root: root}
					if kind == "dependency" {
						p.ManagedPackageDependencies = []project.ManagedPackageDependency{{Status: "loaded", SourceRoot: childRoot, Project: &child}}
					} else {
						p.PackageShims = []project.PackageShim{{Status: "loaded", SourceRoot: childRoot, Project: &child}}
					}
					sources := newSourceCache()
					sources.seedProjectSnapshot(&p)
					data, err := sources.readCapturedProjectMetadata(path)
					if (err != nil) != escape || (!escape && string(data) != "owned") {
						t.Fatalf("dependency fallback bytes=%q err=%v escape=%v", data, err, escape)
					}
					generation := runtimeMetadataGeneration{inputs: map[string]typesys.ApexMetadataInput{}}
					err = sources.captureProjectRuntimeMetadata(typesys.Index{Project: typesys.ProjectInfo{Root: root}}, &generation)
					if (err != nil) != escape {
						t.Fatalf("dependency capture err=%v escape=%v", err, escape)
					}
				})
			}
		}
	}
}

func TestSourceUnitGenerationRunClassAndCustomMetadataRoutesRejectTraversal(t *testing.T) {
	for _, route := range []string{"class", "custom-metadata"} {
		t.Run(route, func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "project")
			writeFile(t, filepath.Join(root, ".keep"), "")
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "force-app", "classes", "OwnedSourceBoundaryTest.cls"), `@IsTest private class OwnedSourceBoundaryTest { @IsTest static void check() { System.assertEquals(1,1); } }`)
			index := loadTestIndex(t, root)
			outside := root + string(filepath.Separator) + "../outside/Owned.cls"
			writeFile(t, filepath.Clean(outside), "public class Owned {}")
			writeFile(t, filepath.Join(filepath.Dir(filepath.Clean(outside)), "sfdx-project.json"), `{}`)
			if route == "class" {
				extra := typesys.Build(project.Project{Root: root, ApexFiles: []string{outside, outside}}, schema.Schema{})
				index.Types = append(index.Types, extra.Types...)
			} else {
				index.CustomMetadataRecords = []schema.CustomMetadataRecord{
					{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: outside},
					{ObjectName: "Owned__mdt", DeveloperName: "Seed", File: outside},
				}
			}
			run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
			if problem := firstRunProblem(run); !strings.Contains(problem, "outside source root") {
				t.Fatalf("Run must reject escaped source-unit marker: summary=%#v problem=%q", run.Summary(), problem)
			}
		})
	}
}

func TestProjectMetadataGenerationRunExternalPackageRoutesRejectTraversal(t *testing.T) {
	for _, version := range []string{"62.0", "67.0"} {
		for _, route := range projectMetadataGenerationRoutes() {
			t.Run(version+"/"+route.name, func(t *testing.T) {
				root := filepath.Join(t.TempDir(), "project")
				writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true},{"path":"../shared"}]}`, version))
				writeFile(t, filepath.Join(root, "force-app", "classes", "OwnedBoundaryTest.cls"), `@IsTest private class OwnedBoundaryTest { @IsTest static void check() { System.assertEquals(1,1); } }`)
				writeFile(t, filepath.Join(root, "..", "shared", ".keep"), "")
				index := loadTestIndex(t, root)
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				// Retain the traversal out of the declared sibling package root
				// in the indexed metadata path supplied to the runner.
				path := root + "/../shared/../outside/" + route.name + ".xml"
				writeFile(t, filepath.Clean(path), "<metadata/>")
				route.set(&p, path)
				run := Run(index, Options{ProjectSnapshot: &p, NoDiskCache: true, Parallelism: 1})
				if problem := firstRunProblem(run); !strings.Contains(problem, "outside source root") {
					t.Fatalf("Run must reject escaped metadata before execution: summary=%#v problem=%q", run.Summary(), problem)
				}
			})
		}
	}
}

func TestProjectMetadataGenerationFallbackWithoutIndexedRootRejectsRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Owned.xml")
	writeFile(t, path, "owned")
	if _, err := newSourceCache().readCapturedProjectMetadata(path); err == nil {
		t.Fatal("uncaptured metadata reader accepted a path without an indexed root")
	}
}

func TestProjectMetadataGenerationLegacyProjectReaderRouteLoadsRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "Owned.layout-meta.xml")
	writeFile(t, path, "owned")
	sources := newSourceCache()
	if _, err := sources.capturedProjectSnapshot(root); err != nil {
		t.Fatal(err)
	}
	if data, err := sources.readCapturedProjectMetadata(path); err != nil || string(data) != "owned" {
		t.Fatalf("legacy reader bytes=%q err=%v", data, err)
	}
}

func TestProjectMetadataGenerationSourceRootPathBoundaries(t *testing.T) {
	root := filepath.Join(t.TempDir(), "source")
	for _, tc := range []struct {
		name, root, path string
		allowed          bool
	}{
		{"child", root, filepath.Join(root, "nested", "file.xml"), true},
		{"cleaned-child", root, root + "/nested/../file.xml", true},
		{"dot-prefixed-child", root, filepath.Join(root, "..owned", "file.xml"), true},
		{"parent", root, root + "/../file.xml", false},
		{"sibling-prefix", root, root + "-other/file.xml", false},
		{"missing-root", "", filepath.Join(root, "file.xml"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path, err := sourceRootPath(tc.root, tc.path)
			if (err == nil) != tc.allowed {
				t.Fatalf("sourceRootPath = %q, %v, allowed=%v", path, err, tc.allowed)
			}
			if tc.allowed && path != filepath.Clean(tc.path) {
				t.Fatalf("path = %q, want cleaned %q", path, tc.path)
			}
		})
	}
}
