package apextest

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestGrandchildProjectionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "grandchild_projection", "OrchardDescendantProbeTest", "62.0", "67.0")
}

func TestGrandchildProjectionRepeatedAnonymous(t *testing.T) {
	const dir = "testdata/conformance/grandchild_projection"
	index := loadTestIndex(t, dir+"/project")
	org := orgFromIndex(index)
	base := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	source, err := os.ReadFile(dir + "/anonymous.apex")
	if err != nil {
		t.Fatal(err)
	}
	program, err := vm.CompileAnonymousWithOptions(string(source), vm.CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	rows, err := os.ReadFile(dir + "/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	var want map[string]bool
	for _, line := range strings.Split(strings.TrimSpace(string(rows)), "\n") {
		if cell, found := strings.CutPrefix(line, "62.0\tanonymous\t"); found {
			if err := json.Unmarshal([]byte(cell), &want); err != nil {
				t.Fatal(err)
			}
		}
	}
	if len(want) != 6 {
		t.Fatal("missing complete native anonymous observation")
	}
	for iteration := range 200 {
		t.Run(fmt.Sprint(iteration), func(t *testing.T) {
			result, err := base.execute(program)
			if err != nil {
				t.Fatal(err)
			}
			if len(result.Debug) != 1 {
				t.Fatalf("missing observation: %v", result.Debug)
			}
			var observed map[string]bool
			if err := json.Unmarshal([]byte(strings.TrimPrefix(result.Debug[0], "OBS|")), &observed); err != nil {
				t.Fatal(err)
			}
			if !maps.Equal(observed, want) {
				t.Errorf("native=%v; Glade=%v", want, observed)
			}
		})
	}
}
