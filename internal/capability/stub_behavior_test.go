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
	search := entries["Search"]
	if search.Status != StubBehaviorUnsupported {
		t.Fatalf("Search status = %q", search.Status)
	}
	pageCtor := findStubBehaviorEntry(entries, "PageReference.<init>(")
	if pageCtor == nil {
		t.Fatalf("missing PageReference constructor")
	}
	if pageCtor.Status != StubBehaviorImplemented {
		t.Fatalf("PageReference constructor status = %q", pageCtor.Status)
	}
}

func TestStubBehaviorSeparatesServiceMethodsFromPassiveDTOs(t *testing.T) {
	report := BuildStubBehaviorReport()
	entries := map[string]StubBehaviorEntry{}
	for _, entry := range report.Entries {
		entries[entry.ID] = entry
	}

	assertStubBehaviorPrefix(t, entries, "ConnectApi.Organization.getSettings(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterUsers.getFollowings(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterUsers.getFollowers(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ChatterFeeds.getFeed(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.Communities.getCommunities(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.ABnExperimentActionEnum.Start()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.FeedElement.getBuildVersion(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "ConnectApi.FeedElement.body(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Schema.describeDataCategoryGroups(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategoryGroupSobjectTypePair.setSobject(String)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategoryGroupSobjectTypePair.getDataCategoryGroupName()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Schema.DataCategory.getChildCategories()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Flow.Interview.start(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Cache.OrgPartition.get(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.Org.get(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.OrgPartition.createFullyQualifiedKey(String,String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Cache.Org.getMissRate()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Continuation.addHttpRequest(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Http.send(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Search.find(", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Assert.isInstanceOfType(Object,Type)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Assert.isNotInstanceOfType(Object,Type)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Apex.Stack.push(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Apex.Stack.pop()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.addMessages(Object)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.Action.getExpression()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "ApexPages.Action.invoke()", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "ApexPages.Component.getComponentById(String)", StubBehaviorImplemented)
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
	assertStubBehaviorPrefix(t, entries, "Collator.getInstance()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Collator.compare(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SObject.getQuickActionName()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "SObject.getValues(String)", StubBehaviorImplemented)
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
	assertStubBehaviorPrefix(t, entries, "CartExtension.CartCalculateExecutorMock.calculate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "CartExtension.PricingCartCalculator.calculate(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "CartExtension.SplitShipmentService.arrangeItems(", StubBehaviorUnsupported)
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
	assertStubBehaviorPrefix(t, entries, "SF_Archive.ArchiverAccessor.performArchiverGlobalSearch(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.ChatPostMessageRequest.builder()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.ChatPostMessageRequest.Builder.channel(String)", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.Message.getText()", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "Slack.Message.canBeSeenByUser(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClient.authTest(", StubBehaviorUnsupported)
	assertStubBehaviorPrefix(t, entries, "Slack.BotClientMock.authTest(", StubBehaviorPassiveDefault)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.<init>(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.next()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.getAttributeValue(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "XmlStreamReader.toString()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.addChildElement(String,String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.getChildElements()", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.getNamespaceFor(String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "dom.XmlNode.removeAttribute(String,String)", StubBehaviorImplemented)
	assertStubBehaviorPrefix(t, entries, "Exception.getInaccessibleFields()", StubBehaviorImplemented)
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
