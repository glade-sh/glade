package apextest

import "testing"

func TestEmailMessageIdentifierOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "email_message_identifier.json", "EmailMessageIdentifier", 7)
}
