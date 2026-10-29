package lsp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native standard_field_policy_combinations/org.tsv:5-12,16-23 and ORG-RUN.txt
// cover accepted and missing-field constructors at both source APIs.
func TestStandardFieldPolicyOrderManagementLSPConstructorRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, operation := range []struct {
			name, body string
			wantError  bool
		}{
			{"constructor", "WebStore row=new WebStore();", false},
			{"missing-constructor", "WebStore row=new WebStore(UnknownOwnedField='x');", true},
		} {
			t.Run(api+"/named/"+operation.name, func(t *testing.T) {
				root := t.TempDir()
				write := func(relative, content string) {
					t.Helper()
					path := filepath.Join(root, relative)
					if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
						t.Fatal(err)
					}
					if err := os.WriteFile(path, []byte(content), 0600); err != nil {
						t.Fatal(err)
					}
				}
				write("sfdx-project.json", `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				write("config/project-scratch-def.json", `{"edition":"Developer","features":["OrderManagement"],"settings":{"orderManagementSettings":{"enableOrderManagement":true}}}`)
				classPath := "force-app/main/default/classes/OrderManagementConstructor.cls"
				write(classPath, "public class OrderManagementConstructor { public static void sample(){"+operation.body+"} }")
				write(classPath+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
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
					t.Fatalf("index: %v", index.Diagnostics)
				}
				handler := NewHandler(index)
				defer handler.parser.Close()
				for _, read := range []string{"first-diagnostics", "repeated-diagnostics"} {
					t.Run(read, func(t *testing.T) {
						diagnostics := handler.diagnosticsForDocument(uriFromPath(filepath.Join(root, classPath)))
						hasError := false
						for _, item := range diagnostics {
							hasError = hasError || item.Severity == diagnostic.Error
						}
						if hasError != operation.wantError {
							t.Fatalf("native constructor error=%t; diagnostics=%v", operation.wantError, diagnostics)
						}
					})
				}
			})
		}
	}
}
