package vm

import (
	"testing"
)

func TestSOQLBindExpansionPreservesDateTimeClockSeparators(t *testing.T) {
	query := "CreatedDate >= 2026-05-01T12:00:00Z AND CreatedDate <= 2026-05-02T12:00:00Z"
	machine := New(nil)
	got, err := machine.expandSOQLBindsWithLiteral(query, nil, func(string) (Value, error) {
		return Null, nil
	}, func(value Value) string { return soqlLiteral(value) })
	if err != nil {
		t.Fatal(err)
	}
	if got != query {
		t.Fatalf("expanded query = %q, want unchanged timestamp window %q", got, query)
	}
}
