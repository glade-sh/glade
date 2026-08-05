package vm

import "strings"

func normalizeStaticCallCasing(callee string) string {
	if canonical, ok := canonicalBuiltinStaticCall(callee); ok {
		return canonical
	}
	dot := strings.IndexByte(callee, '.')
	if dot < 0 {
		return callee
	}
	typeName := callee[:dot]
	member := callee[dot+1:]
	switch strings.ToLower(typeName) {
	case "system":
		switch strings.ToLower(member) {
		case "assert":
			return "System.assert"
		case "assertequals":
			return "System.assertEquals"
		case "assertnotequals":
			return "System.assertNotEquals"
		case "debug":
			return "System.debug"
		}
	case "database":
		if strings.EqualFold(member, "setSavePoint") {
			return "Database.setSavepoint"
		}
	case "pattern":
		switch strings.ToLower(member) {
		case "compile":
			return "Pattern.compile"
		case "matches":
			return "Pattern.matches"
		case "quote":
			return "Pattern.quote"
		}
	case "integer":
		if strings.EqualFold(member, "valueOf") {
			return "Integer.valueOf"
		}
	case "long":
		if strings.EqualFold(member, "valueOf") {
			return "Long.valueOf"
		}
	case "decimal":
		if strings.EqualFold(member, "valueOf") {
			return "Decimal.valueOf"
		}
	case "double":
		if strings.EqualFold(member, "valueOf") {
			return "Double.valueOf"
		}
	case "boolean":
		if strings.EqualFold(member, "valueOf") {
			return "Boolean.valueOf"
		}
	case "userinfo":
		for _, known := range []string{
			"getCurrentUvid",
			"getUserId",
			"getProfileId",
			"getUserName",
			"getName",
			"getFirstName",
			"getLastName",
			"getUserEmail",
			"getOrganizationId",
			"getOrganizationName",
			"getUserType",
			"getUserRoleId",
			"getSessionId",
			"getLocale",
			"getLanguage",
			"getTimeZone",
			"getUiTheme",
			"getUiThemeDisplayed",
			"hasPackageLicense",
			"isCurrentUserLicensed",
			"isCurrentUserLicensedForPackage",
			"isMultiCurrencyOrganization",
		} {
			if strings.EqualFold(member, known) {
				return "UserInfo." + known
			}
		}
	}
	return callee
}

func (vm *VM) shouldUseBuiltinStaticPrecedence(original, canonical string) bool {
	if _, ok := canonicalBuiltinStaticCall(canonical); !ok {
		return false
	}
	if _, systemPrefixed := stripLeadingSystemNamespace(original); systemPrefixed {
		return true
	}
	root, _, ok := strings.Cut(original, ".")
	if !ok {
		return true
	}
	if _, ok := vm.lookupClass(root); ok {
		return false
	}
	if namespace := strings.TrimSpace(vm.currentExecutionNamespace()); namespace != "" {
		if _, ok := vm.lookupClassInNamespace(namespace, root); ok {
			return false
		}
		if _, ok := vm.lookupClass(namespace + "." + root); ok {
			return false
		}
	}
	if _, ok := vm.Globals[root]; ok {
		return false
	}
	if actual, found := vm.lookupGlobalName(root); found {
		if _, ok := vm.Globals[actual]; ok {
			return false
		}
	}
	if vm.currentClass != "" {
		if _, _, ok := vm.lookupStaticField(vm.currentClass, root); ok {
			return false
		}
	}
	return true
}

var canonicalBuiltinStaticCalls = func() map[string]string {
	names := []string{
		"System.assert", "System.assertEquals", "System.assertNotEquals", "System.debug", "System.today",
		"Assert.areEqual", "Assert.areNotEqual", "Assert.isTrue", "Assert.isFalse", "Assert.isNull", "Assert.isNotNull", "Assert.isInstanceOfType", "Assert.isNotInstanceOfType", "Assert.fail",
		"System.equals", "System.hashCode",
		"System.now", "System.currentTimeMillis", "System.isBatch", "System.isFuture", "System.isQueueable",
		"System.isScheduled", "System.isFunctionCallback", "System.isRunningElasticCompute",
		"System.getApplicationReadWriteMode", "System.getQuiddityShortCode", "System.requestVersion",
		"System.abortJob", "System.attachFinalizer", "System.isRunningTest",
		"IntegrationTest.commitTestOnly",
		"Auth.JWTUtil.parseJWTFromStringWithoutValidation",
		"Test.isRunningTest", "System.currentPageReference", "System.setPassword", "System.enqueueJob", "System.schedule",
		"Limits.getQueries", "Limits.getLimitQueries", "Limits.getQueryRows", "Limits.getLimitQueryRows",
		"Limits.getDmlStatements", "Limits.getLimitDmlStatements", "Limits.getDMLStatements", "Limits.getLimitDMLStatements",
		"Limits.getDmlRows", "Limits.getLimitDmlRows", "Limits.getDMLRows", "Limits.getLimitDMLRows",
		"Limits.getHeapSize", "Limits.getLimitHeapSize", "Limits.getCpuTime", "Limits.getLimitCpuTime",
		"Limits.getCallouts", "Limits.getLimitCallouts", "Limits.getAsyncJobs", "Limits.getLimitAsyncJobs",
		"Limits.getAsyncCalls", "Limits.getLimitAsyncCalls", "Limits.getQueueableJobs", "Limits.getLimitQueueableJobs",
		"Limits.getFutureCalls", "Limits.getLimitFutureCalls", "Limits.getBatchJobs", "Limits.getLimitBatchJobs",
		"Limits.getScheduledJobs", "Limits.getLimitScheduledJobs",
		"Limits.getEmailInvocations", "Limits.getLimitEmailInvocations",
		"Limits.getAggregateQueries", "Limits.getLimitAggregateQueries",
		"Limits.getFindSimilarCalls", "Limits.getLimitFindSimilarCalls",
		"Limits.getMobilePushApexCalls", "Limits.getLimitMobilePushApexCalls",
		"Limits.getQueryLocatorRows", "Limits.getLimitQueryLocatorRows",
		"Limits.getSavepointRollbacks", "Limits.getLimitSavepointRollbacks",
		"Limits.getSoslQueries", "Limits.getLimitSoslQueries",
		"OrgLimits.getAll", "OrgLimits.getMap",
		"Database.query", "Database.queryWithBinds", "Database.countQuery", "Database.countQueryWithBinds", "Database.getQueryLocator", "Database.getQueryLocatorWithBinds",
		"Database.getAsyncLocator",
		"Database.getCursor", "Database.getCursorWithBinds", "Database.getPaginationCursor", "Database.getPaginationCursorWithBinds",
		"Database.setSavepoint", "Database.releaseSavepoint", "Database.rollback", "Database.insert", "Database.update", "Database.delete",
		"Database.insertAsync", "Database.updateAsync", "Database.deleteAsync", "Database.insertImmediate", "Database.updateImmediate", "Database.deleteImmediate",
		"Database.getAsyncSaveResult", "Database.getAsyncDeleteResult", "Database.getDeleted", "Database.getUpdated",
		"Database.upsert", "Database.undelete", "Database.emptyRecycleBin", "Database.lock", "Database.unlock", "Database.executeBatch",
		"Database.treeSave", "Database.convertLead", "Database.merge",
		"Security.stripInaccessible",
		"Approval.process", "Approval.lock", "Approval.unlock", "Approval.isLocked",
		"Answers.findSimilar",
		"QuickAction.describeAvailableActions", "QuickAction.describeAvailableQuickActions", "QuickAction.describeQuickActions",
		"QuickAction.retrieveQuickActionTemplate", "QuickAction.retrieveQuickActionTemplates",
		"QuickAction.performQuickAction", "QuickAction.performQuickActions",
		"String.valueOf", "String.isBlank", "String.isNotBlank", "String.isEmpty", "String.isNotEmpty",
		"String.join", "String.format", "String.getCommonPrefix", "String.getLevenshteinDistance",
		"String.fromCharArray", "String.escapeSingleQuotes",
		"Integer.valueOf", "Long.valueOf", "Decimal.valueOf", "Double.valueOf", "Boolean.valueOf",
		"RoundingMode.valueOf", "Id.valueOf", "AccessLevel.withPermissionSetId",
		"AsyncInfo.getCurrentQueueableStackDepth", "AsyncInfo.getMaximumQueueableStackDepth",
		"AsyncInfo.getMinimumQueueableDelayInMinutes",
		"Pattern.compile", "Pattern.matches", "Pattern.quote", "Matcher.quoteReplacement",
		"Math.abs", "Math.floor", "Math.ceil", "Math.round", "Math.rint", "Math.roundToLong", "Math.signum",
		"Math.sqrt", "Math.cbrt", "Math.acos", "Math.asin", "Math.atan", "Math.cos", "Math.sin", "Math.tan",
		"Math.cosh", "Math.sinh", "Math.tanh",
		"Math.exp", "Math.log", "Math.log10", "Math.max", "Math.min", "Math.mod", "Math.pow",
		"Math.atan2", "Math.random",
		"UUID.fromString", "UUID.randomUUID",
		"Date.today", "Date.newInstance", "Date.valueOf", "Date.parse", "Date.daysInMonth",
		"Datetime.now", "Datetime.newInstance", "Datetime.newInstanceGmt", "Datetime.valueOf", "Datetime.valueOfGmt", "Datetime.parse",
		"Time.newInstance", "Time.valueOf",
		"Location.newInstance",
		"Blob.valueOf",
		"EncodingUtil.base64Encode", "EncodingUtil.base64Decode", "EncodingUtil.convertFromHex", "EncodingUtil.convertToHex",
		"EncodingUtil.urlEncode", "EncodingUtil.urlDecode",
		"URL.getSalesforceBaseUrl", "URL.getOrgDomainUrl", "URL.getCurrentRequestUrl", "URL.getFileFieldURL",
		"Crypto.generateDigest", "Crypto.generateMac", "Crypto.verifyHmac",
		"Crypto.encrypt", "Crypto.decrypt", "Crypto.encryptWithManagedIV", "Crypto.decryptWithManagedIV",
		"Crypto.sign", "Crypto.signWithCertificate", "Crypto.verify", "Crypto.verifyWithCertificate",
		"Crypto.generateAESKey", "Crypto.getRandomInteger", "Crypto.getRandomLong",
		"JSON.createGenerator", "JSON.createParser", "JSON.serialize", "JSON.serializePretty",
		"JSON.deserializeUntyped", "JSON.deserialize", "JSON.deserializeStrict",
		"Type.forName",
		"ConnectApi.Organization.getSettings", "ConnectApi.ChatterUsers.getFollowings", "ConnectApi.Communities.getCommunity", "ConnectApi.Communities.getCommunities",
		"System.ConnectApi.Communities.getCommunity", "System.ConnectApi.Communities.getCommunities",
		"ConnectApi.NextBestAction.executeStrategy", "ConnectApi.NextBestAction.setRecommendationReaction",
		"ConnectApi.Orchestration.getOrchestrationInstanceCollection", "ConnectApi.Orchestration.publishOrchestrationEvent", "ConnectApi.Orchestrator.getOrchestrationInstanceCollection", "ConnectApi.Orchestrator.publishOrchestrationEvent",
		"ConnectApi.ChatterFeeds.postFeedElement", "ConnectApi.ChatterFeeds.postFeedElementBatch",
		"ConnectApi.ChatterFeeds.updateComment", "ConnectApi.ChatterFeeds.getComment",
		"ConnectApi.ChatterUsers.setPhoto", "ConnectApi.ChatterUsers.getReputation",
		"ConnectApi.CommerceCart.getCartSummary", "ConnectApi.CommerceCart.addItemToCart", "ConnectApi.CommerceCart.addItemsToCart",
		"ConnectApi.CommerceCart.getCartItems", "ConnectApi.CommerceCatalog.getProduct",
		"ConnectApi.CommerceStorePricing.getProductPrice", "ConnectApi.CommerceStorePricing.getProductPrices",
		"ConnectApi.Topics.getTopicSuggestions", "ConnectApi.Wave.executeQuery",
		"EventBus.publishWithAccessLevel",
		"BusinessHours.add", "BusinessHours.addGmt", "BusinessHours.diff", "BusinessHours.isWithin", "BusinessHours.nextStartDate",
		"Cases.generateThreadingMessageId", "Cases.getCaseIdFromEmailHeaders", "Cases.getCaseIdFromEmailThreadId", "Cases.reparentFeedToCaseId",
		"EmailMessages.getFormattedThreadingToken", "EmailMessages.getRecordIdFromEmail",
		"Datacloud.FindDuplicates.findDuplicates", "Datacloud.FindDuplicatesByIds.findDuplicatesByIds",
		"Cache.Org.getPartition", "Cache.Session.getPartition",
		"Cache.Org.get", "Cache.Session.get", "Cache.Org.put", "Cache.Session.put",
		"Cache.Org.remove", "Cache.Session.remove", "Cache.Org.contains", "Cache.Session.contains",
		"Cache.Org.getKeys", "Cache.Session.getKeys", "Cache.Org.getNumKeys", "Cache.Session.getNumKeys",
		"Cache.Org.getCapacity", "Cache.Session.getCapacity", "Cache.Org.isAvailable", "Cache.Session.isAvailable",
		"Cache.Org.getName", "Cache.Session.getName",
		"Cache.Org.getAvgGetSize", "Cache.Session.getAvgGetSize", "Cache.Org.getAvgGetTime", "Cache.Session.getAvgGetTime",
		"Cache.Org.getAvgValueSize", "Cache.Session.getAvgValueSize", "Cache.Org.getMaxGetSize", "Cache.Session.getMaxGetSize",
		"Cache.Org.getMaxGetTime", "Cache.Session.getMaxGetTime", "Cache.Org.getMaxValueSize", "Cache.Session.getMaxValueSize",
		"Cache.Org.getMissRate", "Cache.Session.getMissRate",
		"Cache.SecondaryKeyApi.get",
		"Metadata.Operations.enqueueDeployment", "Metadata.Operations.checkDeployStatus", "Metadata.Operations.retrieve",
		"reports.ReportManager.describeReport", "reports.ReportManager.getDatatypeFilterOperatorMap",
		"reports.ReportManager.getReportInstance", "reports.ReportManager.getReportInstances",
		"reports.ReportManager.runAsyncReport", "reports.ReportManager.runReport",
		"IsvPartners.AppAnalytics.logCustomInteraction",
		"UserProvisioning.UserProvisioningLog.log",
		"pref_center.TokenUtility.generateToken", "pref_center.TokenUtility.generateTokens",
		"Ideas.findSimilar", "Ideas.getAllRecentReplies", "Ideas.getReadRecentReplies", "Ideas.getUnreadRecentReplies",
		"Datacloud.FindDuplicates.findDuplicates", "Datacloud.FindDuplicatesByIds.findDuplicatesByIds",
		"DomainParser.parse",
		"FeatureManagement.changeProtection",
		"FeatureManagement.checkPackageBooleanValue", "FeatureManagement.setPackageBooleanValue",
		"FeatureManagement.checkPackageIntegerValue", "FeatureManagement.setPackageIntegerValue",
		"FeatureManagement.checkPackageDateValue", "FeatureManagement.setPackageDateValue",
		"Packaging.getCurrentPackageId",
		"NLPPredictions.FAQPrediction.predict",
		"RequestImpl.getCurrent", "UIRequest.getCurrent",
		"Auth.AuthToken.getAccessToken", "Auth.AuthToken.getAccessTokenMap", "Auth.AuthToken.refreshAccessToken", "Auth.AuthToken.revokeAccess", "Auth.SessionManagement.getCurrentSession",
		"Auth.AuthConfiguration.getAuthProviderSsoUrl", "Auth.CommunitiesUtil.isGuestUser",
		"Messaging.sendEmail", "Messaging.renderStoredEmailTemplate",
		"Messaging.reserveSingleEmailCapacity", "Messaging.reserveMassEmailCapacity",
		"ApexPages.hasMessages", "ApexPages.addMessage", "ApexPages.addMessages", "ApexPages.getMessages", "ApexPages.currentPage",
		"Formula.builder", "Formula.recalculateFormulas",
		"Flow.Interview.createInterview",
		"Test.clearApexPageMessages", "Test.setCurrentPage", "Test.setCurrentPageReference",
		"Test.setMock", "Test.testInstall", "Test.testUninstall", "Test.createStub", "Test.createSoqlStub", "Test.createStubQueryRow", "Test.createStubQueryRows", "Test.loadData",
		"Test.getFlexQueueOrder", "Test.enqueueBatchJobs", "Test.calculatePermissionSetGroup", "Test.enableChangeDataCapture", "Test.setReadOnlyApplicationMode", "Test.isSoqlStubDefined",
		"FlexQueue.moveAfterJob", "FlexQueue.moveBeforeJob", "FlexQueue.moveJobToEnd", "FlexQueue.moveJobToFront",
		"System.pauseJobById", "System.pauseJobByName", "System.purgeOldAsyncJobs", "System.resumeJobById", "System.resumeJobByName",
		"Test.newSendEmailQuickActionDefaults",
		"FeatureManagement.changeProtection",
		"FeatureManagement.checkPackageBooleanValue", "FeatureManagement.setPackageBooleanValue",
		"FeatureManagement.checkPackageIntegerValue", "FeatureManagement.setPackageIntegerValue",
		"FeatureManagement.checkPackageDateValue", "FeatureManagement.setPackageDateValue",
		"KbManagement.PublishingService.archiveOnlineArticle", "KbManagement.PublishingService.assignDraftArticleTask",
		"KbManagement.PublishingService.assignDraftTranslationTask", "KbManagement.PublishingService.cancelScheduledArchivingOfArticle",
		"KbManagement.PublishingService.cancelScheduledPublicationOfArticle", "KbManagement.PublishingService.completeTranslation",
		"KbManagement.PublishingService.editArchivedArticle", "KbManagement.PublishingService.editOnlineArticle",
		"KbManagement.PublishingService.editPublishedTranslation", "KbManagement.PublishingService.publishArticle",
		"KbManagement.PublishingService.restoreOldVersion", "KbManagement.PublishingService.scheduleForPublication",
		"KbManagement.PublishingService.setTranslationToIncomplete", "KbManagement.PublishingService.submitForTranslation",
		"Packaging.getCurrentPackageId",
		"RemoteObjectController.create", "RemoteObjectController.del", "RemoteObjectController.retrieve", "RemoteObjectController.updat", "RemoteObjectController.update",
		"SupportPredictiveService.findSimilarCases",
		"BusRuleDtMig.DecisionTableMigrationService.migrateDecisionTables",
		"BusinessRule.CalculationMatrixMigrationService.migrate", "BusinessRule.CalculationProcedureMigrationService.migrate",
		"BusinessRule.DecisionMatrixRowMigratorService.migrate",
		"healthcloudext.AppointmentBookingSelfService.findAssets", "healthcloudext.AppointmentBookingSelfService.findAvailableAppointmentSlots",
		"healthcloudext.AppointmentBookingSelfService.findAvailableAssetSlots", "healthcloudext.AppointmentBookingSelfService.findProviders",
		"healthcloudext.AppointmentBookingSelfService.getGeoLocationCoordinates", "healthcloudext.AppointmentBookingSelfService.logSelfServiceInstrumentation",
		"healthcloudext.AppointmentBookingSelfService.validateSlotStatusSelfService",
		"healthcloudext.AppointmentBookingSelfService.bookSelfServiceAppointment", "healthcloudext.AppointmentBookingSelfService.cancelSelfServiceAppointment",
		"healthcloudext.AppointmentBookingSelfService.createPatient", "healthcloudext.AppointmentBookingSelfService.publishEventForPFT",
		"healthcloudext.IntegratedCareManagementApexHelper.checkEntity", "healthcloudext.IntegratedCareManagementApexHelper.checkObjectCreationAccess",
		"healthcloudext.IntegratedCareManagementApexHelper.convertMultiLineToHtml", "healthcloudext.IntegratedCareManagementApexHelper.fetchSuggestedAssessmentsForPatient",
		"healthcloudext.IntegratedCareManagementApexHelper.getCareBarrierDetails", "healthcloudext.IntegratedCareManagementApexHelper.getMaxAccessLevel",
		"healthcloudext.IntegratedCareManagementApexHelper.getMru", "healthcloudext.IntegratedCareManagementApexHelper.getPicklist",
		"healthcloudext.IntegratedCareManagementApexHelper.getSOSLSearch",
		"LoyaltyManagement.LoyaltyResources.getLoyaltyPromotionBasedOnSalesforceCDP", "LoyaltyManagement.LoyaltyResources.getLoyaltyPromotions",
		"LoyaltyManagement.LoyaltyResources.getPointsBalance", "LoyaltyManagement.LoyaltyResources.getTier",
		"LoyaltyManagement.LoyaltyResources.changeTier", "LoyaltyManagement.LoyaltyResources.creditPoints",
		"LoyaltyManagement.LoyaltyResources.debitPoints", "LoyaltyManagement.LoyaltyResources.issueVoucher",
		"LoyaltyManagement.LoyaltyResources.transferMemberPointsToGroups", "LoyaltyManagement.LoyaltyResources.updateProgressForCumulativePromotionUsage",
		"LoyaltyManagement.WidgetCumulativePromotions.call", "LoyaltyManagement.WidgetMemberBadges.call",
		"LoyaltyManagement.WidgetReferMember.call", "LoyaltyManagement.WidgetVisibility.checkVisibility",
		"industries_docgen.DocGenPermsAndAccessChecksService.hasDocGenMetadataSetting", "industries_docgen.DocGenPermsAndAccessChecksService.hasDocGenOrgPerm",
		"industries_docgen.DocGenPermsAndAccessChecksService.hasMS365InetgrationSettingOrgPerm", "industries_docgen.DocGenPermsAndAccessChecksService.hasOmniStudioOrgPerm",
		"industries_docgen.DocGenPermsAndAccessChecksService.isDesigner", "industries_docgen.DocGenPermsAndAccessChecksService.isRuntimeCCUser",
		"industries_docgen.DocGenPermsAndAccessChecksService.isRuntimeUser",
		"RevSalesTrxn.PlaceSalesTransactionExecutor.execute",
		"Test.getEventBus", "Test.getExternalService", "Test.invokePage",
		"Test.invokeContinuationMethod", "Test.setContinuationResponse",
		"Canvas.Test.mockRenderContext", "Canvas.Test.testCanvasLifecycle",
		"eventbus.TestEventService.publishEvent",
		"functions.MockFunctionInvocationFactory.createErrorResponse", "functions.MockFunctionInvocationFactory.createSuccessResponse",
		"SubMgmt.Test.create", "SubMgmt.Test.modify", "SubMgmt.Test.remove",
		"UserProvisioning.ConnectorTestUtil.createConnectedApp",
		"BcpProvisionService.enableC2C", "DistributedLedgerService.enableC2C",
		"data_mask.DataMaskIntegrationUtil.cancelJob", "data_mask.DataMaskIntegrationUtil.getJobs",
		"data_mask.DataMaskIntegrationUtil.getRunLogResponse", "data_mask.DataMaskIntegrationUtil.isCoreAllowed",
		"data_mask.DataMaskIntegrationUtil.isLibraryInUse", "data_mask.DataMaskIntegrationUtil.runMask",
		"wave.Templates.cdpQueryMetadata", "wave.Templates.getSObject", "wave.Templates.getSObjects",
		"wave.Templates.getTemplate", "wave.Templates.getTemplateConfig", "wave.Templates.getTemplates",
		"sfsqlquery.SqlTester.clearMocks", "sfsqlquery.SqlTester.enqueueMockRows", "sfsqlquery.SqlTester.setMockRows", "sfsqlquery.SqlTester.setMockMetadata",
		"sfsqlquery.SqlTester.isRunningTest", "sfsqlquery.QueryHandle.create", "sfsqlquery.SqlStatement.create",
		"WebServiceCallout.invoke",
		"CURRENCY.newInstance",
		"Collator.getInstance",
		"Test.setCreatedDate", "Test.setFixedSearchResults", "Test.startTest", "Test.stopTest", "Test.getStandardPricebookId",
		"Test.Database.hasRecords",
		"DataWeave.Script.createScript",
		"UserInfo.getDefaultCurrency",
		"Site.getSiteId", "Site.getBaseUrl", "Site.getBaseRequestUrl", "Site.getBaseSecureUrl", "Site.getBaseCustomUrl", "Site.getBaseInsecureUrl",
		"Site.getCurrentSiteUrl", "Site.getCustomWebAddress", "Site.getAnalyticsTrackingCode", "Site.getExperienceId",
		"Site.getOriginalUrl", "Site.getPasswordPolicyStatement", "Site.getPrefix", "Site.isPasswordExpired",
		"Site.getDomain", "Site.getName", "Site.getTemplate", "Site.getSiteType", "Site.getSiteTypeLabel",
		"Site.getPathPrefix", "Site.getAdminEmail", "Site.getAdminId",
		"Site.getMasterLabel", "Site.isRegistrationEnabled", "Site.isLoginEnabled", "Site.isValidUsername",
		"Site.setExperienceId", "Site.getErrorMessage", "Site.getErrorDescription", "Site.forgotPassword",
		"Site.login", "Site.changePassword", "Site.validatePassword", "Site.createExternalUser", "Site.createPortalUser",
		"Site.createPersonAccountPortalUser", "Site.passwordlessLogin", "Site.setPortalUserAsAuthProvider",
		"Network.getNetworkId", "Network.getLoginUrl", "Network.communitiesLanding",
		"Network.forwardToAuthPage", "Network.getLogoutUrl", "Network.getSelfRegUrl",
		"Network.createExternalUserAsync", "Network.createRecordAsync",
		"Network.loadAllPackageDefaultNetworkDashboardSettings", "Network.loadAllPackageDefaultNetworkPulseSettings",
		"Network.loadAllPackageDefaultNetworkWorkspaceMetricSettings",
		"Aura.redirect",
		"ChatterAnswers.AccountCreator.createAccount",
		"LiveAgent.LiveAgentRealTimeSystem.cancelChatRequests", "LiveAgent.LiveAgentRealTimeSystem.routeChatRequests",
		"LiveAgent.LiveAgentRealTimeSystem.setButtonStatus",
		"Support.EinsteinBots.sendMessageToBot", "Support.EmailTemplateSelector.getDefaultEmailTemplateId",
		"Support.EmailTemplateSelector.getDefaultTemplateId",
		"Support.LifeScienceAttendees.parse", "Support.LifeScienceUpdateEmailTransactions.updateRecords",
		"LoggingLevel.values", "ApexPages.Severity.values", "RoundingMode.values",
		"UserManagement.deregisterVerificationMethod", "UserManagement.initPasswordlessLogin",
		"UserManagement.initRegisterVerificationMethod", "UserManagement.initVerificationMethod",
		"UserManagement.obfuscateUser", "UserManagement.registerVerificationMethod",
		"UserManagement.sendAsyncEmailConfirmation", "UserManagement.verifyPasswordlessLogin",
		"UserManagement.verifyRegisterVerificationMethod", "UserManagement.verifyVerificationMethod",
		"Process.SparkPlugApi.describePlugin", "Process.SparkPlugApi.describePlugins", "Process.SparkPlugApi.invokePluginWithJson",
		"TrailblazerIdentity.generateUserEmailVerificationToken", "TrailblazerIdentity.getUserOrgInfo", "TrailblazerIdentity.splunkLog",
	}
	calls := make(map[string]string, len(names))
	for _, name := range names {
		calls[strings.ToLower(name)] = name
	}
	for alias, canonical := range map[string]string{
		"Assert.areEqual":    "System.assertEquals",
		"Assert.areNotEqual": "System.assertNotEquals",
		"Assert.isTrue":      "System.assert",
	} {
		calls[strings.ToLower(alias)] = canonical
	}
	return calls
}()

func canonicalBuiltinStaticCall(callee string) (string, bool) {
	if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(callee)]; ok {
		return canonical, true
	}
	if rest, ok := stripLeadingSystemNamespace(callee); ok {
		if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(rest)]; ok {
			return canonical, true
		}
		if typeName, member, ok := splitDottedTypeMember(rest); ok {
			if canonicalType, typeOK := canonicalSystemNamespaceType(typeName); typeOK {
				callee := canonicalType + "." + member
				if canonical, ok := canonicalBuiltinStaticCalls[strings.ToLower(callee)]; ok {
					return canonical, true
				}
				return callee, true
			}
		}
	}
	return "", false
}

func stripLeadingSystemNamespace(callee string) (string, bool) {
	const prefix = "System."
	if len(callee) <= len(prefix) || !strings.EqualFold(callee[:len(prefix)], prefix) {
		return "", false
	}
	return callee[len(prefix):], true
}

func splitDottedTypeMember(callee string) (string, string, bool) {
	dot := strings.LastIndex(callee, ".")
	if dot <= 0 || dot >= len(callee)-1 {
		return "", "", false
	}
	return callee[:dot], callee[dot+1:], true
}

func canonicalSystemNamespaceType(typeName string) (string, bool) {
	for _, known := range systemNamespaceTypes {
		if strings.EqualFold(typeName, known) {
			return known, true
		}
	}
	return "", false
}

var systemNamespaceTypes = []string{
	"System",
	"Database",
	"String",
	"Integer",
	"Long",
	"Decimal",
	"Double",
	"Boolean",
	"Id",
	"Schema",
	"Pattern",
	"Math",
	"Date",
	"Datetime",
	"Time",
	"Location",
	"Address",
	"URL",
	"Crypto",
	"JSON",
	"Limits",
	"Test",
	"UserInfo",
	"Security",
	"Messaging",
	"ApexPages",
	"RoundingMode",
	"LoggingLevel",
	"StatusCode",
	"Blob",
	"Type",
}

func lexicalOuterClasses(className string) []string {
	outers := []string{}
	for {
		dot := strings.LastIndex(className, ".")
		if dot <= 0 {
			return outers
		}
		className = className[:dot]
		outers = append(outers, className)
	}
}
