package storage

import "testing"

func TestMessagingCampaignOverlayRefreshesExistingDefinition(t *testing.T) {
	definition := ObjectDefinition{APIName: "MessagingSession"}
	EnsureStandardObjectFields(&definition)
	if _, ok := ResolveFieldName(definition, "", "CampaignId"); !ok {
		t.Fatal("fresh standard definition lacks captured field")
	}
	delete(definition.Fields, "CampaignId")
	if !RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("stale cached definition was not refreshed")
	}
	fieldName, ok := ResolveFieldName(definition, "", "CampaignId")
	if !ok {
		t.Fatal("refreshed definition lacks captured field")
	}
	field := definition.Fields[fieldName]
	if field.Type != FieldReference || field.RelationshipName != "Campaign" || len(field.ReferenceTo) != 1 || field.ReferenceTo[0] != "Campaign" {
		t.Fatalf("reference facet=%+v", field)
	}
	if RefreshStandardObjectFieldOverlay(&definition) {
		t.Fatal("unchanged refreshed definition required another write")
	}
}
