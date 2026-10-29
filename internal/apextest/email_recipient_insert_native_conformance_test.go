package apextest

import "testing"

func TestEmailRecipientInsertNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "email_recipient_insert_boundaries", "RuntimeMessageCases", "RuntimeMessageBoundaryProbe")
}

func TestEmailRecipientMixedSourceNativeConformance(t *testing.T) {
	runSettingsJSONBoundaryNativeConformance(t, "email_recipient_mixed_boundaries", "RuntimeMessageCases", "RuntimeMessageBoundaryProbe")
}
