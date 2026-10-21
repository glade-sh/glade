package apextest

import "testing"

func TestEventListAccountingOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "event_list_accounting.json", "EventListAccounting", 10)
}
