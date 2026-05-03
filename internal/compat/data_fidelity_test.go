package compat

import "testing"

func TestRunDataFidelityFixture(t *testing.T) {
	fixture, err := LoadFile("../../docs/fixtures/data-fidelity-soql-dml.json")
	if err != nil {
		t.Fatal(err)
	}
	result, err := Run(fixture)
	if err != nil {
		t.Fatalf("run error: %v, result: %+v", err, result)
	}
	if !result.OK {
		t.Fatalf("result not ok: %+v", result)
	}
}
