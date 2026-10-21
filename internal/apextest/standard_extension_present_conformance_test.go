package apextest

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

// The fresh plain API-62 capture fills only seven previously unanswered named
// cells. Its controls and anonymous twins remain independent observations.
func TestStandardExtensionPresentOrgConformance(t *testing.T) {
	var data struct {
		BindingSHA256, SourceManifestSHA256 string
		Captures                            []standardExtensionNativeCapture
	}
	raw, err := os.ReadFile("testdata/conformance/standard_extension_present.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if data.BindingSHA256 != "capture-revision-034779939749554203387018803763760287586297947630858918213706904080370234777359" || data.SourceManifestSHA256 != "a578d6050e474186bcf857ff9471da9f34b83b1d8af1b9c4b9e6d1181bd0ec5f" || len(data.Captures) != 2 {
		t.Fatal("missing accepted supplemental capture binding")
	}
	wanted := map[string]bool{
		"CTRL_POS": true, "CTRL_CAUGHT": true, "CTRL_UNCAUGHT": true, "CTRL_COMPILE": true,
		"C08":                    true,
		"G01PresentNonnullLocal": true, "G01PresentNonnullSchema": true,
		"G02PresentNonnullLocal": true, "G02PresentNonnullSchema": true,
		"G03PresentNonnullLocal": true, "G03PresentNonnullSchema": true,
	}
	for _, capture := range data.Captures {
		if capture.Context != "plain" || capture.Namespace != "" || capture.API != "62.0" || len(capture.Rows) != len(wanted) {
			t.Fatal("supplement must retain the captured plain API-62 context")
		}
		for _, row := range capture.Rows {
			if !wanted[row.ID] {
				t.Fatalf("unexpected supplemental cell %s", row.ID)
			}
			if capture.Route != "istest" || row.ID[:1] != "G" && row.ID != "C08" {
				continue
			}
			family := "lookup"
			if row.ID == "C08" {
				family = "formula"
			}
			var merged standardExtensionNativeData
			raw, err := os.ReadFile("testdata/conformance/standard_extension_" + family + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(raw, &merged); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, original := range merged.Captures {
				if original.API != capture.API || original.Context != "plain" || original.Route != "istest" {
					continue
				}
				for _, replacement := range original.Rows {
					if replacement.ID != row.ID {
						continue
					}
					found = true
					var native, landed any
					if json.Unmarshal(row.Native, &native) != nil || json.Unmarshal(replacement.Native, &landed) != nil || !reflect.DeepEqual(native, landed) || replacement.CarryReason != "" || replacement.Expression != row.Expression || replacement.ExpressionSHA256 != row.ExpressionSHA256 || !reflect.DeepEqual(replacement.Metadata, capture.Metadata) || !reflect.DeepEqual(replacement.MetadataSHA256, capture.MetadataSHA256) || !reflect.DeepEqual(replacement.ProjectionCarries, row.ProjectionCarries) {
						t.Errorf("named cell %s still lacks its accepted observation and fresh metadata", row.ID)
					}
				}
			}
			if !found {
				t.Errorf("replacement cell %s is absent", row.ID)
			}
		}
	}
	runStandardExtensionNativeCaptures(t, "standard_extension_present", data.Captures, 11)
}
