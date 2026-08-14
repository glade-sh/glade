package ir

import (
	"encoding/json"
	"testing"
)

func TestDMLModesRoundTripForAllOperationsAndNestedInstructions(t *testing.T) {
	operations := []Op{"insert", "update", "upsert", "delete", "undelete", "merge"}
	instructions := make([]Instruction, 0, len(operations))
	for index, operation := range operations {
		instructions = append(instructions, Instruction{
			Op:      OpDML,
			Name:    string(operation),
			DMLMode: DMLMode(index % 3),
			Then: []Instruction{{
				Op:      OpDML,
				Name:    string(operation),
				DMLMode: DMLMode((index + 1) % 3),
			}},
		})
	}
	encoded, err := json.Marshal(Program{Instructions: instructions})
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip Program
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if len(roundTrip.Instructions) != len(instructions) {
		t.Fatalf("instruction count = %d, want %d", len(roundTrip.Instructions), len(instructions))
	}
	for index, instruction := range roundTrip.Instructions {
		want := instructions[index]
		if instruction.Name != want.Name || instruction.DMLMode != want.DMLMode {
			t.Fatalf("instruction[%d] = %#v, want %#v", index, instruction, want)
		}
		if len(instruction.Then) != 1 || instruction.Then[0].DMLMode != want.Then[0].DMLMode {
			t.Fatalf("nested instruction[%d] = %#v, want %#v", index, instruction.Then, want.Then)
		}
	}
}
