package apextest

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestTransientLocalScopeNativeConformance(t *testing.T) {
	data := loadTransientLocalScopeNativeFixture(t)
	// Both native routes invoke these helper methods. The captured locals stay
	// in their original method and nested block scopes during replay.
	runValueCaptureNativeFixture(t, data, "map", "observed", "TRANSIENT_SCOPE|")
}

func loadTransientLocalScopeNativeFixture(t *testing.T) valueCaptureNativeFixture {
	t.Helper()
	raw, err := os.ReadFile("testdata/conformance/transient_local_scope_native.json")
	if err != nil {
		t.Fatal(err)
	}
	var data valueCaptureNativeFixture
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	apis := []string{"59.0", "62.0", "67.0", "68.0"}
	if !reflect.DeepEqual(data.APIs, apis) || len(data.SourcesByAPI) != len(apis) || len(data.Cells) != 2*len(apis) || len(data.Carries) != 0 {
		t.Fatal("incomplete native local-scope API/context matrix")
	}
	want := map[string]any{
		"outerDeclared":   float64(1),
		"methodDeclared":  float64(2),
		"recordLocal":     true,
		"ordinaryControl": float64(3),
		"finalControl":    float64(4),
	}
	seen := make(map[string]bool)
	for _, cell := range data.Cells {
		key := cell.API + "/" + cell.Context
		if seen[key] || (cell.Context != "anonymous" && cell.Context != "istest") || len(cell.Regex) != 0 || !reflect.DeepEqual(cell.Map, want) {
			t.Fatalf("invalid native local-scope cell: %s: %#v", key, cell)
		}
		if _, present := data.SourcesByAPI[cell.API]; !present {
			t.Fatalf("native answer has no captured source: %s", key)
		}
		seen[key] = true
	}
	for _, api := range apis {
		source, present := data.SourcesByAPI[api]
		if !present || source.HelperName == "" || source.HelperSource == "" || source.TestName == "" || source.TestSource == "" || source.Anonymous == "" || !strings.Contains(source.HelperMetadata, "<apiVersion>"+api+"</apiVersion>") || !strings.Contains(source.TestMetadata, "<apiVersion>"+api+"</apiVersion>") {
			t.Fatalf("missing native local-scope source or API metadata: %s", api)
		}
		for _, context := range []string{"anonymous", "istest"} {
			if !seen[api+"/"+context] {
				t.Fatalf("missing native local-scope answer: %s/%s", api, context)
			}
		}
	}
	return data
}
