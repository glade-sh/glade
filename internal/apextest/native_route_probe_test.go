package apextest

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var routeProbeDebug = regexp.MustCompile(`System\.debug\(`)

// routeProbeBody turns a capture's anonymous Apex (which prints R5| lines with
// System.debug) into a body that reports them through one failed assertion.
func routeProbeBody(apex string) string {
	return "List<String> r5out = new List<String>();\n" + routeProbeDebug.ReplaceAllString(apex, "r5out.add(") + "\nSystem.assert(false, String.join(r5out, ';'));"
}

// runAnonymousRouteProbe runs the capture's Apex through the anonymous
// compiler and returns the assertion message that carries its R5| lines.
func runAnonymousRouteProbe(t *testing.T, index typesys.Index, api, apex string) string {
	t.Helper()
	org := orgFromIndex(index)
	org.APIVersion = api
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	program, err := vm.CompileAnonymousWithOptions(routeProbeBody(apex), vm.CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	_, err = runner.newMachine().Execute(program)
	var observation *vm.RuntimeError
	if !errors.As(err, &observation) || flowRefsExceptionType(observation.Type) != "System.AssertException" {
		t.Fatalf("anonymous probe did not report: %v", err)
	}
	return observation.ExceptionMessage()
}

// runIsTestRouteProbe runs one method of a probe class through the test runner
// and returns the assertion message it reports.
func runIsTestRouteProbe(t *testing.T, index typesys.Index, class, method string) string {
	t.Helper()
	run := Run(index, Options{SelectedClasses: []string{class}, SelectedMethod: method, NoDiskCache: true, Parallelism: 1})
	if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
		t.Fatalf("unexpected test selection: %+v", run.Summary())
	}
	return run.Suites[0].Cases[0].Problem.Message
}

func nativeOutcome(value string) string { return strings.ReplaceAll(value, "&#124;", "|") }
