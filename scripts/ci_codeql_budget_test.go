package scripts

import (
	"encoding/json"
	"os"
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCICodeQLRecordedWaiverBudgetRoutes(t *testing.T) {
	body, err := os.ReadFile("../.github/codeql-budget.json")
	if err != nil {
		t.Fatalf("read recorded CodeQL budget: %v", err)
	}
	var budget map[string]int
	if err := json.Unmarshal(body, &budget); err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string
				With struct{ Config string }
			}
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "codeql.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct{ name, key string }{{"Initialize full-branch CodeQL", "full_waivers"}, {"Initialize pull request CodeQL", "pr_waivers"}} {
		t.Run(route.name, func(t *testing.T) {
			count := 0
			for _, step := range workflow.Jobs["codeql"].Steps {
				if step.Name == route.name {
					count++
					var config struct {
						Filters []struct{ Exclude struct{ ID []string } } `yaml:"query-filters"`
					}
					if err := yaml.Unmarshal([]byte(step.With.Config), &config); err != nil {
						t.Fatal(err)
					}
					var ids []string
					for _, filter := range config.Filters {
						ids = append(ids, filter.Exclude.ID...)
					}
					slices.Sort(ids)
					ids = slices.Compact(ids)
					if budget[route.key] <= 0 || len(ids) > budget[route.key] {
						t.Fatalf("%d distinct waivers exceed recorded budget %d", len(ids), budget[route.key])
					}
				}
			}
			if count != 1 {
				t.Fatalf("found %d initialization routes, want 1", count)
			}
		})
	}
}
