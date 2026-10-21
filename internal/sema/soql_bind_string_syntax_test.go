package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestSingleQuotedStringLineBreakGate(t *testing.T) {
	for _, api := range []string{"59.0", "62.0", "67.0", "68.0"} {
		for _, test := range []struct {
			name   string
			body   string
			reject bool
		}{
			{"query", "Object r=Database.query('SELECT Id FROM Account // bind\n');", true},
			{"countQuery", "Object r=Database.countQuery('SELECT COUNT() FROM Account // bind\n');", true},
			{"queryWithBinds", "Object r=Database.queryWithBinds('SELECT Id FROM Account // bind\n',new Map<String,Object>(),AccessLevel.USER_MODE);", true},
			{"jsonMultiline", "Object value=JSON.deserialize('{\n  \"Name\":{}}',String.class);", false},
			{"jsonMultilineStrict", "Object value=JSON.deserializeStrict('{\n  \"Name\":{}}',String.class);", false},
			{"jsonMultilineSlashText", "Object value=JSON.deserialize('{\"Text\":\"//\",\n  \"Name\":{}}',String.class);", false},
			{"ordinaryMultiline", "String payload='{\n  \"Name\":{}}';", false},
			{"queryEscapedNewline", `Object r=Database.query('SELECT Id FROM Account // bind\n');`, false},
			{"escapedNewline", `String query='SELECT Id FROM Account // bind\n';`, false},
			{"tripleQuotedNewline", "String query='''\nSELECT Id FROM Account // bind\n''';", false},
			{"quotedComment", "// '\n/* '\n */\nString query='SELECT Id FROM Account';", false},
		} {
			t.Run(api+"/"+test.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "StringBoundary.cls")
				source := "public class StringBoundary { public static void observe() {" + test.body + "} }"
				if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
					t.Fatal(err)
				}
				index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
				for _, route := range []string{"named", "anonymous"} {
					t.Run(route, func(t *testing.T) {
						result := Analyze(index)
						if route == "anonymous" {
							result = AnalyzeAnonymous(index, test.body, api)
						}
						if !test.reject {
							if result.HasErrors() {
								t.Fatalf("accepted boundary rejected: %v", result.Diagnostics)
							}
							return
						}
						for _, d := range result.Diagnostics {
							if d.NativeMessage == "Illegal string literal: Line breaks are not allowed in string literals" {
								return
							}
						}
						t.Fatalf("missing native newline rejection: %v", result.Diagnostics)
					})
				}
			})
		}
	}
}
