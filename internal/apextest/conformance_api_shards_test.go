package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The existing API subtests read these lists from the native conformance data.
func TestConformanceAPIShardManifest(t *testing.T) {
	data, err := os.ReadFile("testdata/conformance_api_shards.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string][]string
	if err := json.Unmarshal(data, &manifest); err != nil {
		t.Fatal(err)
	}
	fixtures := map[string]string{
		"TestRunAsDefaultSecurityConformance": "runas_default_security/matrix.json",
		"TestSOQLBindParenOrgConformance":     "soql_bind_paren.json",
		"TestSOQLBindIndexOrgConformance":     "soql_bind_index.json",
		"TestSOQLBindLexicalOrgConformance":   "soql_bind_lexical.json",
	}
	want := map[string][]string{}
	for parent, file := range fixtures {
		data, err := os.ReadFile(filepath.Join("testdata/conformance", file))
		if err != nil {
			t.Fatal(err)
		}
		var oracle struct {
			APIVersions []string `json:"apiVersions"`
		}
		if err := json.Unmarshal(data, &oracle); err != nil {
			t.Fatal(err)
		}
		if len(oracle.APIVersions) == 0 {
			t.Fatalf("%s has no API passes", parent)
		}
		want[parent] = oracle.APIVersions
	}
	if !reflect.DeepEqual(manifest, want) {
		t.Fatalf("API shard manifest = %v, want %v", manifest, want)
	}
}
