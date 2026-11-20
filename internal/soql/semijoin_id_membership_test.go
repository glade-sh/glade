package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSemiJoinIDMembershipMatchesScan(t *testing.T) {
	key := storage.ID("00100000000AbCd")
	other := storage.ID("00100000000aBCd")
	inputs := []storage.Value{storage.IDValue(key), storage.IDValue(key + "AAA"),
		storage.IDValue(key + "BBB"), storage.IDValue(other), storage.IDValue(""),
		storage.IDValue("short"), storage.StringValue(string(key)), storage.IntegerValue(1),
		storage.NullValue()}
	for _, right := range inputs {
		// Separately exercise full-width ID equality and each mixed-kind case.
		values := make([]storage.Value, 20)
		for i := range values {
			values[i] = right
		}
		m := buildSemiJoinIDMembership(values)
		if right.Kind != storage.ValueID {
			if m != nil {
				t.Fatal("non-ID populated a typed index")
			}
			continue
		}
		if m == nil {
			t.Fatal("typed IDs did not create a membership index")
		}
		for _, left := range inputs {
			if left.Kind == storage.ValueID && m.contains(left.ID) != equalValues(left, right) {
				t.Fatalf("ID membership disagrees with the existing comparison for %q and %q", left.ID, right.ID)
			}
		}
	}
	if buildSemiJoinIDMembership(nil) != nil {
		t.Fatal("empty list should retain scan path")
	}
	values := make([]storage.Value, 25)
	for i := range values {
		values[i] = storage.IDValue(key)
	}
	values[len(values)-1] = storage.StringValue(string(key))
	if buildSemiJoinIDMembership(values) != nil {
		t.Fatal("mixed list should retain scan path")
	}
}
