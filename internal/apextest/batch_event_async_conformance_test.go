package apextest

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// batch_event_async: platform event (publish after commit and immediately) and
// change event triggers fired from a Batch Apex start() or execute(), each
// calling @future, Database.executeBatch, System.scheduleBatch,
// System.enqueueJob or System.schedule; the same triggers delivered outside a
// batch; two executeBatch calls in one test method; a change event skipped at
// Test.stopTest() and delivered later; and triggers that throw. Each row is one
// BeProbeTest method at source API 62.0 or 67.0, observed natively through a
// named assertion. The observation lists every Obs__c row the method wrote,
// so jobs that natively never run must be absent here too.
func TestBatchEventAsyncNativeConformance(t *testing.T) {
	runObservationClassFamily(t, "batch_event_async", "BeProbeTest", 80)
}

// approval_verify: the B14 approval verify packet. Approval.lock and unlock
// without a process, Approval.process submit, approve, reject and recall
// through a deployed owner-routed approval process, ProcessInstance,
// ProcessInstanceStep, ProcessInstanceWorkitem and StepsAndWorkitems
// (ProcessInstanceHistory) rows, a second process call on a closed workitem,
// and Metadata.Operations.enqueueDeployment with a DeployCallback in a test.
func TestApprovalVerifyNativeConformance(t *testing.T) {
	runObservationClassFamily(t, "approval_verify", "ApvProbeTest", 7)
}

// runObservationClassFamily replays every method of one probe test class at
// source API 62.0 and 67.0 and compares each OBS| assertion observation with
// the native org.tsv row (source_api, method, observation).
func runObservationClassFamily(t *testing.T, family, class string, methods int) {
	t.Helper()
	runObservationClassFamilyWithOptions(t, family, class, methods, Options{})
}

// runObservationClassFamilyWithOptions is runObservationClassFamily with
// runner options (for example the strict governor-limit mode).
func runObservationClassFamilyWithOptions(t *testing.T, family, class string, methods int, base Options) {
	t.Helper()
	dir := filepath.Join("testdata", "conformance", family)
	body, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(lines) != 1+2*methods || lines[0] != "source_api\tmethod\tobservation" {
		t.Fatalf("incomplete native matrix: %d lines", len(lines))
	}
	rows := map[string]map[string]string{}
	for _, line := range lines[1:] {
		cells := strings.SplitN(line, "\t", 3)
		if len(cells) != 3 {
			t.Fatal("malformed native row")
		}
		if rows[cells[0]] == nil {
			rows[cells[0]] = map[string]string{}
		}
		rows[cells[0]][cells[1]] = cells[2]
	}
	for _, api := range []string{"62.0", "67.0"} {
		if len(rows[api]) != methods {
			t.Fatalf("API %s has %d rows", api, len(rows[api]))
		}
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(dir, "project")
			err := filepath.WalkDir(project, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(content), "62.0", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			options := base
			options.SelectedClasses, options.NoDiskCache, options.Parallelism = []string{class}, true, 1
			run := Run(loadTestIndex(t, root), options)
			if len(run.Suites) != 1 || len(run.Suites[0].Cases) != methods {
				t.Fatalf("expected %d observations: %+v", methods, run.Summary())
			}
			for _, result := range run.Suites[0].Cases {
				observation, ok := rows[api][result.MethodName]
				if !ok {
					t.Fatalf("no native row for %s", result.MethodName)
				}
				t.Run(result.MethodName, func(t *testing.T) {
					if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
						t.Fatalf("missing assertion transport: %+v", result.Problem)
					}
					_, observed, _ := strings.Cut(result.Problem.Message, "OBS|")
					var got, want any
					if json.Unmarshal([]byte(observed), &got) != nil || json.Unmarshal([]byte(observation), &want) != nil {
						t.Fatalf("invalid observation: %q", observed)
					}
					if diffs := observationDiffs("", got, want); len(diffs) != 0 {
						t.Fatalf("native/Glade differences: %s", strings.Join(diffs, "; "))
					}
				})
			}
		})
	}
}
