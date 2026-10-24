package project

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestLoadPreservesNativeDuplicateApexClassCases(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "source_collisions", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(records) < 2 || strings.Join(records[0], "\t") != "case_id\tapi_version\tproject_dir\tclass_name\tdeploy_status\tclass_count\tdeployed_marker" {
		t.Fatalf("unexpected native case table header or rows: %d", len(records))
	}

	for _, record := range records[1:] {
		if len(record) != 7 {
			t.Fatalf("native case row has %d fields, want 7", len(record))
		}
		caseID, apiVersion, projectDir, className := record[0], record[1], record[2], record[3]
		if record[4] != "Succeeded" || record[5] != "1" || record[6] == "" {
			t.Fatalf("native case %s does not record the expected single deployed class", caseID)
		}
		t.Run(caseID, func(t *testing.T) {
			projectRoot := filepath.Join("testdata", "source_collisions", projectDir)
			p, err := Load(projectRoot)
			if err != nil {
				t.Fatalf("Load returned an error for API %s case %s: %v", apiVersion, caseID, err)
			}
			if len(p.ApexFiles) != 2 {
				t.Fatalf("API %s case %s loaded %d class files, want both source copies: %#v", apiVersion, caseID, len(p.ApexFiles), p.ApexFiles)
			}
			var sourcePaths []string
			for _, path := range p.ApexFiles {
				if filepath.Base(path) != className+".cls" {
					t.Fatalf("API %s case %s loaded unexpected class source %q", apiVersion, caseID, path)
				}
				relative, err := filepath.Rel(p.Root, path)
				if err != nil {
					t.Fatal(err)
				}
				sourcePaths = append(sourcePaths, filepath.ToSlash(relative))
			}
			sort.Strings(sourcePaths)
			if sourcePaths[0] == sourcePaths[1] {
				t.Fatalf("API %s case %s source paths are not distinct: %#v", apiVersion, caseID, sourcePaths)
			}
			packageDirs := []string{"alpha", "zulu"}
			if !strings.Contains(className, "62") && !strings.Contains(className, "67") {
				packageDirs = []string{"current", "legacy"}
			}
			if len(sourcePaths) != len(packageDirs) {
				t.Fatalf("API %s case %s sources = %#v, want %d", apiVersion, caseID, sourcePaths, len(packageDirs))
			}
			for _, packageDir := range packageDirs {
				want := "packages/" + packageDir + "/main/default/classes/" + className + ".cls"
				found := false
				for _, path := range sourcePaths {
					if path == want {
						found = true
						break
					}
				}
				if !found {
					t.Fatalf("API %s case %s source paths = %#v, missing %q", apiVersion, caseID, sourcePaths, want)
				}
			}
		})
	}
}

func TestLoadUsesGladePackageDirsToSelectActiveApexSources(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"packages/legacy","default":true},{"path":"packages/current"}]}`)
	writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  packageDirs: [packages/current]\n")
	writeFile(t, filepath.Join(root, "packages/legacy/main/default/classes/CollisionProbe.cls"), "public class CollisionProbe {}")
	writeFile(t, filepath.Join(root, "packages/current/main/default/classes/CollisionProbe.cls"), "public class CollisionProbe {}")

	p, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.ApexFiles) != 1 || !strings.HasSuffix(filepath.ToSlash(p.ApexFiles[0]), "packages/current/main/default/classes/CollisionProbe.cls") {
		t.Fatalf("active Apex files = %#v, want only the configured package directory", p.ApexFiles)
	}
}

// Route twins: manifest discovery and explicit glade.yml package directories.
func TestLoadNativeDuplicateClassWinnerWarningsByRoute(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "source_collisions", "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range records[1:] {
		for _, route := range []string{"sfdx_manifest", "glade_package_dirs"} {
			t.Run(row[0]+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				if err := os.CopyFS(root, os.DirFS(filepath.Join("testdata", "source_collisions", row[2]))); err != nil {
					t.Fatal(err)
				}
				if route == "glade_package_dirs" {
					dirs := "packages/legacy, packages/current"
					if row[6] == "zulu" {
						dirs = "packages/zulu, packages/alpha"
					}
					writeFile(t, filepath.Join(root, "glade.yml"), "project:\n  packageDirs: ["+dirs+"]\n")
				}
				p, err := Load(root)
				if err != nil {
					t.Fatal(err)
				}
				if len(p.ApexFiles) != 2 {
					t.Fatalf("sources = %v, want both copies", p.ApexFiles)
				}
				winner := filepath.Join("packages", row[6], "main", "default", "classes", row[3]+".cls")
				if len(p.SourceWarnings) != 1 || !strings.Contains(p.SourceWarnings[0], "selected "+filepath.ToSlash(winner)) {
					t.Errorf("warning must identify native winner: %v", p.SourceWarnings)
				}
			})
		}
	}
}

func TestSelectedApexFilesKeepsInventoryAndSamePackageDiagnostics(t *testing.T) {
	root := t.TempDir()
	file := func(path string) string { return filepath.Join(root, filepath.FromSlash(path)) }
	earlier, later := file("alpha/classes/Shared.cls"), file("zulu/classes/Shared.cls")
	unique, trigger := file("alpha/classes/Unique.cls"), file("alpha/triggers/Shared.trigger")
	p := Project{Root: root, PackageDirectories: []PackageDirectory{{Path: "alpha"}, {Path: "zulu"}}, ApexFiles: []string{later, unique, earlier, trigger}}
	original := append([]string(nil), p.ApexFiles...)
	if got, want := p.SelectedApexFiles(), []string{later, unique, trigger}; !reflect.DeepEqual(got, want) {
		t.Fatalf("selected = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(p.ApexFiles, original) {
		t.Fatal("selection mutated the complete inventory")
	}
	p.PackageDirectories = []PackageDirectory{{Path: "."}}
	if got := p.SelectedApexFiles(); !reflect.DeepEqual(got, original) {
		t.Fatalf("same-package duplicates must remain diagnosable: %v", got)
	}
}
