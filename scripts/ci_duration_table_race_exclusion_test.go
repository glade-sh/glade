package scripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

var raceExcludedTestPattern = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)

// Every package with a checked-in shard duration table also runs under the
// race detector, where tests excluded by a !race build tag do not exist. A
// table row naming such a test fails the package's own partition proof under
// the race detector, so excluded test names must never enter the table.
func TestDurationTablesExcludeRaceBuildTaggedTests(t *testing.T) {
	for _, pkg := range []string{"apextest", "gladecli", "sema", "visualforce", "vm"} {
		t.Run(pkg, func(t *testing.T) {
			t.Parallel()
			tablePath := filepath.Join("..", "internal", pkg, "testdata", "shard_durations.json")
			data, err := os.ReadFile(tablePath)
			if err != nil {
				t.Fatal(err)
			}
			var table struct {
				Tests []struct {
					Name string `json:"name"`
				} `json:"tests"`
			}
			if err := json.Unmarshal(data, &table); err != nil {
				t.Fatal(err)
			}
			rows := make(map[string]bool, len(table.Tests))
			for _, row := range table.Tests {
				rows[row.Name] = true
			}
			sources, err := filepath.Glob(filepath.Join("..", "internal", pkg, "*_test.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, source := range sources {
				body, err := os.ReadFile(source)
				if err != nil {
					t.Fatal(err)
				}
				if !hasRaceExclusionBuildTag(string(body)) {
					continue
				}
				for _, match := range raceExcludedTestPattern.FindAllStringSubmatch(string(body), -1) {
					if rows[match[1]] {
						t.Errorf("%s names race-excluded test %s, which does not exist in a race build", filepath.Base(source), match[1])
					}
				}
			}
		})
	}
}

// hasRaceExclusionBuildTag reports whether a test file's build constraints
// exclude it from race builds. Build tags can appear in either the modern
// //go:build form or the legacy // +build form the compiler also honors.
func hasRaceExclusionBuildTag(body string) bool {
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, "//go:build ") || strings.HasPrefix(line, "// +build ") {
			if line == "//go:build !race" || line == "// +build !race" {
				return true
			}
			return false
		}
	}
	return false
}
