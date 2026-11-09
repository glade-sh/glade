package storage

import (
	"encoding/csv"
	"maps"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Every new-object map is compared with its separately captured Apex route.
func TestStandardAPI68NewObjectNativeShapes(t *testing.T) {
	root := "../apextest/testdata/conformance/standard_describe_api68_drift"
	for _, profile := range []string{"P0", "P1", "P2", "P3", "P4", "P5", "P6", "P7", "P8", "P9", "P10", "P11", "PU"} {
		t.Run(profile, func(t *testing.T) {
			dir := profile + "-new-objects-full"
			if profile == "P0" {
				dir = "P0-new-objects"
			}
			f, err := os.Open(filepath.Join(root, dir, "org.tsv"))
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()
			reader := csv.NewReader(f)
			reader.Comma = '\t'
			rows, err := reader.ReadAll()
			if err != nil {
				t.Fatal(err)
			}
			for _, route := range []string{"anonymous", "named"} {
				want := map[string]map[string]bool{}
				for _, row := range rows[1:] {
					if len(row) != 9 {
						t.Fatalf("malformed native row: %v", row)
					}
					if row[0] != "68.0" || row[1] != "default" || row[2] != route || row[5] != "true" {
						continue
					}
					if want[row[3]] == nil {
						want[row[3]] = map[string]bool{}
					}
					want[row[3]][strings.ToLower(row[4])] = true
				}
				names := StandardAPI68DescribeObjectNames(profile, route == "named")
				if len(names) != len(want) {
					t.Fatalf("%s objects=%d, native=%d", route, len(names), len(want))
				}
				for _, name := range names {
					def, ok := StandardAPI68DescribeObjectForRoute(profile, name, route == "named")
					if !ok {
						t.Fatalf("%s %s missing", route, name)
					}
					got := map[string]bool{}
					for field := range def.Fields {
						got[strings.ToLower(field)] = true
					}
					if !maps.Equal(got, want[name]) {
						t.Errorf("%s %s fields differ: native=%v got=%v", route, name, want[name], got)
					}
				}
			}
		})
	}
}
