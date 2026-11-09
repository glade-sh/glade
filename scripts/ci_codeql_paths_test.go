package scripts

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCICodeQLGeneratedPathsInitializationRoutes(t *testing.T) {
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
		want []string
	}{
		{"pull_request_fast_any_size", "Initialize pull request CodeQL", []string{"**/*_generated.go", "**/*_test.go", "**/testdata/**"}},
		{"full_weekly_release_tag_dispatch", "Initialize full-branch CodeQL", []string{"**/*_generated.go"}},
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
				PathsIgnore []string `yaml:"paths-ignore"`
			}
			if err := yaml.Unmarshal([]byte(configs[0]), &config); err != nil {
				t.Fatalf("parse inline CodeQL config: %v", err)
			}
			if !reflect.DeepEqual(config.PathsIgnore, route.want) {
				t.Errorf("paths-ignore = %q, want %q", config.PathsIgnore, route.want)
			}
		})
	}
}
