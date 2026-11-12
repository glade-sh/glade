package scripts

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCICase319BalanceFailureSchedulingProfile(t *testing.T) {
	t.Setenv("GITHUB_EVENT_NAME", "push")
	for _, invalid := range []bool{false, true} {
		t.Run(map[bool]string{false: "valid", true: "invalid"}[invalid], func(t *testing.T) {
			fixture := durationHistoryRouteFixture(t, fixturePackage, [2]float64{193.89, 51.65})
			setPlanHistoryUsed(t, fixture.shards[:], true)
			profile := filepath.Join(t.TempDir(), "scheduling.json")
			t.Setenv("CI_APEXTEST_SCHEDULING_PROFILE", profile)
			if invalid {
				if err := os.WriteFile(filepath.Join(fixture.shards[1], "events.json"), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			out, err := runDurationHistoryRoute(t, "apex", "cli", fixture)
			if err == nil {
				t.Fatalf("expected rejection: %s", out)
			}
			if _, err := os.Stat(fixture.output); !os.IsNotExist(err) {
				t.Fatalf("accepted history published: %v", err)
			}
			data, err := os.ReadFile(profile)
			if invalid {
				if !os.IsNotExist(err) {
					t.Fatalf("invalid evidence published profile: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("validated observations missing: %v; %s", err, out)
			}
			var got struct {
				Package string `json:"package"`
				Tests   []struct {
					Name           string `json:"name"`
					DurationMillis int64  `json:"durationMillis"`
				} `json:"tests"`
			}
			if err := json.Unmarshal(data, &got); err != nil || got.Package != fixturePackage || len(got.Tests) != 2 || got.Tests[0].DurationMillis != 193890 || got.Tests[1].DurationMillis != 51650 {
				t.Fatalf("unexpected profile %s: %v", data, err)
			}
		})
	}
}

func TestCICase319ApexPredecessorBootstrapWorkflow(t *testing.T) {
	data, err := os.ReadFile("../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	start := strings.Index(text, "  history-input:")
	end := strings.Index(text, "\n  gladecli:\n")
	if start < 0 || end <= start {
		t.Fatal("missing Apex history jobs")
	}
	job := text[start:end]
	for _, required := range []string{"actions: read", "scripts/ci-apex-history-input.sh", "CI_APEXTEST_BENCHMARK_PAIR"} {
		if !strings.Contains(job, required) {
			t.Errorf("missing predecessor bootstrap contract %q", required)
		}
	}
}
