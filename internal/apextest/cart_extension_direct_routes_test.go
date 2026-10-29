package apextest

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func cartRouteRows(t *testing.T, name string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/conformance/cart_extension_routes", name))
	if err != nil {
		t.Fatal(err)
	}
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// This synthetic source project exercises compilation and test execution with
// direct test/trigger bodies from cart_extension_routes/org.tsv:2-49.
func TestCartExtensionDirectProjectNamedIsTestTriggerNativeRoutes(t *testing.T) {
	t.Parallel()
	bodies := map[string]string{}
	for _, row := range cartRouteRows(t, "cases.tsv")[1:] {
		bodies[row[0]] = row[2]
	}
	rows := cartRouteRows(t, "org.tsv")
	if len(rows) != 49 {
		t.Fatalf("native rows=%d, want 48 plus header", len(rows))
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		key := strings.Join(row[:3], "/")
		if seen[key] || (row[0] != "62.0" && row[0] != "67.0") || bodies[row[2]] == "" {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true

		if row[1] == "anonymous" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+row[0]+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			write := func(name, source string) {
				dir, kind := "classes", "ApexClass"
				if strings.HasSuffix(name, ".trigger") {
					dir, kind = "triggers", "ApexTrigger"
				}
				path := filepath.Join(root, "force-app/main/default", dir, name)
				writeFile(t, path, source)
				writeFile(t, path+"-meta.xml", "<"+kind+"><apiVersion>"+row[0]+"</apiVersion><status>Active</status></"+kind+">")
			}
			body, invocation := bodies[row[2]], "GardenCartProbe.observe();"
			switch row[1] {
			case "named":
				write("GardenCartProbe.cls", "public class GardenCartProbe { public static void observe() { "+body+" } }")
			case "istest":
				write("GardenCartProbe.cls", "@IsTest private class GardenCartProbe { @IsTest static void observe() { "+body+" } }")
				invocation = ""
			case "trigger":
				write("GardenCartProbe.trigger", "trigger GardenCartProbe on Account (before insert) { "+body+" }")
				invocation = "insert new Account(Name='Garden');"
			default:
				t.Fatal("unknown route")
			}
			if row[3] == "accepted" && invocation != "" {
				write("GardenCartRunnerTest.cls", "@IsTest private class GardenCartRunnerTest { @IsTest static void observe() { "+invocation+" } }")
			}
			write("GardenCartControlTest.cls", "@IsTest private class GardenCartControlTest { @IsTest static void independent() { System.assert(true); } }")
			index := loadTestIndex(t, root)
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			for _, mode := range []struct {
				name     string
				parallel int
			}{{"serial-cold", 1}, {"serial-warm", 1}, {"parallel", 2}} {
				t.Run(mode.name, func(t *testing.T) {
					run := Run(index, Options{NoDiskCache: true, Parallelism: mode.parallel})
					summary := run.Summary()
					if row[3] == "accepted" {
						if summary.Total != 2 || summary.Passed != 2 || len(run.ExcludedClasses) != 0 {
							t.Fatalf("native accepted; summary=%+v excluded=%+v problem=%s", summary, run.ExcludedClasses, firstRunProblem(run))
						}
						return
					}
					if row[3] != "rejected" || row[4] == "" {
						t.Fatal("invalid rejection")
					}
					want := strings.Split(row[4], " | ")[0]
					for _, entry := range run.ExcludedClasses {
						matches := strings.Contains(entry.Message, want)
						// Native rejections retain Glade's diagnostic wording in this compile gate.
						if row[2] == "list_constructor" {
							matches = matches || strings.Contains(entry.Message, "no matching CartExtension.CartItemList constructor")
						}
						// String controls: org.tsv:17-19,25,41-43,49.
						if row[2] == "list_string_argument" {
							matches = matches || strings.Contains(entry.Message, `invalid collection call "add"`)
						}
						if entry.ClassName == "GardenCartProbe" && matches {
							if summary.Passed != 1 {
								t.Fatalf("independent control did not pass: %+v", summary)
							}
							return
						}
					}
					t.Fatalf("native rejected with %q; excluded=%+v summary=%+v", want, run.ExcludedClasses, summary)
				})
			}
		})
	}
}
