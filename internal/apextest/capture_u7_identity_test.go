package apextest

import (
	"reflect"
	"testing"
)

func TestU7IdentitySeedNormalization(t *testing.T) {
	input := map[string]any{"before": []any{[]any{"001000000000001AAA", "a"}, []any{"001000000000002AAA", "b"}}, "aa": []any{"001000000000002AAA", "001000000000001AAA"}}
	want := map[string]any{"before": []any{[]any{"<001:1>", "a"}, []any{"<001:2>", "b"}}, "aa": []any{"<001:2>", "<001:1>"}}
	if got := normalizeU7Capture(input); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}
