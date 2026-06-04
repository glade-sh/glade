package dml

import "strings"

func isLocalWritableCampaignMemberStatusField(field string) bool {
	return strings.EqualFold(field, "CampaignId")
}
