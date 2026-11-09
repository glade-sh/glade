package apextest

import (
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// event_trigger_exceptions_c4: platform event and change event triggers that
// throw a NullPointerException, a LimitException (SOQL), a DmlException or a
// ListException. Native at API 62.0 and 67.0: neither deliver() nor stopTest
// sees the exception, the trigger's earlier DML stays, the trigger starts
// with its own limits, and a later deliver() still runs. The LimitException
// rows need the strict governor mode (Glade's default mode is permissive).
func TestEventTriggerExceptionsC4NativeConformance(t *testing.T) {
	runObservationClassFamilyWithOptions(t, "event_trigger_exceptions_c4", "C4EvtTest", 16, Options{LimitMode: vm.LimitModeStrict})
}

// portal_sharing_timing_c4 (Dot kcdc3c9cccd60): a Customer Community, Customer
// Community Login or Customer Community Plus user created for a Contact on a
// parent Account with two child Accounts, Account external sharing Private.
// Native at API 62.0 and 67.0: the portal user reads its own Account and
// Contact at once, after Test.startTest/stopTest and when created inside
// them; the child Accounts are never visible. Only the Plus user gets an
// AccountShare (PortalImplicit, Read, on the user) and a portal role.
func TestPortalSharingTimingC4NativeConformance(t *testing.T) {
	runObservationClassFamily(t, "portal_sharing_timing_c4", "C4PortalTest", 9)
}
