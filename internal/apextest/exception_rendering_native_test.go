package apextest

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// Replay the answered exception cells retained in the boundary evidence export.
// JSON rendering, thrown exceptions and other builtin types remain carries.
func TestIllegalArgumentExceptionRenderingNativeConformance(t *testing.T) {
	var data struct {
		Rows []struct {
			ID     string `json:"id"`
			Routes map[string]struct {
				Anonymous string            `json:"anonymous"`
				Classes   map[string]string `json:"classes"`
				Expected  map[string]struct {
					Outcome string         `json:"outcome"`
					Value   map[string]any `json:"value"`
				} `json:"expectedByAPI"`
			} `json:"routes"`
		} `json:"rows"`
	}
	raw, err := os.ReadFile("testdata/conformance/residual_boundary_carries.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	selected := map[string]bool{
		"E-F01-1": true, "E-F01-2": true, "E-F01-3": true, "E-F01-4": true,
		"E-F01-5": true, "E-F02-1": true, "E-F02-2": true,
	}
	cells := 0
	for _, row := range data.Rows {
		if !selected[row.ID] {
			continue
		}
		delete(selected, row.ID)
		for _, api := range []string{"62.0", "67.0", "68.0"} {
			for _, route := range []string{"anonymous", "isTest"} {
				cells++
				t.Run(row.ID+"/"+api+"/"+route, func(t *testing.T) {
					source, ok := row.Routes[route]
					want, answered := source.Expected[api]
					if !ok || !answered || want.Outcome != "runtime" || len(want.Value) != 3 {
						t.Fatal("incomplete native exception observation")
					}
					var message string
					if route == "anonymous" {
						program, err := vm.CompileAnonymousWithOptions(source.Anonymous, vm.CompileOptions{APIVersion: api})
						if err != nil {
							t.Fatal(err)
						}
						_, execErr := vm.New(nil).Execute(program)
						if execErr == nil {
							t.Fatal("missing native observation frame")
						}
						message = execErr.Error()
					} else {
						if len(source.Classes) != 1 {
							t.Fatal("expected one captured named class")
						}
						var name string
						for name = range source.Classes {
						}
						index := nativeBoundaryIndex(t, api, source.Classes, nil)
						run := Run(index, Options{SelectedClasses: []string{name}, SelectedMethod: "observe", NoDiskCache: true, Parallelism: 1})
						if run.Summary().Total != 1 || len(run.Suites) != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
							t.Fatalf("missing named observation: %#v", run)
						}
						message = run.Suites[0].Cases[0].Problem.Message
					}
					_, payload, ok := strings.Cut(message, "BOUNDARY_RESULT|"+row.ID+"|")
					var actual map[string]any
					if !ok || json.Unmarshal([]byte(payload), &actual) != nil {
						t.Fatalf("invalid observation: %s", message)
					}
					if !reflect.DeepEqual(want.Value, actual) {
						t.Fatalf("native=%v; local=%v", want.Value, actual)
					}
				})
			}
		}
	}
	if len(selected) != 0 || cells != 42 {
		t.Fatalf("missing native exception cells: %v; observed=%d", selected, cells)
	}
}
