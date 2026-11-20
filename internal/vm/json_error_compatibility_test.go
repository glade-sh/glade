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
		{`{"x":x}`, 4, "Unexpected character ('x' (code 120)): expected a valid value (number, String, array, object, 'true', 'false' or 'null') at [line:1, column:7]"},
		{`{"x":x}`, 5, "Unexpected character ('x' (code 120)): expected a valid value (number, String, array, object, 'true', 'false' or 'null') at [line:1, column:7]"},
		{`[x]`, 1, "Unexpected character ('x' (code 120)): expected a valid value (number, String, array, object, 'true', 'false' or 'null') at [line:1, column:3]"},
		{`xx`, 0, "Unexpected character ('x' (code 120)): expected a valid value (number, String, array, object, 'true', 'false' or 'null') at input location [1,2]"},
	} {
		if got := jsonSyntaxErrorAtInput(original, tc.source, tc.offset); got.Error() != tc.want {
			t.Errorf("%q at %d: got %v, want %s", tc.source, tc.offset, got, tc.want)
		}
	}
	for _, tc := range []struct {
		source string
		offset int
	}{{`{"a":1,}`, 6}, {`{"a":":}"}`, 4}, {`{x}`, 1}, {`{"a":1,x}`, 6}, {`nullx`, 0}} {
		if got := jsonSyntaxErrorAtInput(original, tc.source, tc.offset); got != original {
			t.Errorf("neighbor %q: got %v, want original error", tc.source, got)
		}
	}
}

func TestJSONInvalidValueNeighbors(t *testing.T) {
	for _, tc := range []struct{ source, want string }{
		{`{"a"::}`, "Unexpected character (':' (code 58)): expected a valid value (number, String, array, object, 'true', 'false' or 'null') at [line:1, column:7]"},
		{`[1,,]`, "Unexpected character (',' (code 44)): expected a valid value (number, String, array, object, 'true', 'false' or 'null') at [line:1, column:5]"},
	} {
		_, err := decodeJSONUntypedValue(tc.source)
		if err == nil || err.Error() != tc.want {
			t.Errorf("%q: got %v, want baseline %s", tc.source, err, tc.want)
		}
	}
}

func TestJSONDeserializeEscapedInputRetry(t *testing.T) {
	for _, source := range []string{`{\"a\":1}`, `{"a":"\""}`} {
		if _, err := decodeJSONValueForDeserialize(source, false, false); err != nil {
			t.Errorf("%q: %v", source, err)
		}
	}
}
