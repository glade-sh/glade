package vm

import "testing"

func TestEmailCapacityReservationsKeepCounters(t *testing.T) {
	machine := New(nil)
	// Local state isolation only; native capacity sizing and counter behavior
	// remain in the pending versioned capture rows.
	machine.singleEmailReserved = 3
	machine.massEmailReserved = 4
	_, err := machine.withAsyncLimitWindow(func() (Value, error) {
		if machine.singleEmailReserved != 0 || machine.massEmailReserved != 0 {
			t.Fatal("async reservations inherited")
		}
		machine.singleEmailReserved = 1
		machine.massEmailReserved = 2
		return Null, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if machine.singleEmailReserved != 3 || machine.massEmailReserved != 4 {
		t.Fatal("async window changed parent reservations")
	}
}
