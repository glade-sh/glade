package scripts

import (
	"encoding/json"
	"os"
	"testing"
)

func TestCIRacePushSemaSlowPartitionBudget(t *testing.T) {
	data, err := os.ReadFile("ci-race-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Isolated map[string]map[string]int `json:"isolatedTests"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	total := 0
	for _, seconds := range cfg.Isolated["./internal/sema"] {
		total += seconds
	}
	// The prior complete scan took over 1,280 seconds in total. The new
	// stable path assignment must retain a budget covering that measured work.
	if total < 1280 {
		t.Fatalf("sema scan estimate %ds is below the observed total", total)
	}
}
