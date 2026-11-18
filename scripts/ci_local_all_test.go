package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"sort"

	"testing"

	"gopkg.in/yaml.v3"
)

func TestCILocalAll(t *testing.T) {
	cmd := exec.Command("python3", "ci_local_all_test.py")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("local CI runner tests: %v\n%s", err, output)
	}
}

// Read YAML independently of the runner so a new required dependency cannot
// silently disappear from its local mapping.
func TestCILocalAllRequiredWorkflowMapping(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]yaml.Node `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "ci-local-all", "--check-mapping")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("mapping: %v\n%s", err, out)
	}
	var got []string
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	sort.Strings(got)
	var job struct {
		Needs []string `yaml:"needs"`
	}
	node := workflow.Jobs["required-ci"]
	if err := node.Decode(&job); err != nil {
		t.Fatal(err)
	}
	want := job.Needs
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("mapping %v, required %v", got, want)
	}
}

func TestCILocalAllShardInventory(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		Jobs map[string]yaml.Node `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(data, &workflow); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("python3", "ci-local-all", "--check-shards")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("shards: %v %s", err, out)
	}
	var got map[string]int
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apextest", "sema", "gladecli", "vm"} {
		var job struct {
			Strategy struct {
				Matrix struct {
					Shard []int `yaml:"shard"`
				} `yaml:"matrix"`
			} `yaml:"strategy"`
		}
		node := workflow.Jobs[name]
		if err := node.Decode(&job); err != nil {
			t.Fatal(err)
		}
		want := make([]int, got[name])
		for i := range want {
			want[i] = i
		}
		if !reflect.DeepEqual(job.Strategy.Matrix.Shard, want) {
			t.Errorf("%s shards: workflow %v local %v", name, job.Strategy.Matrix.Shard, want)
		}
	}
	var job struct {
		Strategy struct {
			Matrix struct {
				Include []struct {
					Lane string `yaml:"lane"`
				} `yaml:"include"`
			} `yaml:"matrix"`
		} `yaml:"strategy"`
	}
	lanes := map[string]bool{}
	for _, jobName := range []string{"test", "lwc-vf"} {
		node := workflow.Jobs[jobName]
		if err := node.Decode(&job); err != nil {
			t.Fatal(err)
		}
		for _, entry := range job.Strategy.Matrix.Include {
			lanes[entry.Lane] = true
		}
	}
	for _, prefix := range []string{"lwc-compile", "visualforce"} {
		for i := 0; i < got[prefix]; i++ {
			name := fmt.Sprintf("%s-%d", prefix, i)
			if !lanes[name] {
				t.Errorf("missing workflow lane %s", name)
			}
			delete(lanes, name)
		}
	}
	delete(lanes, "remaining-go")
	if len(lanes) != 0 {
		t.Errorf("unmapped matrix lanes: %v", lanes)
	}
}
