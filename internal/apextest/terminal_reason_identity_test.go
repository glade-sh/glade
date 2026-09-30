package apextest

import (
	"context"
	"errors"
	"fmt"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/vm"
	"testing"
)

func TestTerminalReasonUsesErrorIdentity(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want testreport.Reason
	}{
		{"wrapped deadline", fmt.Errorf("operation: %w", context.DeadlineExceeded), testreport.ReasonTimeout},
		{"wrapped cancellation", fmt.Errorf("operation: %w", context.Canceled), testreport.ReasonCancelled},
		{"unsupported", &vm.RuntimeError{Type: "UnsupportedFeature", Message: "not available"}, testreport.ReasonUnsupportedFeature},
		{"assertion", &vm.RuntimeError{Type: "System.AssertException", Message: "wrong value"}, testreport.ReasonAssertion},
		{"misleading prose", errors.New("unsupported context deadline exceeded"), testreport.ReasonRuntimeError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := terminalReason(tc.err); got != tc.want {
				t.Fatalf("reason %s want %s", got, tc.want)
			}
		})
	}
}
