package apextest

import (
	"reflect"
	"testing"
)

func TestQ3IdentityAnchorNormalization(t *testing.T) {
	input := map[string]any{
		"expectedParentId": "aLc100000000001CAA",
		"expectedChildId":  "aLa100000000001CAA",
		"childProjection":  []any{map[string]any{"parentId": "aLc100000000001CAA", "childIds": []any{"aLa100000000001CAA"}}},
	}
	want := map[string]any{
		"expectedParentId": "<custom1:1>",
		"expectedChildId":  "<custom2:1>",
		"childProjection":  []any{map[string]any{"parentId": "<custom1:1>", "childIds": []any{"<custom2:1>"}}},
	}
	if got := normalizeQ3HiddenCapture(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
