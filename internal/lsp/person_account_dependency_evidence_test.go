package lsp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The editor sees the same effective-project org shape as check and the
// runner: Person Account evidence in a declared dependency enables the field
// set for the consumer document.
func TestPersonAccountDependencyEvidenceLSPRoutes(t *testing.T) {
	const dir = "../apextest/testdata/conformance/person_account_evidence"
	raw, err := os.ReadFile(filepath.Join(dir, "dependency_layouts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var layouts []struct {
		Name      string
		Enabled   bool
		Call      string
		Rejection string
		Files     map[string]string
	}
	if err := json.Unmarshal(raw, &layouts); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(dir, "observe.apex"))
	if err != nil {
		t.Fatal(err)
	}
	const local = "Account local = new Account(FirstName='C',LastName='D');System.assertEquals('D',local.LastName);"
	for _, api := range []string{"62.0", "67.0"} {
		for _, layout := range layouts {
			for _, route := range []string{"named", "isTest"} {
				t.Run(api+"/"+layout.Name+"/"+route, func(t *testing.T) {
					root := t.TempDir()
					write := func(path, source string) {
						t.Helper()
						path = filepath.Join(root, filepath.FromSlash(path))
						if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(source), 0600); err != nil {
							t.Fatal(err)
						}
					}
					for path, source := range layout.Files {
						write(path, strings.ReplaceAll(strings.ReplaceAll(source, "{{API}}", api), "{{OBSERVE}}", string(body)))
						if strings.HasSuffix(path, ".cls") {
							write(path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
						}
					}
					write("consumer/force-app/main/default/objects/Account/Account.object-meta.xml", `<CustomObject><fields><fullName>Name</fullName><type>Text</type></fields></CustomObject>`)
					path := "consumer/force-app/main/default/classes/PersonEvidenceConsumer.cls"
					source := "public class PersonEvidenceConsumer { public static void sample(){" + layout.Call + local + "} }"
					if route == "isTest" {
						source = "@IsTest private class PersonEvidenceConsumer { @IsTest static void sample(){" + layout.Call + local + "} }"
					}
					write(path, source)
					write(path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
					p, err := project.Load(filepath.Join(root, "consumer"))
					if err != nil {
						t.Fatal(err)
					}
					s, err := schema.LoadProject(p)
					if err != nil {
						t.Fatal(err)
					}
					index := typesys.Build(p, s)
					if index.HasErrors() {
						t.Fatal(index.Diagnostics)
					}
					handler := NewHandler(index)
					defer handler.parser.Close()
					var errors []string
					var rejection diagnostic.Severity
					for _, d := range handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, filepath.FromSlash(path)))) {
						if d.Severity == diagnostic.Error {
							errors = append(errors, d.Message)
						}
						if rejection == "" && strings.Contains(d.Message, "FirstName") {
							rejection = d.Severity
						}
					}
					if layout.Enabled {
						if len(errors) > 0 || rejection != "" {
							t.Fatalf("enabled; LSP errors=%v FirstName diagnostic=%q", errors, rejection)
						}
						return
					}
					// A loaded source-backed dependency downgrades the editor's
					// semantic errors to warnings, as check does.
					want := diagnostic.Error
					if layout.Rejection == "warning" {
						want = diagnostic.Warning
					}
					if rejection != want {
						t.Fatalf("missing native FirstName rejection as %s: got %q, errors=%v", want, rejection, errors)
					}
				})
			}
		}
	}
}
