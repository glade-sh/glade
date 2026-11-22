package apextest

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// The captured anonymous case uses the same class metadata after disk restore.
func TestExceptionRHSAnonymousDiskRestore(t *testing.T) {
	t.Setenv("GLADE_HOME", t.TempDir())
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	t.Cleanup(ResetPerfCounters)
	dir := filepath.Join("testdata", "exception_rhs_call_frames")
	entries, err := os.ReadDir(filepath.Join(dir, "classes"))
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]string{}
	for _, entry := range entries {
		name := strings.TrimSuffix(entry.Name(), ".cls")
		if !strings.HasSuffix(name, "_67") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, "classes", entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		classes[name] = string(data)
	}
	cases, err := os.ReadFile(filepath.Join(dir, "cases.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	testClass := ""
	for _, line := range strings.Split(strings.TrimSpace(string(cases)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 3 {
			t.Fatal("invalid case")
		}
		if row[0] == "direct_test" {
			testClass = row[2] + "_67"
		}
	}
	if testClass == "" {
		t.Fatal("missing named control")
	}
	rows, err := os.ReadFile(filepath.Join(dir, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	want := ""
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n")[1:] {
		row := strings.Split(line, "\t")
		if len(row) != 4 {
			t.Fatal("invalid native row")
		}
		if row[0] == "direct_anonymous" && row[1] == "67.0" {
			want = strings.ReplaceAll(row[3], `\n`, "\n")
		}
	}
	first, _, ok := strings.Cut(want, "\n")
	symbol, _, found := strings.Cut(first, ": line ")
	if !ok || !found || !strings.HasPrefix(symbol, "Class.") || !strings.HasSuffix(symbol, ".fail") {
		t.Fatalf("invalid native trace %q", want)
	}
	probe := strings.TrimSuffix(strings.TrimPrefix(symbol, "Class."), ".fail")
	index := nativeBoundaryIndex(t, "67.0", classes, nil)
	if index.HasErrors() {
		t.Fatalf("index: %v", index.Diagnostics)
	}
	org := orgFromIndex(index)
	org.APIVersion = "67.0"
	cold := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	source, err := os.ReadFile(filepath.Join(dir, "anonymous", "direct.apex"))
	if err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymousWithOptions(strings.ReplaceAll(string(source), strings.TrimSuffix(probe, "_67"), probe), vm.CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	execute := func(runner *conformanceRunner) *vm.RuntimeError {
		_, err := runner.newMachine().Execute(program)
		var fault *vm.RuntimeError
		if !errors.As(err, &fault) {
			t.Fatalf("expected exception: %v", err)
		}
		trace, ok := fault.CapturedAnonymousStackTrace()
		if !ok || trace != want {
			t.Fatalf("captured trace=%q ok=%t", trace, ok)
		}
		return fault
	}
	before := execute(cold)
	InvalidateRuntimeCaches()
	result := Run(index, Options{SelectedClasses: []string{testClass}, Parallelism: 1})
	if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
		t.Fatal("missing named failure")
	}
	InvalidateRuntimeCaches()
	ResetPerfCounters()
	result = Run(index, Options{SelectedClasses: []string{testClass}, Parallelism: 1, PerfCounters: true})
	if SnapshotPerfCounters().Phases.DiskCacheHits != 1 || len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem == nil {
		t.Fatal("runtime was not restored from disk")
	}
	InvalidateRuntimeCaches()
	entry, ok := tryLoadDiskRuntime(index)
	if !ok {
		t.Fatal("disk runtime unavailable")
	}
	runtime := compiledProjectRuntimeFromEntry(entry, index)
	restored := newConformanceRunner(t, index, conformanceRunnerOptions{RequestRuntime: &runtime, Org: &org})
	after := execute(restored)
	if before.Type != after.Type || before.Message != after.Message || !reflect.DeepEqual(before.Stack, after.Stack) {
		t.Fatalf("restored fault=%#v stack=%#v; cold fault=%#v stack=%#v", after, after.Stack, before, before.Stack)
	}
}
