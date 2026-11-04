package storage

import (
	"compress/gzip"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// Native P0 granted describes are the rich source for the supplement. A
// filtered partial picklist must not preempt a fuller native overlay.
func TestStandardFieldSupplementPicklistsAreCompleteOrOmitted(t *testing.T) {
	readGzipJSON := func(path string, into any) {
		t.Helper()
		f, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		z, err := gzip.NewReader(f)
		if err != nil {
			t.Fatal(err)
		}
		defer z.Close()
		if err := json.NewDecoder(z).Decode(into); err != nil {
			t.Fatal(err)
		}
	}
	var native struct {
		Describes map[string]standardDescribeObject `json:"describes"`
	}
	readGzipJSON("../apextest/testdata/conformance/standard_describe_profiles/P0/api62/granted/describes.json.gz", &native)
	var values []string
	readGzipJSON("testdata/standard_picklist_values.json.gz", &values)
	allowed := map[string]bool{}
	for _, value := range values {
		allowed[value] = true
	}
	cases := []struct{ object, field string }{
		{"Account", "CleanStatus"}, {"Account", "Ownership"}, {"Account", "Rating"},
		{"Lead", "CleanStatus"}, {"Lead", "GenderIdentity"}, {"Lead", "Pronouns"},
	}
	for _, tc := range cases {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			var original []standardDescribePicklistValue
			for _, f := range native.Describes[tc.object].Fields {
				if f.Name == tc.field {
					original = f.PicklistValues
				}
			}
			if len(original) == 0 {
				t.Fatal("missing native picklist")
			}
			full := true
			for _, v := range original {
				if !allowed[v.Value] {
					full = false
				}
			}
			describe, ok, err := lookupStandardDescribeCatalogV2(tc.object)
			if err != nil || !ok {
				t.Fatalf("lookup: %v", err)
			}
			var got []standardDescribePicklistValue
			found := false
			for _, f := range describe.Fields {
				if f.Name == tc.field {
					got, found = f.PicklistValues, true
				}
			}
			if !found {
				t.Fatal("missing catalog field")
			}
			if !full {
				if len(got) != 0 {
					t.Errorf("partial catalog list has %d values; native list has %d", len(got), len(original))
				}
			} else if !reflect.DeepEqual(got, original) {
				t.Errorf("complete catalog list differs from native: got %v want %v", got, original)
			}
		})
	}
}
