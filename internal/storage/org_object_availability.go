package storage

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

//go:generate python3 ../../scripts/generate_order_management_objects.py
//go:generate gofmt -w order_management_objects_generated.go b2bavailable_objects_generated.go b2bunavailable_objects_generated.go network_setting_objects_generated.go plain_developer_objects_generated.go

// Only feature transitions backed by native A/B rows belong here. Other
// catalog objects retain their existing policy until native captures establish
// their feature or setting trigger.
var standardObjectFeatures = []struct{ object, feature string }{
	{"AccountContactRelation", "ContactsToMultipleAccounts"},
	{"Knowledge__ka", "Knowledge"},
	{"Knowledge__kav", "Knowledge"},
	{"CurrencyType", "MultiCurrency"},
	{"DatedConversionRate", "MultiCurrency"},
	{"Territory2", "Territory2"},
	{"Territory2Model", "Territory2"},
	{"Territory2Type", "Territory2"},
	{"UserTerritory2Association", "Territory2"},
	{"ObjectTerritory2Association", "Territory2"},
	{"AccountTeamMember", "AccountTeams"},
	{"ForecastingItem", "Forecasts"},
	{"ForecastingQuota", "Forecasts"},
	{"ForecastingType", "Forecasts"},
	{"OpportunityTeamMember", "OpportunityTeam"},
	{"OpportunitySplit", "OpportunitySplits"},
	{"ServiceChannel", "OmniChannel"},
	{"Survey", "Surveys"},
	{"Quote", "Quotes"},
	{"QuoteLineItem", "Quotes"},
	{"QuoteDocument", "Quotes"},
	{"QuoteFeed", "Quotes"},
	{"QuoteHistory", "Quotes"},
	{"QuoteLineItemHistory", "Quotes"},
	{"QuoteShare", "Quotes"},
	{"WebStore", "OrderManagement"},
	{"GiftTransaction", "FundraisingEnabled"},
	{"GiftCommitment", "FundraisingEnabled"},
	{"GiftCommitmentSchedule", "FundraisingEnabled"},
	{"OutreachSourceCode", "FundraisingEnabled"},
	{"PaymentInstrument", "FundraisingEnabled"},
	{"DocumentGenerationProcess", "DocGen"},
	{"DocumentTemplateContentDoc", "DocGen"},
	{"DocumentTemplate", "DocGen"},
	{"DocumentTemplateSection", "DocGen"},
	{"DocumentTemplateToken", "DocGen"},
	{"RegulatoryCode", "PublicSectorAccess"},
	// Native captures (org_split_influence_source): the objects are absent in
	// a plain Developer org at API 62.0 and 67.0. OpportunitySplit and
	// OpportunitySplitType appear after the Opportunity Splits Setup wizard;
	// no deployable setting does that, so only org.features names it.
	// CampaignInfluenceModel appears with campaignSettings.enableCampaignInfluence2.
	{"OpportunitySplitType", "OpportunitySplits"},
	{"CampaignInfluenceModel", "CampaignInfluence2Enabled"},
	{"PickTicket", "OrderManagement"},
	{"PickTicketAssignment", "OrderManagement"},
	{"PickTicketFeed", "OrderManagement"},
	{"PickTicketProduct", "OrderManagement"},
	{"PickTicketProductFeed", "OrderManagement"},
	{"PickTicketShare", "OrderManagement"},
	{"UserLocationAssignment", "OrderManagement"},
	{"PaymentCredit", "OrderManagement"},
	{"PaymentCreditLinePayment", "OrderManagement"},
	{"PaymentCreditTransaction", "OrderManagement"},
	{"WebCartCredit", "OrderManagement"},
	{"PendingOrderSummary", "OrderManagement"},
}

// AllowsStandardObject checks the declared shape, not whether an object is in
// the catalog or physically hydrated. An unspecified project cannot establish
// feature absence. Captured provisioning alternatives share the constructor
// policy so compilation and runtime availability agree.
// HasCapturedB2BCommerceShape identifies the exact native-probed combination.
func (o *OrgState) HasCapturedB2BCommerceShape() bool {
	return o != nil && o.HasFeature("B2BCommerce") && o.HasFeature("Communities") && o.HasFeature("NetworksEnabled") && o.HasFeature("CommerceEnabled")
}

func (o *OrgState) AllowsStandardObject(objectName string) bool {
	if o != nil && o.HasFeature(project.PlainDeveloperObjectShape) && !PlainDeveloperObjectPresent(objectName, PlainDeveloperAnyRoute) {
		return false
	}
	if o.HasCapturedB2BCommerceShape() && !o.HasFeature(project.UnspecifiedStandardFieldShape) {
		name := strings.ToLower(objectName)
		if _, ok := b2bAvailableObjectLookup[name]; ok {
			return true
		}
		if name == "pendingordersummary" || name == "webcartcredit" {
			return true
		}
		if _, ok := b2bUnavailableObjectLookup[name]; ok && !o.HasFeature("OrderManagement") {
			return false
		}
	}
	if _, captured := orderManagementObjectLookup[strings.ToLower(objectName)]; captured {
		if o == nil {
			return false
		}
		if o.HasFeature(project.UnspecifiedStandardFieldShape) || o.HasFeature("OrderManagement") {
			return true
		}
		// Captured rows cover the order-management and exact B2B shapes only.
		// A shape the policy already admitted (for example B2BCommerce,
		// Communities and CommerceEnabled without the Networks setting) has no
		// native row, so it keeps the policy answer it had before.
		if policy := standardFieldPolicyFor(objectName); policy != nil &&
			(len(policy.RequiredFeatures) > 0 || len(policy.RequiredFeaturesAnyOf) > 0) {
			return StandardObjectConstructorAvailableForFeatures(objectName, o.Features)
		}
		return false
	}
	if _, captured := networkSettingObjectLookup[strings.ToLower(objectName)]; captured {
		if o == nil {
			return false
		}
		if stringsEqualFold(objectName, "Network") || stringsEqualFold(objectName, "NetworkMember") {
			return o.HasFeature("NetworksEnabled")
		}
		return o.HasFeature(project.UnspecifiedStandardFieldShape) || o.HasFeature("NetworksEnabled")
	}
	// The captured capture-revision-010093106582497266713559480549844075860237667873875753689875218990361047144977 shapes lack the CRM User license. Glade
	// has no licensed shape yet; keep unspecified projects as the fallback.
	if stringsEqualFold(objectName, "CampaignInfluence") {
		return o != nil && o.HasFeature(project.UnspecifiedStandardFieldShape)
	}
	for _, gate := range standardObjectFeatures {
		if stringsEqualFold(objectName, gate.object) {
			if o == nil {
				return false
			}
			if o.HasFeature(project.UnspecifiedStandardFieldShape) {
				return true
			}
			if (stringsEqualFold(objectName, "OpportunitySplit") || stringsEqualFold(objectName, "OpportunitySplitType")) && !o.HasFeature("OpportunityTeam") {
				return false
			}
			if (strings.HasPrefix(gate.object, "PickTicket") || gate.object == "UserLocationAssignment") && !o.HasFeature("OrderManagementEnabled") {
				return false
			}
			if policy := standardFieldPolicyFor(objectName); policy != nil &&
				(len(policy.RequiredFeatures) > 0 || len(policy.RequiredFeaturesAnyOf) > 0) {
				return StandardObjectConstructorAvailableForFeatures(objectName, o.Features)
			}
			if gate.feature == "FundraisingEnabled" && !o.HasFeature("Fundraising") {
				return false
			}
			// Knowledge objects follow the scratch setting (KnowledgeEnabled);
			// the license feature "Knowledge" alone yields no objects, and an
			// unspecified shape is ungated (captured shape rule).
			if gate.feature == "Knowledge" {
				return o.HasFeature("KnowledgeEnabled")
			}
			return o.HasFeature(gate.feature)
		}
	}
	return true
}

func refreshOrgFeatureObjects(org *OrgState) {
	if org.HasCapturedB2BCommerceShape() && !org.HasFeature(project.UnspecifiedStandardFieldShape) {
		for _, names := range [][]string{b2bAvailableObjectNames, b2bUnavailableObjectNames} {
			for _, name := range names {
				if !org.AllowsStandardObject(name) {
					DeleteObject(org, name)
				} else if _, exists := org.Objects[name]; !exists {
					EnsureStandardObject(org, name)
				}
			}
		}
	}
	for _, name := range orderManagementObjectNames {
		if !org.AllowsStandardObject(name) {
			DeleteObject(org, name)
		} else if _, exists := org.Objects[name]; !exists {
			EnsureStandardObject(org, name)
		}
	}
	for _, name := range networkSettingObjectNames {
		if !org.AllowsStandardObject(name) {
			DeleteObject(org, name)
		} else if _, exists := org.Objects[name]; !exists {
			EnsureStandardObject(org, name)
		}
	}
	if !org.AllowsStandardObject("CampaignInfluence") {
		DeleteObject(org, "CampaignInfluence")
	}
	for _, gate := range standardObjectFeatures {
		if !org.AllowsStandardObject(gate.object) {
			DeleteObject(org, gate.object)
		} else if _, exists := org.Objects[gate.object]; !exists {
			EnsureStandardObject(org, gate.object)
		}
	}
}

// Plain Developer route masks over plainDeveloperObjectLookup bits.
const (
	PlainDeveloperAnonymous62 uint8 = 1
	PlainDeveloperAnonymous67 uint8 = 2
	PlainDeveloperTestOnly62  uint8 = 4
	PlainDeveloperTestOnly67  uint8 = 8
	PlainDeveloperAnyRoute          = PlainDeveloperAnonymous62 | PlainDeveloperAnonymous67 | PlainDeveloperTestOnly62 | PlainDeveloperTestOnly67
)

// PlainDeveloperObjectPresent reports whether a standard object is in the
// native plain Developer global describe for any route in mask
// (plain_global_describe). Names that are neither catalog objects nor derived
// standard entities (custom and project objects) are always present here.
func PlainDeveloperObjectPresent(objectName string, mask uint8) bool {
	name := strings.ToLower(strings.TrimSpace(objectName))
	if bits, listed := plainDeveloperObjectLookup[name]; listed {
		return bits&mask != 0
	}
	if _, known := ResolveKnownStandardObjectName(name); known {
		return false
	}
	// Derived standard entities outside the catalog follow the same list.
	if !strings.Contains(name, "__") {
		for _, suffix := range []string{"changeevent", "share", "feed", "history"} {
			if strings.HasSuffix(name, suffix) {
				return false
			}
		}
	}
	return true
}
