package server

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestRESTDiscoveryNativePayloads(t *testing.T) {
	file, err := os.Open("testdata/rest_discovery_shapes/org.tsv")
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
	if len(rows) != 7 || strings.Join(rows[0], ",") != "api,resource,observation" {
		t.Fatal("incomplete native discovery payload matrix")
	}
	seen := map[string]bool{}
	for _, row := range rows[1:] {
		t.Run(row[0]+"/"+row[1], func(t *testing.T) {
			key := row[0] + "/" + row[1]
			if seen[key] || (row[0] != "62.0" && row[0] != "67.0") {
				t.Fatal("invalid native discovery row")
			}
			seen[key] = true
			var native map[string]any
			if err := json.Unmarshal([]byte(row[2]), &native); err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			handler := New(&org)
			rec := httptest.NewRecorder()
			version := "v" + row[0]
			req := httptest.NewRequest(http.MethodGet, "/services/data/"+version+"/"+row[1], nil)
			// Exercise payload construction separately from the endpoint version window.
			switch row[1] {
			case "sobjects":
				handler.handleSObjects(rec, req, version)
			case "sobjects/Account":
				handler.handleObject(rec, req, version, []string{"Account"})
			case "composite":
				handler.handleComposite(rec, req, version, nil)
			default:
				t.Fatal("unknown native discovery resource")
			}
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			var got map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
				t.Fatal(err)
			}
			switch row[1] {
			case "sobjects":
				for _, key := range []string{"encoding", "maxBatchSize"} {
					if got[key] != native[key] {
						t.Errorf("%s=%v; native=%v", key, got[key], native[key])
					}
				}
			case "sobjects/Account":
				if native["objectDescribeType"] != "dict" || native["recentItemsType"] != "list" {
					t.Fatal("unexpected native envelope types")
				}
				describe, ok := got["objectDescribe"].(map[string]any)
				if !ok {
					t.Errorf("objectDescribe=%T; native object", got["objectDescribe"])
				}
				if _, ok := got["recentItems"].([]any); !ok {
					t.Errorf("recentItems=%T; native array", got["recentItems"])
				}
				for key := range describe {
					if !slices.Contains(native["objectDescribeKeys"].([]any), any(key)) {
						t.Errorf("objectDescribe has nonnative key %s", key)
					}
				}
			case "composite":
				if !reflect.DeepEqual(got, native) {
					t.Errorf("payload=%v; native=%v", got, native)
				}
			}
		})
	}
}

func TestRESTDiscoveryNativeShapes(t *testing.T) {
	raw, err := os.ReadFile("testdata/rest_api_version_routes.json")
	if err != nil {
		t.Fatal(err)
	}
	var table restRouteTable
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	paths := map[string]bool{
		"/services/data/v%s/sobjects":         false,
		"/services/data/v%s/sobjects/Account": false,
		"/services/data/v%s/composite":        false,
	}
	for _, row := range table.Cases {
		if _, wanted := paths[row.Path]; !wanted || len(row.Native["67.0"].Keys) == 0 {
			continue
		}
		if paths[row.Path] {
			t.Fatalf("duplicate native discovery route %s", row.Path)
		}
		paths[row.Path] = true
		for _, api := range []string{"60.0", "65.0", "67.0", "68.0"} {
			t.Run(api+"/"+row.Path, func(t *testing.T) {
				native, captured := row.Native[api]
				if !captured || native.Status != http.StatusOK || len(native.Keys) == 0 {
					t.Fatal("native discovery shape missing")
				}
				org := testOrg()
				rec := httptest.NewRecorder()
				New(&org).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf(row.Path, api), nil))
				if rec.Code != native.Status {
					t.Fatalf("status=%d native=%d body=%s", rec.Code, native.Status, rec.Body.String())
				}
				if got := restRouteKeys(rec.Body.Bytes()); !slices.Equal(got, native.Keys) {
					t.Errorf("local keys %v; native %v", got, native.Keys)
				}
			})
		}
	}
	for path, seen := range paths {
		if !seen {
			t.Errorf("native discovery path missing: %s", path)
		}
	}
}
