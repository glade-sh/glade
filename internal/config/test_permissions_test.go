package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTestPermissionsConfig(t *testing.T) {
	cfg, err := parseYAMLSubset("test:\n  permissionSets: [Synthetic_Access]\n  permissionSetGroups: [Synthetic_Group]\n")
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(cfg)
	for _, want := range []string{`"test":`, `"permissionSets":["Synthetic_Access"]`, `"permissionSetGroups":["Synthetic_Group"]`} {
		if !strings.Contains(string(raw), want) {
			t.Fatalf("config %s lacks %s", raw, want)
		}
	}
	for _, source := range []string{"test:\n  permissionSets: Synthetic_Access\n", "test:\n  permissionSetGroups: Synthetic_Group\n", "test:\n  permissionSets: [\"\"]\n"} {
		if _, err := parseYAMLSubset(source); err == nil {
			t.Fatalf("accepted invalid config: %s", source)
		}
	}
}
