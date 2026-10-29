package lsp

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
func TestReceiverMemberLSPNamedIsTestTriggerRoutes(t *testing.T) {
	bodies := map[string]string{}
	for _, row := range receiverMemberRows(t, "cases.tsv")[1:] {
		bodies[row[0]] = row[1]
	}
	rows := receiverMemberRows(t, "org.tsv")
	if len(rows) != 97 {
		t.Fatalf("native matrix has %d rows, want 96 plus header", len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		t.Run(strings.Join(row[:3], "/"), func(t *testing.T) {
			root := t.TempDir()
			var paths []string
			for _, name := range []string{"ReceiverOwner.cls", "ReceiverParent.cls", "GatewayService.cls"} {
				data, err := os.ReadFile(filepath.Join("../../testdata/conformance/receiver_member_routes", name))
				if err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(root, name)
				writeReceiverMemberFile(t, path, string(data))
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
				writeReceiverMemberFile(t, path, source)
				paths = append(paths, path)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
			if index.HasErrors() {
				t.Fatal(index.Diagnostics)
			}
			handler := NewHandler(index)
			defer handler.parser.Close()
			diagnostics := handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, name)))
			if row[3] == "true" {
				if len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				return
			}
			for _, d := range diagnostics {
				if d.NativeMessage == row[4] || d.Message == row[4] {
					return
				}
			}
			t.Fatalf("native diagnostic %q; LSP: %+v", row[4], diagnostics)
		})
	}
}

func writeReceiverMemberFile(t *testing.T, path, source string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
}

// Missing package symbols stay unresolved even if a local class has the same
// short name. This preserves editor behavior for incomplete dependency indexes.
func TestReceiverMemberLSPOpaquePackageNamedIsTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "istest"} {
			for _, collision := range []bool{false, true} {
				name := "missing-only"
				if collision {
					name = "local-type-collision"
				}
				t.Run(api+"/"+route+"/"+name, func(t *testing.T) {
					root := t.TempDir()
					path := filepath.Join(root, "ReceiverProbe.cls")
					source := "public class ReceiverProbe { missingns.Parcel.Options options; public void observe() { missingns.Parcel.Instance.save(); } }"
					if route == "istest" {
						source = "@IsTest private class ReceiverProbe { static missingns.Parcel.Options options; @IsTest static void observe() { missingns.Parcel.Instance.save(); } }"
					}
					writeReceiverMemberFile(t, path, source)
					paths := []string{path}
					if collision {
						other := filepath.Join(root, "Parcel.cls")
						writeReceiverMemberFile(t, other, "public class Parcel { public String local() { return 'local'; } }")
						paths = append(paths, other)
					}
					index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: paths}, schema.Schema{})
					handler := NewHandler(index)
					defer handler.parser.Close()
					if ds := handler.diagnosticsForDocument(uriFromPath(path)); len(ds) != 0 {
						t.Fatal(ds)
					}
				})
			}
		}
	}
}
