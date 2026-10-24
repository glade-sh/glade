package apextest

import (
	"crypto/sha256"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func conformanceFlowSchemaFiles(files map[string]string) map[string]string {
	schema := make(map[string]string)
	for path, source := range files {
		if !strings.HasSuffix(path, ".flow-meta.xml") {
			schema[path] = source
		}
	}
	return schema
}

func TestConformanceSchemaFlowVariantsShareBase(t *testing.T) {
	raw, err := os.ReadFile("testdata/conformance/flow_core/ordering.json")
	if err != nil {
		t.Fatal(err)
	}
	var data struct {
		Rows []struct {
			ID    string
			ByAPI map[string]struct{ Files map[string]string }
		}
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Rows) != 7 {
		t.Fatal("captured ordering variants changed")
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			generations := make(map[[sha256.Size]byte]bool)
			for _, row := range data.Rows {
				files := conformanceFlowSchemaFiles(row.ByAPI[api].Files)
				raw, err := json.Marshal(files)
				if err != nil {
					t.Fatal(err)
				}
				generations[sha256.Sum256(raw)] = true
			}
			if len(generations) != 1 {
				t.Errorf("flow variants require %d bases; want one object/permission base", len(generations))
			}
		})
	}
}
