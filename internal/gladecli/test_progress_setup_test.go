package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/testreport"
)

type setupProgressEvent struct {
	Kind     string `json:"kind"`
	Label    string `json:"label"`
	Current  int    `json:"current"`
	Total    int    `json:"total"`
	OK       bool   `json:"ok"`
	ExitCode int    `json:"exitCode"`
}

func decodeSetupProgress(t *testing.T, output string) []setupProgressEvent {
	t.Helper()
	var events []setupProgressEvent
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		var event setupProgressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("progress JSON: %v: %s", err, line)
		}
		events = append(events, event)
	}
	return events
}

func TestProgressSeparatesSetupFailuresFromMethodErrors(t *testing.T) {
	var output bytes.Buffer
	reporter := newCLITestProgressReporter(cliui.NewRenderer(cliui.RendererOptions{Stderr: &output, Mode: cliui.ProgressJSON}))
	reporter.setTotal(304)
	for i := 0; i < 15; i++ {
		reporter.enqueue(apextest.TestProgress{Event: "test_done", ClassName: "Passing", MethodName: fmt.Sprintf("method%d", i), Status: "pass"})
	}
	reporter.enqueue(apextest.TestProgress{Event: "test_done", ClassName: "Assertion", MethodName: "fails", Status: "fail"})
	for i := 0; i < 22; i++ {
		reporter.enqueue(apextest.TestProgress{Event: "setup_done", ClassName: fmt.Sprintf("Setup%d", i), Status: "unsupported"})
	}
	for i := 0; i < 288; i++ {
		reporter.enqueue(apextest.TestProgress{Event: "test_done", ClassName: "Cancelled", MethodName: fmt.Sprintf("method%d", i), Status: "unsupported"})
	}
	reporter.finish()
	events := decodeSetupProgress(t, output.String())
	setups, methods, assertions := 0, 0, 0
	for _, event := range events {
		if strings.HasPrefix(event.Label, "setup failed ") {
			setups++
		}
		if strings.HasPrefix(event.Label, "unsupported ") {
			methods++
		}
		if strings.HasPrefix(event.Label, "FAIL ") {
			assertions++
		}
	}
	if setups != 22 || methods != 288 || assertions != 1 {
		t.Fatalf("lost events: setup=%d method=%d assertion=%d", setups, methods, assertions)
	}
	final := events[len(events)-1]
	if final.Kind != "done" || final.OK || final.ExitCode != 1 || !strings.HasPrefix(final.Label, "15 passed, 1 failed, 288 errors · 22 setup failures · ") {
		t.Fatalf("mixed method/setup counts: %#v", final)
	}
	completion := events[len(events)-2]
	if completion.Current != 304 || completion.Total != 304 {
		t.Fatalf("method population changed: %#v", completion)
	}
}

func TestProgressSetupFailureWithoutMethodCompletionStaysUnsuccessful(t *testing.T) {
	var output bytes.Buffer
	reporter := newCLITestProgressReporter(cliui.NewRenderer(cliui.RendererOptions{Stderr: &output, Mode: cliui.ProgressJSON}))
	reporter.enqueue(apextest.TestProgress{Event: "setup_done", ClassName: "OwnedSetup", Status: "fail"})
	reporter.finish()
	events := decodeSetupProgress(t, output.String())
	final := events[len(events)-1]
	if final.OK || final.ExitCode != 1 || !strings.HasPrefix(final.Label, "0 passed, 0 failed, 0 errors · 1 setup failure · ") {
		t.Fatalf("setup-only failure lost or counted as method: %#v", final)
	}
}

func TestProgressFailingSetupCLIHasSeparateCounts(t *testing.T) {
	for _, mode := range []string{"--no-serve", "--daemon"} {
		t.Run(mode, func(t *testing.T) {
			fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{classes: map[string]string{
				"OwnedSetupFailure": `@IsTest private class OwnedSetupFailure {
 @TestSetup static void setup() { System.assert(false, 'owned setup marker'); }
 @IsTest static void first() { System.assert(false, 'first body must not execute'); }
 @IsTest static void second() { System.assert(false, 'second body must not execute'); }
}`,
			}})
			var stdout, stderr bytes.Buffer
			exit := Run(context.Background(), []string{"test", "--project", fixture.projectRoot, mode, "--json", "--progress-json", "--parallelism", "1"}, &stdout, &stderr)
			if exit != 1 {
				t.Fatalf("exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
			}
			var envelope struct {
				Summary testreport.Summary `json:"summary"`
				Tests   []testreport.Case  `json:"tests"`
			}
			if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
				t.Fatal(err)
			}
			if envelope.Summary.Total != 2 || envelope.Summary.Passed != 0 || envelope.Summary.Skipped != 0 || len(envelope.Tests) != 2 {
				t.Fatalf("terminal population changed: %#v", envelope)
			}
			seen := map[string]bool{}
			for _, test := range envelope.Tests {
				seen[test.MethodName] = true
				if test.Status != testreport.StatusRuntimeError || test.Reason != testreport.ReasonRuntimeError {
					t.Fatalf("setup terminal status/reason changed: %#v", test)
				}
				if test.Problem == nil || !strings.Contains(test.Problem.Message, "owned setup marker") {
					t.Fatalf("setup cause lost or test body ran: %#v", test)
				}
			}
			if !seen["first"] || !seen["second"] {
				t.Fatalf("named cases changed: %#v", seen)
			}
			events := decodeSetupProgress(t, stderr.String())
			setups := 0
			for _, event := range events {
				if event.Label == "setup failed OwnedSetupFailure" {
					setups++
				}
			}
			final := events[len(events)-1]
			want := fmt.Sprintf("0 passed, %d failed, %d errors · 1 setup failure · ", envelope.Summary.Failed, envelope.Summary.Errors)
			if setups != 1 || final.OK || final.ExitCode != 1 || !strings.HasPrefix(final.Label, want) {
				t.Fatalf("setup/method summary mismatch: setups=%d want=%q final=%#v", setups, want, final)
			}
		})
	}
}
