package capability

import "testing"

func TestBuildStubBehaviorReportUsesStdlibEvidence(t *testing.T) {
	report := BuildStubBehaviorReport()
	if report.SchemaVersion != StubBehaviorSchemaVersion {
		t.Fatalf("schema version = %d", report.SchemaVersion)
	}
	if report.Totals.Entries == 0 || report.Totals.Members == 0 || report.Totals.Types == 0 {
		t.Fatalf("empty report totals: %+v", report.Totals)
	}
	if report.Totals.ByStatus[string(StubBehaviorImplemented)] == 0 || report.Totals.ByStatus[string(StubBehaviorPassiveDefault)] == 0 {
		t.Fatalf("missing expected status totals: %+v", report.Totals)
	}
	if report.Totals.ByStatus[string(StubBehaviorUnknown)] != 0 {
		t.Fatalf("unexpected unknown behavior entries: %+v", report.Totals)
	}
	if report.Totals.ByStatus["stub-noop"] == 0 {
		t.Fatalf("missing stub/no-op behavior entries: %+v", report.Totals)
	}

	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}
	stringTrim := findStubBehaviorEntry(entries, "String.trim(")
	if stringTrim == nil {
		t.Fatalf("missing String.trim entry")
	}
	if stringTrim.Status != StubBehaviorImplemented {
		t.Fatalf("String.trim status = %q", stringTrim.Status)
	}
	if len(stringTrim.Evidence) == 0 {
		t.Fatalf("String.trim missing evidence")
	}
	stringTemplate := findStubBehaviorEntry(entries, "String.template(")
	if stringTemplate == nil {
		t.Fatalf("missing String.template entry")
	}
	if stringTemplate.Status != StubBehaviorImplemented {
		t.Fatalf("String.template status = %q", stringTemplate.Status)
	}
	searchFind := findStubBehaviorEntry(entries, "Search.find(")
	if searchFind == nil {
		t.Fatalf("missing Search.find entry")
	}
	if searchFind.Status != StubBehaviorImplemented {
		t.Fatalf("Search.find status = %q", searchFind.Status)
	}
	pageCtor := findStubBehaviorEntry(entries, "PageReference.<init>(")
	if pageCtor == nil {
		t.Fatalf("missing PageReference constructor")
	}
	if pageCtor.Status != StubBehaviorImplemented {
		t.Fatalf("PageReference constructor status = %q", pageCtor.Status)
	}
	contextNoOp := findStubBehaviorEntry(entries, "Context.IndustriesContext.addRecordsToContext(")
	if contextNoOp == nil {
		t.Fatalf("missing Context.IndustriesContext.addRecordsToContext entry")
	}
	if got := string(contextNoOp.Status); got != "stub-noop" {
		t.Fatalf("Context.IndustriesContext.addRecordsToContext status = %q, want stub-noop", got)
	}
}

func TestStubBehaviorCapabilityStubMapsToStubNoOp(t *testing.T) {
	status, ok := stubBehaviorStatusFromCapability(StatusStub)
	if !ok {
		t.Fatal("StatusStub did not map to stub behavior")
	}
	if status != StubBehaviorStubNoOp {
		t.Fatalf("StatusStub maps to %q, want %q", status, StubBehaviorStubNoOp)
	}
}

func TestStubBehaviorStatusRankKeepsStubNoOpAbovePassiveDefault(t *testing.T) {
	if stubBehaviorStatusRank(StubBehaviorStubNoOp) >= stubBehaviorStatusRank(StubBehaviorPassiveDefault) {
		t.Fatalf("stub-noop rank should be more specific than passive-default")
	}
	if stubBehaviorStatusRank(StubBehaviorUnsupported) >= stubBehaviorStatusRank(StubBehaviorStubNoOp) {
		t.Fatalf("unsupported rank should remain more specific than stub-noop")
	}
}

func TestStubBehaviorMarksDataSourceAsyncCallbacksImplemented(t *testing.T) {
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range BuildStubBehaviorReport().Entries {
		entries[entry.ID] = entry
	}
	for _, prefix := range []string{
		"DataSource.AsyncSaveCallback.processSave(",
		"DataSource.AsyncDeleteCallback.processDelete(",
	} {
		entry := findStubBehaviorEntry(entries, prefix)
		if entry == nil {
			t.Fatalf("missing %s entry", prefix)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", prefix, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestStubBehaviorMarksCanvasTestHarnessImplemented(t *testing.T) {
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range BuildStubBehaviorReport().Entries {
		entries[entry.ID] = entry
	}

	for _, prefix := range []string{
		"Canvas.Test.mockRenderContext(",
		"Canvas.Test.testCanvasLifecycle(",
	} {
		entry := findStubBehaviorEntry(entries, prefix)
		if entry == nil {
			t.Fatalf("missing %s entry", prefix)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", entry.ID, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestStubBehaviorMarksGeneratedPlatformDescribeConstructorsUnsupported(t *testing.T) {
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range BuildStubBehaviorReport().Entries {
		entries[entry.ID] = entry
	}
	for _, id := range []string{
		"FeatureManagement.<init>()",
		"Schema.DescribeFieldResult.<init>()",
		"Schema.SObjectType.<init>()",
		"Schema.ChildRelationship.<init>()",
	} {
		entry, ok := entries[id]
		if !ok {
			t.Fatalf("missing stub behavior entry %s", id)
		}
		if entry.Status != StubBehaviorUnsupported {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorUnsupported)
		}
	}
	dataCategory := entries["Schema.DataCategory.<init>()"]
	if dataCategory.Status != StubBehaviorImplemented {
		t.Fatalf("Schema.DataCategory.<init>() status = %q, want %q", dataCategory.Status, StubBehaviorImplemented)
	}
	queryLocator := entries["Database.QueryLocator.<init>()"]
	if queryLocator.Status != StubBehaviorImplemented {
		t.Fatalf("Database.QueryLocator.<init>() status = %q, want %q", queryLocator.Status, StubBehaviorImplemented)
	}
}

func TestSchemaRecordTypeInfoPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.RecordTypeInfo.name",
		"Schema.RecordTypeInfo.developername",
		"Schema.RecordTypeInfo.recordtypeid",
		"Schema.RecordTypeInfo.active",
		"Schema.RecordTypeInfo.available",
		"Schema.RecordTypeInfo.defaultrecordtypemapping",
		"Schema.RecordTypeInfo.master",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaDescribeFieldResultPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.DescribeFieldResult.name",
		"Schema.DescribeFieldResult.label",
		"Schema.DescribeFieldResult.type",
		"Schema.DescribeFieldResult.soaptype",
		"Schema.DescribeFieldResult.sobjecttype",
		"Schema.DescribeFieldResult.sobjectfield",
		"Schema.DescribeFieldResult.controllervalues",
		"Schema.DescribeFieldResult.picklistvalues",
		"Schema.DescribeFieldResult.accessible",
		"Schema.DescribeFieldResult.createable",
		"Schema.DescribeFieldResult.updateable",
		"Schema.DescribeFieldResult.defaultvalue",
		"Schema.DescribeFieldResult.defaultvalueformula",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaDescribeSObjectResultPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.DescribeSObjectResult.name",
		"Schema.DescribeSObjectResult.label",
		"Schema.DescribeSObjectResult.labelplural",
		"Schema.DescribeSObjectResult.keyprefix",
		"Schema.DescribeSObjectResult.localname",
		"Schema.DescribeSObjectResult.fields",
		"Schema.DescribeSObjectResult.fieldSets",
		"Schema.DescribeSObjectResult.recordtypeinfos",
		"Schema.DescribeSObjectResult.recordtypeinfosbyname",
		"Schema.DescribeSObjectResult.recordtypeinfosbydevelopername",
		"Schema.DescribeSObjectResult.recordtypeinfosbyid",
		"Schema.DescribeSObjectResult.childrelationships",
		"Schema.DescribeSObjectResult.sobjecttype",
		"Schema.DescribeSObjectResult.accessible",
		"Schema.DescribeSObjectResult.createable",
		"Schema.DescribeSObjectResult.updateable",
		"Schema.DescribeSObjectResult.deletable",
		"Schema.DescribeSObjectResult.queryable",
		"Schema.DescribeSObjectResult.searchable",
		"Schema.DescribeSObjectResult.custom",
		"Schema.DescribeSObjectResult.customsetting",
		"Schema.DescribeSObjectResult.deprecatedandhidden",
		"Schema.DescribeSObjectResult.feedenabled",
		"Schema.DescribeSObjectResult.mruenabled",
		"Schema.DescribeSObjectResult.undeletable",
		"Schema.DescribeSObjectResult.mergeable",
		"Schema.DescribeSObjectResult.sobjectdescribeoption",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaDescribeTabPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.DescribeTabSetResult.description",
		"Schema.DescribeTabSetResult.label",
		"Schema.DescribeTabSetResult.logourl",
		"Schema.DescribeTabSetResult.name",
		"Schema.DescribeTabSetResult.namespace",
		"Schema.DescribeTabSetResult.selected",
		"Schema.DescribeTabSetResult.tabs",
		"Schema.DescribeTabSetResult.tabsetid",
		"Schema.DescribeTabResult.colors",
		"Schema.DescribeTabResult.custom",
		"Schema.DescribeTabResult.icons",
		"Schema.DescribeTabResult.iconurl",
		"Schema.DescribeTabResult.label",
		"Schema.DescribeTabResult.miniiconurl",
		"Schema.DescribeTabResult.mobileurl",
		"Schema.DescribeTabResult.name",
		"Schema.DescribeTabResult.sobjectname",
		"Schema.DescribeTabResult.tabenumorid",
		"Schema.DescribeTabResult.url",
		"Schema.DescribeColorResult.color",
		"Schema.DescribeColorResult.context",
		"Schema.DescribeColorResult.theme",
		"Schema.DescribeIconResult.contenttype",
		"Schema.DescribeIconResult.height",
		"Schema.DescribeIconResult.theme",
		"Schema.DescribeIconResult.url",
		"Schema.DescribeIconResult.width",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaChildRelationshipPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.ChildRelationship.relationshipname",
		"Schema.ChildRelationship.field",
		"Schema.ChildRelationship.childsobject",
		"Schema.ChildRelationship.cascadedelete",
		"Schema.ChildRelationship.deprecatedandhidden",
		"Schema.ChildRelationship.restricteddelete",
		"Schema.ChildRelationship.junctionidlistnames",
		"Schema.ChildRelationship.junctionreferenceto",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaPicklistEntryPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.PicklistEntry.active",
		"Schema.PicklistEntry.defaultvalue",
		"Schema.PicklistEntry.label",
		"Schema.PicklistEntry.value",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaSObjectFieldPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.SObjectField.label",
		"Schema.SObjectField.name",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaDataCategoryPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.DataCategory.childcategories",
		"Schema.DataCategory.label",
		"Schema.DataCategory.name",
		"Schema.DataCategoryGroupSobjectTypePair.datacategorygroupname",
		"Schema.DataCategoryGroupSobjectTypePair.sobject",
		"Schema.DescribeDataCategoryGroupResult.categorycount",
		"Schema.DescribeDataCategoryGroupResult.description",
		"Schema.DescribeDataCategoryGroupResult.label",
		"Schema.DescribeDataCategoryGroupResult.name",
		"Schema.DescribeDataCategoryGroupResult.sobject",
		"Schema.DescribeDataCategoryGroupStructureResult.description",
		"Schema.DescribeDataCategoryGroupStructureResult.label",
		"Schema.DescribeDataCategoryGroupStructureResult.name",
		"Schema.DescribeDataCategoryGroupStructureResult.sobject",
		"Schema.DescribeDataCategoryGroupStructureResult.topcategories",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaFieldSetPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.FieldSet.description",
		"Schema.FieldSet.fields",
		"Schema.FieldSet.label",
		"Schema.FieldSet.name",
		"Schema.FieldSet.namespace",
		"Schema.FieldSet.sobjecttype",
		"Schema.FieldSetMember.dbrequired",
		"Schema.FieldSetMember.fieldpath",
		"Schema.FieldSetMember.label",
		"Schema.FieldSetMember.required",
		"Schema.FieldSetMember.sobjectfield",
		"Schema.FieldSetMember.type",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestSchemaFilteredLookupInfoPropertiesAreImplementedBehavior(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	for _, id := range []string{
		"Schema.FilteredLookupInfo.controllingfields",
		"Schema.FilteredLookupInfo.dependent",
		"Schema.FilteredLookupInfo.optionalfilter",
	} {
		entry := findStubBehaviorEntry(entries, id)
		if entry == nil {
			t.Fatalf("missing %s", id)
		}
		if entry.Status != StubBehaviorImplemented {
			t.Fatalf("%s status = %q, want %q", id, entry.Status, StubBehaviorImplemented)
		}
	}
}

func TestStubBehaviorSeparatesServiceMethodsFromPassiveDTOs(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	assertStubBehaviorPrefix(t, entries, "ConnectApi.Organization.getSettings(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NamedCredentialType.SecuredEndpoint", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CredentialAuthenticationProtocol.Custom", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterUsers.getFollowings(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.setTestGetFeedElementsFromFeed(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterUsers.getFollowers(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.getFeed(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.getFeedElementsFromFeed(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.likeComment(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.shareFeedElement(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.deleteComment(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterGroups.searchGroups(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterGroups.follow(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterGroups.requestGroupMembership(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterMessages.markConversationRead(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterUsers.follow(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Recommendations.getRecommendationsForUser(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManagedContent.getAllManagedContent(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManagedContentDelivery.getChannels(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManagedTopics.getManagedTopics(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManagedContentSpaces.getManagedContentSpaces(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Announcements.getAnnouncements(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFavorites.getFavorites(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceCatalog.getGiftWrapProducts(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceSearch.getSortRules(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NamedCredentials.getNamedCredentials(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NavigationMenu.getCommunityNavigationMenu(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpQuery.getAllMetadata(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpQuery.querySql(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpQuery.querySqlStatus(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpQuery.cancelQuerySql(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpCalculatedInsight.getCalculatedInsights(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpCalculatedInsight.runCalculatedInsight(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpCatalog.getLineage(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpOptimizationConnectApi.postDataModelObjectQueryCount(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpSegment.getSegments()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpQuickAttributes.getQuickAttributes(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CdpMachineLearning.predict(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EinsteinLLM.getPromptTemplates(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EinsteinLLM.generateMessages(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NextBestAction.getRecommendation(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NextBestAction.getRecommendationReaction(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NextBestAction.getRecommendationReactions(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NextBestAction.executeStrategy(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NextBestAction.setRecommendationReaction(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Personalization.getAudiences(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Personalization.updateAudience(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.SmartDataDiscovery.getAIModels(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.SmartDataDiscovery.predict(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Communities.getCommunities(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommunityModeration.getFlagsOnComment(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.RecordAlert.getRecordAlerts(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Records.getMotif(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.RecordUi.getPicklistValuesByRecordType(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Sharing.getRecordAccessDetail(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.UserProfiles.getUserProfile(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceBuyerExperience.calculateAdjustmentAggregates(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceCart.calculateCart(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceCart.getCartSummary(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceCart.getOrCreateActiveCartSummary(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceInventory.checkInventoryAvailability(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommercePromotions.evaluate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceStorePricing.getProductPrice(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.CommerceWishlist.getWishlistSummaries(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.OmnichannelInventoryService.getInventoryAvailability(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.OrderSummary.previewCancel(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Repricing.productDetails(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Payments.authorize(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EventManagementApis.getMngEvents(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EventManagementApis.createMngEvent(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Example.getExampleEntityWithFields(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Example.updateExampleEntity(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ExampleIDLApiFamily.getAbstract(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ExampleIDLApiFamily.createAbstract(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ExternalManagedAccount.getExternalManagedAccounts(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Orchestration.getOrchestrationInstance(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Guardrail.postValidateGuardrail(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.MarketingIntegration.getForm(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.MarketingIntegration.submitForm(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.BotVersionActivation.getVersionActivationInfo(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.BotVersionActivation.updateVersionStatus(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EvfSdk.getEventTypes(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EvfSdk.publishEvent(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.EmailMergeFieldService.getMergeFields(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.FlowApprovalProcesses.getFlowApprovalProcessWithStatus(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManufacturingSampleManagement.getProductRequirementSpecification(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManufacturingSampleManagement.manageProductRequirementSpecification(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Billing.generateInvoices(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.OmnichannelInventoryService.createReservation(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.postFeedElement(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ManagedContent.publish(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.RecordAlert.performRecordAlertAction(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.UserProfiles.setPhoto(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.NamedCredentials.getOAuthCredentialAuthUrl(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Social.DefaultInboundSocialPostHandler.handleInboundSocialPost(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "Social.DefaultInboundSocialPostHandler.createPersonaParent(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "Social.InboundSocialPostHandlerImpl.handleInboundSocialPost(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "Social.InboundSocialPostHandlerImpl.getPostTagsThatCreateCase()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "Process.SparkPlugApi.describePlugins()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "TrailblazerIdentity.getUserOrgInfo(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "TxnSecurity.EventCondition.evaluate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "TxnSecurity.PolicyCondition.evaluate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "workflow.Action.invoke(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "eventbus.EventPublishSuccessCallback.onSuccess(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Ideas.findSimilar(SObject)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Ideas.getAllRecentReplies(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Ideas.getReadRecentReplies(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Ideas.getUnreadRecentReplies(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Ideas.markRead(String)", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "URL.getFileFieldURL(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ABnExperimentActionEnum.Start()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.FeedElement.getBuildVersion(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.FeedElement.body(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Schema.describeDataCategoryGroups(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.describeDataCategoryGroupStructures(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.getGlobalDescribe()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.describeSObjects(List<String>)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.describeSObjects(List<String>,Object)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategoryGroupSobjectTypePair.setSobject(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategoryGroupSobjectTypePair.setDataCategoryGroupName(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategoryGroupSobjectTypePair.getDataCategoryGroupName()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategory.getChildCategories()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Flow.Interview.createInterview(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Flow.Interview.start(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Flow.Interview.getVariableValue(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.OrgPartition.get(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.Org.get(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.OrgPartition.createFullyQualifiedKey(String,String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.Org.getMissRate()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "cache.CacheBuilder.doLoad(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "cache.SecondaryKeyApi.putImmediate(String,Object,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "cache.SecondaryKeyApi.scanForKeyValues(String,String,Integer)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Continuation.addHttpRequest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Http.send(HttpRequest)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Http.send(Object)", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "HttpRequest.setClientCertificate(String,String)", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "HttpRequest.setClientCertificateName(String)", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Search.query(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Search.find(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.describeTabs(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.getAppDescribe(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.getModuleDescribe(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Formula.builder()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Formula.recalculateFormulas(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "DataWeave.Script.createScript(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "DataWeave.Script.execute(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Datacloud.FindDuplicates.findDuplicates(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Datacloud.FindDuplicatesByIds.findDuplicatesByIds(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "DomainParser.parse(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "DomainParser.parse(Url)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FeatureManagement.changeProtection(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "NLPPredictions.FAQPrediction.predict(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "NLPPredictions.PredictionHandler.handlePredictionRequest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "NLPPredictions.PredictionHandler.handlePredictionResponse(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Metadata.Operations.enqueueDeployment(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Metadata.Operations.checkDeployStatus(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Metadata.Operations.retrieve(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "OrgInstrumentationOperation.start(OrgMetricPublishTypeEnum)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "OrgInstrumentationContext.startTime()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "OrgInstrumentationContext.end()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "OrgInstrumentationService.propagateContext(HttpRequest)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "reports.ReportManager.describeReport(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "reports.ReportManager.getDatatypeFilterOperatorMap(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "reports.ReportManager.runReport(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "reports.ReportManager.runAsyncReport(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "IsvPartners.AppAnalytics.logCustomInteraction(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.UserProvisioningLog.log(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "pref_center.TokenUtility.generateToken(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "pref_center.TokenUtility.generateTokens(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Packaging.getCurrentPackageId()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApptBooking.WaitlistController.call(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "ApptBooking.WaitlistController.invokeMethod(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "BcpProvisionService.enableC2C()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "DistributedLedgerService.enableC2C()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "BusRuleDtMig.DecisionTableMigrationService.migrateDecisionTables(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "BusinessRule.CalculationMatrixMigrationService.migrate(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "BusinessRule.CalculationProcedureMigrationService.migrate(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "BusinessRule.DecisionMatrixRowMigratorService.migrate(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "data_mask.DataMaskIntegrationUtil.isCoreAllowed()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "data_mask.DataMaskIntegrationUtil.isLibraryInUse(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "data_mask.DataMaskIntegrationUtil.getJobs(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "data_mask.DataMaskIntegrationUtil.getRunLogResponse(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Aura.redirect(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ChatterAnswers.AccountCreator.createAccount(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "LiveAgent.LiveAgentRealTimeSystem.routeChatRequests(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "workflow.Action.invoke(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "workflow.ActionDml.invoke(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "LiveAgent.LiveChatRouter.doRouting(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "RichMessaging.AuthRequestHandler.handleAuthRequest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "RichMessaging.ProcessCatalogOrderHandler.processCatalogOrderRequest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "RichMessaging.ProcessFormHandler.processFormRequest(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "RichMessaging.ProcessPaymentHandler.processPaymentRequest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Site.UrlRewriter.generateUrlFor(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Site.createPersonAccountPortalUser(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Network.createExternalUserAsync(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Support.EinsteinBots.sendMessageToBot(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Support.EmailTemplateSelector.getDefaultEmailTemplateId(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Support.LifeScienceAttendees.parse(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Support.MilestoneTriggerTimeCalculator.calculateMilestoneTriggerTime(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "data_mask.DataMaskIntegrationUtil.runMask(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "data_mask.DataMaskIntegrationUtil.cancelJob(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "PushUpgradeCustomizationRepository.create(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "PushUpgradeCustomizationRepository.getCustomUpgradeAllowedForId(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "PushUpgradeCustomizationRepository.getCustomUpgradeTypeForIndex(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "PushUpgradeCustomizationRepository.setCustomUpgradeAllowedForIndex(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "PushUpgradeCustomizationRepository.deleteById(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "applauncher.LoginFormController.getUsernamePasswordSelfRegEnabled()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.LoginFormController.login(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.SelfRegisterController.getExtraFields(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.SelfRegisterController.selfRegister(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "applauncher.SocialLoginController.getAuthProviders()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.SocialLoginController.getSsoUrl(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.SocialLoginController.handleIdp()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "fschousehold.FSCFinancialAccountService.call(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "fscwmgen.RecordAlertProvider.getAlertsByWhatId(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "fscwmgen.RecordAlertProvider.dismissAlert(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "healthcloudext.AppointmentBookingInterop.findSlots(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "healthcloudext.AppointmentBookingInterop.bookAppointment(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "healthcloudext.IntegratedCareManagementApexUtil.checkCreateAccess(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "id_verification.IdentityVerificationExt.search(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "ind_docgen_api.OpenInterface.invokeMethod(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "industries_docgen.DocumentTemplate.Call(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "service_cloud_voice.GroupSetup.listGroups(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "service_cloud_voice.GroupSetup.createGroup(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "UserManagement.initPasswordlessLogin(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserManagement.verifyPasswordlessLogin(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserManagement.sendAsyncEmailConfirmation(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.CommittingBatchable.start(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.DeletingBatchable.execute(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.FlowProvisionBase.hasFlow()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.UserProvisioningPlugin.describe()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.UserProvisioningProcessHandler.invoke(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FormulaRecalcResult.isSuccess()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FormulaRecalcResult.getSObject()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FormulaRecalcResult.getErrors()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FormulaRecalcFieldError.getFieldName()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FormulaRecalcFieldError.getFieldError()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SObjectAccessDecision.getRecords()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SObjectAccessDecision.getRemovedFields()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SObjectAccessDecision.getModifiedIndexes()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "InstallContext.previousVersion()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "InstallContext.installerId()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SandboxContext.organizationId()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "SandboxContext.sandboxId()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "SandboxContext.sandboxName()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "RequestImpl.getCurrent()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "UIRequest.getCurrent()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "UIRequest.getRequestHeader(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QueueableContext.getJobId()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SchedulableContext.getTriggerId()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Database.Batchable.start(Database.BatchableContext)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Database.Batchable.execute(Database.BatchableContext,List<Object>)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Database.Batchable.finish(Database.BatchableContext)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Queueable.execute(QueueableContext)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Finalizer.execute(FinalizerContext)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Metadata.DeployCallback.handleResult(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Process.Plugin.describe()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Process.Plugin.invoke(Process.PluginRequest)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.UserProvisioningPlugin.invoke(Process.PluginRequest)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QuickAction.QuickActionDefaultsHandler.onInitDefaults(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "RestResponse.addHeader(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Iterable.iterator()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Iterator.hasNext()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "VisualEditor.DataRow.isSelected()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "VisualEditor.DataRow.compareTo(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Search.SuggestionOption.setFilter(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Search.SuggestionOption.setLimit(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Database.executeBatch(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Database.convertLead(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Database.treeSave(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Callable.call(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "StubProvider.handleMethodCall(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "RestRequest.addHeader(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "RestRequest.addParameter(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Test.createSoqlStub(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Test.invokeContinuationMethod(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Test.setContinuationResponse(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Test.testNotificationActionHandler(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.renderEmailTemplate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.sendEmailMessage(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.extractInboundEmail(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.InboundEmailHandler.handleInboundEmail(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.NotificationActionHandler.executeAction(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.CustomNotification.send(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.PushNotification.send(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Messaging.PushNotificationPayload.apple(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Test.testSandboxPostCopyScript(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SandboxPostCopy.runApexClass(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schedulable.execute(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Assert.isInstanceOfType(Object,Type)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Assert.isNotInstanceOfType(Object,Type)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Apex.Stack.push(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Apex.Stack.pop()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.addMessages(Object)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.Action.getExpression()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.Action.invoke()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ApexPages.Component.getComponentById(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.KnowledgeArticleVersionStandardController.setDataCategory(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.StandardSetController.getListViewOptions()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.StandardSetController.setPageNumber(Integer)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "AsyncInfo.getCurrentQueueableStackDepth()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "AsyncInfo.getMaximumQueueableStackDepth()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "AsyncInfo.getMinimumQueueableDelayInMinutes()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CURRENCY.newInstance(Decimal,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CURRENCY.format()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cases.generateThreadingMessageId(Id)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cases.getCaseIdFromEmailThreadId(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cases.reparentFeedToCaseId(Id,Id,Id)", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "EmailMessages.getFormattedThreadingToken(Id)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "EmailMessages.getRecordIdFromEmail(String,String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Collator.getInstance()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Collator.compare(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SObject.getQuickActionName()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QuickAction.describeAvailableQuickActions(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QuickAction.describeQuickActions(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QuickAction.retrieveQuickActionTemplate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QuickAction.retrieveQuickActionTemplates(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "QuickAction.performQuickAction(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Test.newSendEmailQuickActionDefaults(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Datacloud.FindDuplicates.findDuplicates(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Datacloud.FindDuplicatesByIds.findDuplicatesByIds(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "DomainParser.parse(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "FeatureManagement.changeProtection(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "KbManagement.PublishingService.publishArticle(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "KbManagement.PublishingService.editOnlineArticle(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "KbManagement.PublishingService.deleteArchivedArticle(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "KbManagement.PublishingService.deleteArchivedArticleVersion(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "KbManagement.PublishingService.deleteDraftArticle(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "KbManagement.PublishingService.deleteDraftTranslation(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Packaging.getCurrentPackageId(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "RemoteObjectController.retrieve(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "SupportPredictiveService.findSimilarCases(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "FlexQueue.moveJobToFront(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "System.pauseJobById(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "System.purgeOldAsyncJobs(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "System.changeOwnPassword(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "System.movePassword(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "System.process(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "System.resetPassword(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "System.resetPasswordWithEmailTemplate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "System.submit(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "BusinessHours.add(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "BusinessHours.addGmt(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "BusinessHours.nextStartDate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Communities.communitiesLanding(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Communities.getCSS()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Communities.login(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "healthcloudext.AppointmentBookingSelfService.findProviders(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "healthcloudext.AppointmentBookingSelfService.bookSelfServiceAppointment(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "healthcloudext.IntegratedCareManagementApexHelper.checkObjectCreationAccess(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "LoyaltyManagement.LoyaltyResources.getPointsBalance(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "LoyaltyManagement.LoyaltyResources.creditPoints(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "LoyaltyManagement.WidgetVisibility.checkVisibility(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "industries_docgen.DocGenPermsAndAccessChecksService.hasDocGenOrgPerm(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "inventorypricing.GetInventoryPricing.getInventory(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "ime_mrm.EventManagementBudgetApi.getMngEventBudgets(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "ime_mrm.EventManagementBudgetApi.createMngEventBudget(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "RevSalesTrxn.PlaceSalesTransactionExecutor.execute(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "SObject.getValues(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "List.addToRelationship(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "List.getAddedToRelationship()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "List.getMarkedForDeletion()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "List.markForDelete(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Map.containsKey(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Map.keySet()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Map.values()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Type.isAssignableFrom(Type)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Matcher.hitEnd()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Matcher.pattern()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Matcher.quoteReplacement(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Matcher.requireEnd()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UUID.fromString(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Version.major()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Version.minor()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Version.patch()", StubBehaviorImplemented)
	for _, typeName := range []string{"Boolean", "Date", "Datetime", "Decimal", "Double", "Id", "Integer", "Long", "String", "Time"} {
		assertStubBehaviorPrefix(t, entries, typeName+".addError(String)", StubBehaviorImplemented)
		assertStubBehaviorPrefix(t, entries, typeName+".addError(Exception)", StubBehaviorImplemented)
	}
	assertStubBehaviorPrefix(t, entries, "QueueableDuplicateSignature.Builder.addString(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Builder.addString(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Process.InputParameter.<init>(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.Builder.withDeliverToCity(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.add(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.clear()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.get(Integer)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.indexOf(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.isEmpty()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.iterator()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.remove(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartAdjustmentBasisList.size()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "CartExtension.OptionalCartItem.empty()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.OptionalCartItem.of(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.OptionalCartItem.isPresent()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.OptionalCartItem.get()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartTestUtil.createCart()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartTestUtil.getCart(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartCalculateExecutorMock.calculate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CartExtension.PricingCartCalculator.calculate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "CartExtension.CheckoutPlaceOrder.validate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "CartExtension.SplitShipmentService.arrangeItems(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "CartExtension.SplitShipmentServiceMock.arrangeItems(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "CommerceDxSampleapp.CommerceDx_Inventory.calculateInventoryLevel(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "WebStoreContext.getCommerceContext()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "commerce_inventory.CommerceInventoryService.checkInventory(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "commerce_inventory.CommerceInventoryService.getInventoryLevel(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "commerce_inventory.CommerceInventoryService.getReservation(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "commerce_inventory.CommerceInventoryService.deleteReservation(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "commerce_inventory.CommerceInventoryService.upsertReservation(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "commercepayments.ClientSidePaymentAdapter.getClientComponentName()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "commerce_ordermanagement.ProductExpandService.returnReasons(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "pref_center.LoadFormData.addOption(String,String,String)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "pref_center.LoadFormData.setTextValue(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "sfsqlquery.SqlTester.setMockRows(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "sfsqlquery.SqlTester.clearMocks()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "sfsqlquery.SqlRowIterator.hasNext()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "sfsqlquery.SqlRowIterator.next()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "sfsqlquery.SqlStatement.execute()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "commercestorepricing.PricingRequestItemCollection.size()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "commercestorepricing.PricingRequestItemCollection.get(Integer)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "commercestorepricing.PricingService.processPrice(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "commercestoretax.ProductIdCollection.getFromList(Integer)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "commercestoretax.GetStoreTaxesInfoResponse.addTaxesInfo(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "commercestoretax.TaxService.processCalculateTaxes(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "SF_Archive.ArchiverAccessor.performArchiverGlobalSearch(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "SF_Archive.ArchiverAccessor.maskArchivedRecords(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ime_mrm.EventManagementBudgetApi.invokeMethod(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "ime_mrm.EventManagementBudgetApi.createMngEventBudget(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "wavetemplate.Access.integUserHasAccessToSObjectField(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "wavetemplate.Answers.put(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "wave.Dags.getDags(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "wave.NodeType.valueOf(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "wave.ProjectionType.ordinal()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "wave.QueryNode.execute(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.AppLauncherSetupReordererController.getModel()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "applauncher.ChangePasswordController.changePassword(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "setup_service_livemessage.MessagingChannelAppleDomainController.getApplePayDomain(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "setup_service_livemessage.MessagingChannelAppleDomainController.uploadDomainVerificationCertificate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "pref_center.PreferenceCenterApexHandler.load(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "formulaeval.FormulaInstance.evaluate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "formulaeval.FormulaInstance.getReferencedFields()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "aiaccelerator.CustomFeatureExtractor.extractFeatures(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "sfdc_enablement.LearningItemSerializeDeserializer.serialize(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "omnichannel.RouteWorkApexController.search(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "omnichannel.RouteWorkApexController.routeWork(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "mapslite.MapsLiteUtils.userHasMaps()", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "mapslite.MapsLiteUtils.falconGeocodeRecords(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "mlplatform.PredictionServiceClient.predictions(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "industries_clm.OpenInterface.invokeMethod(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "embeddedMessaging.EmbeddedMessagingSessionHandler.handleRequestWithSfdcSession(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "YubiAuthForAloha.validateYubiKeyLogin(", StubBehaviorStubNoOp)
	assertStubBehaviorPrefix(t, entries, "OrgMonitorFramework.executeBlackTabRequest(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "RevSignaling.SignalingApexProcessor.execute(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.ChatPostMessageRequest.builder()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.ChatPostMessageRequest.Builder.channel(String)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.Message.getText()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.Message.canBeSeenByUser(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.Button.click()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.Channel.sendMessage(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.Checkbox.toggleValue()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.CheckboxGroup.toggleValue(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.ExternalSelect.query(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.Modal.submit()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.Overflow.clickOption(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.authTest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.usersInfo(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.bookmarksList(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.reactionsGet(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.conversationsListConnectInvites(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.conversationsOpen(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.conversationsClose(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.bookmarksEdit(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.filesRemoteShare(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.filesSharedPublicURL(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.migrationExchange(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.chatPostMessage(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.chatUpdate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.viewsOpen(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.viewsPublish(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.workflowsStepCompleted(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.workflowsStepFailed(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.workflowsUpdateStep(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.bookmarksAdd(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.conversationsArchive(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.conversationsKick(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.authRevoke(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.usersProfileSet(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.AppClient.authTest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClientMock.authTest(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClientMock.chatPostMessage(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.apiTest(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.teamInfo(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.searchAll(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.teamAccessLogs(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.usersIdentity(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.chatPostEphemeral(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.chatScheduleMessage(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.viewsOpen(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserClient.viewsUpdate(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserSession.openChannel(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserSession.postMessage(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserSession.getMessageCount()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserSession.getMessages()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserSession.executeSlashCommand(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.ActionDispatcher.allowUnauthenticatedUsers()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.EventDispatcher.invoke(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserProvisioningProvider.importUsers(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.UserMappingUrlServiceProvider.generateSlackAuthorizationUrl(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Slack.RunnableHandler.run()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Canvas.Test.mockRenderContext(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Canvas.Test.testCanvasLifecycle(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "eventbus.TestBroker.deliver()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "eventbus.TestEventService.publishEvent(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "HttpCalloutMock.respond(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "WebServiceMock.doInvoke(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "WebServiceCallout.invoke(Object,Object,Map,List)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "WebServiceCallout.beginInvoke(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "WebServiceCallout.endInvoke(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "SoqlStubProvider.handleSoqlQuery(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ExternalServiceTest.sendCallback(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "TestAsyncHttp.executeHttpRequest(", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.getDescribe()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.DescribeResult.getAction()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.DescribeResult.getInputs()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.DescribeResult.getOutputs()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.DescribeResult.getType()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.InputParameter.getAdditionalAttributes()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.InputParameter.getName()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.InputParameter.getToolingType()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.InputParameter.getType()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.Result.clone()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.Result.getAction()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.Result.getErrors()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.Result.getInvocationParameters()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.Result.getOutputParameters()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "Invocable.Action.Result.isSuccess()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "functions.FunctionInvokeMock.respond(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "functions.MockFunctionInvocationFactory.createSuccessResponse(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "functions.Function.get(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "functions.Function.invoke(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "functions.FunctionCallback.handleResponse(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "functions.FunctionInvocable.invoke(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "CommerceExtension.ResolutionStrategy.resolve()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Readiness.ProductEvaluator.evaluateReadiness(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Readiness.ProductEvaluator.isActive()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SubMgmt.Test.create(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SubMgmt.Test.modify(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SubMgmt.Test.remove(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "UserProvisioning.ConnectorTestUtil.createConnectedApp(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Test.getExternalService()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Test.invokePage(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Canvas.CanvasLifecycleHandler.excludeContextTypes()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.BaseEndpointExtension.beforeGet(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.BaseEndpointExtension.afterGet(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "sfsqlquery.SqlQueueable.getRows()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "wave.Templates.getTemplates()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.<init>(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.next()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.getAttributeValue(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.toString()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.Document.load(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.Document.createRootElement(String,String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.Document.getRootElement()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.Document.toXmlString()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.addChildElement(String,String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.getChildElements()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.getNamespaceFor(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.removeAttribute(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Exception.getInaccessibleFields()", StubBehaviorImplemented)
	assertStubBehaviorExact(t, entries, "JSONException.getInaccessibleFields()", StubBehaviorUnsupported)
	assertStubBehaviorExact(t, entries, "JSONException.initCause(Exception)", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "DmlException.getNumDml()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "EmailException.getNumDml()", StubBehaviorUnsupported)
}

func assertStubBehaviorExact(t *testing.T, entries map[string]StubBehaviorEntry, id string, want StubBehaviorStatus) {
	t.Helper()
	entry, ok := entries[id]
	if !ok {
		t.Fatalf("missing stub behavior entry %q", id)
	}
	if entry.Status != want {
		t.Fatalf("%s status = %q, want %q", entry.ID, entry.Status, want)
	}
}

func assertStubBehaviorPrefix(t *testing.T, entries map[string]StubBehaviorEntry, prefix string, want StubBehaviorStatus) {
	t.Helper()
	entry := findStubBehaviorEntry(entries, prefix)
	if entry == nil {
		t.Fatalf("missing stub behavior entry with prefix %q", prefix)
	}
	if entry.Status != want {
		t.Fatalf("%s status = %q, want %q", entry.ID, entry.Status, want)
	}
}

func findStubBehaviorEntry(entries map[string]StubBehaviorEntry, prefix string) *StubBehaviorEntry {
	for id, entry := range entries {
		if len(id) >= len(prefix) && id[:len(prefix)] == prefix {
			found := entry
			return &found
		}
	}
	return nil
}
