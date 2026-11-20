package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// This digest records the complete ordered accessor results and parser snapshots
// produced by the baseline for mixed, malformed and boundary inputs.
func TestJSONParserOutcomeIdentity(t *testing.T) {
	inputs := []string{`{"a":[1,-2.5,true,false,null,"x\\ny",{"z":3}],"empty":{},"unicode":"☃"}`, `[0,1,2] trailing`, `""`, `null`, ``, `{"a":`, `[01]`, `[[[]]]`}
	var transcript strings.Builder
	machine := New(nil)
	methods := []string{"getCurrentToken", "hasCurrentToken", "getLastClearedToken", "getText", "getCurrentName", "getIntegerValue", "getLongValue", "getDecimalValue", "getDoubleValue", "getBooleanValue", "getDateValue", "getDatetimeValue", "getTimeValue", "getIdValue", "getBlobValue"}
	for _, input := range inputs {
		parser, err := newJSONParser(input)
		fmt.Fprintf(&transcript, "input=%q error=%v\n", input, err)
		if err != nil {
			continue
		}
		for step := 0; step < 100; step++ {
			for _, method := range methods {
				value, _, changed, handled, err := machine.callJSONParserMember(parser, method, nil)
				data, marshalErr := json.Marshal(value)
				if marshalErr != nil {
					t.Fatal(marshalErr)
				}
				fmt.Fprintf(&transcript, "%s:%s:%t:%t:%v\n", method, data, changed, handled, err)
			}
			data, err := json.Marshal(parser)
			if err != nil {
				t.Fatal(err)
			}
			fmt.Fprintf(&transcript, "state:%s\n", data)
			value, updated, _, _, err := machine.callJSONParserMember(parser, "nextToken", nil)
			if err != nil {
				t.Fatal(err)
			}
			parser = updated
			if value.Kind == ValueNull {
				data, err := json.Marshal(parser)
				if err != nil {
					t.Fatal(err)
				}
				fmt.Fprintf(&transcript, "eof:%s\n", data)
				break
			}
			if step == 2 {
				_, parser, _, _, err = machine.callJSONParserMember(parser, "clearCurrentToken", nil)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(transcript.String())))
	const want = "d4157bf22bb2757f32f8e9e13bd2160f19bdf4418a1739186414fda939101c85"
	if got != want {
		t.Fatalf("outcome digest got %s want %s", got, want)
	}
}

func BenchmarkJSONParserNumericArray(b *testing.B) {
	input := "[" + strings.Repeat("0,", 32767) + "0]"
	b.SetBytes(int64(len(input)))
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		parser, err := newJSONParser(input)
		if err != nil {
			b.Fatal(err)
		}
		_, parser, _, _, err = jsonParserNextToken(parser)
		if err != nil {
			b.Fatal(err)
		}
		parser, err = jsonParserSkipChildren(parser)
		if err != nil {
			b.Fatal(err)
		}
		_, _, _, _, err = jsonParserNextToken(parser)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func TestJSONParserAliasSkipIdentity(t *testing.T) {
	parser, err := newJSONParser(`{"items":[1,2],"tail":true}`)
	if err != nil {
		t.Fatal(err)
	}
	alias := parser
	for _, want := range []string{"START_OBJECT", "FIELD_NAME", "START_ARRAY"} {
		value, next, _, _, err := jsonParserNextToken(parser)
		if err != nil || value.Text != want {
			t.Fatalf("next: %v %v", value, err)
		}
		parser = next
	}
	parser, err = jsonParserSkipChildren(parser)
	if err != nil {
		t.Fatal(err)
	}
	current, ok := jsonParserCurrent(alias)
	if !ok || jsonParserTokenKind(current) != "END_ARRAY" {
		t.Fatalf("alias did not see skip: %v", current)
	}
	a, err := json.Marshal(parser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(alias)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) || approxValueSize(parser) != approxValueSize(alias) {
		t.Fatal("alias snapshot or heap differs")
	}
}
