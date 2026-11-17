package sema

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native rows u6_toMatchResult and u6_matchResultType (regex_java_divergence,
// anonymous, API 62.0 and 67.0).
func TestMatcherToMatchResultNativeCompileRejection(t *testing.T) {
	cases := []struct{ source, want string }{
		{"Matcher m = Pattern.compile('(a)').matcher('a'); m.find(); Object r = m.toMatchResult(); String observation = 'compiled';", "Method does not exist or incorrect signature: void toMatchResult() from the type System.Matcher"},
		{"Matcher m = Pattern.compile('(a)').matcher('a'); m.find(); MatchResult r = m.toMatchResult(); String observation = 'compiled';", "Invalid type: MatchResult"},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			index := typesys.Build(project.Project{Root: t.TempDir(), SourceAPIVersion: api}, schema.Schema{})
			result := AnalyzeAnonymous(index, c.source, api)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("%s %q: no diagnostic", api, c.source)
			}
			got := result.Diagnostics[0]
			msg := got.NativeMessage
			if msg == "" {
				msg = got.Message
			}
			if msg != c.want {
				t.Fatalf("%s %q\n got  %q\n want %q", api, c.source, msg, c.want)
			}
		}
	}
}

// Native named @IsTest route (regex_java_divergence RUN-FACTS, u6 named rows):
// deploy errors at 62.0 and 67.0, line 4 (columns 81 and 68 are not modelled).
func TestMatcherToMatchResultNativeNamedRoute(t *testing.T) {
	cases := []struct {
		name, body, want string
		line             int
	}{
		{"Ru6tmr", "Object r = m.toMatchResult();", "Method does not exist or incorrect signature: void toMatchResult() from the type System.Matcher", 4},
		{"Ru6type", "MatchResult r = m.toMatchResult();", "Invalid type: MatchResult", 4},
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, c := range cases {
			root := t.TempDir()
			source := "@IsTest\nprivate class " + c.name + " {\n    @IsTest static void t() {\n        Matcher m = Pattern.compile('(a)').matcher('a'); m.find(); " + c.body + "\n    }\n}\n"
			path := filepath.Join(root, c.name+".cls")
			if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
				t.Fatal(err)
			}
			index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: []string{path}}, schema.Schema{})
			result := Analyze(index)
			if len(result.Diagnostics) == 0 {
				t.Fatalf("%s %s: no diagnostic", api, c.name)
			}
			got := result.Diagnostics[0]
			msg := got.NativeMessage
			if msg == "" {
				msg = got.Message
			}
			if msg != c.want || got.Range == nil || got.Range.Start.Line != c.line {
				t.Fatalf("%s %s: %q at %+v, want %q on line %d", api, c.name, msg, got.Range, c.want, c.line)
			}
		}
	}
}
