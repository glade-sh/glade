package vm

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestJSONParserReferenceSequence(t *testing.T) {
	var results []uint64
	for _, input := range []string{`{"a":[1,2],"b":true}`, `{"a":[1,2`, `[01]`, `"x"`} {
		before := nextValueRef.Load()
		parser, err := newJSONParser(input)
		results = append(results, nextValueRef.Load()-before)
		if err != nil {
			continue
		}
		before = nextValueRef.Load()
		for {
			value, updated, _, _, err := jsonParserNextToken(parser)
			if err != nil {
				t.Fatal(err)
			}
			parser = updated
			if _, err := json.Marshal(parser); err != nil {
				t.Fatal(err)
			}
			_ = approxValueSize(parser)
			if value.Kind == ValueNull {
				break
			}
		}
		results = append(results, nextValueRef.Load()-before)
	}
	if got := fmt.Sprint(results); got != "[11 0 7 3 3 0]" {
		t.Fatalf("reference deltas %s", got)
	}
}

func TestJSONParserMissingTokenListIdentity(t *testing.T) {
	parser := Object("JSONParser")
	before := nextValueRef.Load()
	value, updated, _, _, err := jsonParserNextToken(parser)
	if err != nil || value.Kind != ValueNull || updated.Fields["index"].Int != 0 || nextValueRef.Load()-before != 1 {
		t.Fatalf("empty parser result changed: value=%v index=%v ref delta=%d err=%v", value, updated.Fields["index"], nextValueRef.Load()-before, err)
	}
}
