package sema

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func receiverMemberRows(t *testing.T, name string) [][]string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("../../testdata/conformance/receiver_member_routes", name))
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

// The exact diagnostic matters: a recognized field token with a missing method
// must not be diagnosed as an unknown qualified variable.
func TestReceiverMemberNativeAnonymousNamedIsTestTriggerRoutes(t *testing.T) {
	bodies := map[string]string{}
	for _, row := range receiverMemberRows(t, "cases.tsv")[1:] {
		bodies[row[0]] = row[1]
	}
	rows := receiverMemberRows(t, "org.tsv")
	if len(rows) != 97 {
		t.Fatalf("native matrix has %d rows, want 96 plus header", len(rows))
	}
	for _, row := range rows[1:] {
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for _, name := range []string{"ReceiverOwner.cls", "ReceiverParent.cls", "GatewayService.cls"} {
				data, err := os.ReadFile(filepath.Join("../../testdata/conformance/receiver_member_routes", name))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, name)
				writeSemaFile(t, path, string(data))
				paths = append(paths, path)
			}
			body := bodies[row[2]]
			if body == "" {
				t.Fatal("missing captured body")
			}
			name, source := "ReceiverProbe.cls", "public class ReceiverProbe { public static void observe() { "+body+" } }"
			if row[1] == "istest" {
				source = "@IsTest private class ReceiverProbe { @IsTest static void observe() { " + body + " } }"
			}
			if row[1] == "trigger" {
				name = "ReceiverProbe.trigger"
				source = "trigger ReceiverProbe on Account (before insert) { " + body + " }"
			}
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
			if row[3] == "true" {
				if result.HasErrors() {
					t.Fatal(result.Diagnostics)
				}
				return
			}
			for _, d := range result.Diagnostics {
				if d.NativeMessage == row[4] || d.Message == row[4] {
					return
				}
			}
			t.Fatalf("native diagnostic %q; Glade: %+v", row[4], result.Diagnostics)
		})
	}
}

// An unavailable package declaration establishes only its own receiver boundary.
// These are offline compatibility controls, not native package parity claims.
func TestReceiverMemberOpaquePackageNamedIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "istest"} {
			for _, collision := range []bool{false, true} {
				name := "missing-only"
				if collision {
					name = "local-type-collision"
				}
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					source := "public class ReceiverProbe { missingns.Parcel.Options options; public void observe() { missingns.Parcel.Instance.save(); } }"
					if route == "istest" {
						source = "@IsTest private class ReceiverProbe { static missingns.Parcel.Options options; @IsTest static void observe() { missingns.Parcel.Instance.save(); } }"
					}
					files := map[string]string{"ReceiverProbe.cls": source}
					if collision {
						files["Parcel.cls"] = "public class Parcel { public String local() { return 'local'; } }"
					}
					result := analyzeDeclarationProjectWithAPIVersion(t, files, api)
					if result.HasErrors() {
						t.Fatal(result.Diagnostics)
					}
				})
			}
		}
	}
}
