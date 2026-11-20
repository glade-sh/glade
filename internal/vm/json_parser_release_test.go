package vm

import (
	"strings"
	"testing"
	"unsafe"
)

func TestJSONParserReleasedBufferAfterObservation(t *testing.T) {
	parser, err := newJSONParser("[" + strings.Repeat("0,", 32767) + "0]")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	_, parser, _, _, err = jsonParserNextToken(parser)
	if err != nil {
		t.Fatal(err)
	}
	_, parser, _, _, err = jsonParserNextToken(parser)
	if err != nil {
		t.Fatal(err)
	}
	text, _, _, _, err := machine.callJSONParserMember(parser, "getText", nil)
	if err != nil {
		t.Fatal(err)
	}
	_, parser, _, _, err = machine.callJSONParserMember(parser, "clearCurrentToken", nil)
	if err != nil {
		t.Fatal(err)
	}
	packed := parser.Fields["tokens"].Text
	if len(packed) == 0 {
		t.Fatal("expected packed token buffer")
	}
	begin := uintptr(unsafe.Pointer(unsafe.StringData(packed)))
	end := begin + uintptr(len(packed))
	for _, value := range []Value{text, parser.Fields["lastClearedToken"].Fields["text"], parser.Fields["lastClearedToken"].Fields["kind"]} {
		ptr := uintptr(unsafe.Pointer(unsafe.StringData(value.Text)))
		if ptr >= begin && ptr < end {
			t.Fatal("observed token retains packed buffer")
		}
	}
	parser.Fields["index"] = Int(0)
	parser.Fields["cleared"] = Bool(false)
	parser, err = jsonParserSkipChildren(parser)
	if err != nil {
		t.Fatal(err)
	}
	_, parser, _, _, err = jsonParserNextToken(parser)
	if err != nil {
		t.Fatal(err)
	}
	if parser.Fields["tokens"].Text != "" {
		t.Fatal("token buffer retained after EOF")
	}
}

func TestJSONParserRejectedSerializationDoesNotMaterializeTokens(t *testing.T) {
	parser, err := newJSONParser("[" + strings.Repeat("0,", 32767) + "0]")
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	allocations := testing.AllocsPerRun(3, func() {
		defer func() {
			if recover() == nil {
				t.Fatal("serialization should reject the parser")
			}
		}()
		machine.jsonFromValueForJSONSerialize(parser, false)
	})
	if allocations > 1000 {
		t.Fatalf("rejected serialization allocated %.0f objects", allocations)
	}
}
