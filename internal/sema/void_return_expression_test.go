package sema

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestAnalyzeListAddExpressionReturnTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		element    string
		method     string
		wantReturn string
	}{
		{"Object single argument return", "Object", "public Object append(Object value) { return values.add(value); }", ""},
		{"String single argument return", "String", "public Object append(String value) { return values.add(value); }", ""},
		{"Object assignment and argument", "Object", "public void append(Object value) { Object result = values.add(value); consume(values.add(value)); }", ""},
		{"String assignment and argument", "String", "public void append(String value) { Object result = values.add(value); consume(values.add(value)); }", ""},
		{"Integer rejects single argument Object result", "Object", "public Integer append(Object value) { return values.add(value); }", "returns Object from Integer method"},
		{"String rejects single argument Object result", "String", "public String append(String value) { return values.add(value); }", "returns Object from String method"},
		{"Object rejects indexed void result", "Object", "public Object append(Object value) { return values.add(0, value); }", "returns void from Object method"},
		{"Integer rejects indexed void result", "Object", "public Integer append(Object value) { return values.add(0, value); }", "returns void from Integer method"},
		{"Object rejects user void result", "Object", "public Object append(Object value) { return doNothing(); }", "returns void from Object method"},
		{"ordinary void method", "Object", "public void append(Object value) { values.add(value); values.add(0, value); }", ""},
		{"ordinary Object return", "Object", "public Object append(Object value) { return value; }", ""},
		{"Integer rejects Object", "Object", "public Integer append(Object value) { return value; }", "returns Object from Integer method"},
	}
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range cases {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				path := filepath.Join(root, "AppendAdapter.cls")
				writeSemaFile(t, path, "public class AppendAdapter { private List<"+tc.element+"> values = new List<"+tc.element+">(); public void doNothing() {} public void consume(Object value) {} "+tc.method+" }")
				result := Analyze(typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{}))
				var returns []string
				for _, diag := range result.Diagnostics {
					if diag.Code == "GLADESEMA019" {
						returns = append(returns, diag.Message)
					} else if diag.Severity == "error" {
						t.Fatalf("unexpected diagnostic: %#v", result.Diagnostics)
					}
				}
				if tc.wantReturn == "" {
					if len(returns) != 0 {
						t.Fatalf("list mutation expression should compile: %#v", result.Diagnostics)
					}
				} else if len(returns) != 1 || !strings.Contains(returns[0], tc.wantReturn) {
					t.Fatalf("return diagnostics = %#v, want exactly %q", returns, tc.wantReturn)
				}
			})
		}
	}
}
