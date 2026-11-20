package vm

import (
	"errors"
	"testing"
)

func TestJSONMissingValueDiagnostics(t *testing.T) {
	object := `{"a":}`
	wantObject := "Unexpected character ('}' (code 125)): expected a value at [line:1, column:7]"
	if _, err := decodeJSONUntypedValue(object); err == nil || err.Error() != wantObject {
		t.Errorf("untyped: got %v, want %s", err, wantObject)
	}
	if _, err := decodeJSONValueForDeserialize(object, false, false); err == nil || err.Error() != wantObject {
		t.Errorf("typed: got %v, want %s", err, wantObject)
	}
	for _, tc := range []struct{ source, want string }{
		{object, "Unexpected character ('}' (code 125)): expected a value at input location [1,7]"},
		{`[1,]`, "Unexpected character (']' (code 93)): expected a value at input location [1,5]"},
	} {
		_, err := jsonParserTokenize(tc.source)
		if err == nil || jsonParserInputErrorMessage(err, tc.source) != tc.want {
			t.Errorf("parser %s: got %v, want %s", tc.source, err, tc.want)
		}
	}
	if _, err := decodeJSONUntypedValue(`{"a":null}`); err != nil {
		t.Errorf("valid neighboring value: %v", err)
	}
}

func TestJSONMissingValueInputScan(t *testing.T) {
	original := errors.New("decoder-specific error")
	for _, tc := range []struct {
		source string
		offset int
		want   string
	}{
		{`{"a":}`, 4, "Unexpected character ('}' (code 125)): expected a value at [line:1, column:7]"},
		{`{"a":}`, 5, "Unexpected character ('}' (code 125)): expected a value at [line:1, column:7]"},
		{`[1,]`, 2, "Unexpected character (']' (code 93)): expected a value at [line:1, column:5]"},
		{`[1,]`, 3, "Unexpected character (']' (code 93)): expected a value at [line:1, column:5]"},
	} {
		if got := jsonSyntaxErrorAtInput(original, tc.source, tc.offset); got.Error() != tc.want {
			t.Errorf("%q at %d: got %v, want %s", tc.source, tc.offset, got, tc.want)
		}
	}
	for _, tc := range []struct {
		source string
		offset int
	}{{`{"a":1,}`, 6}, {`{"a":":}"}`, 4}} {
		if got := jsonSyntaxErrorAtInput(original, tc.source, tc.offset); got != original {
			t.Errorf("neighbor %q: got %v, want original error", tc.source, got)
		}
	}
}
