package apextest

import (
	"reflect"
	"testing"
)

// runas_unsaved_user: System.runAs with an unsaved User inserts the user and
// runs as it, with the composed Name visible to SOQL and UserInfo.
func TestRunAsUnsavedUserNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutes(t, "runas_unsaved_user", "RunAsUnsavedProbeTest", []string{"isTest"}, nil, "62.0", "67.0")
}

// apex_class_modified_dates: ApexClass rows carry non-null audit dates that are
// not after now.
func TestApexClassModifiedDatesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "apex_class_modified_dates", "ClassDatesProbeTest", "62.0", "67.0")
}

// entity_definition_sosl: SOSL over EntityDefinition is a metadata search on
// the label. Glob terms that fill the LIMIT or depend on the org's feature set
// (the captured scratch org has fewer entities than Glade's catalog) are held
// to a non-empty result within the limit; every other term is exact.
func TestEntityDefinitionSoslNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "entity_definition_sosl", "EntitySoslProbeTest", func(t *testing.T, got, want any) {
		gotMap, _ := got.(map[string]any)
		wantMap, _ := want.(map[string]any)
		if gotMap == nil || wantMap == nil || len(gotMap) != len(wantMap) {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
		for key, wantShape := range wantMap {
			gotShape, _ := gotMap[key].(map[string]any)
			wantFields, _ := wantShape.(map[string]any)
			if gotShape == nil || wantFields == nil {
				t.Fatalf("%s: native=%v; Glade=%v", key, wantShape, gotMap[key])
			}
			wantRows, _ := wantFields["rows"].(float64)
			gotRows, _ := gotShape["rows"].(float64)
			if wantRows >= 10 {
				if gotRows < 1 || gotRows > 15 {
					t.Fatalf("%s: native rows=%v; Glade rows=%v", key, wantRows, gotRows)
				}
				continue
			}
			if !reflect.DeepEqual(gotShape, wantFields) {
				t.Fatalf("%s: native=%v; Glade=%v", key, wantFields, gotShape)
			}
		}
	}, "62.0", "67.0")
}

// event_subscriber_object_access: an Apex platform event trigger runs as the
// Automated Process user; describe flags and WITH USER_MODE reads succeed there
// at API 62.0 as well as 67.0. The API 62.0 USER_MODE insert refusal on a custom
// field is covered by automated_process_user_mode (compared here in full).
func TestEventSubscriberObjectAccessNativeConformance(t *testing.T) {
	runInnerTypeNativeRoutes(t, "event_subscriber_object_access", "EvtSubProbeTest", []string{"isTest"}, func(t *testing.T, got, want any) {
		gotMap, _ := got.(map[string]any)
		wantMap, _ := want.(map[string]any)
		gotSub, _ := gotMap["subscriber"].(map[string]any)
		wantSub, _ := wantMap["subscriber"].(map[string]any)
		if gotSub == nil || wantSub == nil {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("native=%v; Glade=%v", want, got)
		}
	}, "62.0", "67.0")
}
