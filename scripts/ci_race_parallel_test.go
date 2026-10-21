package scripts

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

// Exercise the real fixture tests in a child so their scheduling is observable
// without a wall-clock threshold that fails on a busy host.
func TestCIRacePackageRunnerFixtureScheduling(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	const parent = "TestCIRacePackageRunnerRejectsInvalidResourceEvidence"
	cmd := exec.Command(binary, "-test.run=^"+parent+"$", "-test.v", "-test.parallel=2", "-test.timeout=30s")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("fixture tests: %v\n%s", err, out)
	}
	paused, continued, passed := 0, 0, 0
	for line := range strings.SplitSeq(string(out), "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(line, "=== PAUSE "+parent+"/"):
			paused++
		case strings.HasPrefix(line, "=== CONT  "+parent+"/"):
			if paused != 10 {
				t.Fatalf("fixture started before all ten fixtures paused:\n%s", out)
			}
			continued++
		case strings.HasPrefix(line, "--- PASS: "+parent+"/"):
			passed++
		}
	}
	if paused != 10 || continued != 10 || passed != 10 {
		t.Fatalf("fixture scheduling: paused=%d continued=%d passed=%d; want 10 each\n%s", paused, continued, passed, out)
	}
}
