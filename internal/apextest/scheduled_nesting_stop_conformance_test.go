package apextest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/vm"
)

// Test.stopTest runs a top-level Schedulable but not an inner-class one, whether
// the inner class sits in a helper class or in the test class. Both keep their
// CronTrigger row. Rows: testdata/conformance/scheduled_nesting_stop/org.tsv.
// The deploy and synchronous routes are both named @IsTest runs, so each row
// binds to the same local named-test execution.
func TestScheduledNestingStopTestNativeConformance(t *testing.T) {
	const dir = "testdata/conformance/scheduled_nesting_stop/"
	var data struct {
		APIVersions, Routes, TestClasses []string
		AnonymousBoundary                string
		Classes, SourceSHA256            map[string]string
	}
	raw, err := os.ReadFile(dir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if strings.Join(data.APIVersions, ",") != "62.0,67.0,68.0" || strings.Join(data.Routes, ",") != "deploy,synchronous" ||
		strings.Join(data.TestClasses, ",") != "SchedCapTest,SchedCapTest2" || data.AnonymousBoundary == "" || len(data.Classes) != 7 {
		t.Fatal("incomplete scheduled nesting matrix")
	}
	for name, source := range data.Classes {
		sum := sha256.Sum256([]byte(source))
		if hex.EncodeToString(sum[:]) != data.SourceSHA256[name] {
			t.Fatalf("captured declaration changed: %s", name)
		}
	}
	native, err := os.ReadFile(dir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	type row struct{ route, observation string }
	rows := map[string]map[string][]row{}
	counts := map[string]int{}
	for i, line := range strings.Split(strings.TrimSuffix(string(native), "\n"), "\n") {
		fields := strings.Split(line, "\t")
		if len(fields) != 4 || (i > 0 && !strings.HasPrefix(fields[3], "OBS ")) {
			t.Fatalf("invalid native row %d", i)
		}
		if i == 0 {
			continue
		}
		if rows[fields[0]] == nil {
			rows[fields[0]] = map[string][]row{}
		}
		rows[fields[0]][fields[2]] = append(rows[fields[0]][fields[2]], row{fields[1], fields[3]})
		counts[fields[0]+"/"+fields[1]]++
	}
	if counts["62.0/deploy"] != 19 || counts["67.0/deploy"] != 19 || counts["68.0/deploy"] != 19 || counts["68.0/synchronous"] != 16 || len(counts) != 4 {
		t.Fatalf("incomplete native rows: %v", counts)
	}
	for _, api := range data.APIVersions {
		runConformanceAPIVersion(t, api, func(t *testing.T) {
			index := nativeBoundaryIndex(t, api, data.Classes, nil)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native declarations accepted: %v %v", index.Diagnostics, analysis.Diagnostics)
			}
			cases := Discover(index, Options{SelectedClasses: data.TestClasses})
			if len(cases) != 19 {
				t.Fatalf("named discovery: %d rows", len(cases))
			}
			compiled, methodErrors := compileTestMethodsForIndex(&index, cases)
			programs, programErrors := compileTestInvokePrograms(cases)
			org := orgFromIndex(index)
			org.APIVersion = api
			base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
			for _, test := range cases {
				want := rows[api][test.MethodName]
				if len(want) == 0 {
					t.Fatalf("method without native row: %s", test.MethodName)
				}
				t.Run(test.MethodName, func(t *testing.T) {
					key := testCaseKey(test)
					if methodErrors[key] != nil || programErrors[key] != nil {
						t.Fatalf("native executes: method=%v program=%v", methodErrors[key], programErrors[key])
					}
					machine := base.newMachine()
					machine.EnableTestContext()
					if err := machine.RegisterMethod(compiled[key]); err != nil {
						t.Fatal(err)
					}
					_, execErr := machine.ExecuteInClass(programs[key], test.ClassName)
					var fault *vm.RuntimeError
					if !errors.As(execErr, &fault) || !strings.HasSuffix(fault.Type, "AssertException") {
						t.Fatalf("missing assertion observation: %v", execErr)
					}
					message := fault.ExceptionMessage()
					got := message[max(strings.Index(message, "OBS "), 0):]
					for _, native := range want {
						if got != native.observation {
							t.Errorf("%s native=%q; local=%q", native.route, native.observation, message)
						}
					}
				})
			}
		})
	}
}
