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

func TestStaticFieldReceiverNativeAndNamespacedAnonymousNamedTestTriggerRoutes(t *testing.T) {
	const fixture = "../../testdata/conformance/static_field_receiver_routes"
	rows, err := readTypeNamePinRows(filepath.Join(fixture, "org.tsv"))
	if err != nil || len(rows) != 9 {
		t.Fatalf("native rows: %d, %v", len(rows), err)
	}
	files := map[string]string{"anonymous": "anonymous.apex", "named": "StaticFieldReceiverProbe.cls", "istest": "StaticFieldReceiverProbeTest.cls", "trigger": "StaticFieldReceiverProbeTrigger.trigger"}
	for _, row := range rows[1:] {
		if len(row) != 4 || row[2] != "true" || row[3] != "true" || files[row[1]] == "" {
			t.Fatalf("invalid native row: %v", row)
		}
		for _, namespace := range []string{"", "orchard"} {
			t.Run(row[0]+"/"+row[1]+"/namespace="+namespace, func(t *testing.T) {
				root := t.TempDir()
				var paths []string
				var anonymous string
				for _, name := range []string{"ReceiverValues.cls", "StaticFieldReceiverProbe.cls", files[row[1]]} {
					if name == "StaticFieldReceiverProbe.cls" && row[1] == "named" && len(paths) == 2 {
						continue
					}
					data, err := os.ReadFile(filepath.Join(fixture, name))
					if err != nil {
						t.Fatal(err)
					}
					source := string(data)
					if namespace != "" {
						source = strings.ReplaceAll(source, "ReceiverValues.Text", namespace+".ReceiverValues.Text")
					}
					if name == "anonymous.apex" {
						anonymous = source
						continue
					}
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
					result = AnalyzeAnonymous(index, anonymous, row[0])
				}
				if result.HasErrors() {
					t.Fatalf("static String field receiver rejected: %v", result.Diagnostics)
				}
			})
		}
	}
}

// These are offline dependency-boundary checks, not native compilation claims.
func TestUnresolvedDeclaredNamespaceReceiverNamedAndTestRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"named", "istest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				source := "public class DeclaredDependencyProbe { missingns.Parcel.Options options; public void observe() { missingns.Parcel.Instance.save(); } }"
				if route == "istest" {
					source = "@IsTest private class DeclaredDependencyProbe { static missingns.Parcel.Options options; @IsTest static void observe() { missingns.Parcel.Instance.save(); } }"
				}
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"DeclaredDependencyProbe.cls": source}, api)
				if result.HasErrors() {
					t.Fatalf("declared dependency received a false local error: %v", result.Diagnostics)
				}
			})
		}
	}
}

func TestUnresolvedDeclaredNamespaceReceiverRejectsDifferentOwnerNamedRoute(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, receiver := range []string{"missingns.Other.Instance", "otherns.Parcel.Instance", "missingns.ParcelExtra.Instance"} {
			t.Run(api+"/"+receiver, func(t *testing.T) {
				source := "public class DeclaredDependencyProbe { missingns.Parcel.Options options; public void observe() { " + receiver + ".save(); } }"
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"DeclaredDependencyProbe.cls": source}, api)
				for _, d := range result.Diagnostics {
					if d.NativeMessage == "Variable does not exist: "+receiver {
						return
					}
				}
				t.Fatalf("unrelated declared dependency hid an unknown receiver: %v", result.Diagnostics)
			})
		}
	}
}
