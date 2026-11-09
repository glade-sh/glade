package apextest

import (
	"reflect"
	"slices"
	"strings"
	"testing"
)

// Feature-gated and generated schema objects. The Communities capture exposes
// Network and NetworkMember; the no-feature baseline does not. Custom-object
// Share tables carry only the eight native fields. RecordAccessEvent__e was
// not available in either scratch org (see ORG-RUN.txt), so both captures keep
// it absent and Glade must agree.
func TestSchemaFeatureObjectsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "schema_feature_objects", "SchemaProbeTest", compareSchemaObservation(
		// Glade carries one Network field set at every API: it has two fields
		// native API 62 lacks and lacks four native API 67 additions held in
		// the SObject stub overlay, a separate generated source (open gap).
		map[string][]string{"62.0/gd_network": {"optionsadvancedauthforsiteenabled", "optionstwiliocallbackapienabled"}},
		map[string][]string{"67.0/gd_network": {"optionspvtpagespwdresetaccessdisabled", "optionssecondaryemailsenderenabled", "pendingsecondaryemailaddress", "verifiedsecondaryemailaddress"}},
	), "62.0", "67.0")
}

func TestSchemaFeatureObjectsBaselineNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "schema_feature_objects_baseline", "SchemaProbeTest", compareSchemaObservation(nil, nil), "62.0", "67.0")
}

func compareSchemaObservation(knownExtras, knownMissing map[string][]string) func(*testing.T, any, any) {
	return func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK || len(actual) != len(expected) {
			t.Fatalf("invalid observation: native=%v; Glade=%v", want, got)
		}
		api, _, _ := strings.Cut(t.Name()[strings.Index(t.Name(), "/")+1:], "/")
		for shape, expectedValue := range expected {
			if reflect.DeepEqual(actual[shape], expectedValue) {
				continue
			}
			if extras, known := knownExtras[api+"/"+shape]; known && withKnownExtraFields(actual[shape], expectedValue, extras, nil) {
				continue
			}
			if missing, known := knownMissing[api+"/"+shape]; known && withKnownExtraFields(actual[shape], expectedValue, nil, missing) {
				continue
			}
			t.Errorf("%s: native=%v; Glade=%v", shape, expectedValue, actual[shape])
		}
	}
}

// withKnownExtraFields accepts a describe projection whose field list differs
// from the native list by exactly the named extra (Glade-only) and missing
// (native-only) fields.
func withKnownExtraFields(got, want any, extras, missing []string) bool {
	gotText, gotOK := got.(string)
	wantText, wantOK := want.(string)
	if !gotOK || !wantOK {
		return false
	}
	gotFields := map[string]bool{}
	for _, name := range strings.Split(gotText[strings.LastIndex(gotText, ";")+1:], ",") {
		gotFields[name] = true
	}
	wantFields := map[string]bool{}
	for _, name := range strings.Split(wantText[strings.LastIndex(wantText, ";")+1:], ",") {
		wantFields[name] = true
	}
	var extra, absent []string
	for name := range gotFields {
		if !wantFields[name] {
			extra = append(extra, name)
		}
	}
	for name := range wantFields {
		if !gotFields[name] {
			absent = append(absent, name)
		}
	}
	slices.Sort(extra)
	slices.Sort(absent)
	slices.Sort(extras)
	slices.Sort(missing)
	return slices.Equal(extra, extras) && slices.Equal(absent, missing)
}
