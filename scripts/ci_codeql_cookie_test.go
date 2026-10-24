package scripts

import (
	"slices"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCICodeQLCookieQueryInitializationRoutes(t *testing.T) {
	var workflow struct {
		Jobs map[string]struct {
			Steps []struct {
				Name string
				With struct {
					Config string
				}
			}
		}
	}
	if err := yaml.Unmarshal([]byte(readWorkflowFile(t, "codeql.yml")), &workflow); err != nil {
		t.Fatal(err)
	}
	for _, route := range []struct {
		name string
		step string
	}{
		{"pull_request_fast_any_size", "Initialize pull request CodeQL"},
		{"full_weekly_release_tag_dispatch", "Initialize full-branch CodeQL"},
	} {
		t.Run(route.name, func(t *testing.T) {
			var configs []string
			for _, step := range workflow.Jobs["codeql"].Steps {
				if step.Name == route.step {
					configs = append(configs, step.With.Config)
				}
			}
			if len(configs) != 1 {
				t.Fatalf("%s config count = %d, want 1", route.step, len(configs))
			}
			var config struct {
				QueryFilters []struct {
					Exclude struct {
						ID []string
					}
				} `yaml:"query-filters"`
			}
			if err := yaml.Unmarshal([]byte(configs[0]), &config); err != nil {
				t.Fatalf("parse inline CodeQL config: %v", err)
			}
			var excluded []string
			for _, filter := range config.QueryFilters {
				excluded = append(excluded, filter.Exclude.ID...)
			}
			for _, query := range []string{"go/cookie-httponly-not-set", "go/cookie-secure-not-set"} {
				if !slices.Contains(excluded, query) {
					t.Errorf("query-filters exclude IDs omit %s", query)
				}
			}
		})
	}
}
