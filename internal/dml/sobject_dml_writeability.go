package dml

import "strings"

func allowSObjectWriteabilityOverride(objectName, field string) bool {
	if strings.EqualFold(objectName, "Account") && strings.EqualFold(field, "IsPersonAccount") {
		return true
	}
	if strings.EqualFold(objectName, "Lead") && isLocalWritableLeadField(field) {
		return true
	}
	if strings.EqualFold(objectName, "CampaignMemberStatus") && isLocalWritableCampaignMemberStatusField(field) {
		return true
	}
	if strings.EqualFold(field, "Name") && (strings.EqualFold(objectName, "Contact") || strings.EqualFold(objectName, "Lead")) {
		return true
	}
	return false
}

func isLocalWritableLeadField(field string) bool {
	switch strings.ToLower(strings.TrimSpace(field)) {
	case "donotcall", "hasoptedoutofemail", "hasoptedoutoffax":
		return true
	default:
		return false
	}
}

func isStandardSObjectCreateIdentityRelationship(objectName, field string) bool {
	switch strings.ToLower(strings.TrimSpace(objectName)) {
	case "pricebookentry":
		return strings.EqualFold(field, "Pricebook2Id") || strings.EqualFold(field, "Product2Id")
	case "opportunitylineitem":
		return strings.EqualFold(field, "OpportunityId") || strings.EqualFold(field, "PricebookEntryId")
	default:
		return false
	}
}

func allowMissingSObjectReferenceTarget(target string) bool {
	return strings.EqualFold(target, "User")
}
