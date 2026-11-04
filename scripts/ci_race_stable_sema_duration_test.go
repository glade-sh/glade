package scripts

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
)

func TestCIRacePushStableSemaDurations(t *testing.T) {
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
	// Rounded-up local race times, GOMAXPROCS=2, with the memory guard enabled.
	measured := []int{54, 150, 93, 274, 347, 91, 135, 103}
	for part, seconds := range measured {
		name := fmt.Sprintf("TestSemaIgnoredTextMatchesReferenceOnRepositoryApex/part_%d", part)
		if got := cfg.Isolated["./internal/sema"][name]; got < seconds {
			t.Errorf("%s estimate %ds is below measured %ds", name, got, seconds)
		}
	}
}
