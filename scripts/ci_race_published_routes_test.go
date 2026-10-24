package scripts

import (
	"encoding/json"
	"os"
	"slices"
	"testing"
)

func TestCIRacePushRetainsPublishedDaemonRoutes(t *testing.T) {
	data, err := os.ReadFile("ci-race-push.json")
	if err != nil {
		t.Fatal(err)
	}
	var cfg struct {
		Packages map[string][]string `json:"packages"`
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"TestConnectedRunUsesPublishedTestSelection", "TestConnectedRunUsesPublishedProjectSnapshot"} {
		if !slices.Contains(cfg.Packages["./internal/gladecli"], name) {
			t.Errorf("push race scope omits %s", name)
		}
	}
}
