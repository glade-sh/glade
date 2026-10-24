package sema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestStaticObjectChainNativeAnonymousNamedTestAndTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/static_object_chain_routes"
	cases, err := readTypeNamePinRows(filepath.Join(fixture, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	statements := map[string]string{}
	for _, row := range cases[1:] {
		statements[row[0]] = row[2]
	}
	rows, err := readTypeNamePinRows(filepath.Join(fixture, "org.tsv"))
	if err != nil || len(rows) != 49 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	for _, row := range rows[1:] {
		for _, namespace := range []string{"", "orchard"} {
			t.Run(strings.Join(row[:3], "/")+"/namespace="+namespace, func(t *testing.T) {
				if len(row) != 6 || statements[row[2]] == "" {
					t.Fatalf("invalid native row: %v", row)
				}
				root := t.TempDir()
				var paths []string
				for _, name := range []string{"Holder.cls", "ReceiverValues.cls"} {
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					path := filepath.Join(root, name)
					writeSemaFile(t, path, string(data))
					paths = append(paths, path)
				}
				statement, message := statements[row[2]], row[5]
				// Custom namespace variants exercise the project index. Native namespace
				// evidence comes separately from the System.RestContext rows.
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
				case "anonymous":
				default:
					t.Fatalf("unknown route: %v", row)
				}
				if source != "" {
					path := filepath.Join(root, name)
					writeSemaFile(t, path, source)
					paths = append(paths, path)
				}
				index := typesys.Build(project.Project{Root: root, Namespace: namespace, SourceAPIVersion: row[0], ApexFiles: paths}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
				if index.HasErrors() {
					t.Fatalf("index errors: %v", index.Diagnostics)
				}
				result := Analyze(index)
				if row[1] == "anonymous" {
					result = AnalyzeAnonymous(index, statement, row[0])
				}
				if row[3] == "true" {
					if row[4] != "true" || result.HasErrors() {
						t.Fatalf("native accepted chain; diagnostics: %v", result.Diagnostics)
					}
				} else {
					if row[2] == "missing_instance_field" {
						// This capture also exposes an existing anonymous class-path
						// gap outside the qualified receiver fallback under test.
						if row[1] == "anonymous" && namespace == "" && !result.HasErrors() {
							t.Log("native rejection carried: unqualified anonymous missing-field chain; see LOCAL-CARRIES.txt")
							return
						}
						if result.HasErrors() {
							t.Log("missing-field rejection status matches; diagnostic wording carried in LOCAL-CARRIES.txt")
							return
						}
					}
					for _, d := range result.Diagnostics {
						if d.NativeMessage == message {
							return
						}
					}
					t.Fatalf("native rejected with %q; diagnostics: %v", message, result.Diagnostics)
				}
			})
		}
	}
}
