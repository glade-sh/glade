package lsp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestPersonAccountFormulaNativeLSPRoutes(t *testing.T) {
	raw, err := os.ReadFile("../apextest/testdata/conformance/person_account_formulas/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases struct {
		Files map[string]string
	}
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, accountSource := range []bool{true, false} {
			t.Run(fmt.Sprintf("%s/accountSource=%t", api, accountSource), func(t *testing.T) {
				root := t.TempDir()
				write := func(name, body string) {
					t.Helper()
					path := filepath.Join(root, name)
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(body), 0600); err != nil {
						t.Fatal(err)
					}
				}
				write("sfdx-project.json", fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
				write("glade.yml", "org:\n  features: [PersonAccounts]\n")
				for name, body := range cases.Files {
					if !strings.Contains(name, "/classes/") && (accountSource || !strings.Contains(name, "/objects/Account/")) {
						write(name, strings.ReplaceAll(body, "62.0", api))
					}
				}
				body := "List<Account> rows=[SELECT Derived__pc,Context__pc,Chained__pc FROM Account WHERE Derived__pc='seed-derived']; String derived=rows[0].Derived__pc; Schema.DescribeFieldResult d=Schema.SObjectType.Account.fields.getMap().get('Derived__pc').getDescribe();"
				source := "public class PersonFieldEditorProbe { public static void probe() {" + body + "} }"
				name := "force-app/main/default/classes/PersonFieldEditorProbe.cls"
				write(name, source)
				write(name+"-meta.xml", "<ApexClass xmlns=\"http://soap.sforce.com/2006/04/metadata\"><apiVersion>"+api+"</apiVersion><status>Active</status></ApexClass>")
				p, err := project.Load(root)
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
				notes := handler.DidOpen(DidOpenTextDocumentParams{TextDocument: TextDocumentItem{URI: uriFromPath(filepath.Join(root, name)), Version: 1, Text: source}})
				var problems []Diagnostic
				published := false
				for _, note := range notes {
					if payload, ok := note.Params.(PublishDiagnosticsParams); ok {
						if payload.URI == uriFromPath(filepath.Join(root, name)) {
							published = true
						}
						for _, d := range payload.Diagnostics {
							if d.Severity == diagnosticSeverityError {
								problems = append(problems, d)
							}
						}
					}
				}
				if !published {
					t.Fatal("no diagnostics notification for the opened formula probe")
				}
				if len(problems) > 0 {
					t.Fatalf("native formula reads accepted; LSP=%v", problems)
				}
			})
		}
	}

}
