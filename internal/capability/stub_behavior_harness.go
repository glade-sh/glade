package capability

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/typesys"
)

func sfsqlqueryHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod && member.Kind != apexast.DeclarationConstructor {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "sfsqlquery.QueryHandle":
		return member.Kind == apexast.DeclarationConstructor ||
			name == "create" || name == "tostring" || name == "withoffset" || name == "withworkloadname"
	case "sfsqlquery.SqlStatement":
		return member.Kind == apexast.DeclarationConstructor ||
			name == "create" || name == "tostring" || name == "withworkloadname"
	case "sfsqlquery.SqlRowIterator":
		switch name {
		case "cancel", "getcolumnnames", "getmetadata", "getqueryid", "hasnext", "iterator", "next", "tostring":
			return true
		default:
			return member.Kind == apexast.DeclarationConstructor
		}
	case "sfsqlquery.SqlTester":
		switch name {
		case "clearmocks", "enqueuemockrows", "isrunningtest", "setmockmetadata", "setmockrows":
			return true
		default:
			return false
		}
	default:
		return false
	}
}
func commerceLocalHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	if strings.HasPrefix(typeName, "CommerceDxSampleapp.") {
		return true
	}
	switch typeName {
	case "WebStoreContext":
		return stubBehaviorMemberStatic(member) && name == "getcommercecontext"
	case "commercepayments.ClientSidePaymentAdapter":
		switch name {
		case "getclientcomponentname", "getclientconfiguration", "processclientrequest":
			return true
		default:
			return false
		}
	case "commerce_ordermanagement.ProductExpandService":
		return name == "returnreasons"
	case "commerce_inventory.CommerceInventoryService":
		switch name {
		case "checkinventory", "getinventorylevel", "getreservation":
			return true
		default:
			return false
		}
	case "CartExtension.AbstractCartCalculator":
		return name == "calculate"
	case "CartExtension.CartCalculate":
		switch name {
		case "calculate", "inventory", "postshipping", "prices", "promotions", "shipping", "taxes":
			return true
		default:
			return false
		}
	case "CartExtension.InventoryCartCalculator", "CartExtension.PricingCartCalculator",
		"CartExtension.PromotionsCartCalculator", "CartExtension.ShippingCartCalculator",
		"CartExtension.TaxCartCalculator":
		return name == "calculate"
	case "CartExtension.CheckoutPlaceOrder":
		return name == "validate"
	case "CartExtension.SplitShipmentService":
		return name == "arrangeitems"
	default:
		return false
	}
}
func localServiceHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	name := strings.ToLower(member.Name)
	switch stubBehaviorTypeName(symbol) {
	case "ApptBooking.WaitlistController":
		return name == "call" || name == "invokemethod"
	case "workflow.Action":
		return name == "invoke"
	case "workflow.ActionDml":
		return name == "invoke"
	case "eventbus.EventPublishFailureCallback":
		return name == "onfailure"
	case "eventbus.EventPublishSuccessCallback":
		return name == "onsuccess"
	case "TxnSecurity.EventCondition", "TxnSecurity.PolicyCondition":
		return name == "evaluate"
	case "Social.DefaultInboundSocialPostHandler", "Social.InboundSocialPostHandlerImpl":
		switch name {
		case "createpersonaparent", "getcasesubject", "getdefaultaccountid",
			"getmaxnumberofdaysclosedtoreopencase", "getpersonafirstname",
			"getpersonalastname", "getposttagsthatcreatecase", "getusingcaseassignmentrule",
			"handleinboundsocialpost":
			return true
		}
	case "Social.InboundSocialPostHandler":
		return name == "handleinboundsocialpost"
	case "Site.UrlRewriter":
		return name == "generateurlfor" || name == "maprequesturl"
	case "LiveAgent.LiveChatRouter":
		return name == "dorouting"
	case "Support.WorkCapacityCalculation":
		return name == "calculateactualusage" || name == "calculateestimatedusage"
	case "Support.MilestoneTriggerTimeCalculator":
		return name == "calculatemilestonetriggertime"
	case "RichMessaging.AuthRequestHandler":
		return name == "handleauthrequest"
	case "RichMessaging.ProcessCatalogOrderHandler":
		return name == "processcatalogorderrequest"
	case "RichMessaging.ProcessFormHandler":
		return name == "processformrequest"
	case "RichMessaging.ProcessPaymentHandler":
		return name == "processpaymentrequest"
	case "BcpProvisionService", "DistributedLedgerService":
		return name == "enablec2c"
	case "BusRuleDtMig.DecisionTableMigrationService":
		return name == "migratedecisiontables"
	case "BusinessRule.CalculationMatrixMigrationService", "BusinessRule.CalculationProcedureMigrationService",
		"BusinessRule.DecisionMatrixRowMigratorService":
		return name == "migrate"
	case "data_mask.DataMaskIntegrationUtil":
		switch name {
		case "getjobs", "getrunlogresponse", "iscoreallowed", "islibraryinuse":
			return true
		default:
			return false
		}
	case "Datacloud.FindDuplicates":
		return name == "findduplicates"
	case "Datacloud.FindDuplicatesByIds":
		return name == "findduplicatesbyids"
	case "DomainParser":
		return name == "parse"
	case "KbManagement.PublishingService":
		switch name {
		case "archiveonlinearticle", "assigndraftarticletask", "assigndrafttranslationtask",
			"cancelscheduledarchivingofarticle", "cancelscheduledpublicationofarticle",
			"completetranslation", "editarchivedarticle", "editonlinearticle",
			"editpublishedtranslation", "publisharticle", "restoreoldversion",
			"scheduleforpublication", "settranslationtoincomplete", "submitfortranslation":
			return true
		default:
			return false
		}
	case "Packaging":
		return name == "getcurrentpackageid"
	case "RemoteObjectController":
		switch name {
		case "create", "del", "retrieve", "updat":
			return true
		default:
			return false
		}
	case "SupportPredictiveService":
		return name == "findsimilarcases"
	default:
		return false
	}
	return false
}
func industryControllerHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "healthcloudext.AppointmentBookingSelfService":
		switch name {
		case "findassets", "findavailableappointmentslots", "findavailableassetslots", "findproviders",
			"getgeolocationcoordinates", "logselfserviceinstrumentation", "validateslotstatusselfservice":
			return true
		default:
			return false
		}
	case "healthcloudext.IntegratedCareManagementApexHelper":
		switch name {
		case "checkentity", "checkobjectcreationaccess", "convertmultilinetohtml",
			"fetchsuggestedassessmentsforpatient", "getcarebarrierdetails", "getmaxaccesslevel",
			"getmru", "getpicklist", "getsoslsearch":
			return true
		default:
			return false
		}
	case "fschousehold.FSCFinancialAccountService", "fschousehold.FSCGoalService",
		"fschousehold.FSCHouseholdService", "fschousehold.FSCPlanService",
		"fschousehold.RetrievalSummaryDataRefresh",
		"healthcloudext.AppointmentBookingSelfServiceWrapper", "healthcloudext.CommunityHelper",
		"healthcloudext.HealthCloudICMCareGapUtil", "healthcloudext.HealthCloudICMDiscoveryFrameworkUtil",
		"healthcloudext.IntegratedCareManagementCPTApexUtil",
		"healthcloudext.IntegratedCareManagementUtil_250", "healthcloudext.ProviderSearchCardUtil",
		"healthcloudext.ReferralManagementUtil", "healthcloudext.SuggestedResponseAssessmentService",
		"healthcloudext.UtilizationManagementWrapper",
		"ind_docgen_api.OpenInterface",
		"industries_docgen.ApryseReplacementService", "industries_docgen.DocumentGenerationProcess",
		"industries_docgen.DocumentTemplate":
		return name == "call" || name == "invokemethod"
	case "healthcloudext.IntegratedCareManagementApexUtil":
		return name == "call" || name == "invokemethod" || name == "checkcaregapaccess" || name == "checkcreateaccess"
	case "fscwmgen.RecordAlertBatchProvider":
		return name == "getalertsbyparentidbatch" || name == "getalertsbywhatidbatch"
	case "fscwmgen.RecordAlertProvider":
		return name == "getalertsbyparentid" || name == "getalertsbywhatid" || name == "getalertsbywhatidandparentid"
	case "healthcloudext.AppointmentBookingInterop", "healthcloudext.AppointmentBookingInteropFhirAdapter":
		return name == "findslots" || name == "getslotstatus"
	case "healthcloudext.IQuotasAndAllocation":
		return name == "validateslotchain"
	case "id_verification.IdentityVerificationExt":
		return name == "getverifiers" || name == "search"
	case "ind_docgen_api.EnvelopeStatusScheduler":
		return name == "execute"
	case "service_cloud_voice.GroupSetup":
		return name == "listgroups"
	case "service_cloud_voice.PhoneNumberProvider":
		return name == "listphonenumbers"
	case "service_cloud_voice.QueueManager":
		return name == "supportsqueueusergrouping"
	case "service_cloud_voice.QueueSetup":
		return name == "listqueues"
	case "LoyaltyManagement.LoyaltyResources":
		switch name {
		case "getloyaltypromotionbasedonsalesforcecdp", "getloyaltypromotions", "getpointsbalance", "gettier":
			return true
		default:
			return false
		}
	case "LoyaltyManagement.WidgetCumulativePromotions", "LoyaltyManagement.WidgetMemberBadges", "LoyaltyManagement.WidgetReferMember":
		return name == "call"
	case "LoyaltyManagement.WidgetVisibility":
		return name == "checkvisibility"
	case "industries_docgen.DocGenPermsAndAccessChecksService":
		return strings.HasPrefix(name, "has") || strings.HasPrefix(name, "is")
	case "inventorypricing.GetInventoryPricing":
		switch name {
		case "createresponse", "getinventory", "getinventoryandpricing", "getpricing", "handleinventorypricingserviceexception", "processinput":
			return true
		default:
			return false
		}
	case "ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi", "ime_mrm.EventManagementSubjectApi":
		return strings.HasPrefix(name, "get")
	default:
		return false
	}
}
func localMockHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Canvas.Test":
		return stubBehaviorMemberStatic(member) && (name == "mockrendercontext" || name == "testcanvaslifecycle")
	case "HttpCalloutMock":
		return name == "respond"
	case "WebServiceMock":
		return name == "doinvoke"
	case "SoqlStubProvider":
		return name == "handlesoqlquery"
	case "eventbus.TestBroker":
		return name == "deliver" || name == "fail"
	case "eventbus.TestEventService":
		return stubBehaviorMemberStatic(member) && name == "publishevent"
	case "ExternalServiceTest":
		return name == "sendcallback"
	case "TestAsyncHttp":
		return name == "executehttprequest"
	case "functions.FunctionInvokeMock":
		return name == "respond"
	case "functions.MockFunctionInvocationFactory":
		return stubBehaviorMemberStatic(member) && (name == "createerrorresponse" || name == "createsuccessresponse")
	case "SubMgmt.Test":
		return stubBehaviorMemberStatic(member) && (name == "create" || name == "modify" || name == "remove")
	case "UserProvisioning.ConnectorTestUtil":
		return stubBehaviorMemberStatic(member) && name == "createconnectedapp"
	case "CartExtension.CartCalculateExecutorMock":
		return strings.EqualFold(member.Type, "void")
	case "CartExtension.SplitShipmentServiceMock":
		return strings.EqualFold(member.Type, "void")
	default:
		return false
	}
}
func connectAPIReadOnlyHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod ||
		!stubBehaviorMemberStatic(member) ||
		!connectAPIReadOnlyHarnessBehaviorType(stubBehaviorTypeName(symbol)) ||
		!connectAPIReadOnlyHarnessBehaviorReturn(member.Type) {
		return false
	}
	return connectAPIReadOnlyHarnessBehaviorMethodAllowed(stubBehaviorTypeName(symbol), member.Name)
}
func connectAPIReadOnlyHarnessBehaviorType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "connectapi.chatterfeeds",
		"connectapi.chattergroups",
		"connectapi.chattermessages",
		"connectapi.chatterusers",
		"connectapi.chatterfavorites",
		"connectapi.chatter",
		"connectapi.topics",
		"connectapi.recommendations",
		"connectapi.actionlinks",
		"connectapi.actionplan",
		"connectapi.announcements",
		"connectapi.botversionactivation",
		"connectapi.cdpcalculatedinsight",
		"connectapi.cdpcatalog",
		"connectapi.cdpoptimizationconnectapi",
		"connectapi.cdpquery",
		"connectapi.cdpquickattributes",
		"connectapi.cdpsegment",
		"connectapi.communities",
		"connectapi.communitymoderation",
		"connectapi.commercebuyerexperience",
		"connectapi.commercecart",
		"connectapi.commercecatalog",
		"connectapi.commerceinventory",
		"connectapi.commercepromotions",
		"connectapi.commercesearch",
		"connectapi.commercestorepricing",
		"connectapi.commercewishlist",
		"connectapi.employeeprofiles",
		"connectapi.fieldset",
		"connectapi.knowledge",
		"connectapi.managedcontent",
		"connectapi.managedcontentchannels",
		"connectapi.managedcontentdelivery",
		"connectapi.managedtopics",
		"connectapi.managedcontentspaces",
		"connectapi.mentions",
		"connectapi.namedcredentials",
		"connectapi.navigationmenu",
		"connectapi.omnichannelinventoryservice",
		"connectapi.nextbestaction",
		"connectapi.ordersummary",
		"connectapi.personalization",
		"connectapi.recordalert",
		"connectapi.records",
		"connectapi.recordui",
		"connectapi.repricing",
		"connectapi.sharing",
		"connectapi.sites",
		"connectapi.smartdatadiscovery",
		"connectapi.einsteinllm",
		"connectapi.userprofiles",
		"connectapi.emailmergefieldservice",
		"connectapi.eventmanagementapis",
		"connectapi.evfsdk",
		"connectapi.example",
		"connectapi.exampleidlapifamily",
		"connectapi.externalmanagedaccount",
		"connectapi.flowapprovalprocesses",
		"connectapi.guardrail",
		"connectapi.manufacturingsamplemanagement",
		"connectapi.marketingintegration",
		"connectapi.orchestration",
		"connectapi.zones":
		return true
	default:
		return false
	}
}
func connectAPIReadOnlyHarnessBehaviorMethodAllowed(typeName, methodName string) bool {
	name := strings.ToLower(methodName)
	switch strings.ToLower(typeName) {
	case "connectapi.cdpcalculatedinsight":
		return name == "getcalculatedinsight" ||
			name == "getcalculatedinsights" ||
			name == "refreshstatuscalculatedinsight" ||
			name == "validatecalculatedinsight"
	case "connectapi.cdpcatalog":
		return name == "getfieldlineage" || name == "getlineage"
	case "connectapi.cdpoptimizationconnectapi":
		switch name {
		case "getdatamodelobject", "getformulafunctions", "getoptimizationdatalakeobject",
			"getoptimizationdatalakeobjects", "getoptimizationdatamodelobjects",
			"getoptimizationdataspaces", "getoptimizationdefinitions",
			"getoptimizationformulaoperators", "getoptimizationorgvalues",
			"getsingleoptimizationdefinition", "getdatamodelobjectquerycount",
			"getoptimizationjobdetails", "getoptimizationjobstatusbyid",
			"getoptimizationjobsfordefinition", "postdatamodelobjectquerycount",
			"validateformulasyntax":
			return true
		default:
			return false
		}
	case "connectapi.cdpquery":
		switch name {
		case "getallmetadata", "getdatagraphmetadata", "getinsightsmetadata",
			"getmetadataentities", "getnextbatchmetadataentities", "getprofilemetadata",
			"getdatagraphdata", "getdatagraphdatawithlookupkeys", "nextbatchansisqlv2",
			"queryansisql", "queryansisqlv2", "querycalculatedinsights",
			"queryprofileapi", "querysql", "querysqlrows", "querysqlstatus",
			"universalidlookupbysourceid":
			return true
		default:
			return false
		}
	case "connectapi.cdpquickattributes":
		return name == "getquickattributebyidorname" || name == "getquickattributes"
	case "connectapi.cdpsegment":
		return name == "getsegment" ||
			name == "getsegmentbyid" ||
			name == "getsegments" ||
			name == "getsegmentsfilteredpaginated" ||
			name == "getsegmentspaginated"
	case "connectapi.einsteinllm":
		return name == "getoutputlanguages" || name == "getprompttemplates"
	case "connectapi.nextbestaction":
		return name == "getrecommendation" ||
			name == "getrecommendationreaction" ||
			name == "getrecommendationreactions"
	case "connectapi.personalization":
		return name == "getaudience" ||
			name == "getaudiencebatch" ||
			name == "getaudiences" ||
			name == "gettarget" ||
			name == "gettargetbatch" ||
			name == "gettargets"
	case "connectapi.smartdatadiscovery":
		return strings.HasPrefix(name, "get")
	case "connectapi.botversionactivation":
		return name == "getversionactivationinfo"
	case "connectapi.emailmergefieldservice":
		return name == "getmergefields"
	case "connectapi.eventmanagementapis":
		return strings.HasPrefix(name, "get")
	case "connectapi.evfsdk":
		return name == "geteventtypes"
	case "connectapi.example":
		return strings.HasPrefix(name, "get")
	case "connectapi.exampleidlapifamily":
		return strings.HasPrefix(name, "get")
	case "connectapi.externalmanagedaccount":
		return strings.HasPrefix(name, "get")
	case "connectapi.flowapprovalprocesses":
		return name == "getflowapprovalprocesswithstatus"
	case "connectapi.guardrail":
		return strings.HasPrefix(name, "get") || name == "postvalidateguardrail"
	case "connectapi.manufacturingsamplemanagement":
		return name == "getproductrequirementspecification" ||
			name == "getproductrequirementspecificationversion"
	case "connectapi.marketingintegration":
		return name == "getform"
	case "connectapi.orchestration":
		return strings.HasPrefix(name, "get")
	case "connectapi.chatter":
		return name == "getfollowers" || name == "getsubscription"
	case "connectapi.chatterfeeds":
		return connectAPIReadOnlyHarnessBehaviorMethodName(methodName) ||
			name == "iscommenteditablebyme" ||
			name == "isfeedelementeditablebyme" ||
			name == "ismodified" ||
			connectAPIChatterSoftNoOpMethod(name)
	case "connectapi.chattergroups":
		return connectAPIReadOnlyHarnessBehaviorMethodName(methodName) ||
			name == "follow" ||
			name == "requestgroupmembership"
	case "connectapi.chattermessages":
		return connectAPIReadOnlyHarnessBehaviorMethodName(methodName) ||
			name == "markconversationread"
	case "connectapi.chatterusers":
		return connectAPIReadOnlyHarnessBehaviorMethodName(methodName) ||
			name == "follow"
	case "connectapi.communities":
		return name == "getcommunities" || name == "getcommunitytemplates"
	case "connectapi.communitymoderation":
		return name == "getflagsoncomment" ||
			name == "getflagsonfeedelement" ||
			name == "getflagsonfeeditem"
	case "connectapi.actionlinks":
		return strings.HasPrefix(name, "getactionlink")
	case "connectapi.actionplan":
		return name == "getactionplantemplateitems"
	case "connectapi.commercebuyerexperience":
		return connectAPICommerceBuyerExperienceReadMethod(name)
	case "connectapi.commercecart":
		return connectAPICommerceCartReadMethod(name)
	case "connectapi.commerceinventory":
		return name == "getinventorylevels" ||
			name == "checkinventoryavailability"
	case "connectapi.commercepromotions":
		return name == "evaluate"
	case "connectapi.commercewishlist":
		return name == "getwishlist" ||
			name == "getwishlistitems" ||
			name == "getwishlistsummaries"
	case "connectapi.ordersummary":
		return name == "adjustpreview" ||
			name == "previewcancel" ||
			name == "previewcancelall" ||
			name == "previewchangeordersummary" ||
			name == "previewreturn"
	case "connectapi.repricing":
		return name == "productdetails" ||
			name == "searchproducts"
	default:
		if connectAPIMutationBehaviorMethodName(name) {
			return false
		}
		return connectAPIReadOnlyHarnessBehaviorMethodName(methodName)
	}
}
func connectAPIReadOnlyHarnessBehaviorMethodName(methodName string) bool {
	name := strings.ToLower(methodName)
	return strings.HasPrefix(name, "get") ||
		strings.HasPrefix(name, "search") ||
		strings.HasPrefix(name, "find") ||
		strings.HasPrefix(name, "list") ||
		strings.HasPrefix(name, "query")
}
func connectAPIReadOnlyHarnessBehaviorReturn(returnType string) bool {
	return returnType != "" && !strings.EqualFold(returnType, "void")
}
func slackTestHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Slack.State":
		return strings.HasPrefix(name, "clear") ||
			strings.HasPrefix(name, "create")
	case "Slack.UserSession":
		switch name {
		case "closeallmodals", "closemodal",
			"executeevent", "executeglobalshortcut", "executemessageshortcut", "executeslashcommand",
			"getapphome", "getmessagecount", "getmessages", "getmodalstack", "getopenchannel", "getstate", "gettopmodal", "getuser",
			"openapphome", "openchannel", "postmessage":
			return true
		default:
			return false
		}
	default:
		return false
	}
}
func slackLocalHarnessComponentBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod {
		return false
	}
	typeName := slackRuntimeBehaviorType(stubBehaviorTypeName(symbol))
	name := strings.ToLower(member.Name)
	switch typeName {
	case "Slack.ActionDispatcher", "Slack.EventDispatcher", "Slack.ShortcutDispatcher", "Slack.SlashCommandDispatcher":
		return name == "allowunauthenticatedusers" && strings.EqualFold(member.Type, "Boolean") ||
			name == "invoke" && strings.EqualFold(member.Type, "Slack.ActionHandler")
	case "Slack.UserMappingUrlServiceProvider":
		return (name == "generatepartnerauthorizationurl" || name == "generateslackauthorizationurl") &&
			strings.EqualFold(member.Type, "String")
	case "Slack.UserProvisioningProvider":
		switch name {
		case "importusers", "revokeusersbysalesforceid", "revokeusersbyslackid":
			return strings.EqualFold(member.Type, "Slack.UserProvisioningResult")
		default:
			return false
		}
	case "Slack.RunnableHandler":
		return name == "run" && strings.EqualFold(member.Type, "void")
	case "Slack.Button":
		return name == "click" && strings.EqualFold(member.Type, "void")
	case "Slack.Channel":
		switch name {
		case "adduser", "removeuser":
			return strings.EqualFold(member.Type, "void")
		case "canbeopenedbyuser":
			return strings.EqualFold(member.Type, "Boolean")
		case "sendmessage":
			return strings.HasPrefix(member.Type, "Slack.")
		default:
			return false
		}
	case "Slack.Checkbox":
		return name == "togglevalue" && strings.EqualFold(member.Type, "void")
	case "Slack.CheckboxGroup":
		return name == "togglevalue" && strings.EqualFold(member.Type, "void")
	case "Slack.ExternalSelect":
		return name == "query" && strings.EqualFold(member.Type, "void")
	case "Slack.Message":
		return name == "canbeseenbyuser" && strings.EqualFold(member.Type, "Boolean")
	case "Slack.Modal":
		switch name {
		case "close":
			return strings.EqualFold(member.Type, "void")
		case "hasinputerrors", "submit":
			return strings.EqualFold(member.Type, "Boolean")
		default:
			return false
		}
	case "Slack.Overflow":
		return name == "clickoption" && strings.EqualFold(member.Type, "void")
	default:
		return false
	}
}
func slackLocalClientHarnessBehaviorMethod(symbol typesys.TypeSymbol, member typesys.MemberSymbol) bool {
	if member.Kind != apexast.DeclarationMethod ||
		strings.EqualFold(member.Type, "void") ||
		member.Type == "" {
		return false
	}
	typeName := stubBehaviorTypeName(symbol)
	if !slackLocalClientHarnessBehaviorType(typeName) {
		return false
	}
	name := strings.ToLower(member.Name)
	if slackLocalClientHarnessSoftNoOpMethodName(name) {
		return true
	}
	if strings.Contains(name, "post") || strings.Contains(name, "open") || strings.Contains(name, "update") {
		return slackLocalClientHarnessCallbackMethodName(name)
	}
	if slackLocalClientHarnessCallbackMethodName(name) {
		return true
	}
	if slackLocalClientHarnessReadMethodName(name) {
		return true
	}
	for _, part := range []string{"add", "archive", "close", "create", "delete", "disable", "enable", "invite", "join", "kick", "leave", "mark", "publish", "push", "remove", "rename", "revoke", "schedule", "send", "set", "share", "unarchive", "uninstall"} {
		if strings.Contains(name, part) {
			return false
		}
	}
	return name == "apitest" ||
		strings.HasPrefix(name, "auth") ||
		strings.HasSuffix(name, "info") ||
		strings.HasSuffix(name, "list") ||
		strings.HasSuffix(name, "history") ||
		strings.HasSuffix(name, "members") ||
		strings.HasSuffix(name, "replies") ||
		strings.HasSuffix(name, "conversations") ||
		strings.HasSuffix(name, "profileget") ||
		strings.HasSuffix(name, "getpresence") ||
		strings.HasSuffix(name, "lookupbyemail")
}
func slackLocalClientHarnessCallbackMethodName(name string) bool {
	switch name {
	case "chatdelete",
		"chatdeletescheduledmessage",
		"chatmemessage",
		"chatpostephemeral",
		"chatpostmessage",
		"chatschedulemessage",
		"chatupdate",
		"viewsopen",
		"viewspublish",
		"viewspush",
		"viewsupdate",
		"workflowsstepcompleted",
		"workflowsstepfailed",
		"workflowsupdatestep":
		return true
	default:
		return false
	}
}
func slackLocalClientHarnessReadMethodName(name string) bool {
	switch name {
	case "bookmarkslist",
		"chatgetpermalink",
		"chatscheduledmessageslist",
		"conversationslistconnectinvites",
		"reactionsget",
		"searchall",
		"searchfiles",
		"searchmessages",
		"teamaccesslogs",
		"teamintegrationlogs",
		"usersidentity":
		return true
	default:
		return false
	}
}
func slackLocalClientHarnessSoftNoOpMethodName(name string) bool {
	switch name {
	case "bookmarksedit",
		"conversationsclose",
		"conversationsmark",
		"conversationsopen",
		"filesremoteshare",
		"filessharedpublicurl",
		"migrationexchange":
		return true
	default:
		return false
	}
}
func slackLocalClientHarnessBehaviorType(typeName string) bool {
	return typeName == "Slack.AppClient" ||
		typeName == "Slack.BotClient" ||
		typeName == "Slack.UserClient"
}
