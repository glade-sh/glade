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

	"github.com/glade-sh/glade/internal/sema"
)

type scheduledFlowDMLCase struct {
	id, api, trigger, shape, operation, context, startFilter, observed string
}

func TestScheduledFlowDMLNativeConformance(t *testing.T) {
	const fixture = "../dml/testdata/conformance/scheduled_flow_dml"
	read := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, path))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}

	caseLines := strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")
	if len(caseLines) != 21 || caseLines[0] != "id\tapi\ttrigger\tflow_shape\toperation\tcontext\tstart_filter\tobserved" {
		t.Fatal("incomplete scheduled flow DML case table")
	}
	cases := make([]scheduledFlowDMLCase, 0, len(caseLines)-1)
	caseByID := make(map[string]scheduledFlowDMLCase, len(caseLines)-1)
	for _, line := range caseLines[1:] {
		cells := strings.Split(line, "\t")
		if len(cells) != 8 || cells[0] == "" || caseByID[cells[0]].id != "" {
			t.Fatalf("invalid or duplicate case row: %s", line)
		}
		row := scheduledFlowDMLCase{
			id: cells[0], api: cells[1], trigger: cells[2], shape: cells[3], operation: cells[4],
			context: cells[5], startFilter: cells[6], observed: cells[7],
		}
		if (row.api != "62" && row.api != "67") || row.observed == "" {
			t.Fatalf("invalid scheduled flow DML case: %s", line)
		}
		cases = append(cases, row)
		caseByID[row.id] = row
	}
	if len(caseByID) != 20 {
		t.Fatalf("case count = %d, want 20", len(caseByID))
	}
	unsupported := strings.Split(strings.TrimSpace(read("unavailable.tsv")), "\n")
	if len(unsupported) != 2 || unsupported[0] != "trigger\tapi\tstatus\treason" ||
		unsupported[1] != "RecordAfterDelete\t62\tmetadata_rejected\tError parsing file: 'RecordAfterDelete' is not a valid value for the enum 'FlowTriggerType'" {
		t.Fatal("missing native metadata rejection for RecordAfterDelete")
	}

	for _, apiNumber := range []string{"62", "67"} {
		api := apiNumber + ".0"
		version := "api" + apiNumber
		t.Run(version, func(t *testing.T) {
			capture := read(filepath.Join(version, "native", "RUN-FACTS.txt"))
			for _, fact := range []string{
				"api=" + api, "context=istest", "rows=10", "native_complete=true",
				"deployment_success=true", "class_api=" + api, "test_outcome=Passed",
				"tests_ran=10", "tests_passed=10", "tests_failed=0",
			} {
				if !strings.Contains("\n"+capture, "\n"+fact+"\n") {
					t.Fatalf("incomplete native capture: %s", fact)
				}
			}

			nativeRows := strings.Split(strings.TrimSpace(read(filepath.Join(version, "native", "org.tsv"))), "\n")
			if len(nativeRows) != 11 || nativeRows[0] != "id\tgroup\troute\tstatus\tvalue" {
				t.Fatal("incomplete native observation rows")
			}
			observations := make(map[string]string, len(nativeRows)-1)
			for _, line := range nativeRows[1:] {
				cells := strings.SplitN(line, "\t", 5)
				if len(cells) != 5 || cells[1] != "SCHEDULED_FLOW_DML" || cells[2] != "istest" || cells[3] != "runtime" || observations[cells[0]] != "" {
					t.Fatalf("invalid native observation row: %s", line)
				}
				observations[cells[0]] = cells[4]
			}

			root := t.TempDir()
			sourceRoot := filepath.Join(fixture, "project")
			hashCount := 0
			for _, line := range strings.Split(capture, "\n") {
				if !strings.HasPrefix(line, "source_sha256:") {
					continue
				}
				entry, digest, ok := strings.Cut(strings.TrimPrefix(line, "source_sha256:"), "=")
				if !ok || entry == "" || digest == "" {
					t.Fatalf("invalid source hash row: %s", line)
				}
				body, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(entry)))
				if err != nil {
					t.Fatal(err)
				}
				body = []byte(strings.ReplaceAll(string(body), "62.0", api))
				if got := fmt.Sprintf("%x", sha256.Sum256(body)); got != digest {
					t.Fatalf("native capture source changed: %s", entry)
				}
				hashCount++
			}
			if hashCount != 7 {
				t.Fatalf("hashed source file count = %d, want 7", hashCount)
			}

			walkErr := filepath.WalkDir(sourceRoot, func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(sourceRoot, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			})
			if walkErr != nil {
				t.Fatal(walkErr)
			}
			index := loadTestIndex(t, root)
			if analysis := sema.Analyze(index); index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted; Glade rejected: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			org := orgFromIndex(index)
			account, ok := org.Objects["Account"]
			if !ok || len(account.Definition.FlowRules) != 4 {
				t.Fatalf("expected three scheduled and one before-delete flow, got %#v", account.Definition.FlowRules)
			}
			triggerCounts := map[string]int{}
			for _, rule := range account.Definition.FlowRules {
				triggerCounts[strings.ToLower(rule.TriggerType)]++
			}
			if triggerCounts["scheduled"] != 3 || triggerCounts["recordbeforedelete"] != 1 {
				t.Fatalf("unexpected flow triggers: %#v", triggerCounts)
			}

			run := Run(index, Options{SelectedClasses: []string{"ScheduledFlowDmlProbeTest"}, NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 10 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 10 {
				t.Fatalf("unexpected test selection: %+v", run.Summary())
			}
			results := make(map[string]struct {
				status  string
				problem string
			}, len(run.Suites[0].Cases))
			for _, result := range run.Suites[0].Cases {
				problem := ""
				if result.Problem != nil {
					problem = result.Problem.Message
				}
				results[result.MethodName] = struct {
					status  string
					problem string
				}{status: string(result.Status), problem: problem}
			}

			seen := 0
			for _, row := range cases {
				if row.api != apiNumber {
					continue
				}
				seen++
				method := scheduledFlowDMLMethod(row)
				got, found := results[method]
				if !found {
					t.Fatalf("case %s has no executed Apex method %s", row.id, method)
				}
				native := observations[row.id]
				if native == "" {
					t.Fatalf("case %s has no native observation", row.id)
				}
				var value map[string]any
				if err := json.Unmarshal([]byte(native), &value); err != nil || value["counterCount"] != float64(0) {
					t.Fatalf("invalid native observation for %s: %s", row.id, native)
				}
				if row.operation == "delete" {
					if value["accountCount"] != float64(0) {
						t.Fatalf("native delete observation for %s: %s", row.id, native)
					}
				} else {
					wantDescription := any(nil)
					if row.operation == "update" {
						wantDescription = "seed"
					}
					if description, ok := value["description"]; !ok || description != wantDescription {
						t.Fatalf("native description observation for %s: %s", row.id, native)
					}
				}
				t.Run(apiNumber+"/"+row.id, func(t *testing.T) {
					if got.status != "pass" {
						t.Fatalf("native=%s; Glade status=%s problem=%q", native, got.status, got.problem)
					}
				})
			}
			if seen != 10 {
				t.Fatalf("API %s case count = %d, want 10", api, seen)
			}
		})
	}
}

func scheduledFlowDMLMethod(row scheduledFlowDMLCase) string {
	switch row.trigger {
	case "Scheduled":
		switch row.shape {
		case "unconditional":
			if row.context == "start_stop_test" {
				return "scheduledStartStopTest"
			}
			return "scheduledUnconditional" + strings.ToUpper(row.operation[:1]) + row.operation[1:]
		case "lookup_decision":
			return "scheduledDecision" + strings.ToUpper(row.operation[:1]) + row.operation[1:]
		case "filtered":
			return "scheduledFilteredInsert"
		}
	case "RecordBeforeDelete":
		return "deleteBefore" + strings.ToUpper(row.operation[:1]) + row.operation[1:] + "Control"
	}
	return ""
}
