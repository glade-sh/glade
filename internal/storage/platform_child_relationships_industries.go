// Standard child relationships that the embedded describe baseline lacks, read from
// parent describes of an owned scratch org (see
// internal/apextest/testdata/conformance/industries_standard_objects/ORG-RUN.txt).

package storage

import (
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

// industriesChildRelationshipFeature gates the edges on the scratch feature whose
// absence removes them natively (child_relationships.tsv, org "none").
const industriesChildRelationshipFeature = "PublicSectorAccess"

// industriesUngatedChildRelationships are the shipped edges that a Developer
// org without the feature also reports (child_relationships.tsv, org "none").
var industriesUngatedChildRelationships = map[string]bool{
	"Asset.AppUsageAssignments": true, "Asset.Emails": true,
	"Contract.AppUsageAssignments": true, "Contract.Emails": true,
	"Order.AppUsageAssignments": true, "Order.Emails": true,
	"Lead.LeadCleanInfos": true,
}

type industriesChildRelationshipName struct{ name, feature string }

var industriesChildRelationshipNames = map[string]industriesChildRelationshipName{}

func init() {
	for parent, rows := range industriesChildRelationshipSupplements {
		for i := range rows {
			if !industriesUngatedChildRelationships[parent+"."+rows[i].name] {
				rows[i].feature = industriesChildRelationshipFeature
			}
			industriesChildRelationshipNames[strings.ToLower(rows[i].child+"."+rows[i].field)] = industriesChildRelationshipName{rows[i].name, rows[i].feature}
		}
		platformChildRelationshipSupplements[parent] = append(platformChildRelationshipSupplements[parent], rows...)
	}
}

// childRelationshipFeatureAllowed reports whether a feature-gated edge applies to
// a definition. A definition without an applied feature signature, or from a
// project that declares no org shape, cannot establish the feature's absence.
func childRelationshipFeatureAllowed(definition ObjectDefinition, feature string) bool {
	if feature == "" {
		return true
	}
	signature, applied := definition.Metadata[standardFieldsOverlayMarker]
	if !applied {
		return true
	}
	return featureSignatureHas(signature, feature) || featureSignatureHas(signature, project.UnspecifiedStandardFieldShape)
}

// NameIndustriesChildRelationships gives a child field its parent-side
// relationship name when the embedded reverse index does not cover the edge, so
// describe and subquery resolution agree for loaded children. The describe
// catalog itself stays unchanged: its projection digest is pinned. Gated edges
// are named only when the child's org shape declares the feature.
func NameIndustriesChildRelationships(definition *ObjectDefinition) {
	for i := range definition.Relations {
		relation := &definition.Relations[i]
		if relation.ChildRelationship != "" {
			continue
		}
		entry, ok := industriesChildRelationshipNames[strings.ToLower(definition.APIName+"."+relation.Field)]
		if ok && childRelationshipFeatureAllowed(*definition, entry.feature) {
			relation.ChildRelationship = entry.name
		}
	}
}

// industriesChildRelationshipSupplements lists edges observed in a Developer scratch org with
// the PublicSectorAccess feature, at API 62.0 and later. Edges whose child field the
// embedded catalog lacks are omitted until the catalog is regenerated.
var industriesChildRelationshipSupplements = map[string][]platformChildRelationship{
	"Account": {
		{name: "AccountApplicants", child: "Applicant", field: "AccountId"},
		{name: "AccountPartyProfiles", child: "PartyProfile", field: "AccountId"},
		{name: "ApplicationForms", child: "ApplicationForm", field: "AccountId"},
		{name: "BusinessLicenseApplicationAccount", child: "BusinessLicenseApplication", field: "AccountId"},
		{name: "BusinessMilestones", child: "BusinessMilestone", field: "PrimaryAccountId"},
		{name: "BusinessProfiles", child: "BusinessProfile", field: "AccountId"},
		{name: "CaseParticipants", child: "CaseParticipant", field: "ParticipantId"},
		{name: "ComplaintParticipants", child: "ComplaintParticipant", field: "ParticipantId"},
		{name: "DocumentChecklistItemAccount", child: "DocumentChecklistItem", field: "AccountId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "EngagementInitiatingAttendee", child: "EngagementInteraction", field: "InitiatingAttendeeId"},
		{name: "EngagementRelatedPerson", child: "EngagementTopic", field: "RelatedPersonId"},
		{name: "ExternalAttendees", child: "EngagementAttendee", field: "ExternalAttendeeId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "HouseholdPartyProfiles", child: "PartyProfile", field: "HouseholdAccountId"},
		{name: "IndividualApplicationAccount", child: "IndividualApplication", field: "AccountId"},
		{name: "IndividualDefinitions", child: "PersonLanguage", field: "IndividualId"},
		{name: "InfoAuthRequestConsenters", child: "InfoAuthorizationRequest", field: "ConsenterId"},
		{name: "InfoAuthRequestPertainsTo", child: "InfoAuthorizationRequest", field: "PertainsToId"},
		{name: "InfoAuthRequestRequesters", child: "InfoAuthorizationRequest", field: "RequesterId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "PartyExpenseParty", child: "PartyExpense", field: "PartyId"},
		{name: "PartyFinAssetParty", child: "PartyFinancialAsset", field: "PartyId"},
		{name: "PartyIncomes", child: "PartyIncome", field: "PartyId"},
		{name: "PersonDisabilities", child: "PersonDisability", field: "IndividualId"},
		{name: "PersonEmploymentDetails", child: "PersonEmployment", field: "RelatedPersonId"},
		{name: "PublicApplicationParticipants", child: "PublicApplicationParticipant", field: "ParticipantId"},
		{name: "PublicComplaintAccount", child: "PublicComplaint", field: "AccountId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
		{name: "RegulatoryTrxnFees", child: "RegulatoryTrxnFee", field: "AccountId"},
	},
	"Asset": {
		{name: "AppUsageAssignments", child: "AppUsageAssignment", field: "RecordId"},
		{name: "AuthRegApplicationAsset", child: "AuthApplicationAsset", field: "AssetId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "Emails", child: "EmailMessage", field: "RelatedToId"},
		{name: "EngagementInitiatedTopic", child: "EngagementTopic", field: "TopicId"},
		{name: "PartyFinancialAssets", child: "PartyFinancialAsset", field: "AssetId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
	},
	"Case": {
		{name: "ApplicationFormRelations", child: "ApplicationFormRelation", field: "ReferenceRecordId"},
		{name: "BusinessLicenseApplicationCase", child: "BusinessLicenseApplication", field: "ApplicationCaseId"},
		{name: "CaseParticipantCase", child: "CaseParticipant", field: "CaseId"},
		{name: "ComplaintCaseCase", child: "ComplaintCase", field: "CaseId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "EngagementInitiatedTopic", child: "EngagementTopic", field: "TopicId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "IndividualApplicationCase", child: "IndividualApplication", field: "ApplicationCaseId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
		{name: "RegCodeResponseContext", child: "RegulatoryCodeViolation", field: "ResponseContextId"},
		{name: "RegulatoryTrxnFees", child: "RegulatoryTrxnFee", field: "ParentRecordId"},
	},
	"Contact": {
		{name: "CaseParticipants", child: "CaseParticipant", field: "ParticipantId"},
		{name: "ComplaintParticipants", child: "ComplaintParticipant", field: "ParticipantId"},
		{name: "ContactPartyProfiles", child: "PartyProfile", field: "ContactId"},
		{name: "Contacts", child: "ContactProfile", field: "ContactId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "DocumentChecklistWho", child: "DocumentChecklistItem", field: "WhoId"},
		{name: "EngagementInitiatingAttendee", child: "EngagementInteraction", field: "InitiatingAttendeeId"},
		{name: "EngagementRelatedPerson", child: "EngagementTopic", field: "RelatedPersonId"},
		{name: "ExternalAttendees", child: "EngagementAttendee", field: "ExternalAttendeeId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "IndividualDefinitions", child: "PersonLanguage", field: "IndividualId"},
		{name: "InfoAuthRequestConsenters", child: "InfoAuthorizationRequest", field: "ConsenterId"},
		{name: "InfoAuthRequestPertainsTo", child: "InfoAuthorizationRequest", field: "PertainsToId"},
		{name: "InfoAuthRequestRequesters", child: "InfoAuthorizationRequest", field: "RequesterId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "PartyExpenseParty", child: "PartyExpense", field: "PartyId"},
		{name: "PartyFinAssetParty", child: "PartyFinancialAsset", field: "PartyId"},
		{name: "PartyIncomes", child: "PartyIncome", field: "PartyId"},
		{name: "PersonDisabilities", child: "PersonDisability", field: "IndividualId"},
		{name: "PersonEmploymentDetails", child: "PersonEmployment", field: "RelatedPersonId"},
		{name: "PersonExamContact", child: "PersonExamination", field: "ContactId"},
		{name: "PersonLifeEvents", child: "PersonLifeEvent", field: "PrimaryPersonId"},
		{name: "PublicApplicationParticipants", child: "PublicApplicationParticipant", field: "ParticipantId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
		{name: "TCPContact", child: "TrainingCourseParticipant", field: "ContactId"},
	},
	"Contract": {
		{name: "AppUsageAssignments", child: "AppUsageAssignment", field: "RecordId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "Emails", child: "EmailMessage", field: "RelatedToId"},
		{name: "EngagementInitiatedTopic", child: "EngagementTopic", field: "TopicId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
	},
	"Individual": {
		{name: "IndividualDefinitions", child: "PersonLanguage", field: "IndividualId"},
		{name: "PersonDisabilities", child: "PersonDisability", field: "IndividualId"},
	},
	"Lead": {
		{name: "ApplicationForms", child: "ApplicationForm", field: "LeadId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "EngagementInitiatedTopic", child: "EngagementTopic", field: "TopicId"},
		{name: "EngagementInitiatingAttendee", child: "EngagementInteraction", field: "InitiatingAttendeeId"},
		{name: "EngagementRelatedPerson", child: "EngagementTopic", field: "RelatedPersonId"},
		{name: "ExternalAttendees", child: "EngagementAttendee", field: "ExternalAttendeeId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "LeadCleanInfos", child: "LeadCleanInfo", field: "LeadId"},
		{name: "LeadPartyProfiles", child: "PartyProfile", field: "LeadId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
	},
	"Opportunity": {
		{name: "ApplicationForms", child: "ApplicationForm", field: "OpportunityId"},
		{name: "DocumentChecklistItemPR", child: "DocumentChecklistItem", field: "ParentRecordId"},
		{name: "EngagementInitiatedTopic", child: "EngagementTopic", field: "TopicId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
	},
	"Order": {
		{name: "AppUsageAssignments", child: "AppUsageAssignment", field: "RecordId"},
		{name: "Emails", child: "EmailMessage", field: "RelatedToId"},
		{name: "EngagementInitiatedTopic", child: "EngagementTopic", field: "TopicId"},
		{name: "FormRequestRecordRelatedRecords", child: "AuthFormRequestRecord", field: "RelatedRecordId"},
		{name: "InfoAuthRequestSourceRecords", child: "InfoAuthorizationRequest", field: "SourceRecordId"},
		{name: "MultipartyInfoAuthRequestSourceRecords", child: "MultipartyInfoAuthRequest", field: "SourceRecordId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
	},
	"User": {
		{name: "ComplaintParticipants", child: "ComplaintParticipant", field: "ParticipantId"},
		{name: "DocumentChecklistItemUploadedBy", child: "DocumentChecklistItem", field: "UploadedById"},
		{name: "DocumentChecklistWho", child: "DocumentChecklistItem", field: "WhoId"},
		{name: "InfoAuthRequestConsenters", child: "InfoAuthorizationRequest", field: "ConsenterId"},
		{name: "InfoAuthRequestPertainsTo", child: "InfoAuthorizationRequest", field: "PertainsToId"},
		{name: "InfoAuthRequestRequesters", child: "InfoAuthorizationRequest", field: "RequesterId"},
		{name: "InternalAttendees", child: "EngagementAttendee", field: "InternalAttendeeId"},
		{name: "OmniInteractionAccessConfigs", child: "OmniInteractionAccessConfig", field: "SetupOwnerId"},
		{name: "Omni_DataPacks", child: "OmniDataPack", field: "MultiPackUpdatedById"},
		{name: "RecordAlertParent", child: "RecordAlert", field: "ParentId"},
		{name: "RecordAlertWhat", child: "RecordAlert", field: "WhatId"},
	},
}
