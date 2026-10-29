//go:build darwin || linux

package apextest

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"testing"
)

// Measure the entire project route, including fixture preparation, loading,
// indexing, compilation, diagnostics and the healthy test's execution. Each
// child starts cold: a parent-process high-water mark or retained runtime cache
// would hide a missing-dependency regression. Children run strictly serially.
// The allocation guard separately protects the early root-attribution shortcut.
func TestCompileGateMissingDependencyProjectRSS(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, limit := range []string{"3GiB", "unset"} {
			t.Run(api+"/"+limit, func(t *testing.T) {
				peaks := make(map[string]int64)
				for _, dependency := range []string{"present", "missing"} {
					selection := "^TestCompileGateMissingDependencyProjectMemory$/^" + strings.ReplaceAll(api, ".", "[.]") + "$/^" + dependency + "$"
					cmd := exec.Command(binary, "-test.run="+selection, "-test.count=1", "-test.v", "-test.timeout=5m")
					for _, entry := range os.Environ() {
						key, _, _ := strings.Cut(entry, "=")
						switch key {
						case "GOMAXPROCS", "GOGC", "GOMEMLIMIT", "GLADE_DEPENDENCY_MEMORY_CLASSES", "GLADE_HOME":
							continue
						}
						cmd.Env = append(cmd.Env, entry)
					}
					cmd.Env = append(cmd.Env, "GOMAXPROCS=1", "GLADE_DEPENDENCY_MEMORY_CLASSES=2500", "GLADE_HOME="+t.TempDir())
					if limit != "unset" {
						cmd.Env = append(cmd.Env, "GOMEMLIMIT="+limit)
					}
					output, err := cmd.CombinedOutput()
					if err != nil {
						t.Fatalf("%s project: %v\n%s", dependency, err, output)
					}
					// A successful process with an empty test selection is not proof.
					marker := fmt.Sprintf("project outcomes: classes=2500 dependency=%s sha256=", dependency)
					if strings.Count(string(output), marker) != 1 {
						t.Fatalf("%s project was not exercised exactly once:\n%s", dependency, output)
					}
					usage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
					if !ok || usage.Maxrss <= 0 {
						t.Fatalf("%s project has no peak RSS measurement", dependency)
					}
					peak := usage.Maxrss
					if runtime.GOOS == "linux" {
						peak *= 1024 // Linux reports KiB; Darwin reports bytes.
					}
					peaks[dependency] = peak
					t.Logf("classes=2500 api=%s GOMEMLIMIT=%s dependency=%s peakRSSBytes=%d\n%s", api, limit, dependency, peak, output)
				}
				// A missing sibling must stay within twice the healthy project's
				// peak, rather than the historical >2.4x error-path amplification.
				if peaks["missing"] > 2*peaks["present"] {
					t.Errorf("missing dependency peak RSS %d exceeds 2x healthy peak RSS %d", peaks["missing"], peaks["present"])
				}
			})
		}
	}
}
