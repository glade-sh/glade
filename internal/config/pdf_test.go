package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPDFCommandConfiguration(t *testing.T) {
	for _, command := range []string{"", "/opt/tools/render-pdf"} {
		cfg, err := parseYAMLSubset("pdf:\n  command: '" + command + "'\n")
		if err != nil {
			t.Fatal(err)
		}
		data, _ := json.Marshal(cfg)
		var fields map[string]json.RawMessage
		json.Unmarshal(data, &fields)
		if !strings.Contains(string(fields["pdf"]), `"command":"`+command+`"`) {
			t.Fatalf("pdf config = %s", data)
		}
	}
	if _, err := parseYAMLSubset("pdf:\n  command: relative-renderer\n"); err == nil {
		t.Fatal("relative executable should be rejected")
	}
}
