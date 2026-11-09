package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestFlowToLabelResidualsNativeConformance replays the flow_tolabel_residuals
// capture through the named-test route at Apex API 62.0 and 67.0:
//   - an after-save CreateAndUpdate Flow (Flow API 62.0 and 67.0) writes a
//     String variable built by Add assignments back to an Html and a Text field
//     of $Record, on insert and on update;
//   - an autolaunched Flow's Create Records writes $User.Id to a User lookup;
//   - toLabel(<picklist>) in aggregate queries: grouped by the same field in
//     another case, without an alias, and aliased but not grouped (MIN of the
//     group's non-null values).
//
// Each probe ends with assertEquals('SENTINEL', observed), so the failure
// message carries the observed text that org.tsv records. The anonymous rows
// need the fixture's permission set, which glade.yml assigns on the test route
// and glade exec only; they are checked with glade exec, not here.
func TestFlowToLabelResidualsNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_tolabel_residuals"
	rows, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	native := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			t.Fatalf("malformed row %q", line)
		}
		native[fields[0]+"/"+fields[1]+"/"+fields[2]] = fields[3]
	}
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS(filepath.Join(fixture, "project"))); err != nil {
		t.Fatal(err)
	}
	index := loadTestIndex(t, root)
	run := Run(index, Options{SelectedClasses: []string{"CedarProbe62Test", "CedarProbe67Test"}, NoDiskCache: true, Parallelism: 1})
	const marker = "Expected: SENTINEL, Actual: "
	seen := 0
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			api := "67.0"
			if strings.Contains(c.ClassName, "62") {
				api = "62.0"
			}
			key := api + "/istest/" + c.MethodName
			want, ok := native[key]
			if !ok {
				t.Fatalf("no native row for %s", key)
			}
			seen++
			if c.Problem == nil || !strings.Contains(c.Problem.Message, marker) {
				t.Errorf("%s: want the sentinel assertion, got %+v", key, c.Problem)
				continue
			}
			// The test API result trims trailing whitespace from the message.
			got := strings.TrimRight(c.Problem.Message[strings.Index(c.Problem.Message, marker)+len(marker):], " \n")
			if got != want {
				t.Errorf("%s:\n got  %s\n want %s", key, got, want)
			}
		}
	}
	if seen != 4 {
		t.Fatalf("ran %d probe cases, want 4: %+v", seen, run.Summary())
	}
}
