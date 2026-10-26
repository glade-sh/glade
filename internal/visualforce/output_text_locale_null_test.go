package visualforce

import (
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

func TestOutputTextDateStyleLocaleNullUnitRoute(t *testing.T) {
	// Null parameters render empty before any style or locale expansion.
	for _, kind := range []string{"date", "time"} {
		got, err := formatOutputTextArgument(vm.Null, []string{kind, "short"}, nil)
		if err != nil || got != "" {
			t.Fatalf("kind=%s got=%q error=%v, want empty null parameter", kind, got, err)
		}
	}
}
