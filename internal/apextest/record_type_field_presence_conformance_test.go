package apextest

import (
	"reflect"
	"testing"
)

// Native observations: testdata/conformance/record_type_field_presence/org.tsv.
// RecordTypeId exists only on objects with record types; get and put of the
// absent field raise SObjectException. Known gap: with no profile record type
// visibility native reports Alpha and Beta unavailable; Glade reports every
// active record type available (record_type_shape infos rows). Master is the
// default mapping in both.
func TestRecordTypeFieldPresenceNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "record_type_field_presence", "RtPresenceProbeTest", func(t *testing.T, got, want any) {
		t.Helper()
		gm, gok := got.(map[string]any)
		wm, wok := want.(map[string]any)
		if !gok || !wok {
			t.Fatal("invalid record type observation")
		}
		for key, value := range wm {
			if key == "infos" {
				continue
			}
			if !reflect.DeepEqual(gm[key], value) {
				t.Errorf("%s: native=%v; Glade=%v", key, value, gm[key])
			}
		}
		gotInfos, _ := gm["infos"].([]any)
		if len(gotInfos) != 3 || gotInfos[2] != "Master:true:true" {
			t.Errorf("infos: native=%v; Glade=%v", wm["infos"], gm["infos"])
		}
		if reflect.DeepEqual(gm["infos"], wm["infos"]) {
			t.Error("infos: stale known gap; Glade now matches native")
		}
	}, "62.0", "67.0", "68.0")
}
