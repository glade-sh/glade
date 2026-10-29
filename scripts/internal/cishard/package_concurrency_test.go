package main

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestCIPackageManifestConcurrencyCounts(t *testing.T) {
	data, err := os.ReadFile("../../ci-package-lanes.json")
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	delete(document, "concurrentTestCounts")
	var current []string
	for _, packages := range document["lanes"].(map[string]any) {
		for _, pkg := range packages.([]any) {
			current = append(current, pkg.(string))
		}
	}
	baseline, _ := json.Marshal(document)
	want, err := validatePackageManifest(baseline, current)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, pkg string
		count     int
		valid     bool
	}{
		{"sema", "./internal/sema", 642, true},
		{"vm", "./internal/vm", 12, true},
		{"lsp", "./internal/lsp", 1, true},
		{"zero", "./internal/sema", 0, false},
		{"negative", "./internal/sema", -1, false},
		{"unowned", "./internal/missing", 1, false},
		{"absolute", "/internal/sema", 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			document["concurrentTestCounts"] = map[string]int{tc.pkg: tc.count}
			data, _ := json.Marshal(document)
			got, err := validatePackageManifest(data, current)
			if tc.valid {
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("concurrency metadata changed lane selection: %v, %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), "concurrency") {
				t.Fatalf("invalid concurrency record needs a specific rejection, got %v", err)
			}
		})
	}
}
