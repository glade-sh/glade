package apextest

import (
	"reflect"
	"testing"
)

func TestCaptureCustomIdentityNormalization(t *testing.T) {
	a := map[string]any{"expectedParent": "a00000000000001AAA", "expectedChild": "a01000000000001AAA", "rows": []any{"a00000000000001AAA", "a01000000000001AAA"}}
	b := map[string]any{"expectedParent": "a21000000000003AAA", "expectedChild": "a22000000000004AAA", "rows": []any{"a21000000000003AAA", "a22000000000004AAA"}}
	if !reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("equivalent custom identities did not match")
	}
	b["rows"] = []any{"a22000000000004AAA", "a21000000000003AAA"}
	if reflect.DeepEqual(normalizeCaptureIdentities(a), normalizeCaptureIdentities(b)) {
		t.Fatal("parent and child identity were swapped")
	}
}
