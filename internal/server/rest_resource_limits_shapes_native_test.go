package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"
)

// The retained native lists contain only the first 40 sorted keys. Compare
// within that observed range, without treating omitted hosted services as local
// capabilities or inventing values for native limits.
func TestRESTResourceLimitsNativeShapes(t *testing.T) {
	file, err := os.Open("testdata/rest_resource_limits_shapes/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 9 || strings.Join(rows[0], ",") != "api,resource,observation" {
		t.Fatal("incomplete native resource/limits matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		name := row[0] + "/" + row[1]
		if seen[name] || !slices.Contains([]string{"60.0", "65.0", "67.0", "68.0"}, row[0]) ||
			!slices.Contains([]string{"resources", "limits"}, row[1]) {
			t.Fatalf("invalid or duplicate native row %s", name)
		}
		seen[name] = true
		t.Run(name, func(t *testing.T) {
			var native restRouteAnswer
			if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
				t.Fatal(err)
			}
			if native.Status != http.StatusOK || len(native.Keys) != 40 || !slices.IsSorted(native.Keys) {
				t.Fatal("native row must contain the captured 200 and first 40 sorted keys")
			}
			path := "/services/data/v" + row[0]
			if row[1] == "limits" {
				path += "/limits"
			}
			org := testOrg()
			handler := New(&org)
			// Repeat through the same handler, with and without a trailing slash.
			for range 2 {
				for _, suffix := range []string{"", "/"} {
					rec := httptest.NewRecorder()
					handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path+suffix, nil))
					if rec.Code != native.Status {
						t.Fatalf("%s: status=%d native=%d body=%s", path+suffix, rec.Code, native.Status, rec.Body.String())
					}
					keys := restRouteKeys(rec.Body.Bytes())
					if len(keys) == 0 {
						t.Fatal("expected a nonempty JSON object")
					}
					for _, key := range keys {
						if key <= native.Keys[len(native.Keys)-1] && !slices.Contains(native.Keys, key) {
							t.Errorf("%s: nonnative key %q; captured native keys %v", path+suffix, key, native.Keys)
						}
					}
				}
			}
		})
	}
}
