package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSourceSyntaxParameterizedClassLiterals(t *testing.T) {
	for _, apiVersion := range []string{"62.0", "67.0"} {
		for _, test := range []struct {
			name       string
			body       string
			wantReject bool
		}{
			// Retain the cast, declaration and class literal from the admitted
			// formula-date evaluator's failing statement.
			{name: "formulaContext", body: `String context='[]'; List<ContextWrapper> contextResult = (List<ContextWrapper>) JSON.deserialize(context, List<ContextWrapper>.class);`},
			// JSON R134/R146/R207 admit these collection class literals.
			{name: "jsonList", body: `List<Account> x=(List<Account>)JSON.deserialize('[]',List<Account>.class); Object r=x.size();`},
			{name: "jsonMap", body: `Map<String,Account> x=(Map<String,Account>)JSON.deserialize('{}',Map<String,Account>.class); Object r=x.size();`},
			{name: "jsonPrimitiveList", body: `List<Integer> x=new List<Integer>{1,0,-1,null}; Object r=JSON.serialize(JSON.deserialize(JSON.serialize(x),List<Integer>.class));`},
			{name: "unparameterizedMemberAccess", body: `Object r=Boolean.class.getName();`},
			{name: "R071", body: `Object r=List<String>.class.getName();`, wantReject: true},
			{name: "R072", body: `Object r=Map<String,Account>.class.getName();`, wantReject: true},
		} {
			t.Run(apiVersion+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				var files []string
				for name, source := range map[string]string{
					"ContextWrapper":             `public class ContextWrapper { public String name; public String value; }`,
					"ParameterizedClassLiterals": "public class ParameterizedClassLiterals { public static void run() {\n" + test.body + "\n} }",
				} {
					path := filepath.Join(root, name+".cls")
					if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
						t.Fatal(err)
					}
					files = append(files, path)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: apiVersion, ApexFiles: files}, gladeschema.Schema{})
				if index.HasErrors() {
					t.Fatal(index.Diagnostics)
				}
				for _, route := range []string{"named", "anonymous"} {
					t.Run(route, func(t *testing.T) {
						var result Result
						if route == "named" {
							result = Analyze(index)
						} else {
							result = AnalyzeAnonymous(index, test.body, apiVersion)
						}
						if test.wantReject {
							if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_SOURCE_SYNTAX") {
								t.Fatalf("direct member access must retain its syntax rejection: %#v", result.Diagnostics)
							}
						} else if result.HasErrors() {
							t.Fatalf("valid class literal rejected: %#v", result.Diagnostics)
						}
					})
				}
			})
		}
	}
}
