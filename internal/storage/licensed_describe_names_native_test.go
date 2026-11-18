package storage

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// Native rows: licensed_describe_names/org.tsv, route rest-describe. A name
// whose describe is NOT_FOUND in every reached org has no standard definition,
// and the control Account does.
func TestStandardCatalogMatchesNativeLicensedDescribes(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/licensed_describe_names/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	seen := map[string]bool{}
	for first := true; ; first = false {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if first || record[2] != "rest-describe" {
			continue
		}
		var observed struct {
			Result string `json:"result"`
		}
		if err := json.Unmarshal([]byte(record[4]), &observed); err != nil {
			t.Fatal(err)
		}
		name := record[3]
		_, defined := StandardObjectDefinition(name)
		if want := observed.Result == "FOUND"; defined != want {
			t.Fatalf("%s %s %s: native %s, catalog defined=%t", record[0], record[1], name, observed.Result, defined)
		}
		seen[record[0]+"/"+strings.ToLower(name)] = true
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, name := range []string{"assessmentstageddata", "docgenerationbatchprocess", "recordaccessevent__e", "account"} {
			if !seen[api+"/"+name] {
				t.Fatalf("missing native %s row for %s", api, name)
			}
		}
	}
}
