package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestJSONParserViewIdentity(t *testing.T) {
	var out strings.Builder
	for _, input := range []string{`{"items":[1,-2.5,true,false,null,"x\\ny",{"z":3}],"tail":{}}`, `null`, `[]`, `[[1],2]`} {
		parser, err := newJSONParser(input)
		if err != nil {
			t.Fatal(err)
		}
		alias := parser
		machine := New(nil)
		for step := 0; step < 20; step++ {
			for _, v := range []Value{parser, alias, cloneValue(parser), List(parser, cloneValue(parser))} {
				snapshot, err := json.Marshal(v)
				if err != nil {
					t.Fatal(err)
				}
				budget, err := json.Marshal(v.JSONBudgetSnapshot())
				if err != nil {
					t.Fatal(err)
				}
				output, err := json.Marshal(machine.jsonFromValueForSerialize(v, false))
				if err != nil {
					t.Fatal(err)
				}
				text, truncated := v.BoundedString(200)
				fmt.Fprintf(&out, "%s|%s|%s|%s|%t|%d\n", snapshot, budget, output, text, truncated, approxValueSize(v))
			}
			value, next, _, _, err := jsonParserNextToken(parser)
			if err != nil {
				t.Fatal(err)
			}
			parser = next
			if value.Kind == ValueNull {
				break
			}
			if step == 1 {
				_, parser, _, _, err = machine.callJSONParserMember(parser, "clearCurrentToken", nil)
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	got := fmt.Sprintf("%x", sha256.Sum256([]byte(out.String())))
	const want = "9790130d297366a736ef276dc94929dd198fad0c2f49b9a7d377de10cdc73605"
	if got != want {
		t.Fatalf("view digest got %s want %s", got, want)
	}
}
