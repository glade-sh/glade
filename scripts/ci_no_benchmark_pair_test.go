package scripts

import (
	"os"
	"strings"
	"testing"
)

func TestCINoBenchmarkCachePairHarness(t *testing.T) {
	for _, name := range []string{"ci.yml", "browser.yml"} {
		workflow := readWorkflowFile(t, name)
		for _, term := range []string{"benchmark_cache_pair", "benchmark-cache-pair", "benchmark-npm-cache"} {
			if strings.Contains(workflow, term) {
				t.Errorf("%s still contains %s", name, term)
			}
		}
		if !strings.Contains(workflow, "cache: npm") {
			t.Errorf("%s lost normal npm caching", name)
		}
	}
	if _, err := os.Stat("ci-benchmark-cache-pair.sh"); !os.IsNotExist(err) {
		t.Errorf("benchmark validator still exists: %v", err)
	}
	history, err := os.ReadFile("ci-apex-history-input.sh")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(history), "CI_APEXTEST_BENCHMARK_PAIR") {
		t.Error("history still has benchmark isolation")
	}
}
