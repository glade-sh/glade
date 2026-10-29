package vm

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestJSONPrettyArraysNative(t *testing.T) {
	// sobject_json queryChildren contains a record array; the existing cycle
	// oracle's dag/deep controls contain shared and deeply nested arrays.
	for _, fixture := range []string{"sobject_json", "json_circular_references"} {
		data, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "conformance", fixture, "org.tsv"))
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n")[1:] {
			row := strings.Split(line, "\t")
			api, shape := row[0], row[1]
			var want string
			if fixture == "sobject_json" {
				if len(row) != 5 || row[1] != "anonymous" || row[2] != "queryChildren" || row[3] != "pretty" {
					continue
				}
				shape = row[2]
				if err := json.Unmarshal([]byte(row[4]), &want); err != nil {
					t.Fatal(err)
				}
			} else {
				if len(row) != 4 || row[2] != "pretty" || (shape != "dag" && shape != "deep") {
					continue
				}
				var observation struct{ Value string }
				if err := json.Unmarshal([]byte(row[3]), &observation); err != nil {
					t.Fatal(err)
				}
				want = observation.Value
			}
			count++
			t.Run(fixture+"/"+api+"/"+shape, func(t *testing.T) {
				var input bytes.Buffer
				if err := json.Indent(&input, []byte(want), "", "  "); err != nil {
					t.Fatal(err)
				}
				if got := formatSalesforcePrettyJSON(input.Bytes()); got != want {
					t.Fatalf("native=%q; Glade=%q", want, got)
				}
			})
		}
		wantCount := 2
		if fixture == "json_circular_references" {
			wantCount = 4
		}
		if count != wantCount {
			t.Fatalf("%s: expected %d native rows, got %d", fixture, wantCount, count)
		}
	}
}
