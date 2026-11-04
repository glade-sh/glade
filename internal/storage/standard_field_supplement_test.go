package storage

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"
	"testing"
)

// TestStandardFieldSupplement checks the complete granted REST field list from
// a single native Developer org. A field absent from REST is not an absence claim.
func TestStandardFieldSupplement(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_field_supplement/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := 0
	for _, row := range rows[1:] {
		if row[1] != "granted" {
			continue
		}
		seen++
		def, ok := StandardObjectDefinition(row[2])
		if !ok {
			t.Errorf("%s: missing object", row[2])
			continue
		}
		if _, ok := def.Fields[row[3]]; !ok {
			t.Errorf("API %s %s.%s: missing native field", row[0], row[2], row[3])
		}
	}
	if seen != 346 {
		t.Fatalf("incomplete native field list: %d", seen)
	}
	for _, row := range rows[1:] {
		if strings.HasPrefix(row[1], "anonymous-") && ((row[1] == "anonymous-boundary") != (row[12] == "true")) {
			t.Fatalf("invalid native control: %v", row)
		}
	}
}

// The packed catalog itself must contain the complete native readable list,
// including fields previously supplied only by separate compatibility overlays.
func TestStandardFieldSupplementCatalog(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_field_supplement/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range rows[1:] {
		if row[1] != "granted" {
			continue
		}
		d, ok, err := lookupStandardDescribeCatalogV2(row[2])
		if err != nil || !ok {
			t.Fatalf("%s: catalog lookup: %v", row[2], err)
		}
		found := false
		for _, field := range d.Fields {
			if field.Name == row[3] {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("API %s %s.%s: missing from catalog", row[0], row[2], row[3])
		}
	}
}

func TestStandardFieldSupplementNativeFacets(t *testing.T) {
	f, err := os.Open("../apextest/testdata/conformance/standard_field_supplement/supplement.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 63 {
		t.Fatalf("incomplete supplement: %d", len(rows)-1)
	}
	for _, row := range rows[1:] {
		d, ok, err := lookupStandardDescribeCatalogV2(row[2])
		if err != nil || !ok {
			t.Fatalf("%s: %v", row[2], err)
		}
		var found *standardDescribeField
		for i := range d.Fields {
			if d.Fields[i].Name == row[3] {
				found = &d.Fields[i]
				break
			}
		}
		if found == nil {
			t.Errorf("%s.%s missing", row[2], row[3])
			continue
		}
		for _, v := range []struct {
			name string
			got  int
			want string
		}{{"length", found.Length, row[5]}, {"precision", found.Precision, row[6]}, {"scale", found.Scale, row[7]}} {
			expected, err := strconv.Atoi(v.want)
			if err != nil || v.got != expected {
				t.Errorf("%s.%s %s=%d want %s", row[2], row[3], v.name, v.got, v.want)
			}
		}
		if !strings.EqualFold(found.Type, row[4]) {
			t.Errorf("%s.%s type=%s want %s", row[2], row[3], found.Type, row[4])
		}
		for _, v := range []struct {
			name string
			got  *bool
			want string
		}{{"createable", found.Createable, row[8]}, {"updateable", found.Updateable, row[9]}, {"nillable", found.Nillable, row[10]}, {"permissionable", found.Permissionable, row[11]}} {
			expected, err := strconv.ParseBool(v.want)
			if err != nil || v.got == nil || *v.got != expected {
				t.Errorf("%s.%s %s=%v want %s", row[2], row[3], v.name, v.got, v.want)
			}
		}
	}
}
