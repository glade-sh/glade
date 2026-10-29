package sema

import (
	"encoding/csv"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
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

// Native direct-body matrix: cart_extension_routes/org.tsv:2-49 and ORG-RUN.txt.
// In particular, a test or trigger contains the operation itself, not a helper call.
func TestCartExtensionDirectAnonymousNamedIsTestTriggerNativeRoutes(t *testing.T) {
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

		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			body := bodies[row[2]]
			name, source := "GardenCartProbe.cls", "public class GardenCartProbe { public static void observe() { "+body+" } }"
			switch row[1] {
			case "anonymous", "named":
			case "istest":
				source = "@IsTest private class GardenCartProbe { @IsTest static void observe() { " + body + " } }"
			case "trigger":
				name, source = "GardenCartProbe.trigger", "trigger GardenCartProbe on Account (before insert) { "+body+" }"
			default:
				t.Fatal("unknown route: " + row[1])
			}
			var paths []string
			if row[1] != "anonymous" {
				path := filepath.Join(root, name)
				writeSemaFile(t, path, source)
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			result := Analyze(index)
			if row[1] == "anonymous" {
				result = AnalyzeAnonymous(index, body, row[0])
			}
			if row[3] == "accepted" {
				if result.HasErrors() {
					t.Fatalf("native accepted; diagnostics=%+v", result.Diagnostics)
				}
				return
			}
			if row[3] != "rejected" || row[4] == "" {
				t.Fatal("invalid rejection")
			}
			want := strings.Split(row[4], " | ")[0]
			for _, d := range result.Diagnostics {
				// Native rows pin compile rejection; Glade retains its diagnostic wording.
				if row[2] == "list_constructor" && d.Code == "GLADESEMA011" && strings.Contains(d.Message, "CartExtension.CartItemList constructor") {
					return
				}
				// String controls: org.tsv:17-19,25,41-43,49.
				if row[2] == "list_string_argument" && d.Code == "GLADESEMA023" && strings.Contains(d.Message, `"add"`) {
					return
				}
				if d.NativeMessage == want || d.Message == want {
					return
				}
			}
			t.Fatalf("native rejected with %q; diagnostics=%+v", want, result.Diagnostics)
		})
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"anonymous", "named", "istest", "trigger"} {
			for id := range bodies {
				if !seen[api+"/"+route+"/"+id] {
					t.Errorf("missing native cell: %s/%s/%s", api, route, id)
				}
			}
		}
	}
}
