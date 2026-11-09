package vm

import (
	"maps"
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// changeEventStandardObjects lists the standard objects whose change event is
// in the native global describe. Every entry comes from the rows of
// global_describe_changeevent_full (API 62 to 68, anonymous and isTest, a
// fresh Developer org with no Change Data Capture setup): name is the
// describe name without "ChangeEvent", label the describe label without
// " Change Event". since is the first API with the entry (0: present at the
// lowest captured API); until is the first API without it (0: still present
// at the highest captured API). APIs outside 62 to 68 are not captured; they
// follow the nearest captured API.
var changeEventStandardObjects = []struct {
	name, label  string
	since, until int
}{
	{"Account", "Account", 0, 0},
	{"AccountCleanInfo", "Account Clean Info", 0, 0},
	{"AccountContactRole", "Account Contact Role", 0, 0},
	{"Address", "Address", 68, 0},
	{"ApiPrtcPolicy", "API Protection Policy", 65, 0},
	{"AppExtension", "App Extension", 0, 0},
	{"AssetAccountParticipant", "Asset Account Participant", 0, 0},
	{"AssetAttribute", "Asset Attribute", 0, 0},
	{"Asset", "Asset", 0, 0},
	{"AssetContactParticipant", "Asset Contact Participant", 65, 0},
	{"AssetRelationship", "Asset Relationship", 0, 0},
	{"AssetWarranty", "Asset Warranty", 0, 0},
	{"AssignedResource", "Assigned Resource", 0, 0},
	{"AssociatedLocation", "Associated Location", 0, 0},
	{"AttributeDefinition", "Attribute Definition", 63, 0},
	{"AttributePicklist", "Attribute Picklist", 63, 0},
	{"AttributePicklistValue", "Attribute Picklist Value", 63, 0},
	{"AuthorizationForm", "Authorization Form", 0, 0},
	{"AuthorizationFormConsent", "Authorization Form Consent", 0, 0},
	{"AuthorizationFormDataUse", "Authorization Form Data Use", 0, 0},
	{"AuthorizationFormText", "Authorization Form Text", 0, 0},
	{"BriefcaseAssignment", "Briefcase Assignment", 0, 0},
	{"BriefcaseDefinition", "Briefcase Definition", 0, 0},
	{"BusinessBrand", "Business Brand", 0, 0},
	{"CalendarView", "Calendar", 0, 0},
	{"Campaign", "Campaign", 0, 0},
	{"CampaignMember", "Campaign Member", 0, 0},
	{"CampaignMemberStatus", "Campaign Member Status", 0, 0},
	{"Case", "Case", 0, 0},
	{"CaseHistory2", "Case History", 0, 0},
	{"CollaborationGroupRecord", "Group Record", 0, 0},
	{"CollabTemplateMetric", "Collaboration Template Metric", 0, 0},
	{"CommitStatus", "Commit Status", 64, 66},
	{"CommSubscription", "Communication Subscription", 0, 0},
	{"CommSubscriptionChannelType", "Communication Subscription Channel Type", 0, 0},
	{"CommSubscriptionConsent", "Communication Subscription Consent", 0, 0},
	{"CommSubscriptionTiming", "Communication Subscription Timing", 0, 0},
	{"ConferenceNumber", "Conference Number", 0, 0},
	{"Contact", "Contact", 0, 0},
	{"ContactCleanInfo", "Contact Clean Info", 0, 0},
	{"ContactPointAddress", "Contact Point Address", 0, 0},
	{"ContactPointConsent", "Contact Point Consent", 0, 0},
	{"ContactPointEmail", "Contact Point Email", 0, 0},
	{"ContactPointPhone", "Contact Point Phone", 0, 0},
	{"ContactPointTypeConsent", "Contact Point Type Consent", 0, 0},
	{"ContentDocument", "Content Document", 0, 0},
	{"ContentDocumentLink", "Content Document Link", 0, 0},
	{"ContentFolder", "Content Folder", 0, 0},
	{"ContentVersion", "Content Version", 0, 0},
	{"Contract", "Contract", 0, 0},
	{"ContractLineItem", "Contract Line Item", 0, 0},
	{"ContractLineOutcomeData", "Contract Line Outcome Data", 0, 0},
	{"DataUseLegalBasis", "Data Use Legal Basis", 0, 0},
	{"DataUsePurpose", "Data Use Purpose", 0, 0},
	{"DevopsRequestInfo", "Request Info", 65, 0},
	{"DigitalSignature", "Digital Signature", 0, 0},
	{"DynamicDataCapture", "Form", 64, 0},
	{"EmailMessage", "Email Message", 0, 0},
	{"EmailTemplate", "Email Template", 0, 0},
	{"EngagementChannelType", "Engagement Channel Type", 0, 0},
	{"Entitlement", "Entitlement", 0, 0},
	{"Event", "Event", 0, 0},
	{"EventRelation", "Event Relation", 0, 0},
	{"EventRelayConfig", "Event Relay Config", 0, 0},
	{"Expense", "Expense", 0, 0},
	{"ExternalEvent", "External Event", 0, 0},
	{"FieldServiceMobileSettings", "Field Service Mobile Settings", 0, 0},
	{"FinanceBalanceSnapshot", "Finance Balance Snapshot", 0, 0},
	{"FinanceTransaction", "Finance Transaction", 0, 0},
	{"FlowOrchestration", "Orchestration", 0, 0},
	{"FlowOrchestrationVersion", "Orchestration Version", 0, 0},
	{"FlowRecord", "Flow", 0, 0},
	{"FlowRecordElement", "Flow Element", 0, 0},
	{"FlowRecordVersion", "Flow Version", 0, 0},
	{"GeolocationBasedAction", "Geolocation Based Action", 64, 0},
	{"Individual", "Individual", 0, 0},
	{"Lead", "Lead", 0, 0},
	{"ListEmail", "List Email", 0, 0},
	{"ListEmailSentResult", "List Email Sent Result", 0, 0},
	{"LiveAgentSession", "Chat Session", 0, 0},
	{"LiveChatTranscript", "Chat Transcript", 0, 0},
	{"LiveChatTranscriptEvent", "Chat Transcript Event", 0, 0},
	{"LiveChatVisitor", "Chat Visitor", 0, 0},
	{"Location", "Location", 0, 0},
	{"LocationGroupAssignment", "Location Group Assignment", 0, 0},
	{"LocationGroup", "Location Group", 0, 0},
	{"Macro", "Macro", 0, 0},
	{"MacroInstruction", "Macro Instruction", 0, 0},
	{"MaintenanceAsset", "Maintenance Asset", 0, 0},
	{"MaintenancePlan", "Maintenance Plan", 0, 0},
	{"MaintenanceWorkRule", "Maintenance Work Rule", 0, 0},
	{"ManagedContent", "Managed Content", 0, 0},
	{"ManagedContentVariant", "Managed Content Variant", 0, 0},
	{"MessagingEndUser", "Messaging User", 0, 0},
	{"MessagingSession", "Messaging Session", 0, 0},
	{"OperatingHours", "Operating Hours", 0, 0},
	{"OperatingHoursHoliday", "Operating Hours Holiday", 0, 0},
	{"Opportunity", "Opportunity", 0, 0},
	{"OpportunityContactRole", "Opportunity Contact Role", 0, 0},
	{"OpportunityLineItem", "Opportunity Product", 0, 0},
	{"Order", "Order", 0, 0},
	{"OrderItem", "Order Product", 0, 0},
	{"PartyConsent", "Party Consent", 0, 0},
	{"PayGroup", "Pay Group", 66, 0},
	{"PaymentPage", "Payment Page", 64, 0},
	{"PayPeriod", "Pay Period", 66, 0},
	{"Pricebook2", "Price Book", 0, 0},
	{"PricebookEntry", "Price Book Entry", 0, 0},
	{"ProcessException", "Process Exception", 0, 0},
	{"ProcessInstance", "Process Instance", 0, 0},
	{"ProcessInstanceStep", "Process Instance Step", 0, 0},
	{"Product2", "Product", 0, 0},
	{"ProductConsumed", "Product Consumed", 0, 0},
	{"ProductConsumedState", "Product Consumed State", 64, 0},
	{"ProductItem", "Product Item", 0, 0},
	{"ProductItemTransaction", "Product Item Transaction", 0, 0},
	{"ProductRequest", "Product Request", 0, 0},
	{"ProductRequestLineItem", "Product Request Line Item", 0, 0},
	{"ProductRequired", "Product Required", 0, 0},
	{"ProductTransfer", "Product Transfer", 0, 0},
	{"ProductTransferState", "Product Transfer State", 65, 0},
	{"ProductWarrantyTerm", "Product Warranty Term", 0, 0},
	{"QuickText", "Quick Text", 0, 0},
	{"QuickTextUsage", "Quick Text Usage", 0, 0},
	{"Recommendation", "Recommendation", 0, 0},
	{"RecordsetFilterCriteria", "Recordset Filter Criteria", 0, 0},
	{"RecordsetFilterCriteriaRule", "Recordset Filter Criteria Rule", 0, 0},
	{"RecordsetFltrCritMonitor", "Recordset Filter Criteria Monitor", 0, 0},
	{"ResourceAbsence", "Resource Absence", 0, 0},
	{"ResourcePreference", "Resource Preference", 0, 0},
	{"ReturnOrder", "Return Order", 0, 0},
	{"ReturnOrderLineItem", "Return Order Line Item", 0, 0},
	{"SerializedProduct", "Serialized Product", 65, 0},
	{"SerializedProductTransaction", "Serialized Product Transaction", 64, 0},
	{"ServiceAppointment", "Service Appointment", 0, 0},
	{"ServiceContract", "Service Contract", 0, 0},
	{"ServiceCrew", "Service Crew", 0, 0},
	{"ServiceCrewMember", "Service Crew Member", 0, 0},
	{"ServiceReport", "Service Report", 0, 0},
	{"ServiceReportLayout", "Service Report Layout", 0, 0},
	{"ServiceResourceCapacity", "Resource Capacity", 0, 0},
	{"ServiceResource", "Service Resource", 0, 0},
	{"ServiceResourceSkill", "Service Resource Skill", 0, 0},
	{"ServiceTerritory", "Service Territory", 0, 0},
	{"ServiceTerritoryLocation", "Service Territory Location", 0, 0},
	{"ServiceTerritoryMember", "Service Territory Member", 0, 0},
	{"Shift", "Shift", 0, 0},
	{"ShiftPattern", "Shift Pattern", 0, 0},
	{"ShiftPatternEntry", "Shift Pattern Entry", 0, 0},
	{"ShiftTemplate", "Shift Template", 0, 0},
	{"Shipment", "Shipment", 0, 0},
	{"Skill", "Skill", 0, 0},
	{"SkillRequirement", "Skill Requirement", 0, 0},
	{"Task", "Task", 0, 0},
	{"TimeSheet", "Time Sheet", 0, 0},
	{"TimeSheetEntry", "Time Sheet Entry", 0, 0},
	{"TimeSlot", "Time Slot", 0, 0},
	{"TodayGoal", "Goal", 0, 0},
	{"UnitOfMeasure", "Unit of Measure", 0, 0},
	{"UserAppInfo", "Last Used App", 0, 0},
	{"User", "User", 0, 0},
	{"UserEmailPreferredPerson", "User Email Preferred Person", 0, 0},
	{"WarrantyTerm", "Warranty Term", 0, 0},
	{"WorkAccess", "Access", 0, 0},
	{"WorkBadge", "Badge Received", 0, 0},
	{"WorkBadgeDefinition", "Badge", 0, 0},
	{"WorkCapacityLimit", "Work Capacity Limit", 0, 0},
	{"WorkOrder", "Work Order", 0, 0},
	{"WorkOrderLineItem", "Work Order Line Item", 0, 0},
	{"WorkPlan", "Work Plan", 0, 0},
	{"WorkPlanSelectionRule", "Work Plan Selection Rule", 0, 0},
	{"WorkPlanTemplate", "Work Plan Template", 0, 0},
	{"WorkPlanTemplateEntry", "Work Plan Template Entry", 0, 0},
	{"WorkStep", "Work Step", 0, 0},
	{"WorkStepTemplate", "Work Step Template", 0, 0},
	{"WorkThanks", "Thanks", 0, 0},
	{"WorkType", "Work Type", 0, 0},
}

type changeEventGlobalExtension struct {
	base     map[string]Value
	baseLen  int
	api      int
	extended map[string]Value
}

// withChangeEventGlobalEntries adds the change event keys the native global
// describe lists for standard objects and, outside a namespaced project, for
// every custom object (project and installed package alike).
func (vm *VM) withChangeEventGlobalEntries(base Value) (Value, bool) {
	if vm == nil || vm.Org == nil || len(base.Map) == 0 {
		// A cleared map keeps its written state in the VM (describe_share_behavior).
		return base, false
	}
	api := vm.currentSourceExecutionPolicy().APIVersion
	major := 0
	for _, candidate := range []int{68, 67, 66, 65, 64, 63, 62} {
		if apexversion.AtLeast(api, candidate) {
			major = candidate
			break
		}
	}
	if major == 0 {
		major = 62
	}
	if cached := vm.changeEventGlobal; cached != nil && sameMapBacking(cached.base, base.Map) && len(base.Map) == cached.baseLen && cached.api == major {
		extended := base
		extended.mapBackingShared = true
		extended.Map = cached.extended
		return extended, true
	}
	additions := map[string]Value{}
	add := func(eventName string) {
		key := mapKey(String(strings.ToLower(eventName)))
		if _, present := base.Map[key]; present {
			return
		}
		additions[key] = sObjectTypeToken(eventName)
	}
	for _, entry := range changeEventStandardObjects {
		if (entry.since != 0 && major < entry.since) || (entry.until != 0 && major >= entry.until) {
			continue
		}
		// The rows come from a plain Developer org: every entry is listed
		// there whether or not Glade models the standard object or its
		// feature (global_describe_changeevent_full).
		add(entry.name + "ChangeEvent")
	}
	if vm.Org.Namespace == "" {
		for name, object := range vm.Org.Objects {
			if !hasSuffixFold(name, "__c") || storage.IsCustomMetadataDefinition(object.Definition) {
				continue
			}
			// A custom-looking name in the standard catalog is not a project
			// object; the native rows list no change event for it.
			if _, catalog := storage.ResolveKnownStandardObjectName(name); catalog {
				continue
			}
			add(name[:len(name)-len("__c")] + "__ChangeEvent")
		}
	}
	if len(additions) == 0 {
		return base, false
	}
	extended := base
	extended.mapBackingShared = true
	extended.Map = maps.Clone(base.Map)
	for key, token := range additions {
		extended.Map[key] = token
	}
	vm.changeEventGlobal = &changeEventGlobalExtension{base: base.Map, baseLen: len(base.Map), api: major, extended: extended.Map}
	return extended, true
}

// changeEventLabel names a change event after its object, as the native
// describe does: "Change Event: <Label>" for a custom object and
// "<Label> Change Event" for a standard one.
func changeEventLabel(baseLabel string, custom bool) string {
	if custom {
		return "Change Event: " + baseLabel
	}
	if definition, ok := storage.StandardObjectDefinition(baseLabel); ok && definition.Label != "" {
		baseLabel = definition.Label
	}
	return baseLabel + " Change Event"
}

// listedChangeEventDefinition resolves a change event the global describe
// lists (standard objects in the native table, and custom objects of a project
// without a namespace) to its canonical name and label.
func (vm *VM) listedChangeEventDefinition(objectName string) (string, string, bool) {
	if vm == nil || vm.Org == nil {
		return "", "", false
	}
	if hasSuffixFold(objectName, "__ChangeEvent") {
		if vm.Org.Namespace != "" {
			return "", "", false
		}
		canonical, ok := vm.resolveObjectName(objectName[:len(objectName)-len("__ChangeEvent")] + "__c")
		if !ok {
			return "", "", false
		}
		if _, catalog := storage.ResolveKnownStandardObjectName(canonical); catalog {
			return "", "", false
		}
		object := vm.Org.Objects[canonical]
		if storage.IsCustomMetadataDefinition(object.Definition) {
			return "", "", false
		}
		return canonical[:len(canonical)-len("__c")] + "__ChangeEvent", changeEventLabel(object.Definition.Label, true), true
	}
	if !hasSuffixFold(objectName, "ChangeEvent") || len(objectName) <= len("ChangeEvent") {
		return "", "", false
	}
	base := objectName[:len(objectName)-len("ChangeEvent")]
	for _, entry := range changeEventStandardObjects {
		if strings.EqualFold(entry.name, base) {
			return entry.name + "ChangeEvent", entry.label + " Change Event", true
		}
	}
	return "", "", false
}

// isChangeEventObjectName reports a listed change event name: any
// "<Object>__ChangeEvent" or a standard "<Object>ChangeEvent" in the table.
func isChangeEventObjectName(name string) bool {
	if hasSuffixFold(name, "__ChangeEvent") {
		return true
	}
	if !hasSuffixFold(name, "ChangeEvent") || len(name) <= len("ChangeEvent") {
		return false
	}
	base := name[:len(name)-len("ChangeEvent")]
	for _, entry := range changeEventStandardObjects {
		if strings.EqualFold(entry.name, base) {
			return true
		}
	}
	return false
}
