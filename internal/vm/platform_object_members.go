package vm

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

func (vm *VM) generatedPlatformStaticDefault(callee string, args []Value) (Value, bool) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		dot := strings.LastIndex(callee, ".")
		if dot <= 0 || dot >= len(callee)-1 {
			return Null, false
		}
		className, methodName = callee[:dot], callee[dot+1:]
	}
	if value, handled := vm.generatedOptionalWrapperStaticDefault(className, methodName, args); handled {
		return value, true
	}
	if strings.EqualFold(className, "wave.QueryBuilder") {
		if value, handled := callWaveQueryBuilderStaticDefault(methodName, args); handled {
			return value, true
		}
	}
	if waveEnumLikeRuntimeType(className) && strings.EqualFold(methodName, "valueOf") {
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, false
		}
		value := Object(className)
		value.Text = args[0].Text
		value.Fields["ordinal"] = Int(0)
		return value, true
	}
	if waveEnumLikeRuntimeType(className) && strings.EqualFold(methodName, "ordinal") {
		if len(args) != 0 {
			return Null, false
		}
		return Int(0), true
	}
	if strings.EqualFold(className, "CartExtension.CartTestUtil") {
		if value, handled := vm.callCartExtensionCartTestUtilStaticDefault(methodName, args); handled {
			return value, true
		}
	}
	if strings.EqualFold(className, "WebStoreContext") && strings.EqualFold(methodName, "getCommerceContext") {
		if len(args) != 0 {
			return Null, false
		}
		return typedMap("Map<String,String>"), true
	}
	if strings.EqualFold(className, "Ideas") && strings.EqualFold(methodName, "findSimilar") {
		method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
		if !ok {
			return Null, false
		}
		return vm.generatedPlatformMethodDefaultReturn(method, Null, args), true
	}
	if strings.EqualFold(className, "Ideas") {
		switch strings.ToLower(methodName) {
		case "getallrecentreplies", "getreadrecentreplies", "getunreadrecentreplies":
			method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
			if !ok {
				return Null, false
			}
			return vm.generatedPlatformMethodDefaultReturn(method, Null, args), true
		}
	}
	if !vm.generatedPlatformMethodFallbackType(className) {
		return Null, false
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
	if !ok {
		return Null, false
	}
	if !vm.generatedPlatformMethodAllowsDefault(method) {
		return Null, false
	}
	if strings.EqualFold(className, "Invocable.Action") && (strings.EqualFold(methodName, "createCustomAction") || strings.EqualFold(methodName, "createStandardAction")) {
		action := Object("Invocable.Action")
		if len(args) > 0 {
			action.Fields["type"] = args[0]
		}
		if strings.EqualFold(methodName, "createCustomAction") {
			switch len(args) {
			case 2:
				action.Fields["name"] = args[1]
			case 3, 4:
				action.Fields["namespace"] = args[1]
				action.Fields["name"] = args[2]
				if len(args) == 4 {
					action.Fields["version"] = args[3]
				}
			}
			action.Fields["standard"] = Bool(false)
		} else {
			if len(args) > 1 {
				action.Fields["version"] = args[1]
			}
			action.Fields["standard"] = Bool(true)
		}
		return action, true
	}
	return vm.generatedPlatformMethodDefaultReturn(method, Null, args), true
}

func (vm *VM) callPushUpgradeCustomizationRepository(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !strings.EqualFold(className, "PushUpgradeCustomizationRepository") {
		return Null, false, nil
	}
	methodName = canonicalStdlibMemberName(methodName,
		"create",
		"deleteById",
		"deleteByIndex",
		"getCustomUpgradeAllowedForId",
		"getCustomUpgradeAllowedForIndex",
		"getCustomUpgradeTypeForId",
		"getCustomUpgradeTypeForIndex",
		"setCustomUpgradeAllowedForId",
		"setCustomUpgradeAllowedForIndex",
	)
	switch methodName {
	case "create":
		if len(args) != 3 || !stringLikeOrNull(args[0]) || !stringLikeOrNull(args[1]) || args[2].Kind != ValueBool {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.create expects String, String, Boolean")
		}
		packageID := stringValueOrEmpty(args[0])
		subscriberOrgID := stringValueOrEmpty(args[1])
		if existing, ok := vm.pushUpgradeCustomizationByIndex(packageID, subscriberOrgID); ok {
			existing.CustomUpgradeAllowed = args[2].Bool
			vm.pushUpgradeCustoms[existing.ID] = existing
			return String(existing.ID), true, nil
		}
		id := vm.nextPushUpgradeCustomizationID()
		vm.pushUpgradeCustoms[id] = pushUpgradeCustomization{
			ID:                   id,
			PackageID:            packageID,
			SubscriberOrgID:      subscriberOrgID,
			CustomUpgradeAllowed: args[2].Bool,
		}
		return String(id), true, nil
	case "deleteById":
		if len(args) != 1 || !stringLikeOrNull(args[0]) {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.deleteById expects String")
		}
		delete(vm.pushUpgradeCustoms, stringValueOrEmpty(args[0]))
		return Null, true, nil
	case "deleteByIndex":
		if len(args) != 2 || !stringLikeOrNull(args[0]) || !stringLikeOrNull(args[1]) {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.deleteByIndex expects String, String")
		}
		if existing, ok := vm.pushUpgradeCustomizationByIndex(stringValueOrEmpty(args[0]), stringValueOrEmpty(args[1])); ok {
			delete(vm.pushUpgradeCustoms, existing.ID)
		}
		return Null, true, nil
	case "getCustomUpgradeAllowedForId":
		if len(args) != 1 || !stringLikeOrNull(args[0]) {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.getCustomUpgradeAllowedForId expects String")
		}
		if existing, ok := vm.pushUpgradeCustoms[stringValueOrEmpty(args[0])]; ok {
			return Bool(existing.CustomUpgradeAllowed), true, nil
		}
		return Bool(false), true, nil
	case "getCustomUpgradeAllowedForIndex":
		if len(args) != 2 || !stringLikeOrNull(args[0]) || !stringLikeOrNull(args[1]) {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.getCustomUpgradeAllowedForIndex expects String, String")
		}
		if existing, ok := vm.pushUpgradeCustomizationByIndex(stringValueOrEmpty(args[0]), stringValueOrEmpty(args[1])); ok {
			return Bool(existing.CustomUpgradeAllowed), true, nil
		}
		return Bool(false), true, nil
	case "getCustomUpgradeTypeForId":
		if len(args) != 1 || !stringLikeOrNull(args[0]) {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.getCustomUpgradeTypeForId expects String")
		}
		if existing, ok := vm.pushUpgradeCustoms[stringValueOrEmpty(args[0])]; ok {
			return pushUpgradeCustomizationType(existing.CustomUpgradeAllowed), true, nil
		}
		return pushUpgradeCustomizationType(true), true, nil
	case "getCustomUpgradeTypeForIndex":
		if len(args) != 2 || !stringLikeOrNull(args[0]) || !stringLikeOrNull(args[1]) {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.getCustomUpgradeTypeForIndex expects String, String")
		}
		if existing, ok := vm.pushUpgradeCustomizationByIndex(stringValueOrEmpty(args[0]), stringValueOrEmpty(args[1])); ok {
			return pushUpgradeCustomizationType(existing.CustomUpgradeAllowed), true, nil
		}
		return pushUpgradeCustomizationType(true), true, nil
	case "setCustomUpgradeAllowedForId":
		if len(args) != 2 || !stringLikeOrNull(args[0]) || args[1].Kind != ValueBool {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.setCustomUpgradeAllowedForId expects String, Boolean")
		}
		id := stringValueOrEmpty(args[0])
		if existing, ok := vm.pushUpgradeCustoms[id]; ok {
			existing.CustomUpgradeAllowed = args[1].Bool
			vm.pushUpgradeCustoms[id] = existing
		}
		return Null, true, nil
	case "setCustomUpgradeAllowedForIndex":
		if len(args) != 3 || !stringLikeOrNull(args[0]) || !stringLikeOrNull(args[1]) || args[2].Kind != ValueBool {
			return Null, true, fmt.Errorf("PushUpgradeCustomizationRepository.setCustomUpgradeAllowedForIndex expects String, String, Boolean")
		}
		packageID := stringValueOrEmpty(args[0])
		subscriberOrgID := stringValueOrEmpty(args[1])
		if existing, ok := vm.pushUpgradeCustomizationByIndex(packageID, subscriberOrgID); ok {
			existing.CustomUpgradeAllowed = args[2].Bool
			vm.pushUpgradeCustoms[existing.ID] = existing
		}
		return Null, true, nil
	default:
		return Null, true, unsupportedCallError(callee + " local push-upgrade customization repository surface")
	}
}

func (vm *VM) pushUpgradeCustomizationByIndex(packageID, subscriberOrgID string) (pushUpgradeCustomization, bool) {
	for _, customization := range vm.pushUpgradeCustoms {
		if customization.PackageID == packageID && customization.SubscriberOrgID == subscriberOrgID {
			return customization, true
		}
	}
	return pushUpgradeCustomization{}, false
}

func (vm *VM) nextPushUpgradeCustomizationID() string {
	for i := 1; ; i++ {
		id := fmt.Sprintf("puc-%012d", i)
		if _, ok := vm.pushUpgradeCustoms[id]; !ok {
			return id
		}
	}
}

func pushUpgradeCustomizationType(customUpgradeAllowed bool) Value {
	name := "BlockedBySubscriber"
	if customUpgradeAllowed {
		name = "None"
	}
	value := Value{Kind: ValueObject, Type: "CustomizationType", Text: name}
	value.Fields = map[string]Value{"ordinal": Int(0)}
	if name == "None" {
		value.Fields["ordinal"] = Int(1)
	}
	return value
}

func stringLikeOrNull(value Value) bool {
	if value.Kind == ValueNull || value.Kind == ValueString {
		return true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		_, ok := platformScalarObjectText(value)
		return ok
	}
	return false
}

func stringValueOrEmpty(value Value) string {
	if value.Kind == ValueString {
		return value.Text
	}
	if text, ok := platformScalarObjectText(value); ok {
		return text
	}
	return ""
}

func (vm *VM) callAppLauncherControllerStatic(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !hasPrefixFold(className, "applauncher.") {
		return Null, false, nil
	}
	name := strings.ToLower(methodName)
	switch strings.ToLower(className) {
	case "applauncher.loginformcontroller":
		switch name {
		case "getforgotpasswordurl":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			return String("/ForgotPassword"), true, nil
		case "getisselfregistrationenabled", "getisusernamepasswordenabled":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			return Bool(true), true, nil
		case "getloginrightframeurl":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			return String(""), true, nil
		case "getselfregistrationurl":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			return String("/SelfRegister"), true, nil
		case "getusernamepasswordselfregenabled":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			out := typedMap("Map<String,Boolean>")
			out.Map[mapKey(String("usernamePasswordEnabled"))] = Bool(true)
			out.Map[mapKey(String("selfRegistrationEnabled"))] = Bool(true)
			return out, true, nil
		case "login", "logingetpagerefurl":
			if len(args) != 3 {
				return Null, true, fmt.Errorf("%s expects 3 arguments", callee)
			}
			return String(vm.appLauncherStartURL(args[2])), true, nil
		case "setexperienceid":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("%s expects 1 argument", callee)
			}
			return String(""), true, nil
		}
	case "applauncher.selfregistercontroller":
		switch name {
		case "getextrafields":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("%s expects 1 argument", callee)
			}
			return typedList("List<Map<String,Object>>"), true, nil
		case "isvalidpassword":
			if len(args) != 2 {
				return Null, true, fmt.Errorf("%s expects 2 arguments", callee)
			}
			password := ""
			confirm := ""
			if args[0].Kind == ValueString {
				password = args[0].Text
			}
			if args[1].Kind == ValueString {
				confirm = args[1].Text
			}
			return Bool(password != "" && password == confirm), true, nil
		case "selfregistergetredirecturl":
			if len(args) != 11 {
				return Null, true, fmt.Errorf("%s expects 11 arguments", callee)
			}
			return String(vm.appLauncherRegistrationRedirectURL(args[6], args[8])), true, nil
		case "commonselfregistergetredirecturl":
			if len(args) != 13 {
				return Null, true, fmt.Errorf("%s expects 13 arguments", callee)
			}
			return String(vm.appLauncherRegistrationRedirectURL(args[6], args[8])), true, nil
		case "setexperienceid":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("%s expects 1 argument", callee)
			}
			return String(""), true, nil
		case "selfregister":
			return Null, true, unsupportedCallError(callee + " local user registration service flow")
		}
	case "applauncher.sociallogincontroller":
		switch name {
		case "getauthproviders":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			return typedList("List<AuthProvider>"), true, nil
		case "getsamlproviders":
			if len(args) != 0 {
				return Null, true, fmt.Errorf("%s expects 0 arguments", callee)
			}
			return typedList("List<SamlSsoConfig>"), true, nil
		case "getcommunitydomainssourl", "getsamlssourl", "getsamlssourlnocache", "getssourl":
			if len(args) != 2 {
				return Null, true, fmt.Errorf("%s expects 2 arguments", callee)
			}
			return String(vm.appLauncherSsoURL(methodName, args[0], args[1])), true, nil
		case "handleidp":
			return Null, true, unsupportedCallError(callee + " local identity provider callback flow")
		}
	case "applauncher.forgotpasswordcontroller":
		switch name {
		case "setexperienceid":
			if len(args) != 1 {
				return Null, true, fmt.Errorf("%s expects 1 argument", callee)
			}
			return String(""), true, nil
		case "forgotpassword":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, true, fmt.Errorf("%s expects username and redirect URL Strings", callee)
			}
			return Null, true, unsupportedCallError(callee + " local password reset flow")
		}
	}
	return Null, false, nil
}

func (vm *VM) appLauncherStartURL(value Value) string {
	if value.Kind == ValueString && strings.TrimSpace(value.Text) != "" {
		return value.Text
	}
	return "/"
}

func (vm *VM) appLauncherRegistrationRedirectURL(confirmURL, startURL Value) string {
	if confirmURL.Kind == ValueString && strings.TrimSpace(confirmURL.Text) != "" {
		return confirmURL.Text
	}
	return vm.appLauncherStartURL(startURL)
}

func (vm *VM) appLauncherSsoURL(methodName string, startURL, provider Value) string {
	path := "sso"
	switch strings.ToLower(methodName) {
	case "getsamlssourl", "getsamlssourlnocache":
		path = "saml"
	}
	providerName := stringValueOrEmpty(provider)
	if providerName == "" {
		providerName = "local"
	}
	query := url.Values{}
	query.Set("startURL", vm.appLauncherStartURL(startURL))
	return strings.TrimRight(vm.salesforceBaseURL(), "/") + "/services/auth/" + path + "/" + url.PathEscape(providerName) + "?" + query.Encode()
}

func (vm *VM) callConnectAPITestFixtureStatic(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !hasTypePrefixFold(className, "ConnectApi") {
		return Null, false, nil
	}
	if connectAPITestSetterName(methodName) {
		method, ok := vm.generatedPlatformStaticMethodByNameArity(className, methodName, len(args))
		if !ok || !strings.EqualFold(method.ReturnType, "void") || len(args) == 0 {
			return Null, false, nil
		}
		if vm.testContext == nil {
			return Null, true, nil
		}
		if vm.testContext.ConnectAPIFixtures == nil {
			vm.testContext.ConnectAPIFixtures = make(map[string]Value)
		}
		targetMethod := connectAPITestSetterTarget(methodName)
		vm.testContext.ConnectAPIFixtures[connectAPITestFixtureKey(className, targetMethod, args[:len(args)-1])] = args[len(args)-1]
		return Null, true, nil
	}
	if vm.testContext == nil || len(vm.testContext.ConnectAPIFixtures) == 0 {
		return Null, false, nil
	}
	if _, ok := vm.generatedPlatformStaticMethodByNameArity(className, methodName, len(args)); !ok {
		return Null, false, nil
	}
	value, ok := vm.testContext.ConnectAPIFixtures[connectAPITestFixtureKey(className, methodName, args)]
	if !ok {
		return Null, false, nil
	}
	return value, true, nil
}

func (vm *VM) callConnectAPIReadOnlyStaticDefault(callee string, args []Value) (Value, bool) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !connectAPIReadOnlyHarnessType(className) {
		return Null, false
	}
	method, ok := vm.generatedPlatformStaticMethodByNameArity(className, methodName, len(args))
	if !ok || !connectAPIReadOnlyHarnessReturn(method.ReturnType) {
		return Null, false
	}
	if !connectAPIReadOnlyHarnessMethodAllowed(className, methodName) {
		return Null, false
	}
	return vm.generatedPlatformMethodDefaultReturn(method, Null, args), true
}

func (vm *VM) callConnectAPIPrimaryUsageStaticOutcome(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !connectAPIPrimaryUsageClass(className) {
		return Null, false, nil
	}
	if connectAPIPrimaryUsageAllowedMethod(className, methodName) {
		return Null, false, nil
	}
	return Null, true, newExceptionError("ConnectApi.ConnectApiException", callee+" is not supported in local tests")
}

func connectAPIPrimaryUsageClass(className string) bool {
	switch strings.ToLower(strings.TrimSpace(className)) {
	case "connectapi.namedcredentials", "connectapi.userprofiles":
		return true
	default:
		return false
	}
}

func connectAPIPrimaryUsageAllowedMethod(className, methodName string) bool {
	classLower := strings.ToLower(strings.TrimSpace(className))
	methodLower := strings.ToLower(strings.TrimSpace(methodName))
	switch classLower {
	case "connectapi.namedcredentials":
		return methodLower == "getnamedcredentials" ||
			methodLower == "createexternalcredential" ||
			methodLower == "createnamedcredential" ||
			methodLower == "getexternalcredential"
	case "connectapi.userprofiles":
		return methodLower == "getuserprofile" ||
			methodLower == "getphoto" ||
			methodLower == "setphoto" ||
			methodLower == "deletephoto"
	default:
		return false
	}
}

func connectAPIReadOnlyHarnessType(typeName string) bool {
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

func connectAPIReadOnlyHarnessMethodAllowed(typeName, methodName string) bool {
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
		return connectAPIReadOnlyHarnessMethod(methodName) ||
			name == "iscommenteditablebyme" ||
			name == "isfeedelementeditablebyme" ||
			name == "ismodified" ||
			connectAPIChatterSoftNoOpMethod(name)
	case "connectapi.chattergroups":
		return connectAPIReadOnlyHarnessMethod(methodName) ||
			name == "follow" ||
			name == "requestgroupmembership"
	case "connectapi.chattermessages":
		return connectAPIReadOnlyHarnessMethod(methodName) ||
			name == "markconversationread"
	case "connectapi.chatterusers":
		return connectAPIReadOnlyHarnessMethod(methodName) ||
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
		if connectAPIMutationMethod(name) {
			return false
		}
		return connectAPIReadOnlyHarnessMethod(methodName)
	}
}

func connectAPICommerceBuyerExperienceReadMethod(name string) bool {
	switch name {
	case "calculateadjustmentaggregates",
		"getbuyerprofile", "getcommerceaccountaddress", "geteffectiveaccountdetail",
		"getorderdeliverygroupsummaries", "getorderitemsummaries",
		"getorderitemsummaryadjustments", "getordershipmentitems",
		"getordershipments", "getordersummaries", "getordersummary",
		"getordersummaryadjustments", "getpurchasedproducts", "lookupordersummary":
		return true
	default:
		return false
	}
}

func connectAPICommerceCartReadMethod(name string) bool {
	switch name {
	case "calculatecart",
		"getcartcollection", "getcartcompactsummary", "getcartcoupons",
		"getcartitempromotion", "getcartitems", "getcartpromotions",
		"getcartsummary", "getchildcartitems", "getproductcartitem",
		"getproductcartitems":
		return true
	default:
		return false
	}
}

func connectAPIChatterSoftNoOpMethod(name string) bool {
	switch name {
	case "likecomment",
		"likefeedelement",
		"likefeeditem",
		"sharefeedelement",
		"sharefeeditem",
		"voteonfeedelementpoll",
		"voteonfeedpoll":
		return true
	default:
		return false
	}
}

func connectAPIReadOnlyHarnessMethod(methodName string) bool {
	name := strings.ToLower(methodName)
	return strings.HasPrefix(name, "get") ||
		strings.HasPrefix(name, "search") ||
		strings.HasPrefix(name, "find") ||
		strings.HasPrefix(name, "list") ||
		strings.HasPrefix(name, "query")
}

func connectAPIMutationMethod(methodName string) bool {
	name := strings.ToLower(methodName)
	if strings.Contains(name, "authurl") {
		return true
	}
	for _, prefix := range []string{
		"add", "assign", "ban", "block", "create", "delete", "edit", "follow",
		"join", "leave", "like", "mute", "pin", "post", "publish", "remove",
		"send", "set", "subscribe", "unfollow", "unpublish", "update",
	} {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

func connectAPIReadOnlyHarnessReturn(returnType string) bool {
	return returnType != "" && !strings.EqualFold(returnType, "void")
}

func (vm *VM) generatedPlatformStaticMethodByNameArity(className, methodName string, arity int) (Method, bool) {
	methodsByName := generatedPlatformMethodIndex[strings.ToLower(className)]
	if len(methodsByName) == 0 {
		return Method{}, false
	}
	for _, method := range methodsByName[strings.ToLower(methodName)] {
		if method.IsStatic && len(method.Params) == arity {
			return method, true
		}
	}
	return Method{}, false
}

func connectAPITestSetterName(methodName string) bool {
	return len(methodName) > len("setTest") && hasPrefixFold(methodName, "settest")
}

func connectAPITestSetterTarget(methodName string) string {
	target := methodName[len("setTest"):]
	if target == "" {
		return target
	}
	return strings.ToLower(target[:1]) + target[1:]
}

func connectAPITestFixtureKey(className, methodName string, args []Value) string {
	parts := []string{strings.ToLower(className), strings.ToLower(methodName)}
	for _, arg := range args {
		parts = append(parts, mapKeyWithSeen(arg, map[uint64]bool{}))
	}
	return strings.Join(parts, "\x00")
}

func (vm *VM) generatedOptionalWrapperStaticDefault(className, methodName string, args []Value) (Value, bool) {
	if !vm.generatedOptionalWrapperType(className) {
		return Null, false
	}
	switch strings.ToLower(methodName) {
	case "empty":
		if len(args) != 0 {
			return Null, false
		}
		return vm.newGeneratedOptionalWrapper(className, false, Null), true
	case "of":
		if len(args) != 1 {
			return Null, false
		}
		return vm.newGeneratedOptionalWrapper(className, true, args[0]), true
	default:
		return Null, false
	}
}

func (vm *VM) callCartExtensionCartTestUtilStaticDefault(methodName string, args []Value) (Value, bool) {
	switch strings.ToLower(methodName) {
	case "createcart":
		if len(args) != 0 && len(args) != 1 {
			return Null, false
		}
	case "getcart":
		if len(args) != 1 {
			return Null, false
		}
	default:
		return Null, false
	}
	cart := vm.generatedPlatformDefaultValue("CartExtension.Cart", Null)
	if cart.Kind != ValueObject {
		cart = Object("CartExtension.Cart")
	}
	if strings.EqualFold(methodName, "createCart") && len(args) == 1 {
		cart.Fields["webStoreType"] = args[0]
	}
	if strings.EqualFold(methodName, "getCart") {
		cart.Fields["id"] = args[0]
	}
	return cart, true
}

func (vm *VM) newGeneratedOptionalWrapper(typeName string, present bool, value Value) Value {
	if generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]; ok {
		object := vm.newGeneratedPlatformObject(generated)
		object.Fields["__optional_present"] = Bool(present)
		object.Fields["__optional_value"] = value
		object.Fields["isPresent"] = Bool(present)
		return object
	}
	object := Object(typeName)
	object.Fields["__optional_present"] = Bool(present)
	object.Fields["__optional_value"] = value
	object.Fields["isPresent"] = Bool(present)
	return object
}

func sfsqlquerySafeHarnessType(typeName string) bool {
	switch typeName {
	case "sfsqlquery.QueryHandle", "sfsqlquery.SqlStatement", "sfsqlquery.SqlRowIterator", "sfsqlquery.SqlQueueable":
		return true
	default:
		return false
	}
}

func (vm *VM) constructSfsqlqueryHarnessValue(generated generatedPlatformType, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	ctorArgs := args
	if len(generated.Constructors) != 0 {
		ctor, ok, ambiguous := vm.matchMethodByArgs(generated.Constructors, args)
		if !ok && len(namedArgs) != 0 {
			ctor, ctorArgs, ok, ambiguous = vm.matchGeneratedPlatformConstructorWithNamedArgs(generated, args, namedArgs)
		}
		if ambiguous {
			return Null, true, fmt.Errorf("ambiguous %s constructor with %d argument(s)", generated.Name, len(args))
		}
		if !ok {
			return Null, true, fmt.Errorf("%s constructor expects %s", generated.Name, generatedPlatformConstructorSummary(generated.Constructors))
		}
		object := vm.newGeneratedPlatformObject(generated)
		bindPassiveConstructorArgs(&object, ctor, ctorArgs)
		if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
			return Null, true, err
		}
		vm.initializeSfsqlqueryHarnessObject(&object, ctorArgs)
		return object, true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s constructor expects 0 arguments", generated.Name)
	}
	object := vm.newGeneratedPlatformObject(generated)
	if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
		return Null, true, err
	}
	vm.initializeSfsqlqueryHarnessObject(&object, nil)
	return object, true, nil
}

func cartExtensionMockBackedRuntimeType(typeName string) bool {
	switch typeName {
	case "CartExtension.AbstractCartCalculator", "CartExtension.CartCalculate", "CartExtension.InventoryCartCalculator",
		"CartExtension.PricingCartCalculator", "CartExtension.PromotionsCartCalculator", "CartExtension.ShippingCartCalculator",
		"CartExtension.TaxCartCalculator", "CartExtension.SplitShipmentService":
		return true
	default:
		return false
	}
}

func (vm *VM) constructCartExtensionMockBackedValue(generated generatedPlatformType, args []Value, namedArgs map[string]Value) (Value, bool, error) {
	ctorArgs := args
	if len(generated.Constructors) != 0 {
		ctor, ok, ambiguous := vm.matchMethodByArgs(generated.Constructors, args)
		if !ok && len(namedArgs) != 0 {
			ctor, ctorArgs, ok, ambiguous = vm.matchGeneratedPlatformConstructorWithNamedArgs(generated, args, namedArgs)
		}
		if ambiguous {
			return Null, true, fmt.Errorf("ambiguous %s constructor with %d argument(s)", generated.Name, len(args))
		}
		if !ok {
			return Null, true, fmt.Errorf("%s constructor expects %s", generated.Name, generatedPlatformConstructorSummary(generated.Constructors))
		}
		object := vm.newGeneratedPlatformObject(generated)
		bindPassiveConstructorArgs(&object, ctor, ctorArgs)
		if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
			return Null, true, err
		}
		return object, true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s constructor expects 0 arguments", generated.Name)
	}
	object := vm.newGeneratedPlatformObject(generated)
	if err := vm.bindGeneratedPlatformNamedFields(&object, namedArgs); err != nil {
		return Null, true, err
	}
	return object, true, nil
}

func (vm *VM) initializeSfsqlqueryHarnessObject(object *Value, args []Value) {
	if object == nil || object.Kind != ValueObject {
		return
	}
	switch object.Type {
	case "sfsqlquery.SqlRowIterator":
		rows := typedList("List<ConnectApi.QuerySqlRow>")
		rows.List = append(rows.List, cloneValueSlice(vm.sfsqlqueryRows)...)
		metadata := typedList("List<ConnectApi.QuerySqlMetadataItem>")
		metadata.List = append(metadata.List, cloneValueSlice(vm.sfsqlqueryMetadata)...)
		object.Fields["rows"] = rows
		object.Fields["metadata"] = metadata
		object.Fields["index"] = Int(0)
		if len(args) > 0 && args[0].Kind == ValueObject {
			if _, queryID, ok := objectFieldValue(args[0], "queryId"); ok {
				object.Fields["queryId"] = queryID
			}
		}
	case "sfsqlquery.SqlQueueable":
		if len(args) > 0 {
			object.Fields["input"] = args[0]
			object.Fields["queryId"] = sfsqlqueryQueryID(args[0])
		}
		object.Fields["rows"] = typedList("List<sfsqlquery.Row>")
		object.Fields["metadata"] = typedList("List<ConnectApi.QuerySqlMetadataItem>")
		object.Fields["columnNames"] = typedList("List<String>")
	}
}

func cloneValueSlice(values []Value) []Value {
	out := make([]Value, 0, len(values))
	for _, value := range values {
		out = append(out, cloneValue(value))
	}
	return out
}

func (vm *VM) newSfsqlqueryQueryHandle(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, fmt.Errorf("sfsqlquery.QueryHandle.create expects String queryId and String dataspace")
	}
	handle := Object("sfsqlquery.QueryHandle")
	handle.Fields["queryId"] = args[0]
	handle.Fields["dataspace"] = args[1]
	return handle, nil
}

func (vm *VM) newSfsqlquerySqlStatement(args []Value) (Value, error) {
	if len(args) != 2 || args[1].Kind != ValueString {
		return Null, fmt.Errorf("sfsqlquery.SqlStatement.create expects query input and String dataspace")
	}
	statement := Object("sfsqlquery.SqlStatement")
	if args[0].Kind == ValueString {
		statement.Fields["sql"] = args[0]
	} else {
		statement.Fields["query"] = args[0]
	}
	statement.Fields["dataspace"] = args[1]
	return statement, nil
}

func sfsqlqueryQueryID(input Value) Value {
	if input.Kind != ValueObject {
		return String("")
	}
	if _, queryID, ok := objectFieldValue(input, "queryId"); ok {
		return queryID
	}
	return String("")
}

func (vm *VM) callSfsqlqueryHarnessMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Type {
	case "sfsqlquery.QueryHandle":
		return callSfsqlqueryQueryHandleMember(receiver, method, args)
	case "sfsqlquery.SqlStatement":
		return callSfsqlquerySqlStatementMember(receiver, method, args)
	case "sfsqlquery.SqlRowIterator":
		return callSfsqlquerySqlRowIteratorMember(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callSfsqlqueryQueryHandleMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "withoffset":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.QueryHandle.withOffset expects Long")
		}
		receiver.Fields["offset"] = args[0]
		return receiver, receiver, true, true, nil
	case "withworkloadname":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.QueryHandle.withWorkloadName expects String")
		}
		receiver.Fields["workloadName"] = args[0]
		return receiver, receiver, true, true, nil
	case "tostring":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.QueryHandle.toString expects 0 arguments")
		}
		if _, queryID, ok := objectFieldValue(receiver, "queryId"); ok {
			return String(queryID.String()), receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSfsqlquerySqlStatementMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "withworkloadname":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlStatement.withWorkloadName expects String")
		}
		receiver.Fields["workloadName"] = args[0]
		return receiver, receiver, true, true, nil
	case "execute":
		return Null, receiver, false, true, unsupportedCallError("sfsqlquery.SqlStatement.execute local SQL service")
	case "tostring":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlStatement.toString expects 0 arguments")
		}
		if _, sql, ok := objectFieldValue(receiver, "sql"); ok {
			return String(sql.String()), receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSfsqlquerySqlRowIteratorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "cancel":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.cancel expects 0 arguments")
		}
		receiver.Fields["cancelled"] = Bool(true)
		return Null, receiver, true, true, nil
	case "getcolumnnames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.getColumnNames expects 0 arguments")
		}
		return sfsqlqueryColumnNames(receiver), receiver, false, true, nil
	case "getmetadata":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.getMetadata expects 0 arguments")
		}
		if _, metadata, ok := objectFieldValue(receiver, "metadata"); ok && metadata.Kind == ValueList {
			return cloneValue(metadata), receiver, false, true, nil
		}
		return typedList("List<ConnectApi.QuerySqlMetadataItem>"), receiver, false, true, nil
	case "getqueryid":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.getQueryId expects 0 arguments")
		}
		if _, queryID, ok := objectFieldValue(receiver, "queryId"); ok {
			return queryID, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "hasnext":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.hasNext expects 0 arguments")
		}
		return Bool(sfsqlqueryIteratorHasNext(receiver)), receiver, false, true, nil
	case "iterator":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.iterator expects 0 arguments")
		}
		return receiver, receiver, false, true, nil
	case "next":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.next expects 0 arguments")
		}
		if !sfsqlqueryIteratorHasNext(receiver) {
			return Null, receiver, false, true, unsupportedCallError("sfsqlquery.SqlRowIterator.next on exhausted mock rows")
		}
		_, rows, _ := objectFieldValue(receiver, "rows")
		_, indexValue, _ := objectFieldValue(receiver, "index")
		index := int(indexValue.Int)
		row := Object("sfsqlquery.Row")
		row.Fields["rawRow"] = cloneValue(rows.List[index])
		if _, metadata, ok := objectFieldValue(receiver, "metadata"); ok {
			row.Fields["metadata"] = cloneValue(metadata)
		}
		receiver.Fields["index"] = Int(int64(index + 1))
		return row, receiver, true, true, nil
	case "tostring":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlRowIterator.toString expects 0 arguments")
		}
		if _, queryID, ok := objectFieldValue(receiver, "queryId"); ok && queryID.Kind == ValueString {
			return String(queryID.Text), receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func sfsqlqueryIteratorHasNext(receiver Value) bool {
	if _, cancelled, ok := objectFieldValue(receiver, "cancelled"); ok && cancelled.Kind == ValueBool && cancelled.Bool {
		return false
	}
	_, rows, ok := objectFieldValue(receiver, "rows")
	if !ok || rows.Kind != ValueList {
		return false
	}
	_, index, ok := objectFieldValue(receiver, "index")
	if !ok || index.Kind != ValueInt {
		return len(rows.List) > 0
	}
	return int(index.Int) < len(rows.List)
}

func sfsqlqueryColumnNames(receiver Value) Value {
	out := typedList("List<String>")
	_, metadata, ok := objectFieldValue(receiver, "metadata")
	if !ok || metadata.Kind != ValueList {
		return out
	}
	for _, item := range metadata.List {
		if item.Kind != ValueObject {
			continue
		}
		if _, name, ok := objectFieldValue(item, "name"); ok {
			out.List = append(out.List, String(name.String()))
		}
	}
	return out
}

func (vm *VM) callGeneratedOptionalWrapperMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Kind != ValueObject || !vm.generatedOptionalWrapperType(receiver.Type) {
		return Null, false, nil
	}
	switch strings.ToLower(method) {
	case "ispresent":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.isPresent expects 0 arguments", receiver.Type)
		}
		present, ok := receiver.Fields["__optional_present"]
		return Bool(ok && present.Kind == ValueBool && present.Bool), true, nil
	case "get":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.get expects 0 arguments", receiver.Type)
		}
		present, ok := receiver.Fields["__optional_present"]
		if !ok || present.Kind != ValueBool || !present.Bool {
			return Null, true, unsupportedCallError(receiver.Type + ".get on empty optional wrapper")
		}
		if value, ok := receiver.Fields["__optional_value"]; ok {
			return value, true, nil
		}
		return Null, true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) generatedOptionalWrapperType(typeName string) bool {
	if !strings.HasPrefix(typeName, "CartExtension.Optional") || strings.EqualFold(typeName, "CartExtension.OptionalNotCheckedException") {
		return false
	}
	return true
}

func (vm *VM) generatedPlatformInstanceDefault(receiverName string, receiver Value, methodName string, args []Value) (Value, bool) {
	for _, receiverType := range vm.generatedPlatformReceiverTypes(receiverName, receiver) {
		if waveEnumLikeRuntimeType(receiverType) && strings.EqualFold(methodName, "ordinal") {
			if len(args) != 0 {
				return Null, false
			}
			return Int(0), true
		}
		if !vm.generatedPlatformMethodFallbackType(receiverType) && !strings.EqualFold(receiverType, "ApexPages.IdeaStandardSetController") {
			continue
		}
		method, ok := vm.generatedPlatformMethodForArgs(receiverType, methodName, args, false)
		if !ok {
			continue
		}
		if method.ClassName == "" {
			method.ClassName = slackTestHarnessRuntimeType(receiverType)
		}
		if value, handled := vm.generatedSlackTestHarnessDefaultReturn(method, receiver, args); handled {
			return value, true
		}
		if !vm.generatedPlatformMethodAllowsDefault(method) {
			continue
		}
		if strings.EqualFold(method.ClassName, "commercepayments.ClientSidePaymentAdapter") &&
			strings.EqualFold(apexMethodMemberName(method.Name), "processClientRequest") {
			continue
		}
		return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), true
	}
	return Null, false
}

func (vm *VM) generatedPlatformMethodFallbackType(typeName string) bool {
	if generatedPlatformTopLevelPassiveTypeName(typeName) || commerceLocalHarnessRuntimeType(typeName) || packagedControllerDefaultType(typeName) || vm.generatedOptionalWrapperType(typeName) {
		return true
	}
	return vm.isPassivePlatformDTOType(typeName)
}

func (vm *VM) generatedPlatformMethodAllowsDefault(method Method) bool {
	if packagedControllerDefaultMethod(method.ClassName, method.Name) {
		return true
	}
	if industryControllerDefaultStatic(method.ClassName, method.Name) || industryControllerDefaultInstance(method.ClassName, method.Name) {
		return true
	}
	if commerceLocalHarnessRuntimeMethod(method.ClassName, method.Name) {
		return true
	}
	if generatedPlatformTopLevelPassiveTypeName(method.ClassName) ||
		strings.EqualFold(method.ClassName, "ApexPages.IdeaStandardSetController") {
		return true
	}
	if slackGeneratedPlatformPassiveDTOTypeName(method.ClassName) {
		return slackGeneratedPlatformPassiveDTOMethod(method)
	}
	return vm.isPassivePlatformDTOType(method.ClassName)
}

func waveEnumLikeRuntimeType(typeName string) bool {
	return strings.EqualFold(typeName, "wave.NodeType") || strings.EqualFold(typeName, "wave.ProjectionType")
}

func commerceLocalHarnessRuntimeType(typeName string) bool {
	if strings.HasPrefix(typeName, "CommerceDxSampleapp.") {
		return true
	}
	switch typeName {
	case "WebStoreContext",
		"commerce_inventory.CommerceInventoryService",
		"commercepayments.ClientSidePaymentAdapter",
		"commerce_ordermanagement.ProductExpandService",
		"CartExtension.CheckoutPlaceOrder":
		return true
	default:
		return false
	}
}

func commerceLocalHarnessRuntimeMethod(className, methodName string) bool {
	name := strings.ToLower(methodName)
	if strings.HasPrefix(className, "CommerceDxSampleapp.") {
		return true
	}
	switch className {
	case "WebStoreContext":
		return name == "getcommercecontext"
	case "commerce_inventory.CommerceInventoryService":
		switch name {
		case "checkinventory", "getinventorylevel", "getreservation":
			return true
		default:
			return false
		}
	case "commercepayments.ClientSidePaymentAdapter":
		switch name {
		case "getclientcomponentname", "getclientconfiguration", "processclientrequest":
			return true
		default:
			return false
		}
	case "commerce_ordermanagement.ProductExpandService":
		return name == "returnreasons"
	case "CartExtension.CheckoutPlaceOrder":
		return name == "validate"
	default:
		return false
	}
}

func generatedPlatformTopLevelPassiveTypeName(typeName string) bool {
	switch {
	case strings.EqualFold(typeName, "Answers"),
		strings.EqualFold(typeName, "AppExchangeTrialTemplate"),
		strings.EqualFold(typeName, "AppExchangeUserPerms"),
		strings.EqualFold(typeName, "licensing.UserLicenseDefinition"),
		strings.EqualFold(typeName, "licensing.PlatformLicenseDefinition"):
		return true
	default:
		return false
	}
}

func packagedControllerDefaultType(typeName string) bool {
	switch typeName {
	case "SF_Archive.ArchiverAccessor",
		"ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi", "ime_mrm.EventManagementSubjectApi",
		"wavetemplate.Access", "wavetemplate.Answers", "wavetemplate.NetZeroBTE_Modifier",
		"wavetemplate.VcommBusinessChecklistRemoter", "wavetemplate.VcommBusinessConfigurationModifier",
		"wavetemplate.WaveTemplateConfigurationModifier", "wave.Dags", "wave.TrendedDatasetProcessor",
		"applauncher.AppLauncherSetupReordererController", "applauncher.ChangePasswordController",
		"applauncher.ForgotPasswordController", "setup_service_livemessage.MessagingChannelAppleDomainController",
		"setup_service_itsm_teams.EinsteinAgentFinalService", "regrelloapex.LoginFormController",
		"publicsectrsltn.GetAccountsAndContacts", "pref_center.PreferenceCenterApexHandler",
		"aiaccelerator.CustomFeatureExtractor",
		"aiaccelerator.SampleCustomFeatureExtractor", "sfdc_enablement.LearningItemEvaluationHandler",
		"sfdc_enablement.LearningItemSerializeDeserializer", "omnichannel.RouteWorkApexController",
		"mapslite.MapsLiteUtils", "mlplatform.PredictionServiceClient", "industries_clm.OpenInterface":
		return true
	default:
		return false
	}
}

func packagedControllerDefaultMethod(typeName, methodName string) bool {
	name := strings.ToLower(methodName)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	switch typeName {
	case "SF_Archive.ArchiverAccessor":
		switch {
		case strings.HasPrefix(name, "get"), strings.HasPrefix(name, "globalsearch"), strings.HasPrefix(name, "view"):
			return true
		case strings.HasPrefix(name, "performarchiverglobalsearch"):
			return true
		default:
			return false
		}
	case "ime_mrm.EventManagementBudgetApi", "ime_mrm.EventManagementManagedEventApi",
		"ime_mrm.EventManagementParticipantApi", "ime_mrm.EventManagementProductApi",
		"ime_mrm.EventManagementSubjectApi":
		return name == "call" || name == "invokemethod" || strings.HasPrefix(name, "get")
	case "wavetemplate.Access":
		return strings.HasPrefix(name, "check") || strings.HasPrefix(name, "get") || strings.Contains(name, "hasaccess")
	case "wavetemplate.Answers":
		return name == "get" || name == "put"
	case "wavetemplate.NetZeroBTE_Modifier", "wavetemplate.VcommBusinessChecklistRemoter",
		"wavetemplate.VcommBusinessConfigurationModifier", "wavetemplate.WaveTemplateConfigurationModifier":
		return true
	case "wave.Dags":
		return name == "getdags"
	case "wave.TrendedDatasetProcessor":
		return name == "getdescription" || name == "getlabel"
	case "applauncher.AppLauncherSetupReordererController":
		return name == "getmodel" || name == "saveorder"
	case "applauncher.ChangePasswordController":
		return name == "getpasswordpolicystatement"
	case "applauncher.ForgotPasswordController":
		return name == "setexperienceid"
	case "setup_service_livemessage.MessagingChannelAppleDomainController":
		return name == "getapplepaydomain"
	case "setup_service_itsm_teams.EinsteinAgentFinalService":
		return name == "einsteinsendmessage"
	case "regrelloapex.LoginFormController":
		return name == "getforgotpasswordurl" || name == "logingetpagerefurl"
	case "publicsectrsltn.GetAccountsAndContacts":
		return name == "invokemethod"
	case "pref_center.PreferenceCenterApexHandler":
		return name == "load" || name == "submit"
	case "aiaccelerator.CustomFeatureExtractor", "aiaccelerator.SampleCustomFeatureExtractor":
		return name == "extractfeatures"
	case "sfdc_enablement.LearningItemEvaluationHandler":
		return name == "evaluate"
	case "sfdc_enablement.LearningItemSerializeDeserializer":
		return name == "deserialize" || name == "serialize"
	case "omnichannel.RouteWorkApexController":
		return name == "isenabledskillbasedrouting" || name == "search"
	case "mapslite.MapsLiteUtils":
		return name == "accesscheck" || name == "userhasmaps"
	case "mlplatform.PredictionServiceClient":
		return name == "predictions"
	case "YubiAuthForAloha":
		return name == "validateyubikeylogin"
	case "industries_clm.OpenInterface":
		return name == "invokemethod"
	default:
		return false
	}
}

func (vm *VM) callPackagedControllerStatic(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok {
		return Null, false, nil
	}
	if packagedControllerUnsupportedMethod(className, methodName) {
		return Null, true, unsupportedCallError(callee)
	}
	if strings.EqualFold(className, "mapslite.MapsLiteUtils") && strings.EqualFold(methodName, "falconGeocodeRecords") {
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, true, fmt.Errorf("%s expects 1 String argument", callee)
		}
		return Null, true, unsupportedCallError(callee + " local maps geocode service flow")
	}
	if !packagedControllerDefaultMethod(className, methodName) {
		return Null, false, nil
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, true)
	if !ok {
		return Null, false, nil
	}
	if strings.EqualFold(vm.resolveTypeNameInClass(method.ClassName, method.ReturnType), "String") {
		return String(""), true, nil
	}
	return vm.generatedPlatformMethodDefaultReturn(method, Null, args), true, nil
}

func (vm *VM) callPackagedControllerMember(receiver Value, methodName string, args []Value) (Value, Value, bool, bool, error) {
	className := receiver.Type
	if packagedControllerUnsupportedMethod(className, methodName) {
		return Null, receiver, false, true, unsupportedCallError(className + "." + methodName)
	}
	if !packagedControllerDefaultMethod(className, methodName) {
		return Null, receiver, false, false, nil
	}
	name := strings.ToLower(methodName)
	if strings.EqualFold(className, "wavetemplate.Answers") {
		switch name {
		case "put":
			if len(args) != 2 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("wavetemplate.Answers.put expects String and Object")
			}
			if receiver.Fields == nil {
				receiver.Fields = map[string]Value{}
			}
			receiver.Fields["answer:"+strings.ToLower(args[0].Text)] = args[1]
			return Null, receiver, true, true, nil
		case "get":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("wavetemplate.Answers.get expects String")
			}
			if value, ok := receiver.Fields["answer:"+strings.ToLower(args[0].Text)]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	}
	if strings.EqualFold(className, "aiaccelerator.SampleCustomFeatureExtractor") && name == "extractfeatures" {
		return typedMap("Map<String,Object>"), receiver, false, true, nil
	}
	method, ok := vm.generatedPlatformMethodForArgs(className, methodName, args, false)
	if !ok {
		return Null, receiver, false, false, nil
	}
	if strings.EqualFold(vm.resolveTypeNameInClass(method.ClassName, method.ReturnType), "String") {
		return String(""), receiver, false, true, nil
	}
	if isMapType(method.ReturnType) || strings.EqualFold(method.Name, "invokeMethod") || strings.EqualFold(method.Name, "call") {
		return industryMapResult(), receiver, false, true, nil
	}
	return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), receiver, false, true, nil
}

func packagedControllerUnsupportedMethod(typeName, methodName string) bool {
	name := strings.ToLower(methodName)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	switch typeName {
	case "SF_Archive.ArchiverAccessor":
		return strings.HasPrefix(name, "forget") || strings.HasPrefix(name, "mask") || strings.HasPrefix(name, "performunarchive")
	case "applauncher.ChangePasswordController":
		return strings.HasPrefix(name, "changepass")
	case "setup_service_livemessage.MessagingChannelAppleDomainController":
		return name == "uploaddomainverificationcertificate"
	case "publicsectrsltn.AssessmentResponses":
		return name == "storeresponses"
	case "placequote.PlaceQuoteExecutor", "placequote.PlaceQuoteRLMApexProcessor",
		"RevSignaling.SignalingApexProcessor", "OrgMonitorFramework":
		return name == "execute" || name == "executeblacktabrequest"
	case "omnichannel.RouteWorkApexController":
		return name == "routework"
	case "embeddedMessaging.EmbeddedMessagingSessionHandler":
		return name == "handlerequestwithsfdcsession"
	default:
		return false
	}
}

func (vm *VM) generatedPlatformReceiverTypes(receiverName string, receiver Value) []string {
	candidates := []string{runtimeObjectType(receiver), receiver.Static, receiver.Type}
	if declaredType := vm.declaredReceiverType(receiverName); declaredType != "" {
		candidates = append(candidates, declaredType)
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		key := strings.ToLower(candidate)
		if candidate == "" || seen[key] {
			continue
		}
		out = append(out, candidate)
		seen[key] = true
		if alias := slackTestHarnessRuntimeType(candidate); alias != candidate && !seen[strings.ToLower(alias)] {
			out = append(out, alias)
			seen[strings.ToLower(alias)] = true
		}
	}
	return out
}

func (vm *VM) generatedPlatformMethodForArgs(className, methodName string, args []Value, static bool) (Method, bool) {
	methodsByName := generatedPlatformMethodIndex[strings.ToLower(className)]
	if len(methodsByName) == 0 {
		return Method{}, false
	}
	candidates := make([]Method, 0, len(methodsByName[strings.ToLower(methodName)]))
	for _, method := range methodsByName[strings.ToLower(methodName)] {
		if method.IsStatic != static {
			continue
		}
		candidates = append(candidates, method)
	}
	method, ok, ambiguous := vm.matchMethodByArgs(candidates, args)
	if ambiguous {
		return Method{}, false
	}
	if ok && method.ClassName == "" {
		method.ClassName = className
	}
	return method, ok
}

func (vm *VM) generatedPlatformMethodDefaultReturn(method Method, receiver Value, args []Value) Value {
	returnType := vm.resolveTypeNameInClass(method.ClassName, method.ReturnType)
	methodName := apexMethodMemberName(method.Name)
	if value, handled, err := vm.callGeneratedOptionalWrapperMember(receiver, apexMethodMemberName(method.Name), args); handled && err == nil {
		return value
	}
	if receiver.Kind == ValueObject {
		if suffix, ok := passiveAccessorSuffix(methodName, "get"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
				return value
			}
		}
		if suffix, ok := passiveAccessorSuffix(methodName, "is"); ok {
			if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
				return value
			}
		}
		if suffix, ok := passiveAccessorSuffix(methodName, "set"); ok && len(args) == 1 {
			receiver.Fields[passiveAccessorFieldName(receiver, suffix)] = args[0]
		}
	}
	switch strings.ToLower(returnType) {
	case "", "void":
		return Null
	case "boolean":
		return Bool(false)
	case "integer", "long":
		return Int(0)
	case "decimal", "double", "currency", "percent":
		return Decimal(0)
	}
	switch {
	case collectionBase(returnType) == "List":
		return typedList(returnType)
	case collectionBase(returnType) == "Set":
		value := Set()
		value.Type = returnType
		return value
	case isMapType(returnType):
		return typedMap(returnType)
	case receiver.Kind == ValueObject && strings.EqualFold(returnType, receiver.Type):
		bindGeneratedPlatformMethodArgs(&receiver, method, args)
		return receiver
	case receiver.Kind == ValueObject && vm.isPassivePlatformDTOType(returnType):
		value := vm.generatedPlatformDefaultValue(returnType, Null)
		if value.Kind == ValueObject {
			for field, fieldValue := range receiver.Fields {
				value.Fields[field] = fieldValue
			}
			bindGeneratedPlatformMethodArgs(&value, method, args)
		}
		return value
	case strings.EqualFold(method.ClassName, "CartExtension.CheckoutPlaceOrder") &&
		commerceLocalHarnessRuntimeMethod(method.ClassName, apexMethodMemberName(method.Name)):
		if generated, ok := generatedPlatformTypeIndex[strings.ToLower(returnType)]; ok &&
			generated.Kind == apexast.DeclarationClass {
			return vm.newGeneratedPlatformObject(generated)
		}
		return Null
	default:
		return vm.generatedPlatformDefaultValue(returnType, Null)
	}
}

func (vm *VM) generatedSlackTestHarnessDefaultReturn(method Method, receiver Value, args []Value) (Value, bool) {
	name := strings.ToLower(method.Name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	switch slackTestHarnessRuntimeType(method.ClassName) {
	case "Slack.State":
		if strings.HasPrefix(name, "clear") {
			return Null, true
		}
		if strings.HasPrefix(name, "create") {
			value := slackTestHarnessDefaultValue(method.ReturnType, vm.generatedPlatformDefaultValue(slackTestHarnessRuntimeType(method.ReturnType), Null))
			if value.Kind == ValueObject {
				bindGeneratedPlatformMethodArgs(&value, method, args)
				if name == "createusersession" {
					value.Fields["state"] = receiver
					if len(args) > 1 {
						value.Fields["openChannel"] = args[1]
					}
					value.Fields["messages"] = typedList("List<Slack.TestHarness.Message>")
					value.Fields["modalStack"] = typedList("List<Slack.TestHarness.Modal>")
				}
			}
			return value, true
		}
	case "Slack.UserSession":
		switch name {
		case "closeallmodals":
			if receiver.Kind == ValueObject {
				receiver.Fields["modalStack"] = typedList("List<Slack.TestHarness.Modal>")
			}
			return Null, true
		case "closemodal":
			if receiver.Kind == ValueObject {
				if stack, ok := receiver.Fields["modalStack"]; ok && stack.Kind == ValueList && len(stack.List) > 0 {
					stack.List = stack.List[:len(stack.List)-1]
					receiver.Fields["modalStack"] = stack
				}
			}
			return Null, true
		case "getapphome", "getopenchannel", "getstate", "gettopmodal", "getuser":
			if receiver.Kind == ValueObject {
				suffix := strings.TrimPrefix(name, "get")
				if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
					return value, true
				}
				if name == "gettopmodal" {
					if stack, ok := receiver.Fields["modalStack"]; ok && stack.Kind == ValueList && len(stack.List) > 0 {
						return stack.List[len(stack.List)-1], true
					}
					return Null, true
				}
			}
			return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), true
		case "getmessagecount":
			if receiver.Kind == ValueObject {
				if messages, ok := receiver.Fields["messages"]; ok && messages.Kind == ValueList {
					return Int(int64(len(messages.List))), true
				}
			}
			return Int(0), true
		case "getmessages", "getmodalstack":
			if receiver.Kind == ValueObject {
				suffix := strings.TrimPrefix(name, "get")
				if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
					return value, true
				}
			}
			return vm.generatedPlatformMethodDefaultReturn(method, receiver, args), true
		case "executeevent", "executeglobalshortcut", "executemessageshortcut", "executeslashcommand":
			return Null, true
		case "openapphome", "openchannel", "postmessage":
			value := slackTestHarnessDefaultValue(method.ReturnType, vm.generatedPlatformDefaultValue(slackTestHarnessRuntimeType(method.ReturnType), Null))
			if value.Kind == ValueObject {
				bindGeneratedPlatformMethodArgs(&value, method, args)
				if receiver.Kind == ValueObject {
					switch name {
					case "openapphome":
						receiver.Fields["appHome"] = value
					case "openchannel":
						receiver.Fields["openChannel"] = value
					case "postmessage":
						messages := typedList("List<Slack.TestHarness.Message>")
						if existing, ok := receiver.Fields["messages"]; ok && existing.Kind == ValueList {
							messages = existing
						}
						messages.List = append(messages.List, value)
						receiver.Fields["messages"] = messages
					}
				}
			}
			return value, true
		}
	}
	return Null, false
}

func slackTestHarnessDefaultValue(typeName string, value Value) Value {
	if strings.HasPrefix(typeName, "Slack.TestHarness.") {
		if value.Kind == ValueObject {
			value.Type = typeName
			return value
		}
		return Object(typeName)
	}
	return value
}

func slackTestHarnessRuntimeType(typeName string) string {
	if strings.HasPrefix(typeName, "Slack.TestHarness.") {
		return "Slack." + strings.TrimPrefix(typeName, "Slack.TestHarness.")
	}
	return typeName
}

func bindGeneratedPlatformMethodArgs(object *Value, method Method, args []Value) {
	if object == nil || object.Kind != ValueObject {
		return
	}
	for i, param := range method.Params {
		if i >= len(args) {
			return
		}
		field := strings.TrimSpace(param.Name)
		if field == "" || hasPrefixFold(field, "arg") {
			continue
		}
		object.Fields[passiveAccessorFieldName(*object, field)] = args[i]
	}
}

func passiveGeneratedSelfReturn(method Method, receiver Value, value Value) bool {
	return methodHasModifier(method.Modifiers, "passive-generated") &&
		receiver.Kind == ValueObject &&
		value.Kind == ValueObject &&
		strings.EqualFold(receiver.Type, value.Type)
}
