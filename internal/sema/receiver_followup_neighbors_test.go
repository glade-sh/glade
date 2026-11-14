package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// The serialized results are the complete origin/main diagnostics for these
// uncaptured expression shapes.
func TestReceiverFollowupExpressionNeighbors(t *testing.T) {
	for _, source := range []string{
		"for (Integer i = Integer.\nvalueOf('1') + 1; i < 2; i++) {}",
		"do {} while (Integer.\nvalueOf('1') > 0 == true);",
		"if (true) {} else if (Integer.\nvalueOf('1') > 0) {}",
		"for (Integer i = Integer.\nvalueOf('1'); i < 2; i++) {}",
		"do {} while (Integer.\nvalueOf('1') == 1);",
		"if (true) {} else if (Integer.\nvalueOf('1') == 1) {}",
	} {
		for _, api := range []string{"62.0", "67.0"} {
			got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, source, api).Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != "null" {
				t.Errorf("api %s source %q: got %s, origin/main null", api, source, got)
			}
		}
	}
}

// These uncaptured neighbors pin complete diagnostics from origin/main.
func TestReceiverFollowupUncapturedNeighbors(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{"System.debug(true ? 'x' : String.\nvalueOf(1));", `null`},
		{"switch on 1 { when String.\nvalueOf { } }", `[{"severity":"error","code":"GLADESEMA030","message":"method \"execute\" has invalid statement: switch branch must be a literal or enum constant","range":{"start":{"line":1,"column":15,"offset":14},"end":{"line":1,"column":16,"offset":15}}}]`},
		{"System.debug(for (Integer i = 0; i < String.\nvalueOf(1); i++) {});", `[{"severity":"error","code":"GLADESEMA_ANONYMOUS_PARSE","message":"expected , at byte 26, got \"i\"","range":{"start":{"line":1,"column":1},"end":{"line":1,"column":67,"offset":66}}}]`},
		{"do {} while (String.\nvalueOf(1) != null);", `null`},
		{"{ (String.\nvalueOf); }", `null`},
		{"String f(Integer x) { return String.\nvalueOf(x); }", `null`},
	} {
		got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, tc.source, "67.0").Diagnostics)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tc.want {
			t.Errorf("source %q: got %s, origin/main %s", tc.source, got, tc.want)
		}
	}
	for _, tc := range []struct{ name, source, want string }{
		{"Probe.cls", "public class Probe<T> { public void run() { System.debug(String.\nvalueOf(1)); } }", `[{"severity":"error","code":"GLADESEMA032","message":"class \"Probe\" cannot declare type parameters","file":"<root>/Probe.cls","range":{"start":{"line":1,"column":1},"end":{"line":2,"column":17,"offset":81}}}]`},
		{"Probe.cls", "public class Probe { public class Part { public void f(System\n.String) {} } }", `[{"severity":"error","code":"APEXPARSE001","message":"syntax error","file":"<root>/Probe.cls","range":{"start":{"line":2,"column":8,"offset":69},"end":{"line":2,"column":8,"offset":69}},"excerpt":".String) {} } }"},{"severity":"error","code":"APEXPARSE003","message":"Invalid identifier: ","file":"<root>/Probe.cls","range":{"start":{"line":2,"column":8,"offset":69},"end":{"line":2,"column":8,"offset":69}},"excerpt":".String) {} } }"}]`},
		{"Probe.trigger", "trigger Probe on Account (before insert) { System.assertEquals(null, Trigger.\nnew); }", `null`},
	} {
		root := t.TempDir()
		path := filepath.Join(root, tc.name)
		if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		index := typesys.Build(project.Project{Root: root, SourceAPIVersion: "67.0", ApexFiles: []string{path}}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
		got := index.Diagnostics
		if !index.HasErrors() {
			got = Analyze(index).Diagnostics
		}
		data, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if actual := strings.ReplaceAll(string(data), root, "<root>"); actual != tc.want {
			t.Errorf("source %q: got %s, origin/main %s", tc.source, actual, tc.want)
		}
	}
}
