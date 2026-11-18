package storage

import (
	"encoding/csv"
	"encoding/json"
	"io"
	"os"
	"testing"
)

// Native rows: case_standard_field_fls/org.tsv, named-code describes (route
// istest) in the baseline orgs at source API 62.0 and 67.0. Named code sees all
// 11 FLS-gated Case fields; the feature fields stay absent.
func TestCaseNamedDescribeMatchesNativeRows(t *testing.T) {
	file, err := os.Open("../apextest/testdata/conformance/case_standard_field_fls/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	reader.FieldsPerRecord = -1
	caseDefinition, ok := StandardObjectDefinition("Case")
	if !ok {
		t.Fatal("missing Case")
	}
	checked := 0
	for first := true; ; first = false {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if first || record[2] != "istest" {
			continue
		}
		var observed map[string]any
		if err := json.Unmarshal([]byte(record[4]), &observed); err != nil {
			t.Fatal(err)
		}
		for key, value := range observed {
			present, isBool := value.(bool)
			if !isBool || len(key) < 6 || key[:5] != "Case." {
				continue
			}
			_, exists := ResolveFieldName(caseDefinition, "", key[5:])
			if exists != present {
				t.Fatalf("%s %s %s: native present=%t, catalog present=%t", record[0], record[1], key, present, exists)
			}
			checked++
		}
		for _, key := range []string{"Contact.IsPersonAccount", "Opportunity.SyncedQuoteId"} {
			if observed[key] != false {
				t.Fatalf("%s %s: native control %s=%v", record[0], record[1], key, observed[key])
			}
			object := key[:len(key)-len(key[indexDot(key):])]
			field := key[indexDot(key)+1:]
			if !StandardObjectFieldRequiresFeature(object, field) {
				t.Fatalf("%s should require an org feature", key)
			}
		}
	}
	if checked < 6*10 {
		t.Fatalf("expected the captured named describes, checked %d", checked)
	}
}

func indexDot(value string) int {
	for i := 0; i < len(value); i++ {
		if value[i] == '.' {
			return i
		}
	}
	return -1
}
