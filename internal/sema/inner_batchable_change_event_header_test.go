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

// Native observations: testdata/inner_batchable_change_event_header
// (ORG-RUN.txt). Each class, test class and trigger was compiled in one
// deploy per API, each anonymous file by executeAnonymous.
func TestInnerBatchableChangeEventHeaderNativeConformance(t *testing.T) {
	const dir = "testdata/inner_batchable_change_event_header"
	file, err := os.Open(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	rows, err := reader.ReadAll()
	if err != nil || len(rows) != 139 || strings.Join(rows[0], ",") != "source_api,route,case,compiled,problem" {
		t.Fatalf("incomplete native matrix: %v", err)
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		api, route, name, compiled, problem := row[0], row[1], row[2], row[3], row[4]
		key := api + "/" + route + "/" + name
		if seen[key] || (api != "62.0" && api != "67.0") || (compiled != "true" && compiled != "false") || (compiled == "false") == (problem == "") {
			t.Fatalf("invalid native row: %v", row)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			var result Result
			switch route {
			case "anonymous":
				source := readInnerBatchableFixture(t, filepath.Join(dir, "anon", name+".apex"))
				if declaration, body, ok := strings.Cut(source, "\nSystem.debug"); ok && strings.HasPrefix(declaration, "public class ") {
					// glade exec moves an anonymous class declaration into a
					// transient file and checks it as an inner type.
					root := t.TempDir()
					path := filepath.Join(root, "GladeAnonymous0.cls")
					writeSemaFile(t, path, declaration+"\n")
					transient := WithAnonymousDeclarationContext(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{}))
					if result = AnalyzeAnonymousDeclarations(transient); !result.HasErrors() {
						result = AnalyzeAnonymous(transient, "System.debug"+body, api)
					}
					break
				}
				result = AnalyzeAnonymous(typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{}), source, api)
			case "class", "isTest", "trigger":
				folder, ext := "classes", ".cls"
				if route == "trigger" {
					folder, ext = "triggers", ".trigger"
				}
				root := t.TempDir()
				path := filepath.Join(root, name+ext)
				writeSemaFile(t, path, readInnerBatchableFixture(t, filepath.Join(dir, "project", "force-app", "main", "default", folder, name+ext)))
				meta := readInnerBatchableFixture(t, filepath.Join(dir, "project", "force-app", "main", "default", folder, name+ext+"-meta.xml"))
				writeSemaFile(t, path+"-meta.xml", strings.ReplaceAll(meta, "62.0", api))
				result = Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{}))
			default:
				t.Fatalf("unknown route %q", route)
			}
			if result.HasErrors() != (compiled == "false") {
				t.Fatalf("native compiled=%s %q; Glade diagnostics=%+v", compiled, problem, result.Diagnostics)
			}
			if compiled == "false" {
				for _, d := range result.Diagnostics {
					if d.Message == problem || d.NativeMessage == problem {
						return
					}
				}
				t.Fatalf("native problem %q; Glade diagnostics=%+v", problem, result.Diagnostics)
			}
		})
	}
}

func readInnerBatchableFixture(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// An inner Batchable<SObject> class gets one diagnostic with the native text
// (org.tsv LsBatInnerSObj). An abstract inner class, which skips the
// generic-contract check, keeps exactly one rejection (no abstract row was
// captured, so only the rejection is asserted).
func TestInnerDatabaseBatchableSingleNativeDiagnostic(t *testing.T) {
	for name, source := range map[string]string{
		"concrete": "public class LsHost {\n public class InnerB implements Database.Batchable<SObject> {\n  public Iterable<SObject> start(Database.BatchableContext bc) { return new List<SObject>(); }\n  public void execute(Database.BatchableContext bc, List<SObject> s) {}\n  public void finish(Database.BatchableContext bc) {}\n }\n}\n",
		"abstract": "public class LsHost {\n public abstract class InnerB implements Database.Batchable<SObject> {}\n}\n",
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "LsHost.cls")
			writeSemaFile(t, path, source)
			writeSemaFile(t, path+"-meta.xml", "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>\n")
			result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{}))
			if len(result.Diagnostics) != 1 || name == "concrete" && result.Diagnostics[0].Message != "Only top-level classes can implement Database.Batchable<SObject>" {
				t.Fatalf("want one native inner Batchable diagnostic, got %+v", result.Diagnostics)
			}
		})
	}
}
