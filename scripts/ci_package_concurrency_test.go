package scripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// Full-package race selection still needs an explicit concurrency inventory:
// otherwise adding a concurrent test never requires a manifest refresh.
func TestCIPackageConcurrentTestInventory(t *testing.T) {
	var lanes struct {
		Lanes                map[string][]string `json:"lanes"`
		ConcurrentTestCounts map[string]int      `json:"concurrentTestCounts"`
	}
	var race struct {
		Packages         map[string][]string `json:"packages"`
		EstimatedSeconds map[string]int      `json:"estimatedSeconds"`
	}
	for path, target := range map[string]any{"ci-package-lanes.json": &lanes, "ci-race-push.json": &race} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}
	// Sema's shared constructor cache must remain covered even though its race
	// selection is the whole package rather than a list of individual tests.
	if _, ok := lanes.ConcurrentTestCounts["./internal/sema"]; !ok {
		t.Errorf("missing sema concurrency inventory; want %d tests", len(concurrentRaceTests(t, "../internal/sema")))
	}
	for pkg, count := range lanes.ConcurrentTestCounts {
		t.Run(pkg, func(t *testing.T) {
			if !strings.HasPrefix(pkg, "./internal/") || strings.Contains(pkg, "..") {
				t.Fatalf("invalid package %q", pkg)
			}
			want := concurrentRaceTests(t, filepath.Join("..", pkg))
			if len(want) == 0 || count != len(want) {
				t.Errorf("concurrent test count = %d, want %d", count, len(want))
			}
			owners := 0
			for _, packages := range lanes.Lanes {
				if slices.Contains(packages, "github.com/glade-sh/glade/"+strings.TrimPrefix(pkg, "./")) {
					owners++
				}
			}
			if owners != 1 {
				t.Errorf("package has %d lane owners, want 1", owners)
			}
			selected, ok := race.Packages[pkg]
			if !ok || race.EstimatedSeconds[pkg] <= 0 {
				t.Fatal("concurrent package needs race selection and a positive time estimate")
			}
			for _, name := range want {
				if selected != nil && !slices.Contains(selected, name) {
					t.Errorf("race selection omits %s", name)
				}
			}
		})
	}
}
