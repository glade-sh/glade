package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestProcessBuilderWaitNativeConformance replays the process_builder_wait
// capture through the named-test route at Apex API 62.0 and 67.0:
//   - process: a Process Builder process (processType Workflow) runs on insert
//     and update; its ISCHANGED decision calls an Apex action that deletes the
//     record it was given;
//   - waits: an autolaunched flow started from Apex stops at its Wait element
//     and leaves one paused FlowInterview.
//
// Each probe ends with assertEquals('SENTINEL', observed), so the failure
// message carries the observed text that org.tsv records.
func TestProcessBuilderWaitNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/process_builder_wait"
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
	run := Run(index, Options{SelectedClasses: []string{"CedarPb62Test", "CedarPb67Test"}, NoDiskCache: true, Parallelism: 1})
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
