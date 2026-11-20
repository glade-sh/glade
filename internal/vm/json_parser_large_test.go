package vm

import (
	"strings"
	"testing"
)

func TestJSONParserLargeArrayIdentity(t *testing.T) {
	parser, err := newJSONParser("[" + strings.Repeat("0,", 32767) + "0]")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	for i := 0; i < 32770; i++ {
		token, updated, _, _, err := jsonParserNextToken(parser)
		if err != nil {
			t.Fatal(err)
		}
		parser = updated
		kind, text := "VALUE_NUMBER_INT", "0"
		if i == 0 {
			kind, text = "START_ARRAY", "["
		}
		if i == 32769 {
			kind, text = "END_ARRAY", "]"
		}
		if token.Text != kind {
			t.Fatalf("token %d: %q", i, token.Text)
		}
		value, _, _, _, err := machine.callJSONParserMember(parser, "getText", nil)
		if err != nil || value.Text != text {
			t.Fatalf("text %d: %v %v", i, value, err)
		}
	}
	token, _, _, _, err := jsonParserNextToken(parser)
	if err != nil || token.Kind != ValueNull {
		t.Fatalf("EOF: %v %v", token, err)
	}
}
