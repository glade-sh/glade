package sema

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

// receiverFollowupRun returns the complete serialized diagnostics of one
// source unit beside the CrProbe fixture class.
func receiverFollowupRun(t *testing.T, route, source, api string) string {
	t.Helper()
	support, err := os.ReadFile("testdata/receiver_split/CrProbe.cls")
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	probe := filepath.Join(root, "CrProbe.cls")
	writeSemaFile(t, probe, string(support))
	files := []string{probe}
	if route != "anonymous" {
		declaration := regexp.MustCompile(`(?i)\b(?:class|trigger)\s+([A-Za-z_][A-Za-z0-9_]*)`).FindStringSubmatch(source)
		if len(declaration) < 2 {
			t.Fatal("missing declaration name")
		}
		ext := ".cls"
		if route == "trigger" {
			ext = ".trigger"
		}
		path := filepath.Join(root, declaration[1]+ext)
		writeSemaFile(t, path, source)
		files = append(files, path)
	}
	index := typesys.Build(project.Project{Root: root, SourceAPIVersion: api, ApexFiles: files}, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})
	diagnostics := index.Diagnostics
	if !index.HasErrors() {
		if route == "anonymous" {
			diagnostics = AnalyzeAnonymous(index, source, api).Diagnostics
		} else {
			diagnostics = Analyze(index).Diagnostics
		}
	}
	data, err := json.Marshal(diagnostics)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(data), root, "<root>")
}

// Each rule's shape is the token stream of its fixture, and nothing else in
// the fixture set matches it.
func TestReceiverFollowupRulesMatchFixtures(t *testing.T) {
	cases := readReceiverGrammarCases(t)
	seen := map[string]bool{}
	for _, rule := range receiverFollowupRules {
		if seen[rule.id] {
			t.Errorf("%s: duplicate rule", rule.id)
		}
		seen[rule.id] = true
	}
	platformSeen := map[string]bool{}
	for id, file := range cases {
		source, err := os.ReadFile(filepath.Join("testdata/receiver_split", file))
		if err != nil {
			t.Fatal(err)
		}
		route := "anonymous"
		switch filepath.Ext(file) {
		case ".cls":
			route = "class"
		case ".trigger":
			route = "trigger"
		}
		matched := ""
		for _, item := range scanReceiverTrivia(string(source)) {
			if rule, ok := receiverFollowupMatch(string(source), route, item); ok {
				matched = rule.id
			}
		}
		want := ""
		if seen[id] {
			want = id
		}
		// NATIVE testdata/receiver_split/org.tsv:470-507 (platform_*): the
		// same unit with UserInfo.getName() as the broken call. The call
		// rule of the CrProbe.value() fixture applies to it.
		if base, platform := strings.CutPrefix(id, "platform_"); platform && !strings.HasSuffix(base, "_same") {
			platformSeen[base] = true
			if matched != base {
				t.Errorf("%s: matched rule %q, want %q", id, matched, base)
			}
			continue
		}
		if matched != want {
			t.Errorf("%s: matched rule %q, want %q", id, matched, want)
		}
		delete(seen, id)
	}
	for id := range seen {
		t.Errorf("%s: rule has no fixture", id)
	}
	// Every call rule has a platform-method row beside its support-class row.
	for _, rule := range receiverFollowupRules {
		if (rule.resolve == "call") != platformSeen[rule.id] {
			t.Errorf("%s: resolve %q, platform-method fixture %v", rule.id, rule.resolve, platformSeen[rule.id])
		}
	}
}

// Differential against origin/main ad1577dbc6, one subtest per rule. Each
// neighbor is one small edit of the rule's fixture: an argument, another
// member or type, a statement or member before it, a blank line at the break.
// Its complete diagnostics at both captured API versions are the ones
// origin/main serialized (testdata/receiver_split/followup-neighbors.json).
func TestReceiverFollowupNeighborsKeepMain(t *testing.T) {
	raw, err := os.ReadFile("testdata/receiver_split/followup-neighbors.json")
	if err != nil {
		t.Fatal(err)
	}
	var neighbors []struct{ Rule, Name, Route, Source, Main62, Main67 string }
	if err := json.Unmarshal(raw, &neighbors); err != nil {
		t.Fatal(err)
	}
	for _, rule := range receiverFollowupRules {
		t.Run(rule.id, func(t *testing.T) {
			count := 0
			for _, neighbor := range neighbors {
				if neighbor.Rule != rule.id {
					continue
				}
				count++
				if neighbor.Route != rule.route {
					t.Fatalf("%s: route %s", neighbor.Name, neighbor.Route)
				}
				for api, want := range map[string]string{"62.0": neighbor.Main62, "67.0": neighbor.Main67} {
					if got := receiverFollowupRun(t, neighbor.Route, neighbor.Source, api); got != want {
						t.Errorf("%s at %s: source %q\n got %s\nmain %s", neighbor.Name, api, neighbor.Source, got, want)
					}
				}
			}
			if count < 3 {
				t.Fatalf("%d neighbors", count)
			}
		})
	}
}

// A rule needs its captured resolution: without the CrProbe class the
// captured call and field units keep the origin/main result.
func TestReceiverFollowupUnresolvedKeepsMain(t *testing.T) {
	for _, tc := range []struct{ source, main string }{
		{"for (String s : CrProbe.\nvalue().split(' ')) { }\n", `[{"severity":"error","code":"GLADESEMA008","message":"method \"execute\" calls unknown method \"CrProbe.value\"","range":{"start":{"line":1,"column":1},"end":{"line":1,"column":14,"offset":13}},"nativeMessage":"Variable does not exist: CrProbe"}]`},
		{"do { } while (CrProbe.\nvalue() == 'ok');\n", `[{"severity":"error","code":"GLADESEMA013","message":"Missing ')' at 'value'","range":{"start":{"line":2,"column":1,"offset":23},"end":{"line":2,"column":6,"offset":28}},"nativeMessage":"Missing ')' at 'value'"}]`},
		{"{ String s = (CrProbe.\nSF); }\n", `[{"severity":"error","code":"GLADESEMA013","message":"Variable does not exist: CrProbe","range":{"start":{"line":1,"column":3,"offset":2},"end":{"line":1,"column":10,"offset":9}},"nativeMessage":"Variable does not exist: CrProbe"}]`},
	} {
		for _, api := range []string{"62.0", "67.0"} {
			got, err := json.Marshal(AnalyzeAnonymous(typesys.Index{}, tc.source, api).Diagnostics)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tc.main {
				t.Errorf("api %s source %q: got %s, origin/main %s", api, tc.source, got, tc.main)
			}
		}
	}
}
