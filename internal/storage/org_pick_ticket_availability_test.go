package storage

import (
	"github.com/glade-sh/glade/internal/project"
	"testing"
)

// Both inputs are required by the retained anonymous and named API 62/67 rows.
func TestOrgPickTicketAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"feature-only", []string{"OrderManagement"}, false},
		{"setting-only", []string{"OrderManagementEnabled"}, false},
		{"both", []string{"OrderManagement", "OrderManagementEnabled"}, true},
		{"unspecified", []string{project.UnspecifiedStandardFieldShape}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			for _, name := range []string{"PickTicket", "PickTicketAssignment", "PickTicketFeed", "PickTicketProduct", "PickTicketProductFeed", "PickTicketShare"} {
				EnsureStandardObject(&org, name)
			}
			ApplyProjectOrgShape(&org, tc.features)
			for _, name := range []string{"PickTicket", "PickTicketAssignment", "PickTicketFeed", "PickTicketProduct", "PickTicketProductFeed", "PickTicketShare"} {
				if got := org.AllowsStandardObject(name); got != tc.want {
					t.Errorf("%s allowed=%t want=%t", name, got, tc.want)
				}
			}
		})
	}
}
