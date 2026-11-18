package ir

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
	"testing"
)

// The startup runtime cache stores programs with gob and the IR has JSON tags;
// both keep the conditional-operand start a coverage run reads.
func TestExprStartPosSurvivesEncoding(t *testing.T) {
	right := Expr{Kind: ExprVariable, Name: "b", StartPos: 17}
	program := Program{Instructions: []Instruction{{Op: OpExpr, Expr: Expr{Kind: ExprBinary, Operator: "&&", Left: &Expr{Kind: ExprVariable, Name: "a"}, Right: &right}}}}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(program); err != nil {
		t.Fatal(err)
	}
	var fromGob Program
	if err := gob.NewDecoder(&buf).Decode(&fromGob); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	var fromJSON Program
	if err := json.Unmarshal(data, &fromJSON); err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]Program{"gob": fromGob, "json": fromJSON} {
		if pos := got.Instructions[0].Expr.Right.StartPos; pos != 17 {
			t.Fatalf("%s StartPos = %d", name, pos)
		}
	}
}
