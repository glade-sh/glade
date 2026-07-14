package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/testdaemon"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

func TestDaemonEquivalenceMatrix(t *testing.T) {
	scenarios := []daemonEquivalenceScenario{
		{
			name: "baseline_cache",
			classes: map[string]string{
				"AlphaTest": `@isTest private class AlphaTest {
  @isTest static void passes() { System.assertEquals(2, 1 + 1); }
  @isTest static void alsoPasses() { System.assertEquals('alpha', 'al' + 'pha'); }
}`,
				"BetaTest": `@isTest private class BetaTest {
  @isTest static void passes() { System.assertEquals(3, 1 + 2); }
}`,
			},
			wantExit:  0,
			wantTotal: 3,
			wantPass:  3,
		},
		{
			name: "selector_exact_method",
			classes: map[string]string{
				"AlphaTest": `@isTest private class AlphaTest {
  @isTest static void passes() { System.assert(true); }
  @isTest static void unselected() { System.assert(false); }
}`,
				"BetaTest": `@isTest private class BetaTest {
  @isTest static void unselected() { System.assert(false); }
}`,
			},
			args:      []string{"--class", "AlphaTest", "--method", "passes"},
			wantExit:  0,
			wantTotal: 1,
			wantPass:  1,
			wantCase:  "AlphaTest.passes",
		},
		{
			name: "timeout",
			classes: map[string]string{
				"TimeoutTest": `@isTest private class TimeoutTest {
  @isTest static void neverFinishes() {
    Integer work = 0;
    while (true) {
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
      work = work + 1;
    }
  }
}`,
			},
			args:               []string{"--test-timeout", "20ms"},
			wantExit:           1,
			wantTotal:          1,
			wantCaseStatus:     testreport.StatusUnsupported,
			wantProblemType:    "Canceled",
			wantProblemMessage: context.DeadlineExceeded.Error(),
		},
		{
			name: "strict_limit",
			classes: map[string]string{
				"StrictLimitTest": `@isTest private class StrictLimitTest {
  @isTest static void exceedsQueryLimit() {
    List<Account> firstQuery = [SELECT Id FROM Account];
    List<Account> secondQuery = [SELECT Id FROM Account];
    System.assertEquals(0, firstQuery.size() + secondQuery.size());
  }
}`,
			},
			args:               []string{"--limit-mode", "strict", "--limit-queries", "1"},
			wantExit:           1,
			wantTotal:          1,
			wantCaseStatus:     testreport.StatusFail,
			wantProblemType:    "System.LimitException",
			wantProblemMessage: "Too many queries: 2 out of 1",
		},
		{
			name: "parallel",
			classes: map[string]string{
				"AlphaTest": `@isTest private class AlphaTest { @isTest static void passes() { System.assert(true); } }`,
				"BetaTest":  `@isTest private class BetaTest { @isTest static void passes() { System.assert(true); } }`,
				"GammaTest": `@isTest private class GammaTest { @isTest static void passes() { System.assert(true); } }`,
				"DeltaTest": `@isTest private class DeltaTest { @isTest static void passes() { System.assert(true); } }`,
			},
			clientInputs: true,
			args:         []string{"--parallelism", "4", "--parallel-methods"},
			wantExit:     0,
			wantTotal:    4,
			wantPass:     4,
		},
	}
	modes := []daemonEquivalenceMode{
		{name: "local", args: []string{"--no-serve"}},
		{name: "in_process_daemon", args: []string{"--daemon"}},
		{name: "auto_connect", realServer: true},
		{name: "explicit_connect", args: []string{"--connect"}, realServer: true},
	}
	if len(scenarios) != 5 {
		t.Fatalf("scenario count = %d, want exactly 5", len(scenarios))
	}
	if len(modes) != 4 {
		t.Fatalf("mode count = %d, want exactly 4", len(modes))
	}

	for _, scenario := range scenarios {
		t.Run(scenario.name, func(t *testing.T) {
			var oracle daemonEquivalenceSnapshot
			for modeIndex, mode := range modes {
				got := runDaemonEquivalenceInvocation(t, scenario, mode)
				assertDaemonEquivalenceScenario(t, scenario, mode, got)
				if modeIndex == 0 {
					oracle = got
					continue
				}
				if !reflect.DeepEqual(got, oracle) {
					t.Errorf("%s diverged from local oracle (-want +got):\nwant %s\n got %s", mode.name, formatDaemonEquivalenceSnapshot(oracle), formatDaemonEquivalenceSnapshot(got))
				}
			}
		})
	}
}

func TestDaemonClassShardFilesEquivalent(t *testing.T) {
	modes := []daemonEquivalenceMode{
		{name: "local", args: []string{"--no-serve"}},
		{name: "in_process_daemon", args: []string{"--daemon"}},
		{name: "explicit_connect", args: []string{"--connect"}, realServer: true},
	}
	oracle := runDaemonEquivalenceShardPlan(t, modes[0])
	for _, mode := range modes[1:] {
		got := runDaemonEquivalenceShardPlan(t, mode)
		if !reflect.DeepEqual(got, oracle) {
			t.Fatalf("%s class shards differ from local:\nwant %#v\n got %#v", mode.name, oracle, got)
		}
	}
}

func TestInProcessDaemonPreservesProgress(t *testing.T) {
	fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{
		classes: map[string]string{
			"ProgressTest": `@isTest private class ProgressTest {
  @isTest static void firstPasses() { System.assert(true); }
  @isTest static void secondPasses() { System.assert(true); }
}`,
			"OtherProgressTest": `@isTest private class OtherProgressTest { @isTest static void passes() { System.assert(true); } }`,
		},
	})
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), []string{
		"test", "--project", fixture.projectRoot, "--daemon", "--progress-json",
	}, &stdout, &stderr)
	if exit != 0 {
		t.Fatalf("progress run exit=%d stdout=%s stderr=%s", exit, stdout.String(), stderr.String())
	}
	for _, want := range []string{`"label":"compile test harness"`, `"label":"3 passed, 0 failed, 0 errors`} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("progress stderr missing %q:\n%s", want, stderr.String())
		}
	}
	type progressEvent struct {
		Kind     string `json:"kind"`
		Label    string `json:"label"`
		Current  int    `json:"current"`
		Total    int    `json:"total"`
		ExitCode *int   `json:"exitCode"`
	}
	var sawRunningTotal, sawCompletion, sawDone bool
	for _, line := range strings.Split(strings.TrimSpace(stderr.String()), "\n") {
		var event progressEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatalf("decode progress event %q: %v", line, err)
		}
		if event.Kind == "phase_tick" && event.Label == "Running tests" && event.Current == 0 && event.Total == 3 {
			sawRunningTotal = true
		}
		if event.Kind == "phase_tick" && event.Label == "tests complete" && event.Current == 3 && event.Total == 3 {
			sawCompletion = true
		}
		if event.Kind == "done" && strings.HasPrefix(event.Label, "3 passed, 0 failed, 0 errors") && event.ExitCode != nil && *event.ExitCode == 0 {
			sawDone = true
		}
	}
	if !sawRunningTotal || !sawCompletion || !sawDone {
		t.Fatalf("progress totals running=%t completion=%t done=%t:\n%s", sawRunningTotal, sawCompletion, sawDone, stderr.String())
	}
}

func TestNoCacheRoutingContract(t *testing.T) {
	t.Run("suppresses_auto_connect", func(t *testing.T) {
		fixture := newDaemonEquivalenceFixture(t, daemonEquivalenceScenario{
			classes: map[string]string{
				"CachedTest": `@isTest private class CachedTest { @isTest static void passes() { System.assert(true); } }`,
			},
		})
		stopServer := startDaemonEquivalenceServer(t, fixture.projectRoot)
		defer stopServer()
		writeDaemonEquivalenceFile(t, filepath.Join(fixture.projectRoot, "force-app/main/default/classes/FreshLocalTest.cls"),
			`@isTest private class FreshLocalTest { @isTest static void passes() { System.assert(true); } }`)

		local := runNoCacheRoutingInvocation(t, fixture.projectRoot, "--no-cache")
		assertNoCacheRoutingResult(t, local, 2, "CachedTest.passes", "FreshLocalTest.passes")

		connected := runNoCacheRoutingInvocation(t, fixture.projectRoot, "--connect", "--no-cache")
		assertNoCacheRoutingResult(t, connected, 1, "CachedTest.passes")
	})

	t.Run("compatible_mode_parity", func(t *testing.T) {
		scenario := daemonEquivalenceScenario{
			name: "no_cache_compatible_modes",
			classes: map[string]string{
				"AlphaTest": `@isTest private class AlphaTest { @isTest static void passes() { System.assert(true); } }`,
				"BetaTest":  `@isTest private class BetaTest { @isTest static void passes() { System.assert(true); } }`,
				"GammaTest": `@isTest private class GammaTest { @isTest static void passes() { System.assert(true); } }`,
				"DeltaTest": `@isTest private class DeltaTest { @isTest static void passes() { System.assert(true); } }`,
			},
			clientInputs: true,
			args:         []string{"--parallelism", "4", "--parallel-methods", "--no-cache"},
			wantExit:     0,
			wantTotal:    4,
			wantPass:     4,
		}
		modes := []daemonEquivalenceMode{
			{name: "local", args: []string{"--no-serve"}},
			{name: "in_process_daemon", args: []string{"--daemon"}},
			{name: "explicit_connect", args: []string{"--connect"}, realServer: true},
		}
		oracle := runDaemonEquivalenceInvocation(t, scenario, modes[0])
		assertDaemonEquivalenceScenario(t, scenario, modes[0], oracle)
		for _, mode := range modes[1:] {
			got := runDaemonEquivalenceInvocation(t, scenario, mode)
			assertDaemonEquivalenceScenario(t, scenario, mode, got)
			if !reflect.DeepEqual(got, oracle) {
				t.Errorf("%s diverged from no-cache local oracle (-want +got):\nwant %s\n got %s", mode.name, formatDaemonEquivalenceSnapshot(oracle), formatDaemonEquivalenceSnapshot(got))
			}
		}
	})
}

func runNoCacheRoutingInvocation(t *testing.T, root string, extraArgs ...string) testreport.Run {
	t.Helper()
	args := []string{"test", "--project", root, "--json", "--no-progress"}
	args = append(args, extraArgs...)
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), args, &stdout, &stderr)
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Data     testreport.Run     `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("decode no-cache JSON envelope: %v\nstdout=%s\nstderr=%s", err, stdout.String(), stderr.String())
	}
	if exit != 0 || envelope.ExitCode != exit || envelope.Status != "passed" {
		t.Fatalf("no-cache exit/status = %d/%d/%q; stdout=%s stderr=%s", exit, envelope.ExitCode, envelope.Status, stdout.String(), stderr.String())
	}
	if summary := envelope.Data.Summary(); !reflect.DeepEqual(envelope.Summary, summary) {
		t.Fatalf("no-cache envelope summary = %#v, data summary = %#v", envelope.Summary, summary)
	}
	return envelope.Data
}

func assertNoCacheRoutingResult(t *testing.T, run testreport.Run, wantTotal int, wantCases ...string) {
	t.Helper()
	summary := run.Summary()
	if summary.Total != wantTotal || summary.Passed != wantTotal || summary.Failed != 0 || summary.Errors != 0 {
		t.Fatalf("no-cache summary = %#v, want %d passes", summary, wantTotal)
	}
	gotCases := make(map[string]bool, summary.Total)
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			gotCases[testCase.ClassName+"."+testCase.MethodName] = true
		}
	}
	wantCaseSet := make(map[string]bool, len(wantCases))
	for _, testCase := range wantCases {
		wantCaseSet[testCase] = true
	}
	if !reflect.DeepEqual(gotCases, wantCaseSet) {
		t.Fatalf("no-cache cases = %#v, want %#v", gotCases, wantCaseSet)
	}
}

type daemonEquivalenceScenario struct {
	name               string
	classes            map[string]string
	args               []string
	clientInputs       bool
	wantExit           int
	wantTotal          int
	wantPass           int
	wantCase           string
	wantCaseStatus     testreport.Status
	wantProblemType    string
	wantProblemMessage string
}

type daemonEquivalenceMode struct {
	name       string
	args       []string
	realServer bool
}

type daemonEquivalenceSnapshot struct {
	Exit      int
	Status    string
	Summary   testreport.Summary
	Run       testreport.Run
	Selection watch.TestSelection
	Stderr    string
	Artifacts daemonEquivalenceArtifacts
}

type daemonEquivalenceArtifacts struct {
	LastFailed      *lastFailedState
	DurationHistory *cliDurationHistory
	ClassFile       string
	HistoryInput    string
}

func runDaemonEquivalenceInvocation(t *testing.T, scenario daemonEquivalenceScenario, mode daemonEquivalenceMode) daemonEquivalenceSnapshot {
	t.Helper()
	fixture := newDaemonEquivalenceFixture(t, scenario)
	var stopServer func()
	if mode.realServer {
		stopServer = startDaemonEquivalenceServer(t, fixture.projectRoot)
		defer stopServer()
		poisonDaemonEquivalenceLocalLoad(t, fixture.projectRoot)
	}
	args := []string{"test", "--project", fixture.projectRoot, "--json", "--no-progress"}
	args = append(args, scenario.args...)
	if scenario.clientInputs {
		args = append(args, "--class-file", fixture.classFile, "--duration-history", fixture.durationHistory)
	}
	args = append(args, mode.args...)
	var stdout, stderr bytes.Buffer
	exit := Run(context.Background(), args, &stdout, &stderr)
	var envelope struct {
		Status   string             `json:"status"`
		ExitCode int                `json:"exitCode"`
		Summary  testreport.Summary `json:"summary"`
		Data     testreport.Run     `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("%s/%s decode JSON envelope: %v\nstdout=%s\nstderr=%s", scenario.name, mode.name, err, stdout.String(), stderr.String())
	}
	if envelope.ExitCode != exit {
		t.Fatalf("%s/%s envelope exit=%d process exit=%d", scenario.name, mode.name, envelope.ExitCode, exit)
	}
	run := normalizeDaemonEquivalenceRun(t, envelope.Data, fixture.baseRoot)
	derivedSummary := run.Summary()
	derivedSummary.DurationMS = 0
	envelope.Summary.DurationMS = 0
	if !reflect.DeepEqual(envelope.Summary, derivedSummary) {
		t.Fatalf("%s/%s envelope summary = %#v, data summary = %#v", scenario.name, mode.name, envelope.Summary, derivedSummary)
	}
	return daemonEquivalenceSnapshot{
		Exit:      exit,
		Status:    envelope.Status,
		Summary:   envelope.Summary,
		Run:       run,
		Selection: watch.TestSelection{},
		Stderr:    strings.ReplaceAll(stderr.String(), fixture.baseRoot, "$ROOT"),
		Artifacts: readDaemonEquivalenceArtifacts(t, fixture),
	}
}

func assertDaemonEquivalenceScenario(t *testing.T, scenario daemonEquivalenceScenario, mode daemonEquivalenceMode, got daemonEquivalenceSnapshot) {
	t.Helper()
	if got.Exit != scenario.wantExit || got.Summary.Total != scenario.wantTotal || got.Summary.Passed != scenario.wantPass {
		t.Errorf("%s/%s exit/summary = %d/%#v", scenario.name, mode.name, got.Exit, got.Summary)
	}
	if len(got.Run.Dependencies) == 0 {
		t.Errorf("%s/%s omitted the fixture dependency", scenario.name, mode.name)
	}
	if scenario.wantCase != "" {
		if len(got.Run.Suites) != 1 || len(got.Run.Suites[0].Cases) != 1 {
			t.Errorf("%s/%s exact selector run = %#v", scenario.name, mode.name, got.Run)
		} else {
			gotCase := got.Run.Suites[0].Cases[0]
			if gotCase.ClassName+"."+gotCase.MethodName != scenario.wantCase {
				t.Errorf("%s/%s selected case = %s.%s", scenario.name, mode.name, gotCase.ClassName, gotCase.MethodName)
			}
		}
	}
	if scenario.wantCaseStatus != "" {
		testCase := firstDaemonEquivalenceCase(got.Run)
		if testCase == nil {
			t.Errorf("%s/%s omitted the expected result case", scenario.name, mode.name)
		} else {
			if testCase.Status != scenario.wantCaseStatus {
				t.Errorf("%s/%s case status = %q, want %q", scenario.name, mode.name, testCase.Status, scenario.wantCaseStatus)
			}
			if testCase.Problem == nil {
				t.Errorf("%s/%s case problem is nil", scenario.name, mode.name)
			} else if testCase.Problem.Type != scenario.wantProblemType || testCase.Problem.Message != scenario.wantProblemMessage {
				t.Errorf("%s/%s case problem = %#v, want type %q message %q", scenario.name, mode.name, testCase.Problem, scenario.wantProblemType, scenario.wantProblemMessage)
			}
		}
	}
}

type daemonEquivalenceFixture struct {
	baseRoot        string
	projectRoot     string
	classFile       string
	durationHistory string
}

func newDaemonEquivalenceFixture(t *testing.T, scenario daemonEquivalenceScenario) daemonEquivalenceFixture {
	t.Helper()
	baseRoot, err := os.MkdirTemp("/tmp", "glade-28c-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(baseRoot); err != nil {
			t.Errorf("remove daemon equivalence fixture %s: %v", baseRoot, err)
		}
	})
	projectRoot := filepath.Join(baseRoot, "project")
	dependencyRoot := filepath.Join(baseRoot, "dependency")
	writeDaemonEquivalenceFile(t, filepath.Join(projectRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeDaemonEquivalenceFile(t, filepath.Join(projectRoot, "glade.yml"), "project:\n  managedPackageDependencies: [\"dep:../dependency:1.0\"]\n")
	writeDaemonEquivalenceFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{"namespace":"dep","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeDaemonEquivalenceFile(t, filepath.Join(dependencyRoot, "force-app/main/default/classes/Dependency.cls"), "global class Dependency { global static Integer value() { return 7; } }\n")
	for className, source := range scenario.classes {
		writeDaemonEquivalenceFile(t, filepath.Join(projectRoot, "force-app/main/default/classes", className+".cls"), source+"\n")
	}
	fixture := daemonEquivalenceFixture{baseRoot: baseRoot, projectRoot: projectRoot}
	if scenario.clientInputs {
		fixture.classFile = filepath.Join(baseRoot, "client", "classes.txt")
		fixture.durationHistory = filepath.Join(baseRoot, "client", "durations.json")
		writeDaemonEquivalenceFile(t, fixture.classFile, "DeltaTest\nAlphaTest\nGammaTest\nBetaTest\n")
		writeDaemonEquivalenceFile(t, fixture.durationHistory, `{
  "classDurations": {"AlphaTest": 40, "BetaTest": 30, "GammaTest": 20, "DeltaTest": 10},
  "methodDurations": {"AlphaTest.passes": 4, "BetaTest.passes": 3, "GammaTest.passes": 2, "DeltaTest.passes": 1}
}
`)
	}
	return fixture
}

func startDaemonEquivalenceServer(t *testing.T, root string) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	server, err := testdaemon.NewServer(testdaemon.ServerConfig{Root: root, Warm: false, Watch: false})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cancel()
		if err := server.Close(); err != nil {
			t.Errorf("close daemon equivalence server: %v", err)
		}
	})
	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe(ctx, io.Discard) }()
	waitForServer(t, ctx, testdaemon.ServeSocketPath(root))
	return func() {
		cancel()
		select {
		case err := <-errCh:
			if err != nil {
				t.Errorf("serve exit: %v", err)
			}
		case <-time.After(5 * time.Second):
			if err := server.Close(); err != nil {
				t.Errorf("force-close daemon equivalence server: %v", err)
			}
			t.Error("server did not stop")
		}
	}
}

func poisonDaemonEquivalenceLocalLoad(t *testing.T, root string) {
	t.Helper()
	writeDaemonEquivalenceFile(t, filepath.Join(root, "sfdx-project.json"), "{\n")
}

func normalizeDaemonEquivalenceRun(t *testing.T, run testreport.Run, baseRoot string) testreport.Run {
	t.Helper()
	run.DurationMS = 0
	for suiteIndex := range run.Suites {
		run.Suites[suiteIndex].DurationMS = 0
		for caseIndex := range run.Suites[suiteIndex].Cases {
			run.Suites[suiteIndex].Cases[caseIndex].DurationMS = 0
		}
	}
	data, err := json.Marshal(run)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte(baseRoot), []byte("$ROOT"))
	if err := json.Unmarshal(data, &run); err != nil {
		t.Fatal(err)
	}
	return run
}

func readDaemonEquivalenceArtifacts(t *testing.T, fixture daemonEquivalenceFixture) daemonEquivalenceArtifacts {
	t.Helper()
	artifacts := daemonEquivalenceArtifacts{}
	failedStatePath, err := lastFailedPath(fixture.projectRoot)
	if err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(failedStatePath); err == nil {
		var state lastFailedState
		if err := json.Unmarshal(data, &state); err != nil {
			t.Fatal(err)
		}
		state.ProjectRoot = strings.ReplaceAll(state.ProjectRoot, fixture.baseRoot, "$ROOT")
		state.UpdatedAt = time.Time{}
		artifacts.LastFailed = &state
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	durationPath := defaultCLIDurationHistoryPath(fixture.projectRoot)
	if history, err := loadCLIDurationHistory(durationPath); err == nil {
		for key := range history.Classes {
			history.Classes[key] = 0
		}
		for key := range history.Methods {
			history.Methods[key] = 0
		}
		artifacts.DurationHistory = &history
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if fixture.classFile != "" {
		data, err := os.ReadFile(fixture.classFile)
		if err != nil {
			t.Fatal(err)
		}
		artifacts.ClassFile = string(data)
	}
	if fixture.durationHistory != "" {
		data, err := os.ReadFile(fixture.durationHistory)
		if err != nil {
			t.Fatal(err)
		}
		artifacts.HistoryInput = string(data)
	}
	return artifacts
}

func firstDaemonEquivalenceCase(run testreport.Run) *testreport.Case {
	for suiteIndex := range run.Suites {
		for caseIndex := range run.Suites[suiteIndex].Cases {
			return &run.Suites[suiteIndex].Cases[caseIndex]
		}
	}
	return nil
}

func formatDaemonEquivalenceSnapshot(snapshot daemonEquivalenceSnapshot) string {
	data, _ := json.MarshalIndent(snapshot, "", "  ")
	return string(data)
}

type daemonEquivalenceShardFile struct {
	Name   string
	Bytes  string
	SHA256 string
}

func runDaemonEquivalenceShardPlan(t *testing.T, mode daemonEquivalenceMode) []daemonEquivalenceShardFile {
	t.Helper()
	scenario := daemonEquivalenceScenario{
		classes: map[string]string{
			"AlphaTest": `@isTest private class AlphaTest { @isTest static void passes() { System.assert(true); } }`,
			"BetaTest":  `@isTest private class BetaTest { @isTest static void passes() { System.assert(true); } }`,
			"GammaTest": `@isTest private class GammaTest { @isTest static void passes() { System.assert(true); } }`,
			"DeltaTest": `@isTest private class DeltaTest { @isTest static void passes() { System.assert(true); } }`,
		},
	}
	fixture := newDaemonEquivalenceFixture(t, scenario)
	historyPath := filepath.Join(fixture.baseRoot, "client", "shard-history.json")
	writeDaemonEquivalenceFile(t, historyPath, `{"classDurations":{"AlphaTest":40,"BetaTest":30,"GammaTest":20,"DeltaTest":10}}`)
	shardDir := filepath.Join(fixture.baseRoot, "client", "shards")
	args := []string{
		"test", "--project", fixture.projectRoot, "--json", "--no-progress",
		"--write-class-shards", shardDir, "--shard-count", "3", "--duration-history", historyPath,
	}
	var stopServer func()
	if mode.realServer {
		stopServer = startDaemonEquivalenceServer(t, fixture.projectRoot)
		defer stopServer()
		poisonDaemonEquivalenceLocalLoad(t, fixture.projectRoot)
	}
	args = append(args, mode.args...)
	var stdout, stderr bytes.Buffer
	if code := Run(context.Background(), args, &stdout, &stderr); code != 0 {
		t.Fatalf("class shard exit=%d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	entries, err := os.ReadDir(shardDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("class shard files = %d, want 3", len(entries))
	}
	out := make([]daemonEquivalenceShardFile, 0, len(entries))
	for index, entry := range entries {
		wantName := fmt.Sprintf("shard-%03d.txt", index)
		if entry.Name() != wantName {
			t.Fatalf("class shard file %d = %q, want %q", index, entry.Name(), wantName)
		}
		data, err := os.ReadFile(filepath.Join(shardDir, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(data)
		out = append(out, daemonEquivalenceShardFile{Name: entry.Name(), Bytes: string(data), SHA256: hex.EncodeToString(digest[:])})
	}
	if err := filepath.WalkDir(fixture.projectRoot, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if strings.HasPrefix(entry.Name(), "shard-") {
			return fmt.Errorf("server-side shard artifact crossed client boundary: %s", path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return out
}

func writeDaemonEquivalenceFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
