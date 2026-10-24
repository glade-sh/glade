package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
)

func TestFlowIntegerErrorBoundaryDefersInterviewActionDMLExceptionSelection(t *testing.T) {
	// The same rejected native input throws FlowException for interviews, returns
	// UNKNOWN_EXCEPTION for actions, and exposes CANNOT_EXECUTE_FLOW_TRIGGER for DML.
	// Select that envelope only once the caller route is known.
	machine := New(nil)
	if err := machine.RegisterClass(Class{Name: "IntegerInput", Fields: map[string]Field{"amount": {Name: "amount", Type: "Integer"}}}); err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"7.75", "-7.75", "2147483648", "-2147483649"} {
		t.Run(input, func(t *testing.T) {
			number, err := decimalFromText(input)
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.flowActionDeclaredInput("IntegerInput", "amount", number)
			var boundary *dml.FlowActionInputError
			if !errors.As(err, &boundary) {
				t.Fatalf("expected Flow input failure: %v", err)
			}
			var thrown *apexThrowError
			if errors.As(err, &thrown) {
				t.Fatalf("input conversion selected catchable %s before the caller route was known", thrown.value.Type)
			}
		})
	}
}

func TestFlowIntegerErrorDMLAdapterRetainsUnrelatedFailures(t *testing.T) {
	prior := []dml.Result{{ID: "001000000000001AAA", StatusCode: "REQUIRED_FIELD_MISSING", Error: "Name is required"}, {ID: "001000000000002AAA", Success: true}}
	if _, ok := flowActionInputDMLResults("insert", prior, errors.New("unrelated engine failure")); ok {
		t.Fatal("unrelated error became a Flow input result")
	}
	got, ok := flowActionInputDMLResults("insert", prior, &dml.FlowActionInputError{Cause: errors.New("invalid Integer input")})
	if !ok || got[0].StatusCode != prior[0].StatusCode || got[0].ID != prior[0].ID || got[1].StatusCode != "CANNOT_EXECUTE_FLOW_TRIGGER" || got[1].Success || got[1].ID != "" {
		t.Fatalf("unexpected Flow input results: %+v", got)
	}
	if !prior[1].Success {
		t.Fatal("adapter changed the caller result slice")
	}
}
