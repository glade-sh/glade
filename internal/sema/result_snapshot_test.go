package sema

import (
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestResultSnapshotCoversCompleteResultShape(t *testing.T) {
	fields := reflect.VisibleFields(reflect.TypeOf(Result{}))
	gotNames := make([]string, 0, len(fields))
	for _, field := range fields {
		gotNames = append(gotNames, field.Name)
	}
	wantNames := []string{"Project", "Summary", "Diagnostics", "Types"}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("Result fields changed; update ResultSnapshot to preserve every field\nwant: %v\n got: %v", wantNames, gotNames)
	}
}

func TestResultSnapshotDeepCopyClassifiesReferenceFields(t *testing.T) {
	assertReferenceFields := func(name string, value any, allowed map[string]reflect.Kind) {
		t.Helper()
		for _, field := range reflect.VisibleFields(reflect.TypeOf(value)) {
			switch field.Type.Kind() {
			case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
				if allowed[field.Name] != field.Type.Kind() {
					t.Errorf("%s.%s has reference kind %s; update cloneResult before accepting it", name, field.Name, field.Type.Kind())
				}
			}
		}
	}
	assertReferenceFields("Result", Result{}, map[string]reflect.Kind{
		"Diagnostics": reflect.Slice,
		"Types":       reflect.Map,
	})
	assertReferenceFields("Diagnostic", diagnostic.Diagnostic{}, map[string]reflect.Kind{
		"Range": reflect.Pointer,
	})
	assertReferenceFields("ProjectInfo", typesys.ProjectInfo{}, nil)
	assertReferenceFields("Summary", Summary{}, nil)
	assertReferenceFields("TypeReference", TypeReference{}, nil)
}

func TestEstimateResultRetainedBytesCountsContainersAndRanges(t *testing.T) {
	small := EstimateResultRetainedBytes(Result{})
	large := EstimateResultRetainedBytes(Result{
		Diagnostics: []diagnostic.Diagnostic{{Range: &diagnostic.Range{}}},
		Types:       map[string]TypeReference{"Account": {Name: "Account", Kind: TypeSchema}},
	})
	if small <= 0 || large <= small {
		t.Fatalf("retained estimates small=%d large=%d", small, large)
	}
}
