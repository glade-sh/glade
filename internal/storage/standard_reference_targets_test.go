package storage

import (
	"encoding/csv"
	"os"
	"path/filepath"
	"testing"
)

// The captured standard describe catalog lists these objects as reference
// targets (Assessment.AssessmentStagedDataId and
// DocumentGenerationProcess.DocGenerationBatchProcessId). Native scratch
// captures (testdata/standard_reference_targets_native) show neither target
// has a describe at API 62.0 or 67.0, so the names are admitted as field targets without a
// standalone type or describe.
func TestReferenceBackedStandardObjectNamesCoverCatalogReferenceTargets(t *testing.T) {
	resetKnownStandardObjectCacheForTest()
	defer resetKnownStandardObjectCacheForTest()
	for _, tc := range []struct{ owner, field, target string }{
		{"Assessment", "AssessmentStagedDataId", "AssessmentStagedData"},
		{"DocumentGenerationProcess", "DocGenerationBatchProcessId", "DocGenerationBatchProcess"},
	} {
		describe, found, err := lookupStandardDescribeCatalogV2(tc.owner)
		if err != nil || !found {
			t.Fatalf("catalog missing %s: %v", tc.owner, err)
		}
		listed := false
		for _, field := range describe.Fields {
			if field.Name != tc.field {
				continue
			}
			for _, target := range field.ReferenceTo {
				listed = listed || target == tc.target
			}
		}
		if !listed {
			t.Fatalf("catalog field %s.%s no longer references %s", tc.owner, tc.field, tc.target)
		}
		if !IsReferenceOnlyStandardObject(tc.target) || IsKnownStandardObject(tc.target) {
			t.Fatalf("reference target %s of %s.%s must be reference-only", tc.target, tc.owner, tc.field)
		}
	}
}

func TestReferenceTargetNativeCaptureHasNoStandaloneDescribe(t *testing.T) {
	file, err := os.Open(filepath.Join("testdata", "standard_reference_targets_native", "org.tsv"))
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
	seen := map[string]int{}
	for _, row := range rows[1:] {
		if row[2] == "AssessmentStagedData" || row[2] == "DocGenerationBatchProcess" || row[2] == "RecordAccessEvent__e" {
			if row[3] != "absent" {
				t.Fatalf("native %s/%s/%s describe = %s, want absent", row[0], row[1], row[2], row[3])
			}
			seen[row[0]]++
		}
	}
	if seen["62.0"] != 6 || seen["67.0"] != 6 {
		t.Fatalf("native rows per API = %v, want 6 each (3 objects x 2 org states)", seen)
	}
}
