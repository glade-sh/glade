package lsp

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStaticObjectChainLSPNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/static_object_chain_routes"
	read := func(name string) [][]string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(fixture, name))
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
	statements := map[string]string{}
	for _, row := range read("cases.tsv")[1:] {
		statements[row[0]] = row[2]
	}
	rows := read("org.tsv")
	if len(rows) != 49 {
		t.Fatalf("native rows: %d", len(rows))
	}
	for _, row := range rows[1:] {
		if row[1] == "anonymous" {
			continue
		}
		for _, namespace := range []string{"", "orchard"} {
			t.Run(strings.Join(row[:3], "/")+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				for _, name := range []string{"Holder.cls", "ReceiverValues.cls"} {
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, name)
					if err := os.WriteFile(path, data, 0600); err != nil {
						t.Fatal(err)
					}
					paths = append(paths, path)
				}
				statement, message := statements[row[2]], row[5]
				if namespace != "" && row[2] != "missing_namespace" {
					statement = strings.ReplaceAll(statement, "ReceiverValues.", namespace+".ReceiverValues.")
					message = strings.ReplaceAll(message, "ReceiverValues.", namespace+".ReceiverValues.")
				}
				source, name := "", "ChainProbe.cls"
				switch row[1] {
				case "named":
					source = "public class ChainProbe { public static void observe() { " + statement + " } }"
				case "istest":
					source = "@IsTest private class ChainProbe { @IsTest static void observe() { " + statement + " } }"
				case "trigger":
					name = "ChainProbe.trigger"
					source = "trigger ChainProbe on Account (before insert) { " + statement + " }"
				default:
					t.Fatalf("unknown route: %v", row)
				}
				path := filepath.Join(root, name)
				if err := os.WriteFile(path, []byte(source), 0600); err != nil {
					t.Fatal(err)
				}
				paths = append(paths, path)
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
				if index.HasErrors() {
					t.Fatalf("index errors: %v", index.Diagnostics)
				}
				handler := NewHandler(index)
				defer handler.parser.Close()
				ds := handler.diagnosticsForDocument(uriFromPath(path))
				if row[3] == "true" {
					for _, d := range ds {
						if d.Severity == diagnostic.Error {
							t.Errorf("native accepted chain; LSP: %v", d)
						}
					}
				} else {
					for _, d := range ds {
						if row[2] == "missing_instance_field" && d.Severity == diagnostic.Error && strings.Contains(d.Message, ".Missing") {
							t.Log("missing-field rejection status matches; diagnostic wording carried in LOCAL-CARRIES.txt")
							return
						}
						if d.Severity == diagnostic.Error && d.NativeMessage == message {
							return
						}
					}
					t.Fatalf("native rejected with %q; LSP: %v", message, ds)
				}
			})
		}
	}
}
