package sema

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native missing_static rows: ../apextest/testdata/conformance/unloaded_dependency/
// api{62,67}/org.tsv:2. Package-to-namespace attribution is local resolver policy.
func TestManagedDependencyAbsentStaticCallReleaseRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, receiver := range []string{"zzns.SomeApi", "zzns.OtherApi", "zzns.Tools.Dispatcher"} {
			t.Run(api+"/"+receiver, func(t *testing.T) {
				root := t.TempDir()
				writeSemaFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true,"dependencies":[{"package":"BoundaryPackage"}]}]}`, api))
				path := filepath.Join(root, "force-app/main/default/classes/BoundaryHelper.cls")
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				body := receiver + ".run('owned');"
				writeSemaFile(t, path, "public class BoundaryHelper { public static void observe() { "+body+" } }")
				writeSemaFile(t, path+"-meta.xml", "<ApexClass><apiVersion>"+api+"</apiVersion></ApexClass>")
				p, err := project.Load(root)
				if err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(p, schema.Schema{})
				assertCall := func(route string, diagnostics []diagnostic.Diagnostic, want bool) {
					t.Helper()
					found := false
					for _, d := range diagnostics {
						found = found || d.Severity == diagnostic.Error && d.Code == "GLADESEMA008"
					}
					if found != want {
						t.Fatalf("%s call diagnostic=%v want=%v: %+v", route, found, want, diagnostics)
					}
				}
				assertCall("named", Analyze(index).Diagnostics, true)
				for _, warm := range []bool{false, true} {
					assertCall(fmt.Sprintf("anonymous/warm=%v", warm), AnalyzeAnonymous(index, body, api).Diagnostics, true)
					for _, control := range []string{
						"String zzns = 'owned'; zzns.length();",
						receiver + " value = null; " + receiver + ".run('owned');",
						"Type value = Type.forName('" + receiver + "');",
					} {
						assertCall("anonymous control: "+control, AnalyzeAnonymous(index, control, api).Diagnostics, false)
					}
				}
				// The release text fallback leaves this call unresolved. Ordinary
				// Apex lowers to IR and retains the compile rejection above.
				a := NewAnalyzer()
				a.prepareAnalysisContext(index, AnalyzeOptions{})
				a.prepareAnalysisModel(index)
				model := buildSemaTypeMemberView(index)
				for _, typ := range index.Types {
					if !strings.EqualFold(typ.Name, "BoundaryHelper") {
						continue
					}
					source, err := os.ReadFile(path)
					if err != nil {
						t.Fatal(err)
					}
					for _, member := range typ.Members {
						if member.Kind != apexast.DeclarationMethod {
							continue
						}
						text, offset, ok := semaBodyFromRange(string(source), member.BodyRange)
						if !ok {
							t.Fatal("missing method body")
						}
						scopes, _ := a.collectBodyScopes(typ, member, text, offset, string(source), semaBodyBaseScope(typ, member, model), model)
						assertCall("text", a.checkBodyCalls(typ, member, text, offset, string(source), scopes, model), false)
					}
				}
			})
		}
	}
}
