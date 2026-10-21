package apextest

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestStandardExtensionNamespacePresentOrgConformance(t *testing.T) {
	var data struct {
		Captures []standardExtensionNativeCapture
	}
	raw, err := os.ReadFile("testdata/conformance/standard_extension_namespace_present.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil || len(data.Captures) != 1 {
		t.Fatal("missing namespace named supplemental capture")
	}
	capture := data.Captures[0]
	if capture.API != "62.0" || capture.Context != "namespace" || capture.Route != "istest" || capture.Namespace != "gladens003" || capture.BindingSHA256 != "capture-revision-094511018274603542474392345255231533155257931117113614034609222374167858913306" || capture.SourceManifestSHA256 != "5d44644104e243559ecbe338dd99504322b1da1ecf284bda10d307d42a930d6d" {
		t.Fatal("invalid observed namespace/API/route or binding")
	}
	var merged standardExtensionNativeData
	raw, err = os.ReadFile("testdata/conformance/standard_extension_lookup.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &merged); err != nil {
		t.Fatal(err)
	}
	wanted := map[string]bool{
		"CTRL_POS": true, "CTRL_CAUGHT": true, "CTRL_UNCAUGHT": true, "CTRL_COMPILE": true,
		"G01PresentNonnullLocal": true, "G01PresentNonnullSchema": true,
		"G02PresentNonnullLocal": true, "G02PresentNonnullSchema": true,
		"G03PresentNonnullLocal": true, "G03PresentNonnullSchema": true,
	}
	if len(capture.Rows) != len(wanted) {
		t.Fatal("incomplete namespace supplemental inventory")
	}
	for _, row := range capture.Rows {
		if !wanted[row.ID] {
			t.Fatalf("unexpected supplemental cell %s", row.ID)
		}
		if row.ID[:1] != "G" {
			continue
		}
		found := false
		for _, original := range merged.Captures {
			if original.API != capture.API || original.Context != capture.Context || original.Route != capture.Route || original.Namespace != capture.Namespace {
				continue
			}
			for _, replacement := range original.Rows {
				if replacement.ID != row.ID {
					continue
				}
				found = true
				var native, landed any
				if json.Unmarshal(row.Native, &native) != nil || json.Unmarshal(replacement.Native, &landed) != nil || !reflect.DeepEqual(native, landed) || replacement.CarryReason != "" || replacement.Expression != row.Expression || replacement.ExpressionSHA256 != row.ExpressionSHA256 || !reflect.DeepEqual(replacement.Metadata, capture.Metadata) || !reflect.DeepEqual(replacement.MetadataSHA256, capture.MetadataSHA256) || !reflect.DeepEqual(replacement.ProjectionCarries, row.ProjectionCarries) {
					t.Errorf("namespace cell %s lacks accepted observation and fresh metadata", row.ID)
				}
			}
		}
		if !found {
			t.Errorf("namespace replacement cell %s is absent", row.ID)
		}
	}
	runStandardExtensionNativeCaptures(t, "standard_extension_namespace_present", data.Captures, 10)
}
