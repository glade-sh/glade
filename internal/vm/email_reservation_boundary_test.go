package vm

import (
	"errors"
	"testing"
)

// Transaction reservation exhaustion is pending; no OrgLimit counter inference.
func TestEmailCapacityReservationsAccumulateWithoutCounterMutation(t *testing.T) {
	m := New(nil)
	m.limitCaps.EmailInvokes = 5
	if _, e := m.reserveEmailCapacity("Messaging.reserveSingleEmailCapacity", []Value{Int(4)}, &Result{}); e != nil {
		t.Fatal(e)
	}
	_, e := m.reserveEmailCapacity("Messaging.reserveSingleEmailCapacity", []Value{Int(2)}, &Result{})
	var ex *apexThrowError
	if !errors.As(e, &ex) || exceptionQualifiedTypeName(ex.value.Type) != "System.HandledException" {
		t.Fatalf("exhaustion: %v", e)
	}
	if m.limits.EmailInvokes != 0 {
		t.Fatal("reservation incremented email invocation count")
	}
}
