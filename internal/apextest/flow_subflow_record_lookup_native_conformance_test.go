package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/automation"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
)

func TestFlowSubflowRecordLookupNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/flow_subflow_record_lookup"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			version := "api" + strings.TrimSuffix(api, ".0")
			capture, err := os.ReadFile(filepath.Join(fixture, version, "native/RUN-FACTS.txt"))
			if err != nil {
				t.Fatal(err)
			}
			for _, fact := range []string{"api=" + api, "context=istest", "rows=12", "native_complete=true", "deployment_success=true"} {
				if !strings.Contains("\n"+string(capture), "\n"+fact+"\n") {
					t.Fatalf("missing capture fact %s", fact)
				}
			}
			raw, err := os.ReadFile(filepath.Join(fixture, version, "native/org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			lines := strings.Split(strings.TrimSuffix(string(raw), "\n"), "\n")
			if len(lines) != 13 || lines[0] != "id\tvalue" {
				t.Fatal("expected twelve native rows")
			}
			want := map[string]string{}
			for _, line := range lines[1:] {
				id, value, ok := strings.Cut(line, "\t")
				if !ok {
					t.Fatalf("invalid native row %q", line)
				}
				if _, duplicate := want[id]; duplicate {
					t.Fatal("duplicate native row")
				}
				if err := json.Unmarshal([]byte(value), &value); err != nil {
					t.Fatal(err)
				}
				want[id] = value
			}
			observation, err := os.ReadFile(filepath.Join(fixture, version, "native/observation.json"))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(capture), fmt.Sprintf("observation_sha256=%x\n", sha256.Sum256(observation))) {
				t.Fatal("native observation hash mismatch")
			}
			var observed map[string]string
			if err := json.Unmarshal(observation, &observed); err != nil || len(observed) != len(want) {
				t.Fatal("invalid native observation")
			}
			for id, value := range observed {
				if normalizeFlowLookupObservation(value) != want[id] {
					t.Fatalf("native TSV detached from observation: %s", id)
				}
			}
			root := t.TempDir()
			sourceRoot := filepath.Join(fixture, version)
			err = filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() && (entry.Name() == "native" || strings.HasPrefix(entry.Name(), ".")) {
					return filepath.SkipDir
				}
				if entry.IsDir() {
					return nil
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				if !strings.Contains(string(capture), fmt.Sprintf("source_sha256:%s=%x\n", filepath.ToSlash(name), sha256.Sum256(body))) {
					t.Fatalf("native source mismatch: %s", name)
				}
				writeFile(t, filepath.Join(root, name), string(body))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native compiled; Glade diagnostics: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			automations, err := automation.LoadProject(p)
			if err != nil || len(automations.Diagnostics) != 0 {
				t.Fatalf("native metadata accepted; Glade diagnostics: %v %v", err, automations.Diagnostics)
			}
			run := Run(index, Options{SelectedClasses: []string{"WillowLookupProbeTest"}, SelectedMethod: "observeLookup", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			result := run.Suites[0].Cases[0]
			if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
				t.Fatalf("missing observation: %+v", result.Problem)
			}
			_, payload, ok := strings.Cut(result.Problem.Message, "WILLOW_RESULT|")
			var got map[string]string
			if !ok || json.Unmarshal([]byte(payload), &got) != nil {
				t.Fatalf("invalid observation: %s", result.Problem.Message)
			}
			if len(got) != len(want) {
				t.Fatalf("native rows=%d; Glade rows=%d", len(want), len(got))
			}
			for _, mode := range []string{"single", "missing", "collection", "record", "variable", "direct"} {
				for _, suffix := range []string{"_set", "_null"} {
					id := mode + suffix
					t.Run(id, func(t *testing.T) {
						expected, exists := want[id]
						if !exists || normalizeFlowLookupObservation(got[id]) != expected {
							t.Fatalf("native %q; Glade %q", expected, got[id])
						}
					})
				}
			}
		})
	}
}

// Native hosted Flow fault prose includes deployment details. Compare its
// exception type while retaining the full native message in observation.json.
func normalizeFlowLookupObservation(value string) string {
	if strings.HasPrefix(value, "ERROR|") && strings.Count(value, "|") >= 2 {
		parts := strings.SplitN(value, "|", 3)
		return strings.Join(parts[:2], "|")
	}
	return value
}
