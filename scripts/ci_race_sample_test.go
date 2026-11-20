package scripts

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func writeRaceSampleFixture(t *testing.T, dir string, files map[string]string) {
	t.Helper()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o640); err != nil {
			t.Fatal(err)
		}
	}
}

// The nightly conformance sample selects by name hash within the 120 s
// normal-build cut, adds the package's own t.Parallel() tests, and drops the
// ten slowest race tests ever measured so the nightly stays cheap; the weekly
// backstop still runs those whole.
func TestCIRaceSampleSelectionRule(t *testing.T) {
	dir := t.TempDir()
	writeRaceSampleFixture(t, dir, map[string]string{
		"shard_durations.json": `{"version":1,"package":"fixture","tests":[
  {"name":"TestSampleSlowRow","durationMillis":120000},
  {"name":"TestSampleHashPickedName76","durationMillis":119999},
  {"name":"TestSampleParallelNever","durationMillis":0}
]}`,
		"sample_a_test.go": `package fixture

import "testing"

func TestSampleHashPickedName5(t *testing.T) {}
func TestSampleNeverHashed(t *testing.T) {}
func TestSampleSlowRow(t *testing.T) {}
func TestSampleHashPickedName76(t *testing.T) {}
func TestSampleParallelNever(t *testing.T) { t.Parallel() }
func TestSampleParallelAlways(t *testing.T) { t.Parallel() }
func TestCartExtensionDirectProjectNamedIsTestTriggerNativeRoutes(t *testing.T) { t.Parallel() }
func TestSampleParallelBoundary(t *testing.T) {}

func helper() {
	// A t.Parallel() call below the next top-level func must not attach to
	// the test above it.
}
`,
		"sample_b_test.go": `package fixture

import "testing"

func TestPlain(t *testing.T) {}
`,
	})
	names := strings.Join([]string{
		"TestSampleHashPickedName5", "TestSampleNeverHashed", "TestSampleSlowRow",
		"TestSampleHashPickedName76", "TestSampleParallelNever", "TestSampleParallelAlways",
		"TestCartExtensionDirectProjectNamedIsTestTriggerNativeRoutes", "TestSampleParallelBoundary", "TestPlain",
	}, "\n") + "\nok github.com/glade-sh/glade/internal/fixture 0.01s\n"

	run := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("python3", append([]string{"ci-race-sample.py", filepath.Join(dir, "shard_durations.json"), dir}, args...)...)
		cmd.Stdin = strings.NewReader(names + "\n")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("race sample: %v\n%s", err, out)
		}
		return strings.TrimSpace(string(out))
	}

	// Hash selection: the sampled name with no duration row stays eligible,
	// the never-sampled names are out, the 120000 ms row sits at the cut and
	// the 119999 ms row inside it.
	regex := run()
	for _, want := range []string{"TestSampleHashPickedName5", "TestSampleHashPickedName76"} {
		if !strings.Contains(regex, want) {
			t.Errorf("sample regex %q must keep %q", regex, want)
		}
	}
	for _, drop := range []string{"TestSampleNeverHashed", "TestSampleSlowRow", "TestSampleParallelAlways", "TestPlain"} {
		if strings.Contains(regex, drop) {
			t.Errorf("sample regex %q must not select %q without --parallel", regex, drop)
		}
	}

	// The parallel additions: t.Parallel() tests join the sample, the call in
	// the helper below the next top-level func stays unattached, and the
	// slowest-measured parallel test stays excluded.
	regex = run("--parallel")
	for _, want := range []string{"TestSampleHashPickedName5", "TestSampleHashPickedName76", "TestSampleParallelNever", "TestSampleParallelAlways"} {
		if !strings.Contains(regex, want) {
			t.Errorf("parallel sample regex %q must keep %q", regex, want)
		}
	}
	for _, drop := range []string{"TestCartExtensionDirectProjectNamedIsTestTriggerNativeRoutes", "TestSampleParallelBoundary"} {
		if strings.Contains(regex, drop) {
			t.Errorf("parallel sample regex %q must not select %q", regex, drop)
		}
	}
}

func TestCIRaceSampleRejectsEmptyAndDuplicateInput(t *testing.T) {
	dir := t.TempDir()
	writeRaceSampleFixture(t, dir, map[string]string{
		"shard_durations.json": `{"version":1,"package":"fixture","tests":[]}`,
	})
	for _, tc := range []struct {
		name  string
		stdin string
	}{
		{"no names", ""},
		{"duplicate names", "TestPlain\nTestPlain\n"},
		{"empty selection", "TestPlain\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command("python3", "ci-race-sample.py", filepath.Join(dir, "shard_durations.json"), dir)
			cmd.Stdin = strings.NewReader(tc.stdin)
			out, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(out), "race sample:") {
				t.Fatalf("input must fail closed: %v\n%s", err, out)
			}
		})
	}
}
