package vm

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
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
	if !ok || !strings.HasPrefix(strings.ToLower(className), "applauncher.") {
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
	return len(methodName) > len("setTest") && strings.HasPrefix(strings.ToLower(methodName), "settest")
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
		if field == "" || strings.HasPrefix(strings.ToLower(field), "arg") {
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

func (vm *VM) displayString(value Value, result *Result) (string, error) {
	if idText, ok := typedIDValueText(value); ok {
		return displayIDText(idText), nil
	}
	switch value.Kind {
	case ValueList:
		return vm.displayList(value.List, result)
	case ValueSet:
		return vm.displaySet(value.Set, result)
	case ValueMap:
		return value.String(), nil
	case ValueObject:
	default:
		return value.String(), nil
	}
	if value.Type == "Type" {
		if text := typeValueText(value); text != "" {
			return text, nil
		}
	}
	if strings.EqualFold(value.Type, "UUID") {
		if text, err := platformScalarText(value, "UUID"); err == nil {
			return text, nil
		}
	}
	if strings.EqualFold(value.Type, "String") {
		if text, err := platformScalarText(value, "String"); err == nil {
			return text, nil
		}
	}
	if strings.EqualFold(value.Type, "Date") {
		if text, err := platformScalarText(value, "Date"); err == nil {
			return text + " 00:00:00", nil
		}
	}
	if strings.EqualFold(value.Type, "Schema.SObjectField") {
		if fieldName, ok := value.Fields["field"]; ok && fieldName.Kind == ValueString {
			return fieldName.Text, nil
		}
	}
	if value.Type == "LoggingLevel" && isLoggingLevelName(value.Text) {
		return value.Text, nil
	}
	if value.Type == "RoundingMode" && isDecimalRoundingModeName(value.Text) {
		return value.Text, nil
	}
	if value.Type == "StatusCode" && value.Text != "" {
		return value.Text, nil
	}
	if class, ok := vm.Classes[value.Type]; ok && len(class.EnumValues) > 0 && value.Text != "" {
		return value.Text, nil
	}
	if value.Text != "" && strings.Contains(value.Type, ".") {
		return value.Text, nil
	}
	if isExceptionType(value.Type) {
		return exceptionToString(value), nil
	}
	target, ok, ambiguous := vm.resolveInstanceMethodForArgs(value.Type, "toString", nil)
	if ambiguous {
		return "", vm.ambiguousOverloadError("toString", nil)
	}
	if !ok {
		return vm.defaultObjectDisplayString(value), nil
	}
	out, err := vm.callMethodWithReceiver(target, value, nil, result)
	if err != nil {
		return "", err
	}
	if out.Kind != ValueString {
		return "", fmt.Errorf("%s returned %s, want String", target.Name, out.Kind)
	}
	return out.Text, nil
}

func (vm *VM) defaultObjectDisplayString(value Value) string {
	text := value.String()
	if value.Kind != ValueObject || !strings.Contains(value.Type, ".") || vm == nil {
		return text
	}
	if _, ok := vm.Classes[value.Type]; !ok {
		return text
	}
	shortName := value.Type[strings.LastIndex(value.Type, ".")+1:]
	return shortName + strings.TrimPrefix(text, value.Type)
}

func (vm *VM) displayList(values []Value, result *Result) (string, error) {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		text, err := vm.displayString(item, result)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return "(" + strings.Join(parts, ", ") + ")", nil
}

func (vm *VM) displaySet(values []Value, result *Result) (string, error) {
	parts := make([]string, 0, len(values))
	for _, item := range values {
		text, err := vm.displayString(item, result)
		if err != nil {
			return "", err
		}
		parts = append(parts, text)
	}
	return "{" + strings.Join(parts, ", ") + "}", nil
}

const (
	sobjectErrorsField                       = "__glade_errors"
	sobjectReadOnlyField                     = "__glade_readonly"
	sobjectQueriedFieldsField                = "__glade_queried_fields"
	sobjectExplicitFieldsField               = "__glade_explicit_fields"
	sobjectSetFieldsField                    = "__glade_set_fields"
	sobjectUserSetFieldsField                = "__glade_user_set_fields"
	sobjectDMLAccessibleField                = "__glade_dml_accessible"
	sobjectTriggerField                      = "__glade_trigger_record"
	sobjectParentProjectionField             = "__glade_parent_projection"
	sobjectPopulatedFieldsAliasContainsField = "__glade_populated_fields_alias_contains"
)

func isInternalSObjectField(field string) bool {
	return field == sobjectErrorsField || field == sobjectReadOnlyField || field == sobjectQueriedFieldsField || field == sobjectExplicitFieldsField || field == sobjectSetFieldsField || field == sobjectUserSetFieldsField || field == sobjectDMLAccessibleField || field == sobjectTriggerField || field == sobjectParentProjectionField
}

func vmImplicitDMLField(field storage.Field) bool {
	if vmCalculatedOrSummaryField(field) {
		return true
	}
	return !storage.FieldFlagValue(field.Createable, true) || !storage.FieldFlagValue(field.Updateable, true)
}

func vmImplicitDMLFieldDefaultValue(field storage.Field, value Value) bool {
	if vmCalculatedOrSummaryField(field) {
		return false
	}
	switch value.Kind {
	case ValueNull:
		return true
	case ValueBool:
		return !value.Bool
	case ValueInt:
		return value.Int == 0
	case ValueDecimal:
		return value.Decimal == 0
	case ValueString:
		return value.Text == ""
	default:
		return false
	}
}

func vmCalculatedOrSummaryField(field storage.Field) bool {
	return field.Type == storage.FieldCalculated || field.Type == storage.FieldSummary || strings.TrimSpace(field.Formula) != ""
}

func vmImplicitGeneratedFieldValue(field string, value Value) bool {
	if !strings.EqualFold(field, "RecordTypeId") {
		return false
	}
	if value.Kind == ValueString {
		return strings.HasPrefix(value.Text, "012")
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "Id") {
		id, ok := sObjectIDFromValue(value)
		return ok && strings.HasPrefix(string(id), "012")
	}
	return false
}

func markExplicitSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || strings.TrimSpace(field) == "" {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		selected = Map()
		selected.Type = "Map<String,Boolean>"
	}
	keyValue := String(strings.ToLower(field))
	encoded := mapKey(keyValue)
	selected.Map[encoded] = Bool(true)
	if selected.MapKeys == nil {
		selected.MapKeys = make(map[string]Value)
	}
	selected.MapKeys[encoded] = keyValue
	filteredOrder := selected.MapOrder[:0]
	for _, key := range selected.MapOrder {
		if key == encoded {
			continue
		}
		filteredOrder = append(filteredOrder, key)
	}
	selected.MapOrder = append([]string{encoded}, filteredOrder...)
	value.Fields[sobjectExplicitFieldsField] = selected
}

func isExplicitSObjectField(value Value, field string) bool {
	if value.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func explicitSObjectFieldNames(value Value) []string {
	if value.Fields == nil {
		return nil
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		return nil
	}
	fields := make([]string, 0, len(selected.Map))
	for _, key := range orderedValueMapKeys(selected) {
		flag := selected.Map[key]
		if flag.Kind != ValueBool || !flag.Bool {
			continue
		}
		if strings.HasPrefix(key, "string:") {
			fields = append(fields, strings.TrimPrefix(key, "string:"))
		}
	}
	return fields
}

func unmarkExplicitSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	selected, ok := value.Fields[sobjectExplicitFieldsField]
	if !ok || selected.Kind != ValueMap {
		return
	}
	delete(selected.Map, mapKey(String(strings.ToLower(field))))
	if len(selected.Map) == 0 {
		delete(value.Fields, sobjectExplicitFieldsField)
		return
	}
	value.Fields[sobjectExplicitFieldsField] = selected
}

func markSetSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || strings.TrimSpace(field) == "" {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		selected = Map()
		selected.Type = "Map<String,Boolean>"
	}
	keyValue := String(strings.ToLower(field))
	encoded := mapKey(keyValue)
	selected.Map[encoded] = Bool(true)
	if selected.MapKeys == nil {
		selected.MapKeys = make(map[string]Value)
	}
	selected.MapKeys[encoded] = keyValue
	value.Fields[sobjectSetFieldsField] = selected
}

func markUserSetSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || strings.TrimSpace(field) == "" {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	selected, ok := value.Fields[sobjectUserSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		selected = Map()
		selected.Type = "Map<String,Boolean>"
	}
	keyValue := String(strings.ToLower(field))
	encoded := mapKey(keyValue)
	selected.Map[encoded] = Bool(true)
	if selected.MapKeys == nil {
		selected.MapKeys = make(map[string]Value)
	}
	selected.MapKeys[encoded] = keyValue
	value.Fields[sobjectUserSetFieldsField] = selected
}

func isUserSetSObjectField(value Value, field string) bool {
	if value.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	selected, ok := value.Fields[sobjectUserSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func isSetSObjectField(value Value, field string) bool {
	if value.Fields == nil || strings.TrimSpace(field) == "" {
		return false
	}
	selected, ok := value.Fields[sobjectSetFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	flag, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok && flag.Kind == ValueBool && flag.Bool
}

func setExplicitSObjectField(value *Value, field string, fieldValue Value) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields[field] = fieldValue
	markExplicitSObjectField(value, field)
}

func markTriggerSObject(value *Value) {
	if value == nil || value.Kind != ValueObject {
		return
	}
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	value.Fields[sobjectTriggerField] = Bool(true)
}

func isTriggerSObject(value Value) bool {
	if value.Kind != ValueObject || value.Fields == nil {
		return false
	}
	marker, ok := value.Fields[sobjectTriggerField]
	return ok && marker.Kind == ValueBool && marker.Bool
}

func queriedSObjectFieldsValue(objectName string, fields map[string]bool) Value {
	value := Map()
	value.Type = "Map<String,Boolean>"
	value.Map[mapKey(String("object"))] = String(objectName)
	value.MapKeys[mapKey(String("object"))] = String("object")
	for field := range fields {
		value.Map[mapKey(String(field))] = Bool(true)
		value.MapKeys[mapKey(String(field))] = String(field)
	}
	return value
}

func markQueriedSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	selected, ok := value.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return
	}
	selected.Map[mapKey(String(strings.ToLower(field)))] = Bool(true)
	value.Fields[sobjectQueriedFieldsField] = selected
}

func unmarkQueriedSObjectField(value *Value, field string) {
	if value == nil || value.Kind != ValueObject || value.Fields == nil || strings.TrimSpace(field) == "" {
		return
	}
	selected, ok := value.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return
	}
	delete(selected.Map, mapKey(String(strings.ToLower(field))))
	value.Fields[sobjectQueriedFieldsField] = selected
}

func dmlVisibleSObjectFields(value *Value) map[string]bool {
	fields := map[string]bool{"id": true, "lastmodifiedbyid": true}
	if value == nil || value.Kind != ValueObject {
		return fields
	}
	for field := range value.Fields {
		if isInternalSObjectField(field) || isSObjectSystemField(field) {
			continue
		}
		fields[strings.ToLower(field)] = true
	}
	return fields
}

func (vm *VM) unqueriedSObjectFieldError(receiver Value, field string, enforceDML bool) error {
	if receiver.Kind != ValueObject {
		return nil
	}
	if marker, ok := receiver.Fields[sobjectDMLAccessibleField]; ok && marker.Kind == ValueBool && marker.Bool {
		if _, _, exists := objectFieldValue(receiver, field); !exists {
			return nil
		}
	}
	selected, ok := receiver.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return nil
	}
	if vm.queriedSObjectFieldsIncludes(receiver, field) {
		return nil
	}
	if isExplicitSObjectField(receiver, field) {
		return nil
	}
	if vm.loadedChildRelationshipForField(receiver, field) {
		return nil
	}
	if vm.loadedParentRelationshipForField(receiver, field) {
		return nil
	}
	if vm.unqueriedLookupFieldCanDefaultNull(receiver, field) {
		return nil
	}
	if vm.unqueriedStoredMissingFieldCanDefaultNull(receiver, field) {
		return nil
	}
	if _, ok := vm.unqueriedStoredDefaultFieldValue(receiver, field); ok {
		return nil
	}
	if vm.unqueriedParentRelationshipCanDefaultNull(receiver, field) {
		return nil
	}
	if vm.unqueriedChildRelationshipOnParentProjectionCanDefaultEmpty(receiver, field) {
		return nil
	}
	if !enforceDML {
		if _, ok := vm.lazyChildRelationshipValue(receiver, field); ok {
			return nil
		}
	}
	return newExceptionError("SObjectException", fmt.Sprintf("SObject row was retrieved via SOQL without querying the requested field: %s.%s", receiver.Type, field))
}

func (vm *VM) unqueriedStoredDefaultFieldValue(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || receiver.Fields == nil {
		return Null, false
	}
	objectName := receiver.Type
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		objectName = resolved
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	canonical := field
	if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
		canonical = resolved
	}
	fieldDef, ok := object.Definition.Fields[canonical]
	if !ok {
		return Null, false
	}
	if strings.TrimSpace(fieldDef.DefaultValue) == "" {
		return Null, false
	}
	record := storage.Record{Object: objectName, Fields: map[string]storage.Value{}}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return Null, false
	}
	stored, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return Null, false
	}
	record = stored
	defaultValue, ok := vm.defaultValueForRecordField(object.Definition, record, fieldDef)
	if !ok || defaultValue.Kind == storage.ValueNull {
		return Null, false
	}
	if record.HasExplicitNull(canonical) {
		return Null, false
	}
	if current, ok := record.GetField(canonical); ok && !storageValuesEqualForVM(fieldDef, current, defaultValue) {
		return Null, false
	}
	if _, fieldValue, exists := objectFieldValue(receiver, field); exists {
		current, err := storageValueFromVMForField(fieldValue, fieldDef.Type)
		if err != nil || !storageValuesEqualForVM(fieldDef, current, defaultValue) {
			return Null, false
		}
	}
	return vmValueFromStorage(defaultValue), true
}

func (vm *VM) unqueriedStoredMissingFieldCanDefaultNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || receiver.Fields == nil {
		return false
	}
	objectName := receiver.Type
	if resolved, ok := vm.resolveObjectName(objectName); ok {
		objectName = resolved
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	canonical := field
	if resolved, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
		canonical = resolved
	}
	if _, ok := object.Definition.Fields[canonical]; !ok {
		return false
	}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return false
	}
	stored, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return false
	}
	if stored.HasExplicitNull(canonical) {
		return true
	}
	_, hasStored := stored.GetField(canonical)
	return !hasStored
}

func (vm *VM) unqueriedChildRelationshipOnParentProjectionCanDefaultEmpty(receiver Value, field string) bool {
	if receiver.Kind != ValueObject || receiver.Fields == nil {
		return false
	}
	marker, ok := receiver.Fields[sobjectParentProjectionField]
	if !ok || marker.Kind != ValueBool || !marker.Bool {
		return false
	}
	_, ok = vm.jsonSObjectChildRelationshipType(receiver.Type, field)
	return ok
}

func (vm *VM) lazyChildRelationshipValue(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || receiver.Fields == nil || strings.TrimSpace(receiver.Type) == "" || strings.TrimSpace(field) == "" {
		return Null, false
	}
	parentObject, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		parentObject = receiver.Type
	}
	parentID := sObjectIDFromFields(receiver.Fields)
	childType := ""
	children := []Value(nil)
	seen := map[storage.ID]bool{}
	for childName, childState := range vm.Org.Objects {
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) || strings.TrimSpace(relation.Field) == "" {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if childRelationshipName == "" || !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
				continue
			}
			if childType == "" {
				childType = childName
			}
			children = vm.appendLazyChildRelationshipRecords(children, seen, childName, childState, relation.Field, parentID)
		}
		for fieldName, fieldDef := range childState.Definition.Fields {
			if fieldDef.Type != storage.FieldReference || len(fieldDef.ReferenceTo) == 0 {
				continue
			}
			if !relationshipTargetsObject(storage.Relationship{ParentObjects: append([]string(nil), fieldDef.ReferenceTo...)}, parentObject) {
				continue
			}
			if fieldDef.APIName == "" {
				fieldDef.APIName = fieldName
			}
			for _, childRelationshipName := range vmFieldChildRelationshipNames(childState.Definition, fieldDef) {
				if !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
					continue
				}
				if childType == "" {
					childType = childName
				}
				children = vm.appendLazyChildRelationshipRecords(children, seen, childName, childState, fieldDef.APIName, parentID)
			}
		}
	}
	if childType == "" {
		return Null, false
	}
	list := List(children...)
	list.Type = "List<" + childType + ">"
	return list, true
}

func (vm *VM) appendLazyChildRelationshipRecords(out []Value, seen map[storage.ID]bool, childName string, childState storage.ObjectState, lookupField string, parentID storage.ID) []Value {
	if parentID == "" {
		return out
	}
	ids := make([]string, 0, len(childState.Records))
	if indexed, ok := storage.LookupIndex(childState, lookupField, storage.IDValue(parentID)); ok && len(indexed) > 0 {
		for _, id := range indexed {
			ids = append(ids, string(id))
		}
	} else {
		for id, record := range childState.Records {
			if record.System.IsDeleted {
				continue
			}
			value, ok := record.GetField(lookupField)
			if !ok || !storage.IDsEqual(storageIDFromValue(value), parentID) {
				continue
			}
			ids = append(ids, string(id))
		}
	}
	sort.Strings(ids)
	for _, idText := range ids {
		id := storage.ID(idText)
		if seen[id] {
			continue
		}
		record, ok := childState.Records[id]
		if !ok || record.System.IsDeleted {
			continue
		}
		record.Object = childName
		out = append(out, vm.vmValueFromRecord(record))
		seen[id] = true
	}
	return out
}

func (vm *VM) loadedChildRelationshipForField(receiver Value, field string) bool {
	_, _, ok := vm.loadedChildRelationshipValue(receiver, field)
	return ok
}

func (vm *VM) loadedChildRelationshipValue(receiver Value, field string) (string, Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" || strings.TrimSpace(field) == "" {
		return "", Null, false
	}
	parentObject, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		parentObject = receiver.Type
	}
	lookup := vm.loadedChildRelationshipLookup(receiver.Type, parentObject, field)
	if len(lookup.ChildRelationshipNames) == 0 {
		return "", Null, false
	}
	parentRelationshipExists := lookup.ParentRelationshipExists
	for _, candidate := range lookup.CandidateNames {
		if actualName, value, ok := objectFieldValue(receiver, candidate); ok && loadedChildRelationshipRuntimeValue(value) {
			if parentRelationshipExists && value.Kind == ValueNull {
				continue
			}
			return actualName, value, true
		}
	}
	for actualName, value := range receiver.Fields {
		if !loadedChildRelationshipRuntimeValue(value) {
			continue
		}
		for _, childRelationshipName := range lookup.ChildRelationshipNames {
			if vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, actualName) {
				if parentRelationshipExists && value.Kind == ValueNull {
					continue
				}
				return actualName, value, true
			}
		}
	}
	return "", Null, false
}

func (vm *VM) loadedChildRelationshipLookup(receiverType, parentObject, field string) loadedChildRelationshipLookup {
	key := strings.ToLower(strings.TrimSpace(receiverType)) + "\x00" + strings.ToLower(strings.TrimSpace(parentObject)) + "\x00" + strings.ToLower(strings.TrimSpace(field))
	if vm.loadedChildRelCache == nil {
		vm.loadedChildRelCache = make(map[string]loadedChildRelationshipLookup)
	}
	if cached, ok := vm.loadedChildRelCache[key]; ok {
		return cached
	}
	lookup := loadedChildRelationshipLookup{
		ParentRelationshipExists: vm.sObjectParentRelationshipField(receiverType, field),
	}
	for _, childState := range vm.Org.Objects {
		for _, relation := range childState.Definition.Relations {
			if !relationshipTargetsObject(relation, parentObject) {
				continue
			}
			childRelationshipName := relation.ChildRelationship
			if childRelationshipName == "" && canDeriveChildRelationshipName(relation) {
				childRelationshipName = derivedVMChildRelationshipName(childState.Definition)
			}
			if childRelationshipName == "" || !vmRelationshipNameMatches(vm.Org.Namespace, childRelationshipName, field) {
				continue
			}
			lookup.ChildRelationshipNames = appendUniqueStringFold(lookup.ChildRelationshipNames, childRelationshipName)
			if vm.Org.Namespace != "" {
				lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, storage.NamespaceTokenName(vm.Org.Namespace, childRelationshipName))
				lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, storage.NamespaceTokenName(vm.Org.Namespace, field))
			}
			lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, childRelationshipName)
			lookup.CandidateNames = appendUniqueStringFold(lookup.CandidateNames, field)
		}
	}
	vm.loadedChildRelCache[key] = lookup
	return lookup
}

func loadedChildRelationshipRuntimeValue(value Value) bool {
	return value.Kind == ValueList || value.Kind == ValueNull
}

func (vm *VM) unqueriedParentRelationshipCanDefaultNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	for name, fieldDef := range object.Definition.Fields {
		apiName := fieldDef.APIName
		if apiName == "" {
			apiName = name
		}
		if fieldDef.Type == storage.FieldReference && vmParentRelationshipNameMatches(vm.Org.Namespace, apiName, field) {
			return true
		}
	}
	return false
}

func (vm *VM) unqueriedLookupFieldCanDefaultNull(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	fieldName := vm.resolveSObjectFieldName(receiver.Type, field)
	fieldDef, ok := object.Definition.Fields[fieldName]
	return ok && fieldDef.Type == storage.FieldReference
}

func (vm *VM) loadedParentRelationshipForField(receiver Value, field string) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || strings.TrimSpace(receiver.Type) == "" {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	relationship, ok := vm.parentRelationshipNameForField(object.Definition, field)
	if !ok {
		if strings.HasSuffix(field, "__c") {
			relationship = strings.TrimSuffix(field, "__c") + "__r"
		} else if strings.HasSuffix(field, "Id") && len(field) > len("Id") {
			relationship = strings.TrimSuffix(field, "Id")
		}
	}
	if relationship == "" {
		return false
	}
	_, value, exists := objectFieldValue(receiver, relationship)
	return exists && value.Kind != ValueNull
}

func sobjectReadOnlyReason(value Value) (string, bool) {
	if value.Kind != ValueObject {
		return "", false
	}
	reason, ok := value.Fields[sobjectReadOnlyField]
	if !ok || reason.Kind != ValueString || reason.Text == "" {
		return "", false
	}
	return reason.Text, true
}

func addSObjectError(value *Value, message string, fields []string) {
	if value.Fields == nil {
		value.Fields = make(map[string]Value)
	}
	errorValue := Object("Database.Error")
	errorValue.Fields["message"] = String(message)
	errorValue.Fields["statusCode"] = String("FIELD_CUSTOM_VALIDATION_EXCEPTION")
	fieldsList := List()
	for _, field := range fields {
		fieldsList.List = append(fieldsList.List, String(field))
	}
	errorValue.Fields["fields"] = fieldsList
	errorsList, ok := value.Fields[sobjectErrorsField]
	if !ok || errorsList.Kind != ValueList {
		errorsList = List()
	}
	errorsList.List = append(errorsList.List, errorValue)
	value.Fields[sobjectErrorsField] = errorsList
}

func sobjectErrors(value Value) []Value {
	errorsList, ok := value.Fields[sobjectErrorsField]
	if !ok || errorsList.Kind != ValueList {
		return nil
	}
	return append([]Value(nil), errorsList.List...)
}

func dmlResultsFromSObjectErrors(records []storage.Record, values []Value) []dml.Result {
	results := make([]dml.Result, len(records))
	for i, value := range values {
		if i >= len(results) {
			break
		}
		errors := sobjectErrors(value)
		if len(errors) == 0 {
			continue
		}
		dmlErrors := make([]dml.Error, 0, len(errors))
		messages := make([]string, 0, len(errors))
		aggregateFields := make([]string, 0, len(errors))
		for _, errValue := range errors {
			dmlError := dml.Error{
				Message:    "record blocked by addError",
				StatusCode: "FIELD_CUSTOM_VALIDATION_EXCEPTION",
			}
			if errValue.Kind == ValueObject {
				if value, ok := errValue.Fields["message"]; ok {
					dmlError.Message = value.String()
				}
				if value, ok := errValue.Fields["statusCode"]; ok {
					dmlError.StatusCode = value.String()
				}
				if value, ok := errValue.Fields["fields"]; ok && value.Kind == ValueList {
					for _, field := range value.List {
						dmlError.Fields = append(dmlError.Fields, field.String())
					}
				}
			}
			messages = append(messages, dmlError.Message)
			aggregateFields = append(aggregateFields, dmlError.Fields...)
			dmlErrors = append(dmlErrors, dmlError)
		}
		results[i] = dml.Result{
			ID:         records[i].ID,
			Success:    false,
			Error:      strings.Join(messages, "; "),
			StatusCode: dmlErrors[0].StatusCode,
			Fields:     aggregateFields,
			Errors:     dmlErrors,
		}
	}
	return results
}

func databaseErrorsList(result dml.Result) Value {
	errors := dmlResultErrors(result)
	values := make([]Value, 0, len(errors))
	for _, err := range errors {
		values = append(values, databaseErrorValue(err))
	}
	return List(values...)
}

func dmlResultErrors(result dml.Result) []dml.Error {
	if len(result.Errors) > 0 {
		out := make([]dml.Error, len(result.Errors))
		copy(out, result.Errors)
		return out
	}
	if result.Error == "" {
		return nil
	}
	code := result.StatusCode
	if code == "" {
		code = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	}
	return []dml.Error{{
		Message:    result.Error,
		StatusCode: code,
		Fields:     append([]string(nil), result.Fields...),
	}}
}

func databaseErrorValue(err dml.Error) Value {
	value := Object("Database.Error")
	value.Fields["message"] = String(err.Message)
	code := err.StatusCode
	if code == "" {
		code = "FIELD_CUSTOM_VALIDATION_EXCEPTION"
	}
	value.Fields["statusCode"] = String(code)
	fields := List()
	for _, field := range err.Fields {
		fields.List = append(fields.List, String(field))
	}
	value.Fields["fields"] = fields
	value.Fields["extendedErrorDetails"] = List()
	return value
}

func dmlExceptionDetail(receiver Value, method string, args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueInt {
		return Null, fmt.Errorf("%s.%s expects Integer index", receiver.Type, method)
	}
	details, ok := receiver.Fields["__dmlErrors"]
	if !ok || details.Kind != ValueList {
		return Null, fmt.Errorf("%s.%s index out of bounds: %d", receiver.Type, method, args[0].Int)
	}
	index := int(args[0].Int)
	if index < 0 || index >= len(details.List) {
		return Null, fmt.Errorf("%s.%s index out of bounds: %d", receiver.Type, method, args[0].Int)
	}
	detail := details.List[index]
	if detail.Kind != ValueObject {
		return Null, fmt.Errorf("%s.%s detail is not available: %d", receiver.Type, method, args[0].Int)
	}
	return detail, nil
}

func (vm *VM) resolveObjectName(name string) (string, bool) {
	if vm == nil || vm.Org == nil {
		return "", false
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		return "", false
	}
	if vm.objectNameCache == nil {
		vm.objectNameCache = make(map[string]objectNameLookup)
	}
	if cached, ok := vm.objectNameCache[key]; ok {
		return cached.Name, cached.OK
	}
	resolved, ok := storage.ResolveObjectName(*vm.Org, name)
	vm.objectNameCache[key] = objectNameLookup{Name: resolved, OK: ok}
	return resolved, ok
}

func (vm *VM) resolveSObjectFieldName(typeName, field string) string {
	if vm.Org == nil {
		return field
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return storage.StripNamespaceToken(vm.Org.Namespace, field)
	}
	if dot := strings.LastIndex(field, "."); dot >= 0 && dot+1 < len(field) {
		prefix := field[:dot]
		if resolvedPrefix, prefixOK := vm.resolveObjectName(prefix); prefixOK && strings.EqualFold(resolvedPrefix, objectName) {
			field = field[dot+1:]
		}
	}
	if canonical, ok := storage.ResolveFieldName(vm.Org.Objects[objectName].Definition, vm.Org.Namespace, field); ok {
		return canonical
	}
	return storage.StripNamespaceToken(vm.Org.Namespace, field)
}

func (vm *VM) hasSObjectField(typeName, field string) bool {
	_, _, ok := vm.sObjectFieldDefinition(typeName, field)
	return ok
}

func dmlAccessibleSObject(value Value) bool {
	if value.Kind != ValueObject {
		return false
	}
	marker, ok := value.Fields[sobjectDMLAccessibleField]
	return ok && marker.Kind == ValueBool && marker.Bool
}

func shouldEvaluateSObjectFormulaField(value Value, field storage.Field) bool {
	if strings.TrimSpace(field.Formula) == "" {
		return false
	}
	if !dmlAccessibleSObject(value) {
		return true
	}
	switch strings.ToUpper(field.DisplayType) {
	case "STRING", "TEXT", "TEXTAREA", "URL", "EMAIL", "PHONE":
		return true
	default:
		return false
	}
}

func (vm *VM) sObjectFieldDefinition(typeName, field string) (storage.ObjectDefinition, storage.Field, bool) {
	if vm.Org == nil {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	objectName, ok := vm.resolveObjectName(typeName)
	if !ok {
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	definition := vm.describePreparedDefinition(objectName, vm.Org.Objects[objectName].Definition)
	fieldName, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, field)
	if !ok {
		if systemField, systemOK := syntheticSObjectSystemField(field); systemOK {
			return definition, systemField, true
		}
		if isCustomObjectLikeName(objectName) {
			if synthetic := syntheticSchemaField(field); synthetic.APIName != "" {
				return definition, synthetic, true
			}
		}
		return storage.ObjectDefinition{}, storage.Field{}, false
	}
	return definition, definition.Fields[fieldName], true
}

func (vm *VM) missingSObjectFieldValue(receiver Value, field string) (Value, bool) {
	if isExplicitSObjectField(receiver, field) {
		if _, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field); ok {
			return storageFieldNullValue(fieldDef), true
		}
		return Null, true
	}
	if value, ok := vm.parentRelationshipValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.parentRelationshipValueFromLookupID(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.lazyChildRelationshipValue(receiver, field); ok {
		return value, true
	}
	if relationshipType, ok := vm.jsonSObjectChildRelationshipType(receiver.Type, field); ok {
		children := List()
		children.Type = relationshipType
		return children, true
	}
	definition, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok {
		if value, ok := vm.sObjectCompoundAddressValueByPrefix(receiver, field); ok {
			return value, true
		}
		if value, ok := vm.parentRelationshipValue(receiver, field); ok {
			return value, true
		}
		return Null, false
	}
	if fieldDef.Type == storage.FieldCalculated {
		if shouldEvaluateSObjectFormulaField(receiver, fieldDef) {
			if record, ok := vm.formulaRecordFromSObject(receiver); ok {
				if value, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record); ok {
					formulaValue := vmValueFromStorage(value)
					if calculatedDateFormulaBlankValue(fieldDef, formulaValue) {
						return Null, true
					}
					return formulaValue, true
				}
			}
		}
		switch strings.ToUpper(fieldDef.DisplayType) {
		case "INTEGER":
			return Int(0), true
		case "DECIMAL", "DOUBLE", "CURRENCY", "PERCENT":
			return Decimal(0), true
		case "BOOLEAN":
			return Bool(false), true
		default:
			return Null, true
		}
	}
	if fieldDef.Type == storage.FieldSummary {
		if value, ok := vm.evaluateSummaryField(receiver, fieldDef); ok {
			return vmValueFromStorage(value), true
		}
		return storageFieldNullValue(fieldDef), true
	}
	if value, ok := vm.storedSObjectFieldValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.unqueriedStoredDefaultFieldValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.sObjectCompoundAddressValue(receiver, field); ok {
		return value, true
	}
	if fieldDef.Type == storage.FieldReference {
		if isExplicitSObjectField(receiver, field) {
			return Null, true
		}
		if value, ok := vm.lookupIDFromLoadedParentRelationship(receiver, definition, field); ok {
			return value, true
		}
	}
	if storage.IsCustomMetadataDefinition(definition) || storage.IsCustomSettingDefinition(definition) {
		if defaultValue, ok := storage.DefaultValueForField(fieldDef); ok {
			return vmValueFromStorage(defaultValue), true
		}
	}
	if fieldDef.Type == storage.FieldBoolean {
		if storage.IsCustomMetadataDefinition(definition) || storage.IsCustomSettingDefinition(definition) {
			return Value{Kind: ValueNull, Type: "Boolean"}, true
		}
		return Bool(false), true
	}
	return storageFieldNullValue(fieldDef), true
}

func calculatedDateFormulaBlankValue(fieldDef storage.Field, value Value) bool {
	if !strings.EqualFold(fieldDef.DisplayType, "DATE") && !strings.EqualFold(fieldDef.DisplayType, "DATETIME") {
		return false
	}
	switch value.Kind {
	case ValueInt:
		return value.Int == 0
	case ValueDecimal:
		return value.Decimal == 0
	case ValueString:
		return strings.TrimSpace(value.Text) == "" || strings.TrimSpace(value.Text) == "0"
	case ValueObject:
		if !strings.EqualFold(value.Type, "Date") && !strings.EqualFold(value.Type, "Datetime") {
			return false
		}
		if raw, ok := value.Fields["value"]; ok && raw.Kind == ValueString {
			text := strings.TrimSpace(raw.Text)
			return text == "" || text == "0"
		}
		return false
	default:
		return false
	}
}

func (vm *VM) storedSObjectFieldValue(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	if marker, ok := receiver.Fields[sobjectTriggerField]; ok && marker.Kind == ValueBool && marker.Bool {
		return Null, false
	}
	if _, hasQueriedFields := receiver.Fields[sobjectQueriedFieldsField]; hasQueriedFields {
		return Null, false
	}
	if isExplicitSObjectField(receiver, "Id") && !isExplicitSObjectField(receiver, field) {
		_, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
		if !ok || strings.TrimSpace(fieldDef.DefaultValue) == "" {
			return Null, false
		}
	}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	record, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return Null, false
	}
	if record.HasExplicitNull(field) {
		if _, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field); ok {
			return storageFieldNullValue(fieldDef), true
		}
		return Null, true
	}
	if value, ok := record.GetField(field); ok {
		return vmValueFromStorage(value), true
	}
	return Null, false
}

func (vm *VM) sObjectCompoundAddressValue(receiver Value, field string) (Value, bool) {
	definition, fieldDef, ok := vm.sObjectFieldDefinition(receiver.Type, field)
	if !ok {
		return vm.sObjectCompoundAddressValueByPrefix(receiver, field)
	}
	if fieldDef.Type != storage.FieldAddress {
		return Null, false
	}
	address := Object("Address")
	for componentName, component := range definition.Fields {
		if !strings.EqualFold(component.CompoundFieldName, fieldDef.APIName) && !strings.EqualFold(component.CompoundFieldName, field) {
			continue
		}
		value, ok := vm.sObjectComponentFieldValue(receiver, componentName)
		if !ok || value.Kind == ValueNull {
			continue
		}
		addressField, ok := compoundAddressComponentField(componentName, fieldDef.APIName)
		if !ok {
			continue
		}
		address.Fields[addressField] = value
	}
	if len(address.Fields) == 0 {
		return vm.sObjectCompoundAddressValueByPrefix(receiver, field)
	}
	return address, true
}

func (vm *VM) sObjectCompoundAddressValueByPrefix(receiver Value, field string) (Value, bool) {
	prefix := strings.TrimSuffix(field, "Address")
	if prefix == field || prefix == "" {
		return Null, false
	}
	address := Object("Address")
	for _, component := range []struct {
		suffix string
		field  string
	}{
		{"Street", "street"},
		{"City", "city"},
		{"State", "state"},
		{"StateCode", "stateCode"},
		{"PostalCode", "postalCode"},
		{"Country", "country"},
		{"CountryCode", "countryCode"},
		{"Latitude", "latitude"},
		{"Longitude", "longitude"},
		{"GeocodeAccuracy", "geocodeAccuracy"},
	} {
		value, ok := vm.sObjectComponentFieldValue(receiver, prefix+component.suffix)
		if !ok || value.Kind == ValueNull {
			continue
		}
		address.Fields[component.field] = value
	}
	if len(address.Fields) == 0 {
		return Null, false
	}
	return address, true
}

func (vm *VM) sObjectComponentFieldValue(receiver Value, field string) (Value, bool) {
	if _, value, ok := objectFieldValue(receiver, field); ok {
		return value, true
	}
	if value, ok := vm.storedSObjectFieldValueIgnoringProjection(receiver, field); ok {
		return value, true
	}
	return Null, false
}

func (vm *VM) storedSObjectFieldValueIgnoringProjection(receiver Value, field string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	id := sObjectIDFromFields(receiver.Fields)
	if id == "" {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	record, ok := vm.findOrgRecord(objectName, id)
	if !ok {
		return Null, false
	}
	if record.HasExplicitNull(field) {
		return Null, true
	}
	if value, ok := record.GetField(field); ok {
		return vmValueFromStorage(value), true
	}
	return Null, false
}

func compoundAddressComponentField(componentName, compoundName string) (string, bool) {
	prefix := strings.TrimSuffix(compoundName, "Address")
	if prefix == "" || !strings.HasPrefix(strings.ToLower(componentName), strings.ToLower(prefix)) {
		return "", false
	}
	suffix := componentName[len(prefix):]
	switch strings.ToLower(suffix) {
	case "street":
		return "street", true
	case "city":
		return "city", true
	case "state":
		return "state", true
	case "statecode":
		return "stateCode", true
	case "postalcode":
		return "postalCode", true
	case "country":
		return "country", true
	case "countrycode":
		return "countryCode", true
	case "latitude":
		return "latitude", true
	case "longitude":
		return "longitude", true
	case "geocodeaccuracy":
		return "geocodeAccuracy", true
	default:
		return "", false
	}
}

func (vm *VM) emptyParentRelationshipShell(receiver Value, field string, relationship Value) bool {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject || relationship.Kind != ValueObject {
		return false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		return false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return false
	}
	isParentRelationship := false
	for _, relation := range object.Definition.Relations {
		if vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, field) ||
			vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, field) {
			isParentRelationship = true
			break
		}
	}
	if !isParentRelationship {
		return false
	}
	for fieldName, value := range relationship.Fields {
		if isInternalSObjectField(fieldName) {
			continue
		}
		if value.Kind != ValueNull {
			return false
		}
	}
	return true
}

func (vm *VM) lookupIDFromLoadedParentRelationship(receiver Value, definition storage.ObjectDefinition, field string) (Value, bool) {
	relationship, ok := vm.parentRelationshipNameForField(definition, field)
	if !ok {
		if strings.HasSuffix(field, "__c") {
			relationship = strings.TrimSuffix(field, "__c") + "__r"
		} else if strings.HasSuffix(field, "Id") && len(field) > len("Id") {
			relationship = strings.TrimSuffix(field, "Id")
		}
	}
	if relationship == "" {
		return Null, false
	}
	if !vm.queriedSObjectFieldsIncludes(receiver, field) && !vm.queriedSObjectFieldsIncludes(receiver, relationship) {
		return Null, false
	}
	_, parent, ok := objectFieldValue(receiver, relationship)
	if !ok || parent.Kind != ValueObject {
		return Null, false
	}
	if isExplicitSObjectField(receiver, relationship) && !isExplicitSObjectField(receiver, field) {
		return Null, false
	}
	if id := sObjectIDFromFields(parent.Fields); id != "" {
		return String(string(id)), true
	}
	return Null, false
}

func (vm *VM) queriedSObjectFieldsIncludes(receiver Value, field string) bool {
	selected, ok := receiver.Fields[sobjectQueriedFieldsField]
	if !ok || selected.Kind != ValueMap {
		return false
	}
	if queriedSObjectFieldsMapIncludes(selected, field) {
		return true
	}
	objectName := receiver.Type
	if rawObject, ok := selected.Map[mapKey(String("object"))]; ok && rawObject.Kind == ValueString {
		objectName = rawObject.Text
	}
	if vm == nil || vm.Org == nil || strings.TrimSpace(objectName) == "" {
		return false
	}
	if canonicalObject, ok := vm.resolveObjectName(objectName); ok {
		objectName = canonicalObject
	}
	if queriedSObjectFieldsMapIncludes(selected, storage.NamespaceTokenName(vm.Org.Namespace, field)) {
		return true
	}
	if vm.Org.Namespace != "" && queriedSObjectFieldsMapIncludes(selected, storage.StripNamespaceToken(vm.Org.Namespace, field)) {
		return true
	}
	if object, ok := vm.Org.Objects[objectName]; ok {
		if canonical, ok := storage.ResolveFieldName(object.Definition, vm.Org.Namespace, field); ok {
			return queriedSObjectFieldsMapIncludes(selected, canonical) ||
				queriedSObjectFieldsMapIncludes(selected, storage.NamespaceTokenName(vm.Org.Namespace, canonical)) ||
				(vm.Org.Namespace != "" && queriedSObjectFieldsMapIncludes(selected, storage.StripNamespaceToken(vm.Org.Namespace, canonical)))
		}
	}
	return false
}

func queriedSObjectFieldsMapIncludes(selected Value, field string) bool {
	_, ok := selected.Map[mapKey(String(strings.ToLower(field)))]
	return ok
}

func (vm *VM) parentRelationshipValue(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		return vm.parentRelationshipValueForRelation(receiver, relation), true
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		return vm.parentRelationshipValueForRelation(receiver, relation), true
	}
	return Null, false
}

func (vm *VM) hydrateQueriedRecordTypeRelationships(value Value) {
	if vm == nil || vm.Org == nil || value.Kind != ValueObject {
		return
	}
	objectName, ok := vm.resolveObjectName(value.Type)
	if !ok {
		objectName = value.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return
	}
	for _, relation := range object.Definition.Relations {
		if !relationReferencesObject(relation, "RecordType") ||
			!vm.queriedSObjectFieldsIncludes(value, relation.ParentRelationship) {
			continue
		}
		_, lookupValue, lookupOK := objectFieldValue(value, relation.Field)
		if (!lookupOK || lookupValue.Kind == ValueNull) && value.Kind == ValueObject {
			if storedValue, storedOK := vm.storedSObjectFieldValueIgnoringProjection(value, relation.Field); storedOK {
				lookupValue = storedValue
				lookupOK = true
			}
		}
		if !lookupOK || lookupValue.Kind == ValueNull {
			continue
		}
		lookupID, idOK := sObjectIDFromValue(lookupValue)
		if !idOK {
			continue
		}
		if recordType, ok := vm.recordTypeRelationshipValue(object.Definition, lookupID); ok {
			value.Fields[relation.ParentRelationship] = recordType
		}
	}
}

func (vm *VM) parentRelationshipValueForRelation(receiver Value, relation storage.Relationship) Value {
	if _, relationship, ok := objectFieldValue(receiver, relation.ParentRelationship); ok && relationship.Kind == ValueObject {
		if !vm.isSObjectLikeType(relationship.Type) && len(relation.ParentObjects) > 0 {
			relationship.Type = relation.ParentObjects[0]
		}
		return relationship
	}
	_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
	if (!ok || lookupValue.Kind == ValueNull) && receiver.Kind == ValueObject {
		if storedValue, storedOK := vm.storedSObjectFieldValueIgnoringProjection(receiver, relation.Field); storedOK {
			lookupValue = storedValue
			ok = true
		}
	}
	if !ok || lookupValue.Kind == ValueNull {
		return vm.parentRelationshipTypedNull(relation)
	}
	if relationReferencesObject(relation, "RecordType") && !vm.queriedSObjectFieldsIncludes(receiver, relation.ParentRelationship) {
		return vm.parentRelationshipTypedNull(relation)
	}
	return vm.parentRelationshipTypedNull(relation)
}

func (vm *VM) syntheticParentRelationship(definition storage.ObjectDefinition, relationshipName string) (storage.Relationship, bool) {
	if strings.EqualFold(definition.APIName, "RelationshipDomain") {
		switch strings.ToLower(strings.TrimSpace(relationshipName)) {
		case "childsobject":
			return storage.Relationship{Field: "ChildSobjectId", ParentObjects: []string{"EntityDefinition"}, ParentRelationship: "ChildSobject"}, true
		case "field":
			return storage.Relationship{Field: "FieldId", ParentObjects: []string{"FieldDefinition"}, ParentRelationship: "Field"}, true
		}
	}
	for name, field := range definition.Fields {
		apiName := field.APIName
		if apiName == "" {
			apiName = name
		}
		if field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		if !vmParentRelationshipNameMatches(vm.Org.Namespace, apiName, relationshipName) {
			continue
		}
		parentRelationship := vm.parentRelationshipNameForReferenceField(definition, field)
		return storage.Relationship{Field: apiName, ParentObjects: append([]string(nil), field.ReferenceTo...), ParentRelationship: parentRelationship, Polymorphic: len(field.ReferenceTo) > 1}, true
	}
	for _, fieldName := range []string{relationshipName + "Id", lookupFieldRelationshipName(relationshipName)} {
		fieldName = strings.TrimSpace(fieldName)
		if fieldName == "" {
			continue
		}
		_, field, ok := vm.sObjectFieldDefinition(definition.APIName, fieldName)
		if !ok || field.Type != storage.FieldReference || len(field.ReferenceTo) == 0 {
			continue
		}
		parentRelationship := vm.parentRelationshipNameForReferenceField(definition, field)
		if !vmRelationshipNameMatches(vm.Org.Namespace, parentRelationship, relationshipName) {
			continue
		}
		return storage.Relationship{Field: field.APIName, ParentObjects: append([]string(nil), field.ReferenceTo...), ParentRelationship: parentRelationship, Polymorphic: len(field.ReferenceTo) > 1}, true
	}
	return storage.Relationship{}, false
}

func (vm *VM) parentRelationshipValueFromLookupID(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		if marker, ok := receiver.Fields[sobjectDMLAccessibleField]; ok && marker.Kind == ValueBool && marker.Bool {
			return vm.parentRelationshipTypedNull(relation), true
		}
		_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
		if (!ok || lookupValue.Kind == ValueNull) && receiver.Kind == ValueObject {
			if storedValue, storedOK := vm.storedSObjectFieldValueIgnoringProjection(receiver, relation.Field); storedOK {
				lookupValue = storedValue
				ok = true
			}
		}
		if !ok || lookupValue.Kind == ValueNull {
			return Null, false
		}
		if sObjectIDFromFields(receiver.Fields) == "" {
			return Null, false
		}
		return vm.parentRelationshipShellFromLookupID(relation, lookupValue)
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		_, lookupValue, ok := objectFieldValue(receiver, relation.Field)
		if !ok || lookupValue.Kind == ValueNull {
			return Null, false
		}
		return vm.parentRelationshipShellFromLookupID(relation, lookupValue)
	}
	return Null, false
}

func (vm *VM) parentRelationshipShellFromLookupID(relation storage.Relationship, lookupValue Value) (Value, bool) {
	if vm == nil || vm.Org == nil {
		return Null, false
	}
	lookupID, ok := sObjectIDFromValue(lookupValue)
	if !ok || lookupID == "" {
		return Null, false
	}
	for _, parentName := range relation.ParentObjects {
		parentObject, ok := vm.resolveObjectName(parentName)
		if !ok {
			parentObject = parentName
		}
		if strings.TrimSpace(parentObject) == "" {
			continue
		}
		parent := Object(parentObject)
		parent.Fields["Id"] = platformScalar("Id", string(lookupID))
		return parent, true
	}
	return Null, false
}

func (vm *VM) recordTypeRelationshipValue(definition storage.ObjectDefinition, id storage.ID) (Value, bool) {
	if id == "" {
		return Null, false
	}
	for _, recordType := range definition.RecordTypes {
		if recordType.ID != "" && !apexIDTextEqual(string(recordType.ID), string(id)) {
			continue
		}
		value := Object("RecordType")
		value.Fields["Id"] = platformScalar("Id", string(id))
		name := recordType.Name
		if name == "" {
			name = recordType.DeveloperName
		}
		value.Fields["Name"] = String(name)
		value.Fields["DeveloperName"] = String(recordType.DeveloperName)
		value.Fields["SObjectType"] = String(definition.APIName)
		return value, true
	}
	return Null, false
}

func (vm *VM) parentRelationshipTypedNull(relation storage.Relationship) Value {
	if vm == nil || vm.Org == nil || len(relation.ParentObjects) == 0 {
		return Null
	}
	parentObject, ok := vm.resolveObjectName(relation.ParentObjects[0])
	if !ok {
		parentObject = relation.ParentObjects[0]
	}
	value := Null
	value.Type = parentObject
	value.Runtime = relationshipNullRuntime
	return value
}

func (vm *VM) parentRelationshipTypedShell(relation storage.Relationship) Value {
	if vm == nil || vm.Org == nil || len(relation.ParentObjects) == 0 {
		return Null
	}
	parentObject, ok := vm.resolveObjectName(relation.ParentObjects[0])
	if !ok {
		parentObject = relation.ParentObjects[0]
	}
	if strings.TrimSpace(parentObject) == "" {
		return Null
	}
	return Object(parentObject)
}

func (vm *VM) parentRelationshipShell(receiver Value, relationshipName string) (Value, bool) {
	if vm == nil || vm.Org == nil || receiver.Kind != ValueObject {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(receiver.Type)
	if !ok {
		objectName = receiver.Type
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return Null, false
	}
	for _, relation := range object.Definition.Relations {
		if !vmRelationshipNameMatches(vm.Org.Namespace, relation.ParentRelationship, relationshipName) &&
			!vmParentRelationshipNameMatches(vm.Org.Namespace, relation.Field, relationshipName) {
			continue
		}
		for _, parentName := range relation.ParentObjects {
			parentObject, ok := vm.resolveObjectName(parentName)
			if !ok {
				parentObject = parentName
			}
			return Object(parentObject), true
		}
		return Null, false
	}
	if relation, ok := vm.syntheticParentRelationship(object.Definition, relationshipName); ok {
		for _, parentName := range relation.ParentObjects {
			parentObject, ok := vm.resolveObjectName(parentName)
			if !ok {
				parentObject = parentName
			}
			return Object(parentObject), true
		}
		return Null, false
	}
	return Null, false
}

func (vm *VM) findOrgRecord(objectName string, id storage.ID) (storage.Record, bool) {
	if vm == nil || vm.Org == nil || id == "" {
		return storage.Record{}, false
	}
	object, ok := vm.Org.Objects[objectName]
	if !ok {
		return storage.Record{}, false
	}
	if record, ok := object.Records[id]; ok {
		return record, true
	}
	for candidateID, record := range object.Records {
		if apexIDTextEqual(string(candidateID), string(id)) {
			return record, true
		}
	}
	return storage.Record{}, false
}

func (vm *VM) formulaRecordFromSObject(value Value) (storage.Record, bool) {
	record, err := vm.recordFromValue(&value)
	if err != nil {
		return storage.Record{}, false
	}
	if vm.Org == nil || record.ID == "" {
		return record, true
	}
	objectName, ok := vm.resolveObjectName(record.Object)
	if !ok {
		return record, true
	}
	if persisted, ok := vm.Org.Objects[objectName].Records[record.ID]; ok {
		for field, fieldValue := range persisted.Fields {
			if _, exists := record.GetField(field); !exists && !record.HasExplicitNull(field) {
				record.Fields[field] = fieldValue.Clone()
			}
		}
	}
	return record, true
}

func (vm *VM) evaluateSummaryField(receiver Value, fieldDef storage.Field) (storage.Value, bool) {
	operation := strings.ToLower(strings.TrimSpace(fieldDef.SummaryOperation))
	if vm.Org == nil || (operation != "sum" && operation != "count") {
		return storage.Value{}, false
	}
	parent, ok := vm.formulaRecordFromSObject(receiver)
	if !ok || parent.ID == "" {
		return storage.Value{}, false
	}
	childObject, childField := splitQualifiedField(fieldDef.SummarizedField)
	fkObject, fkField := splitQualifiedField(fieldDef.SummaryForeignKey)
	if childObject == "" && operation == "count" {
		childObject = fkObject
	}
	if childObject == "" || fkObject == "" || fkField == "" || !strings.EqualFold(childObject, fkObject) {
		return storage.Value{}, false
	}
	if operation != "count" && childField == "" {
		return storage.Value{}, false
	}
	canonicalChild, ok := vm.resolveObjectName(childObject)
	if !ok {
		return storage.Value{}, false
	}
	childState := vm.Org.Objects[canonicalChild]
	childFieldName := ""
	if childField != "" {
		var ok bool
		childFieldName, ok = storage.ResolveFieldName(childState.Definition, vm.Org.Namespace, childField)
		if !ok {
			return storage.Value{}, false
		}
	}
	fkFieldName, ok := storage.ResolveFieldName(childState.Definition, vm.Org.Namespace, fkField)
	if !ok {
		return storage.Value{}, false
	}
	count := int64(0)
	total := 0.0
	matched := false
	for _, child := range childState.Records {
		if child.System.IsDeleted {
			continue
		}
		if !apexIDTextEqual(storageValueIDText(child.Fields[fkFieldName]), string(parent.ID)) {
			continue
		}
		if !vm.summaryFiltersMatch(childState.Definition, child, fieldDef.SummaryFilterItems) {
			continue
		}
		if operation == "count" {
			count++
			matched = true
			continue
		}
		value, ok := vm.summaryRecordFieldValue(childState.Definition, child, childFieldName)
		if !ok {
			continue
		}
		number, ok := storageNumericValue(value)
		if !ok {
			continue
		}
		total += number
		matched = true
	}
	if operation == "count" {
		return storage.IntegerValue(count), true
	}
	if !matched {
		return storage.DecimalValue("0"), true
	}
	return storage.DecimalValue(strconv.FormatFloat(total, 'f', -1, 64)), true
}

func splitQualifiedField(name string) (string, string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ""
	}
	parts := strings.Split(name, ".")
	if len(parts) < 2 {
		return "", name
	}
	return strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1]
}

func (vm *VM) summaryFiltersMatch(definition storage.ObjectDefinition, record storage.Record, filters []storage.SummaryFilterItem) bool {
	for _, filter := range filters {
		_, fieldName := splitQualifiedField(filter.Field)
		if fieldName == "" {
			fieldName = filter.Field
		}
		canonical, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
		if !ok {
			return false
		}
		value, ok := vm.summaryRecordFieldValue(definition, record, canonical)
		if !ok {
			value = storage.NullValue()
		}
		if !summaryFilterMatches(value, filter) {
			return false
		}
	}
	return true
}

func (vm *VM) summaryRecordFieldValue(definition storage.ObjectDefinition, record storage.Record, fieldName string) (storage.Value, bool) {
	if value, ok := record.GetField(fieldName); ok {
		return value, true
	}
	fieldDef, ok := definition.Fields[fieldName]
	if !ok || fieldDef.Type != storage.FieldCalculated || strings.TrimSpace(fieldDef.Formula) == "" {
		return storage.Value{}, false
	}
	value, _, ok := dml.EvaluateRecordFormulaValueInOrg(fieldDef.Formula, fieldDef, vm.Org, definition, record)
	return value, ok
}

func summaryFilterMatches(value storage.Value, filter storage.SummaryFilterItem) bool {
	switch strings.ToLower(strings.TrimSpace(filter.Operation)) {
	case "", "equals":
		return storageValueMatchesText(value, filter.Value)
	default:
		return false
	}
}

func storageValueMatchesText(value storage.Value, text string) bool {
	text = strings.TrimSpace(text)
	switch value.Kind {
	case storage.ValueBoolean:
		return strings.EqualFold(strconv.FormatBool(value.Boolean), text)
	case storage.ValueString:
		return strings.EqualFold(value.String, text)
	case storage.ValueID:
		return apexIDTextEqual(string(value.ID), text)
	case storage.ValueInteger:
		parsed, err := strconv.ParseInt(text, 10, 64)
		return err == nil && value.Integer == parsed
	case storage.ValueDecimal:
		return strings.TrimRight(strings.TrimRight(value.Decimal, "0"), ".") == strings.TrimRight(strings.TrimRight(text, "0"), ".")
	case storage.ValueNull:
		return strings.EqualFold(text, "null") || text == ""
	default:
		return false
	}
}

func storageNumericValue(value storage.Value) (float64, bool) {
	switch value.Kind {
	case storage.ValueInteger:
		return float64(value.Integer), true
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		return parsed, err == nil
	case storage.ValueString:
		parsed, err := strconv.ParseFloat(value.String, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func (vm *VM) sObjectFieldArg(receiverType string, value Value) (string, error) {
	if value.Kind == ValueString {
		return value.Text, nil
	}
	if value.Kind == ValueObject && isSObjectFieldTokenType(value.Type) {
		if objectValue, ok := value.Fields["object"]; ok && objectValue.Kind == ValueString && receiverType != "" && !strings.EqualFold(receiverType, "SObject") {
			if vm.Org != nil {
				if receiverObject, ok := vm.resolveObjectName(receiverType); ok {
					if tokenObject, ok := vm.resolveObjectName(objectValue.Text); ok && !strings.EqualFold(tokenObject, receiverObject) {
						return "", fmt.Errorf("%w: field token belongs to %s, not %s", errSObjectFieldTokenWrongObject, objectValue.Text, receiverType)
					}
				}
			}
		}
		field, ok := value.Fields["field"]
		if !ok || field.Kind != ValueString {
			return "", fmt.Errorf("field token missing field name")
		}
		return field.Text, nil
	}
	if value.Kind == ValueNull && isSObjectFieldTokenType(value.Type) {
		return "", errSObjectFieldTokenNull
	}
	return "", fmt.Errorf("expected field name")
}

var errSObjectFieldTokenWrongObject = errors.New("field token belongs to another SObject type")

var errSObjectFieldTokenNull = errors.New("field token is null")

func isSObjectFieldTokenType(typeName string) bool {
	return strings.EqualFold(typeName, "Schema.SObjectField") || strings.EqualFold(typeName, "SObjectField")
}

func apexPagesSeverityStaticValue(name string) (Value, bool) {
	prefix := "ApexPages.Severity."
	if len(name) < len(prefix) || !strings.EqualFold(name[:len(prefix)], prefix) {
		return Null, false
	}
	severity := name[len(prefix):]
	for i, candidate := range apexPagesSeverityNames {
		if strings.EqualFold(severity, candidate) {
			return Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	return Null, false
}

func apexPagesSeverityName(value Value) (string, bool) {
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "ApexPages.Severity") && value.Text != "" {
		return value.Text, true
	}
	if value.Kind == ValueString {
		for _, candidate := range apexPagesSeverityNames {
			if strings.EqualFold(value.Text, candidate) {
				return candidate, true
			}
		}
	}
	return "", false
}

func apexPagesSeverityValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("ApexPages.Severity.values expects 0 arguments")
	}
	values := make([]Value, 0, len(apexPagesSeverityNames))
	for i, name := range apexPagesSeverityNames {
		value := Value{Kind: ValueObject, Type: "ApexPages.Severity", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func metadataDeployStatusStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Metadata.DeployStatus", metadataDeployStatusNames, name)
}

func metadataMetadataTypeStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Metadata.MetadataType", metadataMetadataTypeNames, name)
}

func soapTypeForStorageField(field storage.Field) string {
	switch field.Type {
	case storage.FieldID, storage.FieldReference:
		return "ID"
	case storage.FieldBoolean:
		return "BOOLEAN"
	case storage.FieldInteger:
		return "INTEGER"
	case storage.FieldDecimal:
		return "DOUBLE"
	case storage.FieldDate:
		return "DATE"
	case storage.FieldDateTime:
		return "DATETIME"
	case storage.FieldBlob:
		return "BASE64BINARY"
	default:
		return "STRING"
	}
}

var schemaSOAPTypeNames = []string{"ID", "STRING", "BOOLEAN", "INTEGER", "DOUBLE", "DATE", "DATETIME", "TIME", "BASE64BINARY", "ANYTYPE"}

var schemaDisplayTypeNames = []string{"BOOLEAN", "CURRENCY", "DATE", "DATETIME", "DOUBLE", "ID", "INTEGER", "PERCENT", "PICKLIST", "REFERENCE", "STRING", "TEXTAREA"}

var accessTypeNames = []string{"CREATABLE", "READABLE", "UPDATABLE", "UPSERTABLE"}

func schemaSOAPTypeStaticValue(name string) (Value, bool) {
	if value, ok := namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, name); ok {
		return value, true
	}
	if strings.HasPrefix(name, "Schema.SoapType.") {
		return namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, "Schema.SOAPType."+strings.TrimPrefix(name, "Schema.SoapType."))
	}
	return Null, false
}

func schemaSOAPTypeValue(name string) Value {
	value, _ := namedEnumStaticValue("Schema.SOAPType", schemaSOAPTypeNames, "Schema.SOAPType."+name)
	return value
}

func schemaDisplayTypeStaticValue(name string) (Value, bool) {
	return namedEnumStaticValue("Schema.DisplayType", schemaDisplayTypeNames, name)
}

func schemaDisplayTypeValue(name string) Value {
	value, ok := namedEnumStaticValue("Schema.DisplayType", schemaDisplayTypeNames, "Schema.DisplayType."+name)
	if ok {
		return value
	}
	return Value{Kind: ValueObject, Type: "Schema.DisplayType", Text: name}
}

func namedEnumStaticValue(typeName string, names []string, name string) (Value, bool) {
	prefix := typeName + "."
	if !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
		return Null, false
	}
	member := name[len(prefix):]
	for i, candidate := range names {
		if member == candidate {
			return Value{Kind: ValueObject, Type: typeName, Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	for i, candidate := range names {
		if strings.EqualFold(member, candidate) {
			return Value{Kind: ValueObject, Type: typeName, Text: candidate, Fields: map[string]Value{"ordinal": Int(int64(i))}}, true
		}
	}
	return Null, false
}

func metadataDeployStatusValues(args []Value) (Value, error) {
	return namedEnumValues("Metadata.DeployStatus", metadataDeployStatusNames, args)
}

func metadataMetadataTypeValues(args []Value) (Value, error) {
	return namedEnumValues("Metadata.MetadataType", metadataMetadataTypeNames, args)
}

func (vm *VM) reportsDescribeReport(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.describeReport expects report Id")
	}
	describe := Object("reports.ReportDescribeResult")
	describe.Fields["reportMetadata"] = vm.reportsReportMetadata(args[0], Null)
	describe.Fields["reportExtendedMetadata"] = vm.reportsReportExtendedMetadata()
	describe.Fields["reportTypeMetadata"] = vm.reportsReportTypeMetadata()
	return describe, nil
}

func (vm *VM) reportsDatatypeFilterOperatorMap(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("reports.ReportManager.getDatatypeFilterOperatorMap expects 0 arguments")
	}
	return typedMap("Map<String,List<reports.FilterOperator>>"), nil
}

func (vm *VM) reportsGetReportInstance(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.getReportInstance expects instance Id")
	}
	instanceID := scalarText(args[0])
	if vm.reportInstances != nil {
		if value, ok := vm.reportInstances[instanceID]; ok {
			return cloneValue(value), nil
		}
	}
	return vm.reportsReportInstance(args[0], Null, vm.reportsReportResults(Null, Null, false)), nil
}

func (vm *VM) reportsGetReportInstances(args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("reports.ReportManager.getReportInstances expects report Id")
	}
	out := typedList("List<reports.ReportInstance>")
	reportID := scalarText(args[0])
	for _, instance := range vm.reportInstances {
		if _, value, found := objectFieldValue(instance, "reportId"); found && scalarText(value) == reportID {
			out.List = append(out.List, cloneValue(instance))
		}
	}
	return out, nil
}

func (vm *VM) reportsRunAsyncReport(args []Value) (Value, error) {
	reportID, metadata, includeDetails, err := vm.reportsReportArgs(args, "reports.ReportManager.runAsyncReport")
	if err != nil {
		return Null, err
	}
	results := vm.reportsReportResults(reportID, metadata, includeDetails)
	instanceID := platformScalar("Id", vm.nextReportInstanceID())
	instance := vm.reportsReportInstance(instanceID, reportID, results)
	if vm.reportInstances == nil {
		vm.reportInstances = make(map[string]Value)
	}
	vm.reportInstances[scalarText(instanceID)] = cloneValue(instance)
	return instance, nil
}

func (vm *VM) reportsRunReport(args []Value) (Value, error) {
	reportID, metadata, includeDetails, err := vm.reportsReportArgs(args, "reports.ReportManager.runReport")
	if err != nil {
		return Null, err
	}
	return vm.reportsReportResults(reportID, metadata, includeDetails), nil
}

func (vm *VM) reportsReportArgs(args []Value, callee string) (Value, Value, bool, error) {
	if len(args) < 1 || len(args) > 3 {
		return Null, Null, false, fmt.Errorf("%s expects report Id[, ReportMetadata][, includeDetails]", callee)
	}
	reportID := args[0]
	metadata := Null
	includeDetails := false
	for _, arg := range args[1:] {
		switch {
		case arg.Kind == ValueBool:
			includeDetails = arg.Bool
		case arg.Kind == ValueObject && strings.EqualFold(arg.Type, "reports.ReportMetadata"):
			metadata = arg
		default:
			return Null, Null, false, fmt.Errorf("%s expects report Id[, ReportMetadata][, includeDetails]", callee)
		}
	}
	return reportID, metadata, includeDetails, nil
}

func (vm *VM) reportsReportResults(reportID, metadata Value, includeDetails bool) Value {
	results := Object("reports.ReportResults")
	results.Fields["allData"] = Bool(false)
	results.Fields["factMap"] = typedMap("Map<String,reports.ReportFact>")
	results.Fields["groupingsAcross"] = Object("reports.Dimension")
	results.Fields["groupingsDown"] = Object("reports.Dimension")
	results.Fields["hasDetailRows"] = Bool(includeDetails)
	results.Fields["reportExtendedMetadata"] = vm.reportsReportExtendedMetadata()
	results.Fields["reportMetadata"] = vm.reportsReportMetadata(reportID, metadata)
	return results
}

func (vm *VM) reportsReportMetadata(reportID, override Value) Value {
	if override.Kind == ValueObject && strings.EqualFold(override.Type, "reports.ReportMetadata") {
		metadata := cloneValue(override)
		if _, _, ok := objectFieldValue(metadata, "id"); !ok && reportID.Kind != ValueNull {
			metadata.Fields["id"] = reportID
		}
		return metadata
	}
	metadata := Object("reports.ReportMetadata")
	if reportID.Kind != ValueNull {
		metadata.Fields["id"] = reportID
	}
	metadata.Fields["name"] = String("Local Report")
	metadata.Fields["developerName"] = String("Local_Report")
	metadata.Fields["groupingsAcross"] = typedList("List<reports.GroupingInfo>")
	metadata.Fields["groupingsDown"] = typedList("List<reports.GroupingInfo>")
	metadata.Fields["aggregates"] = typedList("List<String>")
	metadata.Fields["buckets"] = typedList("List<reports.BucketField>")
	metadata.Fields["detailColumns"] = typedList("List<String>")
	metadata.Fields["reportFilters"] = typedList("List<reports.ReportFilter>")
	metadata.Fields["historicalSnapshotDates"] = typedList("List<String>")
	metadata.Fields["sortBy"] = typedList("List<reports.SortColumn>")
	metadata.Fields["standardFilters"] = typedList("List<reports.StandardFilter>")
	metadata.Fields["customSummaryFormula"] = typedMap("Map<String,reports.ReportCsf>")
	metadata.Fields["crossFilters"] = typedList("List<reports.CrossFilter>")
	metadata.Fields["hasDetailRows"] = Bool(false)
	metadata.Fields["hasRecordCount"] = Bool(false)
	metadata.Fields["showSubtotals"] = Bool(false)
	metadata.Fields["showGrandTotal"] = Bool(false)
	return metadata
}

func (vm *VM) reportsReportExtendedMetadata() Value {
	metadata := Object("reports.ReportExtendedMetadata")
	metadata.Fields["aggregateColumnInfo"] = typedMap("Map<String,reports.AggregateColumn>")
	metadata.Fields["detailColumnInfo"] = typedMap("Map<String,reports.DetailColumn>")
	metadata.Fields["groupingColumnInfo"] = typedMap("Map<String,reports.GroupingColumn>")
	return metadata
}

func (vm *VM) reportsReportTypeMetadata() Value {
	metadata := Object("reports.ReportTypeMetadata")
	metadata.Fields["categories"] = typedList("List<reports.ReportTypeColumnCategory>")
	metadata.Fields["standardDateFilterDurationGroups"] = typedList("List<reports.StandardDateFilterDurationGroup>")
	metadata.Fields["standardFilterInfos"] = typedMap("Map<String,reports.StandardFilterInfo>")
	return metadata
}

func (vm *VM) reportsReportInstance(instanceID, reportID, results Value) Value {
	instance := Object("reports.ReportInstance")
	instance.Fields["id"] = instanceID
	instance.Fields["reportId"] = reportID
	instance.Fields["reportResults"] = results
	instance.Fields["status"] = String("Success")
	instance.Fields["ownerId"] = platformScalar("Id", vm.currentUserInfoField("Id", "005000000000001"))
	now := platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow))
	instance.Fields["requestDate"] = now
	instance.Fields["completionDate"] = now
	return instance
}

func (vm *VM) nextReportInstanceID() string {
	return fmt.Sprintf("0LG000000%06d", len(vm.reportInstances)+1)
}

func prefCenterGenerateToken(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 2 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("pref_center.TokenUtility.generateToken expects String[, TokenType]")
	}
	return String(prefCenterLocalToken(args[0], args[1:])), nil
}

func prefCenterGenerateTokens(args []Value) (Value, error) {
	if len(args) < 1 || len(args) > 3 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("pref_center.TokenUtility.generateTokens expects List<String>[, TokenType][, DataCloudIdTokenType]")
	}
	out := typedMap("Map<String,String>")
	for _, tokenValue := range args[0].List {
		if tokenValue.Kind != ValueString {
			return Null, fmt.Errorf("pref_center.TokenUtility.generateTokens expects List<String>")
		}
		key := mapKey(tokenValue)
		out.Map[key] = String(prefCenterLocalToken(tokenValue, args[1:]))
		out.MapKeys[key] = tokenValue
	}
	return out, nil
}

func prefCenterLocalToken(tokenValue Value, options []Value) string {
	parts := []string{tokenValue.Text}
	for _, option := range options {
		parts = append(parts, scalarText(option))
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "local-token-" + hex.EncodeToString(sum[:])[:24]
}

func functionInvocationSuccess(args []Value) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
		return Null, fmt.Errorf("functions.MockFunctionInvocationFactory.createSuccessResponse expects invocation Id and response")
	}
	invocation := Object("functions.FunctionInvocation")
	invocation.Fields["invocationId"] = args[0]
	invocation.Fields["response"] = args[1]
	invocation.Fields["status"] = Value{Kind: ValueObject, Type: "functions.FunctionInvocationStatus", Text: "SUCCESS"}
	invocation.Fields["error"] = Null
	return invocation, nil
}

func functionInvocationError(args []Value) (Value, error) {
	if len(args) != 3 || args[0].Kind != ValueString || args[2].Kind != ValueString {
		return Null, fmt.Errorf("functions.MockFunctionInvocationFactory.createErrorResponse expects invocation Id, error type, and message")
	}
	errValue := Object("functions.FunctionInvocationError")
	errValue.Fields["type"] = args[1]
	errValue.Fields["message"] = args[2]
	invocation := Object("functions.FunctionInvocation")
	invocation.Fields["invocationId"] = args[0]
	invocation.Fields["response"] = Null
	invocation.Fields["status"] = Value{Kind: ValueObject, Type: "functions.FunctionInvocationStatus", Text: "ERROR"}
	invocation.Fields["error"] = errValue
	return invocation, nil
}

func waveTemplatesStaticDefault(callee string, args []Value) (Value, error) {
	switch callee {
	case "wave.Templates.cdpQueryMetadata":
		if len(args) != 4 {
			return Null, fmt.Errorf("wave.Templates.cdpQueryMetadata expects 4 arguments")
		}
	case "wave.Templates.getSObject", "wave.Templates.getTemplate", "wave.Templates.getTemplateConfig":
		if len(args) != 1 && len(args) != 3 && len(args) != 4 {
			return Null, fmt.Errorf("%s expects supported template lookup arguments", callee)
		}
	case "wave.Templates.getTemplates":
		if len(args) > 1 {
			return Null, fmt.Errorf("wave.Templates.getTemplates expects optional search options")
		}
	}
	out := typedMap("Map<String,Object>")
	out.Map[mapKey(String("local"))] = Bool(true)
	out.MapKeys[mapKey(String("local"))] = String("local")
	return out, nil
}

func flowInterviewCreate(args []Value) (Value, error) {
	if len(args) != 2 && len(args) != 3 {
		return Null, fmt.Errorf("Flow.Interview.createInterview expects flow name and input variables")
	}
	offset := 0
	if len(args) == 3 {
		if args[0].Kind != ValueString {
			return Null, fmt.Errorf("Flow.Interview.createInterview expects namespace String")
		}
		offset = 1
	}
	if args[offset].Kind != ValueString || args[offset+1].Kind != ValueMap {
		return Null, fmt.Errorf("Flow.Interview.createInterview expects flow name and input variables")
	}
	interview := Object("Flow.Interview")
	if offset == 1 {
		interview.Fields["namespace"] = args[0]
	}
	interview.Fields["flowName"] = args[offset]
	interview.Fields["variables"] = args[offset+1]
	return interview, nil
}

func (vm *VM) metadataEnqueueDeployment(args []Value, result *Result) (Value, error) {
	if len(args) != 2 || args[0].Kind != ValueObject || args[0].Type != "Metadata.DeployContainer" {
		return Null, fmt.Errorf("Metadata.Operations.enqueueDeployment expects DeployContainer and DeployCallback")
	}
	if args[1].Kind != ValueNull {
		return Null, unsupportedCallError("Metadata.Operations.enqueueDeployment deploy callback invocation")
	}
	deploymentID := "0Af000000000001"
	items := args[0].Fields["metadata"]
	if items.Kind == ValueNull || (items.Kind == ValueList && len(items.List) == 0) {
		vm.recordMetadataDeployment(deploymentID, nil)
		appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
			"deploymentId": deploymentID,
			"components":   0,
			"success":      true,
		})
		return platformScalar("Id", deploymentID), nil
	}
	if items.Kind != ValueList {
		return Null, fmt.Errorf("Metadata.DeployContainer.metadata must be a list")
	}
	if vm.Org == nil {
		return Null, unsupportedCallError("Metadata.Operations.enqueueDeployment requires org storage for local metadata mutation")
	}
	originalOrg := vm.Org
	candidateOrg := originalOrg.Clone()
	vm.Org = &candidateOrg
	for _, item := range items.List {
		if err := vm.applyMetadataDeployment(item); err != nil {
			vm.Org = originalOrg
			var runtimeErr *RuntimeError
			if errors.As(err, &runtimeErr) && runtimeErr.Type == "UnsupportedFeature" {
				return Null, err
			}
			vm.recordMetadataDeploymentFailure(deploymentID, items.List, item, err)
			appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
				"deploymentId": deploymentID,
				"components":   len(items.List),
				"success":      false,
				"error":        err.Error(),
			})
			return platformScalar("Id", deploymentID), nil
		}
	}
	*originalOrg = candidateOrg
	vm.Org = originalOrg
	vm.recordMetadataDeployment(deploymentID, items.List)
	appendTrace(result, "apex.metadata.deploy.enqueue", "apex.metadata", map[string]any{
		"deploymentId": deploymentID,
		"components":   len(items.List),
		"success":      true,
	})
	return platformScalar("Id", deploymentID), nil
}

func (vm *VM) metadataCheckDeployStatus(args []Value, result *Result) (Value, error) {
	if len(args) < 1 || len(args) > 2 || !metadataDeploymentIDValue(args[0]) {
		return Null, fmt.Errorf("Metadata.Operations.checkDeployStatus expects deployment Id[, includeDetails]")
	}
	includeDetails := false
	if len(args) == 2 {
		if args[1].Kind != ValueBool {
			return Null, fmt.Errorf("Metadata.Operations.checkDeployStatus includeDetails expects Boolean")
		}
		includeDetails = args[1].Bool
	}
	deploymentID := args[0].Text
	if args[0].Kind == ValueObject {
		var err error
		deploymentID, err = platformScalarText(args[0], "Id")
		if err != nil {
			return Null, err
		}
	}
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	storedResult, ok := vm.metadataDeploys[deploymentID]
	if !ok && len(deploymentID) == 18 {
		storedResult, ok = vm.metadataDeploys[deploymentID[:15]]
	}
	if !ok {
		return Null, unsupportedCallError("Metadata.Operations.checkDeployStatus unknown local deployment " + deploymentID)
	}
	deployResult := cloneMetadataDeployResult(storedResult)
	if !includeDetails {
		deployResult.Fields["details"] = Null
	}
	appendTrace(result, "apex.metadata.deploy.status", "apex.metadata", map[string]any{
		"deploymentId":   deploymentID,
		"includeDetails": includeDetails,
		"success":        deployResult.Fields["success"].Bool,
		"status":         deployResult.Fields["status"].Text,
	})
	return deployResult, nil
}

func metadataDeploymentIDValue(value Value) bool {
	return value.Kind == ValueString || (value.Kind == ValueObject && value.Type == "Id")
}

func (vm *VM) recordMetadataDeployment(deploymentID string, items []Value) {
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	result := metadataDeployResultObject(deploymentID, items)
	vm.metadataDeploys[deploymentID] = result
	if len(deploymentID) == 15 {
		vm.metadataDeploys[apexIDTo18(deploymentID)] = result
	}
}

func (vm *VM) recordMetadataDeploymentFailure(deploymentID string, items []Value, failedItem Value, err error) {
	if vm.metadataDeploys == nil {
		vm.metadataDeploys = make(map[string]Value)
	}
	result := metadataDeployFailureResultObject(deploymentID, items, failedItem, err)
	vm.metadataDeploys[deploymentID] = result
	if len(deploymentID) == 15 {
		vm.metadataDeploys[apexIDTo18(deploymentID)] = result
	}
}

func (vm *VM) applyMetadataDeployment(item Value) error {
	if item.Kind != ValueObject {
		return unsupportedCallError("Metadata.Operations.enqueueDeployment " + string(item.Kind) + " metadata deploy")
	}
	switch item.Type {
	case "Metadata.CustomMetadata":
		return vm.applyCustomMetadataDeployment(item)
	case "Metadata.CustomObject":
		return vm.applyCustomObjectDeployment(item)
	case "Metadata.CustomField":
		return vm.applyCustomFieldDeployment(item)
	default:
		typeName := item.Type
		if typeName == "" {
			typeName = string(item.Kind)
		}
		return unsupportedCallError("Metadata.Operations.enqueueDeployment " + typeName + " metadata deploy")
	}
}

func (vm *VM) applyCustomMetadataDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomMetadata.fullName is required")
	}
	objectName, developerName := metadataCustomMetadataNames(fullName)
	if objectName == "" || developerName == "" {
		return fmt.Errorf("Metadata.CustomMetadata.fullName must be Type.Record")
	}
	state := vm.metadataCustomMetadataState(objectName)
	definition := state.Definition
	recordFields := map[string]storage.Value{
		"DeveloperName":    storage.StringValue(developerName),
		"MasterLabel":      storage.StringValue(metadataLabelOrDefault(item, developerName)),
		"Label":            storage.StringValue(metadataLabelOrDefault(item, developerName)),
		"NamespacePrefix":  storage.StringValue(metadataNamespacePrefix(vm.Org.Namespace, definition.APIName)),
		"QualifiedApiName": storage.StringValue(metadataQualifiedAPIName(vm.Org.Namespace, definition.APIName, developerName)),
	}
	values := item.Fields["values"]
	if values.Kind != ValueNull {
		if values.Kind != ValueList {
			return fmt.Errorf("Metadata.CustomMetadata.values must be a list")
		}
		for _, valueItem := range values.List {
			fieldName, fieldValue, err := vm.metadataCustomMetadataValue(definition, valueItem)
			if err != nil {
				return err
			}
			recordFields[fieldName] = fieldValue
		}
	}
	var recordID storage.ID
	for _, existing := range state.Records {
		if customDataRecordMatches(definition, "custom metadata", existing, developerName, vm.Org.Namespace) ||
			customDataRecordMatches(definition, "custom metadata", existing, fullName, vm.Org.Namespace) {
			recordID = existing.ID
			break
		}
	}
	if recordID == "" {
		recordID = nextMetadataRecordID(state)
	}
	record := storage.Record{ID: recordID, Object: definition.APIName, Fields: recordFields}
	record.Fields["Id"] = storage.IDValue(recordID)
	state.Records[recordID] = record
	vm.Org.Objects[definition.APIName] = state
	vm.clearMetadataCaches()
	return nil
}

func (vm *VM) applyCustomObjectDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomObject.fullName is required")
	}
	objectName := strings.TrimSpace(fullName)
	if !isCustomObjectLikeName(objectName) {
		return fmt.Errorf("Metadata.CustomObject.fullName must be a custom object API name")
	}
	objectName = storage.NamespaceTokenName(vm.Org.Namespace, objectName)
	state := vm.Org.Objects[objectName]
	state.Definition = state.Definition.Clone()
	state.Definition.APIName = objectName
	if state.Definition.Label == "" {
		state.Definition.Label = metadataTextFieldOrDefault(item, "label", strings.TrimSuffix(objectName, "__c"))
	}
	if state.Definition.PluralLabel == "" {
		state.Definition.PluralLabel = metadataTextFieldOrDefault(item, "pluralLabel", state.Definition.Label+"s")
	}
	if state.Definition.SharingModel == "" {
		state.Definition.SharingModel = metadataTextFieldOrDefault(item, "sharingModel", "ReadWrite")
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	if _, ok := state.Definition.Fields["Name"]; !ok {
		state.Definition.Fields["Name"] = storage.Field{APIName: "Name", Label: "Name", Type: storage.FieldString}
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customObject"}
	}
	storage.EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.Org.Objects[objectName] = state
	vm.clearMetadataCaches()
	return nil
}

func (vm *VM) applyCustomFieldDeployment(item Value) error {
	fullName, ok := metadataStringField(item, "fullName")
	if !ok || strings.TrimSpace(fullName) == "" {
		return fmt.Errorf("Metadata.CustomField.fullName is required")
	}
	objectName, fieldName := metadataCustomFieldNames(fullName)
	if objectName == "" || fieldName == "" {
		return fmt.Errorf("Metadata.CustomField.fullName must be Object.Field")
	}
	objectName = storage.NamespaceTokenName(vm.Org.Namespace, objectName)
	fieldName = storage.NamespaceTokenName(vm.Org.Namespace, fieldName)
	state, ok := vm.Org.Objects[objectName]
	if !ok {
		return fmt.Errorf("Metadata.CustomField.fullName references unknown object %s", objectName)
	}
	state.Definition = state.Definition.Clone()
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	fieldType, displayType := metadataCustomFieldType(item)
	field := state.Definition.Fields[fieldName]
	field.APIName = fieldName
	field.Label = metadataTextFieldOrDefault(item, "label", fieldName)
	field.Type = fieldType
	field.DisplayType = displayType
	field.Required = metadataBoolField(item, "required")
	field.ExternalID = metadataBoolField(item, "externalId")
	field.Unique = metadataBoolField(item, "unique")
	if referenceTo := metadataReferenceTo(item); len(referenceTo) > 0 {
		field.ReferenceTo = referenceTo
	}
	state.Definition.Fields[fieldName] = field
	vm.Org.Objects[objectName] = state
	vm.clearMetadataCaches()
	return nil
}

func (vm *VM) metadataCustomMetadataState(objectName string) storage.ObjectState {
	state := vm.Org.Objects[objectName]
	state.Definition = state.Definition.Clone()
	if state.Definition.APIName == "" {
		state.Definition.APIName = objectName
	}
	if state.Definition.KeyPrefix == "" {
		state.Definition.KeyPrefix = storage.AssignDeterministicPrefixes([]string{objectName}, nil)[objectName]
	}
	if state.Definition.Metadata == nil {
		state.Definition.Metadata = map[string]string{"kind": "customMetadata"}
	}
	if state.Definition.Fields == nil {
		state.Definition.Fields = make(map[string]storage.Field)
	}
	for _, field := range []storage.Field{
		{APIName: "DeveloperName", Type: storage.FieldString},
		{APIName: "MasterLabel", Type: storage.FieldString},
		{APIName: "Label", Type: storage.FieldString},
		{APIName: "NamespacePrefix", Type: storage.FieldString},
		{APIName: "QualifiedApiName", Type: storage.FieldString},
	} {
		if _, ok := state.Definition.Fields[field.APIName]; !ok {
			state.Definition.Fields[field.APIName] = field
		}
	}
	storage.EnsureStandardObjectFields(&state.Definition)
	if state.Records == nil {
		state.Records = make(map[storage.ID]storage.Record)
	}
	vm.Org.Objects[objectName] = state
	return state
}

func (vm *VM) metadataCustomMetadataValue(definition storage.ObjectDefinition, item Value) (string, storage.Value, error) {
	if item.Kind != ValueObject || item.Type != "Metadata.CustomMetadataValue" {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadata.values expects CustomMetadataValue entries")
	}
	fieldName, ok := metadataStringField(item, "field")
	if !ok || strings.TrimSpace(fieldName) == "" {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadataValue.field is required")
	}
	resolved, ok := storage.ResolveFieldName(definition, vm.Org.Namespace, fieldName)
	if !ok {
		value := item.Fields["value"]
		fieldType := metadataFieldTypeFromValue(value)
		resolved = storage.NamespaceTokenName(vm.Org.Namespace, fieldName)
		field := storage.Field{APIName: resolved, Type: fieldType}
		if state, exists := vm.Org.Objects[definition.APIName]; exists {
			state.Definition = state.Definition.Clone()
			if state.Definition.Fields == nil {
				state.Definition.Fields = make(map[string]storage.Field)
			}
			state.Definition.Fields[resolved] = field
			vm.Org.Objects[definition.APIName] = state
		}
		definition.Fields = map[string]storage.Field{resolved: field}
	}
	converted, err := storageValueFromVMForField(item.Fields["value"], definition.Fields[resolved].Type)
	if err != nil {
		return "", storage.Value{}, fmt.Errorf("Metadata.CustomMetadataValue.%s %v", fieldName, err)
	}
	return resolved, converted, nil
}

func metadataFieldTypeFromValue(value Value) storage.FieldType {
	switch value.Kind {
	case ValueBool:
		return storage.FieldBoolean
	case ValueInt:
		return storage.FieldInteger
	case ValueDecimal:
		return storage.FieldDecimal
	case ValueObject:
		switch strings.ToLower(value.Type) {
		case "date":
			return storage.FieldDate
		case "datetime":
			return storage.FieldDateTime
		case "id":
			return storage.FieldID
		}
	}
	return storage.FieldString
}

func (vm *VM) metadataRetrieve(args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 3 {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type and full names")
	}
	if args[0].Kind != ValueObject || args[0].Type != "Metadata.MetadataType" {
		return Null, fmt.Errorf("Metadata.Operations.retrieve expects metadata type")
	}
	if !strings.EqualFold(args[0].Text, "CustomMetadata") {
		return Null, unsupportedCallError("Metadata.Operations.retrieve " + args[0].Text)
	}
	names, err := metadataStringList(args[1])
	if err != nil {
		return Null, err
	}
	if vm.Org == nil {
		return List(), nil
	}
	out := make([]Value, 0, len(names))
	for _, fullName := range names {
		objectName, developerName := metadataCustomMetadataNames(fullName)
		objectName, ok := vm.resolveObjectName(objectName)
		if !ok {
			continue
		}
		state := vm.Org.Objects[objectName]
		if !storage.IsCustomMetadataDefinition(state.Definition) {
			continue
		}
		for _, record := range sortedCustomDataRecords(state.Records, state.Definition, "custom metadata", vm.Org.Namespace) {
			if record.System.IsDeleted {
				continue
			}
			if customDataRecordMatches(state.Definition, "custom metadata", record, developerName, vm.Org.Namespace) ||
				customDataRecordMatches(state.Definition, "custom metadata", record, fullName, vm.Org.Namespace) {
				out = append(out, metadataCustomMetadataObject(state.Definition, record))
				break
			}
		}
	}
	return List(out...), nil
}

func metadataCustomMetadataObject(definition storage.ObjectDefinition, record storage.Record) Value {
	item := Object("Metadata.CustomMetadata")
	developerName := firstStringField(record, "DeveloperName", "Name")
	fullName := strings.TrimSuffix(definition.APIName, "__mdt") + "." + developerName
	item.Fields["fullName"] = String(fullName)
	item.Fields["label"] = String(firstStringField(record, "MasterLabel", "Label", "DeveloperName", "Name"))
	values := make([]Value, 0, len(record.Fields))
	for fieldName, fieldValue := range record.Fields {
		if isCustomMetadataSystemField(fieldName) {
			continue
		}
		value := Object("Metadata.CustomMetadataValue")
		value.Fields["field"] = String(fieldName)
		value.Fields["value"] = vmValueFromStorage(fieldValue)
		values = append(values, value)
	}
	sort.Slice(values, func(i, j int) bool {
		return values[i].Fields["field"].Text < values[j].Fields["field"].Text
	})
	item.Fields["values"] = List(values...)
	return item
}

func isCustomMetadataSystemField(fieldName string) bool {
	switch fieldName {
	case "Id", "DeveloperName", "MasterLabel", "Label", "NamespacePrefix", "QualifiedApiName", "Name":
		return true
	default:
		return false
	}
}

func metadataStringField(value Value, field string) (string, bool) {
	raw, ok := value.Fields[field]
	if !ok || raw.Kind != ValueString {
		return "", false
	}
	return raw.Text, true
}

func metadataTextFieldOrDefault(value Value, field, fallback string) string {
	if raw, ok := metadataStringField(value, field); ok && strings.TrimSpace(raw) != "" {
		return strings.TrimSpace(raw)
	}
	return fallback
}

func metadataBoolField(value Value, field string) bool {
	raw, ok := value.Fields[field]
	return ok && raw.Kind == ValueBool && raw.Bool
}

func metadataCustomFieldNames(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), ".", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
}

func metadataCustomFieldType(item Value) (storage.FieldType, string) {
	raw := metadataTextFieldOrDefault(item, "type", "Text")
	switch strings.ToLower(strings.ReplaceAll(raw, "_", "")) {
	case "checkbox", "boolean":
		return storage.FieldBoolean, "BOOLEAN"
	case "number", "integer", "int":
		return storage.FieldInteger, "INTEGER"
	case "currency", "percent", "double", "decimal":
		return storage.FieldDecimal, "DOUBLE"
	case "date":
		return storage.FieldDate, "DATE"
	case "datetime":
		return storage.FieldDateTime, "DATETIME"
	case "picklist", "multipicklist":
		return storage.FieldPicklist, "PICKLIST"
	case "lookup", "masterdetail", "reference":
		return storage.FieldReference, "REFERENCE"
	case "textarea", "longtextarea", "html", "email", "phone", "url", "text":
		return storage.FieldString, "STRING"
	default:
		return storage.FieldString, strings.ToUpper(raw)
	}
}

func metadataReferenceTo(item Value) []string {
	raw, ok := item.Fields["referenceTo"]
	if !ok || raw.Kind == ValueNull {
		return nil
	}
	switch raw.Kind {
	case ValueString:
		if strings.TrimSpace(raw.Text) == "" {
			return nil
		}
		return []string{strings.TrimSpace(raw.Text)}
	case ValueList, ValueSet:
		items := raw.List
		if raw.Kind == ValueSet {
			items = raw.Set
		}
		out := make([]string, 0, len(items))
		for _, item := range items {
			if item.Kind == ValueString && strings.TrimSpace(item.Text) != "" {
				out = append(out, strings.TrimSpace(item.Text))
			}
		}
		return out
	default:
		return nil
	}
}

func metadataStringList(value Value) ([]string, error) {
	if value.Kind != ValueList && value.Kind != ValueSet {
		return nil, fmt.Errorf("Metadata.Operations.retrieve expects full names list")
	}
	items := value.List
	if value.Kind == ValueSet {
		items = value.Set
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item.Kind != ValueString {
			return nil, fmt.Errorf("Metadata.Operations.retrieve expects String full names")
		}
		out = append(out, item.Text)
	}
	return out, nil
}

func metadataCustomMetadataNames(fullName string) (string, string) {
	parts := strings.SplitN(strings.TrimSpace(fullName), ".", 2)
	if len(parts) != 2 {
		return "", strings.TrimSpace(fullName)
	}
	objectName := strings.TrimSpace(parts[0])
	if !strings.HasSuffix(strings.ToLower(objectName), "__mdt") {
		objectName += "__mdt"
	}
	return objectName, strings.TrimSpace(parts[1])
}

func metadataLabelOrDefault(item Value, developerName string) string {
	if label, ok := metadataStringField(item, "label"); ok && strings.TrimSpace(label) != "" {
		return label
	}
	return developerName
}

func metadataNamespacePrefix(namespace, objectName string) string {
	if namespace == "" {
		return ""
	}
	if strings.HasPrefix(objectName, namespace+"__") {
		return namespace
	}
	return ""
}

func metadataQualifiedAPIName(namespace, objectName, developerName string) string {
	if metadataNamespacePrefix(namespace, objectName) != "" {
		return namespace + "__" + developerName
	}
	return developerName
}

func nextMetadataRecordID(state storage.ObjectState) storage.ID {
	generator := storage.NewIDGenerator(map[string]string{state.Definition.APIName: state.Definition.KeyPrefix})
	for {
		id, err := generator.Next(state.Definition.APIName)
		if err != nil {
			return storage.ID(state.Definition.KeyPrefix + "000000000001")
		}
		if _, exists := state.Records[id]; !exists {
			return id
		}
	}
}

func namedEnumValues(typeName string, names []string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s.values expects 0 arguments", typeName)
	}
	values := make([]Value, 0, len(names))
	for i, name := range names {
		value := Value{Kind: ValueObject, Type: typeName, Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func loggingLevelValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("LoggingLevel.values expects 0 arguments")
	}
	values := make([]Value, 0, len(loggingLevelNames))
	for i, name := range loggingLevelNames {
		value := Value{Kind: ValueObject, Type: "LoggingLevel", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func (vm *VM) callEnumStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method, "values", "valueOf")
	if method != "values" && method != "valueOf" {
		return Null, false, nil
	}
	if value, handled, err := vm.callGeneratedPlatformEnumStaticMember(typeName, method, args); handled || err != nil {
		return value, handled, err
	}
	if typeName == "LoggingLevel" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := loggingLevelValues(args)
		return value, true, err
	}
	if typeName == "RoundingMode" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := roundingModeValues(args)
		return value, true, err
	}
	if strings.EqualFold(typeName, "Schema.DisplayType") || strings.EqualFold(typeName, "DisplayType") {
		value, err := callNamedEnumStaticMember("Schema.DisplayType", schemaDisplayTypeNames, method, args)
		return value, true, err
	}
	if strings.EqualFold(typeName, "Schema.SOAPType") || strings.EqualFold(typeName, "SOAPType") {
		value, err := callNamedEnumStaticMember("Schema.SOAPType", schemaSOAPTypeNames, method, args)
		return value, true, err
	}
	if typeName == "Metadata.DeployStatus" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := metadataDeployStatusValues(args)
		return value, true, err
	}
	if typeName == "Metadata.MetadataType" {
		if method != "values" {
			return Null, false, nil
		}
		value, err := metadataMetadataTypeValues(args)
		return value, true, err
	}
	class, ok := vm.resolveEnumClass(typeName)
	if !ok || len(class.EnumValues) == 0 {
		return Null, false, nil
	}
	if err := vm.ensureClassInitialized(class.Name); err != nil {
		return Null, true, err
	}
	class, _ = vm.lookupClass(class.Name)
	switch method {
	case "values":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.values expects 0 arguments", typeName)
		}
		values := make([]Value, 0, len(class.EnumValues))
		for i, name := range class.EnumValues {
			value := Value{Kind: ValueObject, Type: class.Name, Text: name}
			value.Fields = map[string]Value{"ordinal": Int(int64(i))}
			values = append(values, value)
		}
		return List(values...), true, nil
	case "valueOf":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		argText, ok := stringLikeValueText(args[0])
		if !ok {
			return Null, true, newExceptionError("TypeException", fmt.Sprintf("%s.valueOf expects String", typeName))
		}
		for i, name := range class.EnumValues {
			if name == argText {
				value := Value{Kind: ValueObject, Type: class.Name, Text: name}
				value.Fields = map[string]Value{"ordinal": Int(int64(i))}
				return value, true, nil
			}
		}
		return Null, true, newExceptionError("NoSuchElementException", fmt.Sprintf("No enum value found called %s", argText))
	default:
		return Null, false, nil
	}
}

func callNamedEnumStaticMember(typeName string, names []string, method string, args []Value) (Value, error) {
	switch method {
	case "values":
		return namedEnumValues(typeName, names, args)
	case "valueOf":
		if len(args) != 1 {
			return Null, fmt.Errorf("%s.valueOf expects String", typeName)
		}
		argText, ok := stringLikeValueText(args[0])
		if !ok {
			return Null, newExceptionError("TypeException", fmt.Sprintf("%s.valueOf expects String", typeName))
		}
		if value, ok := namedEnumStaticValue(typeName, names, typeName+"."+argText); ok {
			return value, nil
		}
		return Null, newExceptionError("NoSuchElementException", fmt.Sprintf("No enum value found called %s", argText))
	default:
		return Null, fmt.Errorf("%s.%s is not supported", typeName, method)
	}
}

func (vm *VM) callGeneratedPlatformEnumStaticMember(typeName, method string, args []Value) (Value, bool, error) {
	generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]
	if !ok || generated.Kind != apexast.DeclarationEnum {
		return Null, false, nil
	}
	names := generatedPlatformEnumNames(generated)
	if len(names) == 0 {
		return Null, false, nil
	}
	switch method {
	case "values":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.values expects 0 arguments", generated.Name)
		}
		values := make([]Value, 0, len(names))
		for i, name := range names {
			value := Value{Kind: ValueObject, Type: generated.Name, Text: name}
			value.Fields = map[string]Value{"ordinal": Int(int64(i))}
			values = append(values, value)
		}
		return List(values...), true, nil
	case "valueOf":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.valueOf expects String", generated.Name)
		}
		argText, ok := stringLikeValueText(args[0])
		if !ok {
			return Null, true, newExceptionError("TypeException", fmt.Sprintf("%s.valueOf expects String", generated.Name))
		}
		for i, name := range names {
			if strings.EqualFold(name, argText) {
				value := Value{Kind: ValueObject, Type: generated.Name, Text: name}
				value.Fields = map[string]Value{"ordinal": Int(int64(i))}
				return value, true, nil
			}
		}
		return Null, true, newExceptionError("NoSuchElementException", fmt.Sprintf("No enum value found called %s", argText))
	default:
		return Null, false, nil
	}
}

func generatedPlatformEnumNames(generated generatedPlatformType) []string {
	names := make([]string, 0, len(generated.StaticFields))
	seen := make(map[string]bool, len(generated.StaticFields))
	for _, name := range generated.StaticFieldOrder {
		field, ok := generated.StaticFields[name]
		if !ok {
			continue
		}
		if field.Type == "" || strings.EqualFold(field.Type, generated.Name) {
			names = append(names, name)
			seen[strings.ToLower(name)] = true
		}
	}
	remaining := make([]string, 0)
	for name, field := range generated.StaticFields {
		if seen[strings.ToLower(name)] {
			continue
		}
		if field.Type == "" || strings.EqualFold(field.Type, generated.Name) {
			remaining = append(remaining, name)
		}
	}
	sort.Strings(remaining)
	names = append(names, remaining...)
	return names
}

func stringLikeValueText(value Value) (string, bool) {
	if value.Kind == ValueString {
		return value.Text, true
	}
	if value.Kind == ValueObject && value.Text != "" {
		return value.Text, true
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "String") {
		if text, ok := platformScalarObjectText(value); ok {
			return text, true
		}
	}
	return "", false
}

func generatedPlatformEnumOrdinal(generated generatedPlatformType, value string) int {
	for i, name := range generatedPlatformEnumNames(generated) {
		if strings.EqualFold(name, value) {
			return i
		}
	}
	return -1
}

func roundingModeValues(args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("RoundingMode.values expects 0 arguments")
	}
	values := make([]Value, 0, len(roundingModeNames))
	for i, name := range roundingModeNames {
		value := Value{Kind: ValueObject, Type: "RoundingMode", Text: name}
		value.Fields = map[string]Value{"ordinal": Int(int64(i))}
		values = append(values, value)
	}
	return List(values...), nil
}

func (vm *VM) callEnumMember(receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method, "equals", "name", "ordinal", "toString")
	if receiver.Type == "JSONToken" {
		if method == "equals" {
			if len(args) != 1 {
				return Null, true, fmt.Errorf("JSONToken.equals expects 1 argument")
			}
			return Bool(enumValuesEqual(receiver, args[0])), true, nil
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("JSONToken.%s expects 0 arguments", method)
		}
		switch method {
		case "name", "toString":
			return String(receiver.Text), true, nil
		case "ordinal":
			for i, name := range jsonTokenNames {
				if name == receiver.Text {
					return Int(int64(i)), true, nil
				}
			}
			return Int(-1), true, nil
		default:
			return Null, false, nil
		}
	}
	if receiver.Type == "ApexPages.Severity" {
		return callNamedEnumMember("ApexPages.Severity", apexPagesSeverityNames, receiver, method, args)
	}
	if receiver.Type == "LoggingLevel" {
		return callNamedEnumMember("LoggingLevel", loggingLevelNames, receiver, method, args)
	}
	if receiver.Type == "RoundingMode" {
		return callNamedEnumMember("RoundingMode", roundingModeNames, receiver, method, args)
	}
	if receiver.Type == "AccessType" {
		return callNamedEnumMember("AccessType", accessTypeNames, receiver, method, args)
	}
	if receiver.Type == "TriggerOperation" {
		return callNamedEnumMember("TriggerOperation", triggerOperationNames, receiver, method, args)
	}
	if receiver.Type == "StatusCode" {
		return callStatusCodeMember(receiver, method, args)
	}
	if receiver.Type == "Schema.DisplayType" {
		return callNamedEnumMember("Schema.DisplayType", schemaDisplayTypeNames, receiver, method, args)
	}
	if receiver.Type == "Schema.SOAPType" {
		return callNamedEnumMember("Schema.SOAPType", schemaSOAPTypeNames, receiver, method, args)
	}
	if receiver.Type == "Metadata.DeployStatus" {
		return callNamedEnumMember("Metadata.DeployStatus", metadataDeployStatusNames, receiver, method, args)
	}
	if receiver.Type == "Metadata.MetadataType" {
		return callNamedEnumMember("Metadata.MetadataType", metadataMetadataTypeNames, receiver, method, args)
	}
	if generated, ok := generatedPlatformTypeIndex[strings.ToLower(receiver.Type)]; ok && generated.Kind == apexast.DeclarationEnum {
		return callGeneratedPlatformEnumMember(generated, receiver, method, args)
	}
	class, ok := vm.Classes[receiver.Type]
	if !ok || len(class.EnumValues) == 0 {
		return Null, false, nil
	}
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", receiver.Type)
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range class.EnumValues {
			if name == receiver.Text {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func callGeneratedPlatformEnumMember(generated generatedPlatformType, receiver Value, method string, args []Value) (Value, bool, error) {
	method = canonicalStdlibMemberName(method, "equals", "name", "ordinal", "toString")
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", receiver.Type)
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range generatedPlatformEnumNames(generated) {
			if strings.EqualFold(name, receiver.Text) {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func enumValuesEqual(left, right Value) bool {
	if right.Kind != ValueObject {
		return false
	}
	return strings.EqualFold(left.Type, right.Type) && left.Text == right.Text
}

func metadataDeployDetailsObject() Value {
	details := Object("Metadata.DeployDetails")
	details.Fields["componentFailures"] = typedList("List<Metadata.DeployMessage>")
	details.Fields["componentSuccesses"] = typedList("List<Metadata.DeployMessage>")
	details.Fields["runTestResult"] = Null
	return details
}

func metadataDeployResultObject(deploymentID string, items []Value) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("SUCCEEDED")
	result.Fields["success"] = Bool(true)
	result.Fields["done"] = Bool(true)
	result.Fields["numberComponentErrors"] = Int(0)
	result.Fields["numberComponentsDeployed"] = Int(int64(len(items)))
	result.Fields["numberComponentsTotal"] = Int(int64(len(items)))
	result.Fields["numberTestErrors"] = Int(0)
	result.Fields["numberTestsCompleted"] = Int(0)
	result.Fields["checkOnly"] = Bool(false)
	details := metadataDeployDetailsObject()
	successes := make([]Value, 0, len(items))
	for _, item := range items {
		successes = append(successes, metadataDeploySuccessMessage(item))
	}
	details.Fields["componentSuccesses"] = List(successes...)
	result.Fields["details"] = details
	return result
}

func metadataDeployFailureResultObject(deploymentID string, items []Value, failedItem Value, err error) Value {
	result := Object("Metadata.DeployResult")
	result.Fields["id"] = platformScalar("Id", deploymentID)
	result.Fields["status"] = metadataDeployStatusValue("FAILED")
	result.Fields["success"] = Bool(false)
	result.Fields["done"] = Bool(true)
	result.Fields["numberComponentErrors"] = Int(1)
	result.Fields["numberComponentsDeployed"] = Int(0)
	result.Fields["numberComponentsTotal"] = Int(int64(len(items)))
	result.Fields["numberTestErrors"] = Int(0)
	result.Fields["numberTestsCompleted"] = Int(0)
	result.Fields["checkOnly"] = Bool(false)
	details := metadataDeployDetailsObject()
	details.Fields["componentFailures"] = List(metadataDeployFailureMessage(failedItem, err))
	result.Fields["details"] = details
	return result
}

func metadataDeploySuccessMessage(item Value) Value {
	message := Object("Metadata.DeployMessage")
	fullName := metadataDeployItemFullName(item)
	message.Fields["fullName"] = String(fullName)
	message.Fields["fileName"] = String(fullName)
	message.Fields["componentType"] = String(metadataDeployItemComponentType(item))
	message.Fields["success"] = Bool(true)
	message.Fields["problem"] = Null
	return message
}

func metadataDeployFailureMessage(item Value, err error) Value {
	message := Object("Metadata.DeployMessage")
	fullName := metadataDeployItemFullName(item)
	message.Fields["fullName"] = String(fullName)
	message.Fields["fileName"] = String(fullName)
	message.Fields["componentType"] = String(metadataDeployItemComponentType(item))
	message.Fields["success"] = Bool(false)
	if err == nil {
		message.Fields["problem"] = String("metadata deployment failed")
	} else {
		message.Fields["problem"] = String(err.Error())
	}
	return message
}

func metadataDeployItemFullName(item Value) string {
	if item.Kind == ValueObject {
		if fullName, ok := metadataStringField(item, "fullName"); ok {
			return fullName
		}
	}
	return ""
}

func metadataDeployItemComponentType(item Value) string {
	if item.Kind != ValueObject {
		return string(item.Kind)
	}
	switch item.Type {
	case "Metadata.CustomMetadata":
		return "CustomMetadata"
	case "":
		return string(item.Kind)
	default:
		return strings.TrimPrefix(item.Type, "Metadata.")
	}
}

func metadataAsyncResultObject(id string, done bool, state, message string) Value {
	result := Object("Metadata.AsyncResult")
	result.Fields["id"] = platformScalar("Id", id)
	result.Fields["done"] = Bool(done)
	result.Fields["state"] = String(state)
	result.Fields["statusCode"] = Null
	if message == "" {
		result.Fields["message"] = Null
	} else {
		result.Fields["message"] = String(message)
	}
	return result
}

func metadataDeployStatusValue(name string) Value {
	return Value{Kind: ValueObject, Type: "Metadata.DeployStatus", Text: name, Fields: map[string]Value{"ordinal": Int(metadataDeployStatusOrdinal(name))}}
}

func metadataDeployStatusOrdinal(name string) int64 {
	for i, candidate := range metadataDeployStatusNames {
		if candidate == name {
			return int64(i)
		}
	}
	return -1
}

func cloneMetadataDeployResult(result Value) Value {
	cloned := cloneValue(result)
	if cloned.Fields == nil {
		cloned.Fields = make(map[string]Value)
	}
	return cloned
}

func callNamedEnumMember(typeName string, names []string, receiver Value, method string, args []Value) (Value, bool, error) {
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.equals expects 1 argument", typeName)
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", typeName, method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		for i, name := range names {
			if name == receiver.Text {
				return Int(int64(i)), true, nil
			}
		}
		return Int(-1), true, nil
	default:
		return Null, false, nil
	}
}

func callStatusCodeMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if method == "equals" {
		if len(args) != 1 {
			return Null, true, fmt.Errorf("StatusCode.equals expects 1 argument")
		}
		return Bool(enumValuesEqual(receiver, args[0])), true, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("StatusCode.%s expects 0 arguments", method)
	}
	switch method {
	case "name", "toString":
		return String(receiver.Text), true, nil
	case "ordinal":
		return Int(0), true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) callCommerceInventoryServiceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "commerce_inventory.CommerceInventoryService") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "getinventorylevel":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("commerce_inventory.CommerceInventoryService.getInventoryLevel expects InventoryLevelsRequest")
		}
		response := Object("commerce_inventory.InventoryLevelsResponse")
		items := Set()
		items.Type = "Set<commerce_inventory.InventoryLevelsItemResponse>"
		response.Fields["itemsInventoryLevels"] = items
		return response, receiver, false, true, nil
	case "getreservation":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("commerce_inventory.CommerceInventoryService.getReservation expects reservationId String")
		}
		reservation := Object("commerce_inventory.InventoryReservation")
		reservation.Fields["id"] = platformScalar("Id", args[0].Text)
		reservation.Fields["durationInSeconds"] = Int(0)
		reservation.Fields["items"] = typedList("List<commerce_inventory.InventoryItemReservation>")
		return reservation, receiver, false, true, nil
	case "checkinventory":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("commerce_inventory.CommerceInventoryService.checkInventory expects InventoryCheckAvailability")
		}
		check := Object("commerce_inventory.InventoryCheckAvailability")
		items := Set()
		items.Type = "Set<commerce_inventory.InventoryCheckItemAvailability>"
		check.Fields["inventoryCheckItemAvailability"] = items
		return check, receiver, false, true, nil
	case "deletereservation", "upsertreservation":
		return Null, receiver, false, true, unsupportedCallError("commerce_inventory.CommerceInventoryService." + method + " local commerce inventory mutation surface")
	default:
		return Null, receiver, false, false, nil
	}
}

func compareVersionValues(left, right Value) int {
	for _, field := range []string{"major", "minor"} {
		lv := versionComponent(left, field)
		rv := versionComponent(right, field)
		if lv < rv {
			return -1
		}
		if lv > rv {
			return 1
		}
	}
	if !versionPatchSpecified(left) || !versionPatchSpecified(right) {
		return 0
	}
	lv := versionComponent(left, "patch")
	rv := versionComponent(right, "patch")
	if lv < rv {
		return -1
	}
	if lv > rv {
		return 1
	}
	return 0
}

func versionComponent(version Value, field string) int64 {
	value, ok := version.Fields[field]
	if !ok || value.Kind != ValueInt {
		return 0
	}
	return value.Int
}

func versionValueString(version Value) string {
	if !versionPatchSpecified(version) {
		return fmt.Sprintf("%d.%d", versionComponent(version, "major"), versionComponent(version, "minor"))
	}
	return fmt.Sprintf("%d.%d.%d", versionComponent(version, "major"), versionComponent(version, "minor"), versionComponent(version, "patch"))
}

func versionPatchSpecified(version Value) bool {
	value, ok := version.Fields["__gladePatchSpecified"]
	return ok && value.Kind == ValueBool && value.Bool
}

func (vm *VM) callCanvasMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	switch receiver.Type {
	case "Canvas.RenderContext":
		method = canonicalStdlibMemberName(method, "getApplicationContext", "getEnvironmentContext")
		switch method {
		case "getApplicationContext":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.RenderContext.getApplicationContext expects 0 arguments")
			}
			return receiver.Fields["applicationContext"], receiver, false, true, nil
		case "getEnvironmentContext":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.RenderContext.getEnvironmentContext expects 0 arguments")
			}
			return receiver.Fields["environmentContext"], receiver, false, true, nil
		}
	case "Canvas.ApplicationContext":
		method = canonicalStdlibMemberName(method, "getCanvasUrl", "getDeveloperName", "getName", "getNamespace", "getVersion", "setCanvasUrlPath")
		switch method {
		case "getCanvasUrl", "getDeveloperName", "getName", "getNamespace", "getVersion":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.ApplicationContext.%s expects 0 arguments", method)
			}
			return canvasStringField(receiver, strings.TrimPrefix(method, "get")), receiver, false, true, nil
		case "setCanvasUrlPath":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Canvas.ApplicationContext.setCanvasUrlPath expects String")
			}
			receiver.Fields["canvasUrl"] = args[0]
			return Null, receiver, true, true, nil
		}
	case "Canvas.EnvironmentContext":
		method = canonicalStdlibMemberName(method, "addEntityField", "addEntityFields", "getDisplayLocation", "getEntityFields", "getLocationUrl", "getParametersAsJSON", "getSublocation", "setParametersAsJSON")
		switch method {
		case "addEntityField":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Canvas.EnvironmentContext.addEntityField expects String")
			}
			fields := receiver.Fields["entityFields"]
			if fields.Kind != ValueList {
				fields = typedList("List<String>")
			}
			fields.List = append(fields.List, args[0])
			receiver.Fields["entityFields"] = fields
			return Null, receiver, true, true, nil
		case "addEntityFields":
			if len(args) != 1 || args[0].Kind != ValueSet {
				return Null, receiver, false, true, fmt.Errorf("Canvas.EnvironmentContext.addEntityFields expects Set<String>")
			}
			fields := receiver.Fields["entityFields"]
			if fields.Kind != ValueList {
				fields = typedList("List<String>")
			}
			fields.List = append(fields.List, args[0].Set...)
			receiver.Fields["entityFields"] = fields
			return Null, receiver, true, true, nil
		case "getDisplayLocation", "getLocationUrl", "getSublocation":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.EnvironmentContext.%s expects 0 arguments", method)
			}
			return canvasStringField(receiver, strings.TrimPrefix(method, "get")), receiver, false, true, nil
		case "getEntityFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.EnvironmentContext.getEntityFields expects 0 arguments")
			}
			fields := receiver.Fields["entityFields"]
			if fields.Kind != ValueList {
				return typedList("List<String>"), receiver, false, true, nil
			}
			return fields, receiver, false, true, nil
		case "getParametersAsJSON":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.EnvironmentContext.getParametersAsJSON expects 0 arguments")
			}
			if raw, ok := receiver.Fields["parametersAsJSON"]; ok && raw.Kind == ValueString {
				return raw, receiver, false, true, nil
			}
			text, err := canvasContextJSON(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return String(text), receiver, false, true, nil
		case "setParametersAsJSON":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Canvas.EnvironmentContext.setParametersAsJSON expects String")
			}
			receiver.Fields["parametersAsJSON"] = args[0]
			return Null, receiver, true, true, nil
		}
	case "Canvas.CanvasLifecycleHandler":
		method = canonicalStdlibMemberName(method, "excludeContextTypes", "onRender")
		switch method {
		case "excludeContextTypes":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Canvas.CanvasLifecycleHandler.excludeContextTypes expects 0 arguments")
			}
			return typedSet("Set<Canvas.ContextTypeEnum>"), receiver, false, true, nil
		case "onRender":
			if len(args) != 1 || (args[0].Kind != ValueNull && (args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Canvas.RenderContext"))) {
				return Null, receiver, false, true, fmt.Errorf("Canvas.CanvasLifecycleHandler.onRender expects Canvas.RenderContext")
			}
			return Null, receiver, false, true, nil
		}
	}
	_ = vm
	_ = result
	return Null, receiver, false, false, nil
}

func canvasStringField(receiver Value, suffix string) Value {
	field := canvasContextFieldName(suffix)
	if value, ok := receiver.Fields[field]; ok && (value.Kind == ValueString || value.Kind == ValueNull) {
		return value
	}
	return String("")
}

func canvasContextJSON(receiver Value) (string, error) {
	values := make(map[string]string)
	for field, value := range receiver.Fields {
		if value.Kind == ValueString {
			values[field] = value.Text
		}
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (vm *VM) callPlatformObjectMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	if value, updated, mutated, handled, err := vm.callSfsqlqueryHarnessMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := callContextIndustriesContextMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := callOrgInstrumentationOperationMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := callOrgInstrumentationContextMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := callOrgInstrumentationServiceMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if strings.EqualFold(receiver.Type, "Site.UrlRewriter") {
		switch strings.ToLower(method) {
		case "generateurlfor":
			if len(args) != 1 || args[0].Kind != ValueList {
				return Null, receiver, false, true, fmt.Errorf("Site.UrlRewriter.generateUrlFor expects PageReference list")
			}
			return cloneValue(args[0]), receiver, false, true, nil
		case "maprequesturl":
			if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "PageReference") {
				return Null, receiver, false, true, fmt.Errorf("Site.UrlRewriter.mapRequestUrl expects PageReference")
			}
			return args[0], receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "LiveAgent.LiveChatRouter") {
		if !strings.EqualFold(method, "doRouting") {
			return Null, receiver, false, false, nil
		}
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("LiveAgent.LiveChatRouter.doRouting expects routing request list")
		}
		return Null, receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "Support.WorkCapacityCalculation") {
		switch strings.ToLower(method) {
		case "calculateactualusage", "calculateestimatedusage":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Support.WorkCapacityCalculation.%s expects WorkCapacityInfo", method)
			}
			return Object("Support.WorkCapacityDuration"), receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "Support.MilestoneTriggerTimeCalculator") && strings.EqualFold(method, "calculateMilestoneTriggerTime") {
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("Support.MilestoneTriggerTimeCalculator.calculateMilestoneTriggerTime expects String, String")
		}
		return Int(0), receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "ChatterAnswers.AccountCreator") && strings.EqualFold(method, "createAccount") {
		if len(args) != 3 {
			return Null, receiver, false, true, fmt.Errorf("ChatterAnswers.AccountCreator.createAccount expects first name, last name, and user Id")
		}
		return String("001000000000001"), receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "Support.EinsteinBots") && strings.EqualFold(method, "sendMessageToBot") {
		if len(args) != 3 {
			return Null, receiver, false, true, fmt.Errorf("Support.EinsteinBots.sendMessageToBot expects bot Id, bot version Id, and prompt")
		}
		return String(""), receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "Support.EmailTemplateSelector") && strings.EqualFold(method, "getDefaultEmailTemplateId") {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Support.EmailTemplateSelector.getDefaultEmailTemplateId expects context Id")
		}
		return Null, receiver, false, true, nil
	}
	if value, updated, mutated, handled, err := vm.callCommerceInventoryServiceMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if strings.HasPrefix(receiver.Type, "RichMessaging.") {
		switch receiver.Type {
		case "RichMessaging.AuthRequestHandler":
			if strings.EqualFold(method, "handleAuthRequest") {
				if len(args) != 1 {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.AuthRequestHandler.handleAuthRequest expects AuthRequestResponse")
				}
				return Object("RichMessaging.AuthRequestResult"), receiver, false, true, nil
			}
		case "RichMessaging.ProcessCatalogOrderHandler":
			if strings.EqualFold(method, "processCatalogOrderRequest") {
				if len(args) != 1 {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.ProcessCatalogOrderHandler.processCatalogOrderRequest expects ProcessCatalogOrderRequest")
				}
				return Object("RichMessaging.ProcessCatalogOrderResult"), receiver, false, true, nil
			}
		case "RichMessaging.ProcessFormHandler":
			if strings.EqualFold(method, "processFormRequest") {
				if len(args) != 1 {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.ProcessFormHandler.processFormRequest expects ProcessFormResponse")
				}
				return Null, receiver, false, true, nil
			}
		case "RichMessaging.ProcessPaymentHandler":
			if strings.EqualFold(method, "processPaymentRequest") {
				if len(args) != 1 {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.ProcessPaymentHandler.processPaymentRequest expects ProcessPaymentRequest")
				}
				return Object("RichMessaging.ProcessPaymentResult"), receiver, false, true, nil
			}
		}
	}
	if strings.EqualFold(receiver.Type, "ApptBooking.WaitlistController") {
		switch strings.ToLower(method) {
		case "call":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueMap {
				return Null, receiver, false, true, fmt.Errorf("ApptBooking.WaitlistController.call expects action String and args Map")
			}
			return typedMap("Map<String,Object>"), receiver, false, true, nil
		case "invokemethod":
			if len(args) != 4 || args[0].Kind != ValueString || args[1].Kind != ValueMap || args[2].Kind != ValueMap || args[3].Kind != ValueMap {
				return Null, receiver, false, true, fmt.Errorf("ApptBooking.WaitlistController.invokeMethod expects action String, input Map, output Map, and options Map")
			}
			return typedMap("Map<String,Object>"), receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "NLPPredictions.PredictionHandler") {
		switch strings.ToLower(method) {
		case "handlepredictionrequest":
			if len(args) != 1 || args[0].Kind != ValueObject || !strings.HasPrefix(strings.ToLower(args[0].Type), "nlppredictions.predictionrequestcontext") {
				return Null, receiver, false, true, fmt.Errorf("NLPPredictions.PredictionHandler.handlePredictionRequest expects PredictionRequestContext")
			}
			return Null, receiver, false, true, nil
		case "handlepredictionresponse":
			if len(args) != 1 || args[0].Kind != ValueObject || !strings.HasPrefix(strings.ToLower(args[0].Type), "nlppredictions.predictionresponsecontext") {
				return Null, receiver, false, true, fmt.Errorf("NLPPredictions.PredictionHandler.handlePredictionResponse expects PredictionResponseContext")
			}
			return Null, receiver, false, true, nil
		}
	}
	if value, updated, mutated, handled, err := callUserProvisioningBatchableMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if strings.EqualFold(receiver.Type, "UserProvisioning.FlowProvisionBase") {
		switch strings.ToLower(method) {
		case "getflowname", "getflownamespace":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UserProvisioning.FlowProvisionBase.%s expects 0 arguments", method)
			}
			return String(""), receiver, false, true, nil
		case "hasflow", "hasfloworapex":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UserProvisioning.FlowProvisionBase.%s expects 0 arguments", method)
			}
			return Bool(false), receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "UserProvisioning.UserProvisioningPlugin") {
		switch strings.ToLower(method) {
		case "builddescribecall", "describe":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UserProvisioning.UserProvisioningPlugin.%s expects 0 arguments", method)
			}
			return Object("Process.PluginDescribeResult"), receiver, false, true, nil
		case "getpluginclassname":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UserProvisioning.UserProvisioningPlugin.getPluginClassName expects 0 arguments")
			}
			return String("UserProvisioning.UserProvisioningPlugin"), receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "UserProvisioning.UserProvisioningProcessHandler") ||
		strings.EqualFold(receiver.Type, "UserProvisioning.DummyConnectorApexHandler") {
		if strings.EqualFold(method, "invoke") {
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("%s.invoke expects ProvisioningProcessHandlerInput", receiver.Type)
			}
			return Object("UserProvisioning.ProvisioningProcessHandlerOutput"), receiver, false, true, nil
		}
	}
	if value, handled, err := callPlatformCallbackDefaultMember(receiver, method, args); handled || err != nil {
		return value, receiver, false, true, err
	}
	if value, updated, mutated, handled, err := vm.callPackagedControllerMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := vm.callIndustryControllerMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := callWaveQueryMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, updated, mutated, handled, err := callCompressionZipMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, handled, err := vm.callGeneratedOptionalWrapperMember(receiver, method, args); handled || err != nil {
		return value, receiver, false, true, err
	}
	if value, updated, mutated, handled, err := vm.callSlackLocalHarnessMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if strings.EqualFold(receiver.Type, "Slack.UserSession") || strings.EqualFold(receiver.Type, "Slack.TestHarness.UserSession") {
		for _, receiverType := range []string{runtimeObjectType(receiver), receiver.Static, receiver.Type, slackTestHarnessRuntimeType(receiver.Type), "Slack.UserSession"} {
			if generated, ok := vm.generatedPlatformMethodForArgs(receiverType, method, args, false); ok {
				if generated.ClassName == "" {
					generated.ClassName = slackTestHarnessRuntimeType(receiverType)
				}
				if value, handled := vm.generatedSlackTestHarnessDefaultReturn(generated, receiver, args); handled {
					return value, receiver, true, true, nil
				}
			}
		}
	}
	if slackLocalClientHarnessType(receiver.Type) {
		if generated, ok := vm.generatedPlatformMethodForArgs(receiver.Type, method, args, false); ok && slackLocalClientHarnessMethod(generated) {
			return vm.generatedPlatformMethodDefaultReturn(generated, receiver, args), receiver, false, true, nil
		}
	}
	if strings.HasPrefix(receiver.Type, "QuickAction.") {
		if value, updated, mutated, handled, err := callQuickActionMember(receiver, method, args); handled || err != nil {
			return value, updated, mutated, true, err
		}
	}
	if strings.HasPrefix(receiver.Type, "Canvas.") {
		if value, updated, mutated, handled, err := vm.callCanvasMember(receiver, method, args, result); handled || err != nil {
			return value, updated, mutated, true, err
		}
	}
	if strings.EqualFold(receiver.Type, "SandboxContext") {
		switch strings.ToLower(method) {
		case "organizationid":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SandboxContext.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["organizationId"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "sandboxid":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SandboxContext.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["sandboxId"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "sandboxname":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SandboxContext.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["sandboxName"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	}
	if isExceptionType(receiver.Type) {
		switch method {
		case "getMessage":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getMessage expects 0 arguments", receiver.Type)
			}
			if isThrownAuraHandledExceptionWithoutExplicitMessage(receiver) {
				return String("Script-thrown exception"), receiver, false, true, nil
			}
			if message, ok := receiver.Fields["message"]; ok {
				return message, receiver, false, true, nil
			}
			if exceptionTypeName(receiver.Type) == "JSONException" {
				return String("Script-thrown exception"), receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "setMessage":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("%s.setMessage expects 1 argument", receiver.Type)
			}
			if args[0].Kind == ValueString || args[0].Kind == ValueNull {
				receiver.Fields["message"] = args[0]
			} else {
				receiver.Fields["message"] = String(args[0].String())
			}
			receiver.Fields["__messageSet"] = Bool(true)
			return Null, receiver, true, true, nil
		case "getNumDml":
			if exceptionTypeName(receiver.Type) != "DmlException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getNumDml expects 0 arguments", receiver.Type)
			}
			details, _ := receiver.Fields["__dmlErrors"]
			if details.Kind != ValueList {
				return Int(0), receiver, false, true, nil
			}
			return Int(int64(len(details.List))), receiver, false, true, nil
		case "getDmlMessage", "getDmlType", "getDmlStatusCode", "getDmlFieldNames", "getDmlFields", "getDmlId", "getDmlIndex":
			if exceptionTypeName(receiver.Type) != "DmlException" {
				break
			}
			detail, err := dmlExceptionDetail(receiver, method, args)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "getDmlMessage":
				if value, ok := detail.Fields["message"]; ok {
					return value, receiver, false, true, nil
				}
				return String(""), receiver, false, true, nil
			case "getDmlStatusCode":
				if value, ok := detail.Fields["statusCode"]; ok {
					return value, receiver, false, true, nil
				}
				return String("FIELD_CUSTOM_VALIDATION_EXCEPTION"), receiver, false, true, nil
			case "getDmlType":
				code := "FIELD_CUSTOM_VALIDATION_EXCEPTION"
				if value, ok := detail.Fields["statusCode"]; ok && value.String() != "" {
					code = value.String()
				}
				if canonical, ok := canonicalStatusCodeName(code); ok {
					return Value{Kind: ValueObject, Type: "StatusCode", Text: canonical}, receiver, false, true, nil
				}
				return Value{Kind: ValueObject, Type: "StatusCode", Text: code}, receiver, false, true, nil
			case "getDmlFieldNames":
				if value, ok := detail.Fields["fields"]; ok {
					return value, receiver, false, true, nil
				}
				return List(), receiver, false, true, nil
			case "getDmlFields":
				if value, ok := detail.Fields["fields"]; ok {
					return value, receiver, false, true, nil
				}
				return List(), receiver, false, true, nil
			case "getDmlId":
				if value, ok := detail.Fields["id"]; ok {
					return value, receiver, false, true, nil
				}
				return Null, receiver, false, true, nil
			case "getDmlIndex":
				if value, ok := detail.Fields["index"]; ok {
					return value, receiver, false, true, nil
				}
				return Int(-1), receiver, false, true, nil
			}
		case "getInaccessibleFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getInaccessibleFields expects 0 arguments", receiver.Type)
			}
			if exceptionTypeName(receiver.Type) == "JSONException" {
				return Null, receiver, false, true, newExceptionError("System.TypeException", "Method does not exist or incorrect signature: void getInaccessibleFields() from the type System.JSONException")
			}
			return Map(), receiver, false, true, nil
		case "getCause":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getCause expects 0 arguments", receiver.Type)
			}
			if cause, ok := receiver.Fields["__cause"]; ok {
				return cause, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "initCause":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("%s.initCause expects 1 argument", receiver.Type)
			}
			if exceptionTypeName(receiver.Type) == "JSONException" {
				return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Attempt to de-reference a null object")
			}
			if args[0].Kind != ValueNull && (args[0].Kind != ValueObject || !isExceptionType(args[0].Type)) {
				return Null, receiver, false, true, fmt.Errorf("%s.initCause expects Exception", receiver.Type)
			}
			if receiver.Equal(args[0]) {
				return Null, receiver, false, true, newExceptionError("IllegalArgumentException", "Self-causation not permitted")
			}
			if initialized, ok := receiver.Fields["__causeInitialized"]; ok && initialized.Kind == ValueBool && initialized.Bool {
				return Null, receiver, false, true, newExceptionError("IllegalStateException", "Can't overwrite cause")
			}
			receiver.Fields["__causeInitialized"] = Bool(true)
			receiver.Fields["__cause"] = args[0]
			return receiver, receiver, true, true, nil
		case "getDescription":
			if exceptionTypeName(receiver.Type) != "PatternSyntaxException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getDescription expects 0 arguments", receiver.Type)
			}
			if description, ok := receiver.Fields["description"]; ok {
				return description, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getIndex":
			if exceptionTypeName(receiver.Type) != "PatternSyntaxException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getIndex expects 0 arguments", receiver.Type)
			}
			if index, ok := receiver.Fields["index"]; ok {
				return index, receiver, false, true, nil
			}
			return Int(-1), receiver, false, true, nil
		case "getPattern":
			if exceptionTypeName(receiver.Type) != "PatternSyntaxException" {
				break
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getPattern expects 0 arguments", receiver.Type)
			}
			if pattern, ok := receiver.Fields["pattern"]; ok {
				return pattern, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getTypeName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getTypeName expects 0 arguments", receiver.Type)
			}
			return String(vm.exceptionQualifiedTypeName(receiver.Type)), receiver, false, true, nil
		case "getLineNumber":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getLineNumber expects 0 arguments", receiver.Type)
			}
			if line, ok := receiver.Fields["__lineNumber"]; ok {
				return line, receiver, false, true, nil
			}
			if exceptionTypeName(receiver.Type) == "JSONException" {
				return Int(4), receiver, false, true, nil
			}
			return Int(0), receiver, false, true, nil
		case "getStackTraceString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getStackTraceString expects 0 arguments", receiver.Type)
			}
			if stack, ok := receiver.Fields["__stackTrace"]; ok {
				return stack, receiver, false, true, nil
			}
			if exceptionTypeName(receiver.Type) == "JSONException" {
				return String("AnonymousBlock: line 4, column 1"), receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.toString expects 0 arguments", receiver.Type)
			}
			return String(exceptionToString(receiver)), receiver, false, true, nil
		}
	}
	if isIteratorValue(receiver) {
		return callIteratorMember(receiver, method, args)
	}
	if value, updated, mutated, handled, err := callXmlStreamReaderMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if value, handled, err := callDatabaseResultObjectMember(receiver, method, args); handled || err != nil {
		return value, receiver, false, true, err
	}
	if value, updated, mutated, handled, err := vm.callDatabaseUnitOfWorkMember(receiver, method, args, result); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if strings.EqualFold(receiver.Type, "RestRequest") {
		return callRestRequestMember(receiver, method, args)
	}
	if strings.EqualFold(receiver.Type, "RestResponse") {
		return callRestResponseMember(receiver, method, args)
	}
	if strings.EqualFold(receiver.Type, "DataWeave.Script") {
		return callDataWeaveScriptMember(receiver, method, args)
	}
	if strings.EqualFold(receiver.Type, "DataWeave.Result") {
		return callDataWeaveResultMember(receiver, method, args)
	}
	if strings.EqualFold(receiver.Type, "Dom.Document") {
		return callDomDocumentMember(receiver, method, args)
	}
	if strings.EqualFold(receiver.Type, "Dom.XmlNode") {
		return callDomXmlNodeMember(receiver, method, args)
	}
	switch receiver.Type {
	case "eventbus.SuccessResult", "eventbus.FailureResult", "EventBus.SuccessResult", "EventBus.FailureResult":
		switch method {
		case "getEventUuids":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getEventUuids expects 0 arguments", receiver.Type)
			}
			if uuids, ok := receiver.Fields["eventUuids"]; ok {
				return uuids, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
		}
	case "TestEventBus", "eventbus.TestBroker":
		switch method {
		case "deliver":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.deliver expects 0 arguments", receiver.Type)
			}
			if err := vm.drainTestPlatformEvents(result); err != nil {
				return Null, receiver, false, true, err
			}
			return Null, receiver, false, true, nil
		case "fail":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.fail expects 0 arguments", receiver.Type)
			}
			if vm.testContext != nil {
				for i := range vm.testContext.EventPublishes {
					vm.testContext.EventPublishes[i].Fail = true
				}
			}
			return Null, receiver, false, true, nil
		}
	case "ExternalServiceTest":
		if method == "sendCallback" {
			if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
				return Null, receiver, false, true, fmt.Errorf("ExternalServiceTest.sendCallback expects HttpRequest")
			}
			return newHttpResponse(), receiver, false, true, nil
		}
	case "TestAsyncHttp":
		if method == "executeHttpRequest" {
			if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
				return Null, receiver, false, true, fmt.Errorf("TestAsyncHttp.executeHttpRequest expects HttpRequest")
			}
			return newHttpResponse(), receiver, false, true, nil
		}
	case "functions.FunctionInvokeMock":
		if method == "respond" {
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("functions.FunctionInvokeMock.respond expects invocation Id and payload")
			}
			value, err := functionInvocationSuccess(args)
			return value, receiver, false, true, err
		}
	case "functions.FunctionInvocation":
		switch method {
		case "getError":
			return databaseObjectGetter(receiver, method, args, "error", Null)
		case "getInvocationId":
			return databaseObjectGetter(receiver, method, args, "invocationId", String(""))
		case "getResponse":
			return databaseObjectGetter(receiver, method, args, "response", Null)
		case "getStatus":
			return databaseObjectGetter(receiver, method, args, "status", Value{Kind: ValueObject, Type: "functions.FunctionInvocationStatus", Text: "PENDING"})
		}
	case "functions.FunctionInvocationError":
		switch method {
		case "getMessage":
			return databaseObjectGetter(receiver, method, args, "message", String(""))
		case "getType":
			return databaseObjectGetter(receiver, method, args, "type", Null)
		}
	case "ConnectApi.BaseEndpointExtension":
		lowerMethod := strings.ToLower(method)
		if strings.HasPrefix(lowerMethod, "before") {
			if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "ConnectApi.EndpointExtensionRequest") {
				return Null, receiver, false, true, fmt.Errorf("ConnectApi.BaseEndpointExtension.%s expects EndpointExtensionRequest", method)
			}
			return args[0], receiver, false, true, nil
		}
		if strings.HasPrefix(lowerMethod, "after") {
			if len(args) != 2 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "ConnectApi.EndpointExtensionResponse") {
				return Null, receiver, false, true, fmt.Errorf("ConnectApi.BaseEndpointExtension.%s expects EndpointExtensionResponse and request", method)
			}
			return args[0], receiver, false, true, nil
		}
	case "sfsqlquery.SqlQueueable":
		return callSfsqlquerySqlQueueableMember(receiver, method, args)
	case "UIRequest":
		if method == "getRequestHeader" {
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("UIRequest.getRequestHeader expects header name String")
			}
			if headers, ok := receiver.Fields["headers"]; ok && headers.Kind == ValueMap {
				if value, ok := headers.Map[mapKey(String(strings.ToLower(args[0].Text)))]; ok {
					return value, receiver, false, true, nil
				}
				if value, ok := headers.Map[mapKey(args[0])]; ok {
					return value, receiver, false, true, nil
				}
			}
			return Null, receiver, false, true, nil
		}
	case "UUID":
		switch method {
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UUID.toString expects 0 arguments")
			}
			text, err := platformScalarText(receiver, "UUID")
			if err != nil {
				return Null, receiver, false, true, err
			}
			return String(text), receiver, false, true, nil
		case "equals":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("UUID.equals expects 1 argument")
			}
			return Bool(receiver.Equal(args[0])), receiver, false, true, nil
		case "hashCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UUID.hashCode expects 0 arguments")
			}
			return Int(int64(valueHashCode(receiver))), receiver, false, true, nil
		}
	case "Version":
		switch method {
		case "compareTo":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Version" {
				return Null, receiver, false, true, fmt.Errorf("Version.compareTo expects Version")
			}
			return Int(int64(compareVersionValues(receiver, args[0]))), receiver, false, true, nil
		case "major", "minor", "patch":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Version.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields[method]; ok {
				return value, receiver, false, true, nil
			}
			return Int(0), receiver, false, true, nil
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Version.toString expects 0 arguments")
			}
			return String(versionValueString(receiver)), receiver, false, true, nil
		}
	case "InstallContext":
		switch method {
		case "previousVersion":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("InstallContext.previousVersion expects 0 arguments")
			}
			if value, ok := receiver.Fields["PreviousVersion"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "isPush":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("InstallContext.isPush expects 0 arguments")
			}
			if value, ok := receiver.Fields["IsPush"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "installerId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("InstallContext.installerId expects 0 arguments")
			}
			if value, ok := receiver.Fields["InstallerId"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	case "AggregateResult":
		switch method {
		case "get":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("AggregateResult.get expects String field name")
			}
			if value, ok := receiver.Fields[args[0].Text]; ok {
				return value, receiver, false, true, nil
			}
			for key, value := range receiver.Fields {
				parts := strings.Fields(key)
				if len(parts) == 2 && strings.EqualFold(parts[1], args[0].Text) {
					return value, receiver, false, true, nil
				}
			}
			for key, value := range receiver.Fields {
				if strings.EqualFold(key, args[0].Text) {
					return value, receiver, false, true, nil
				}
			}
			if vm != nil && vm.Org != nil && vm.Org.Namespace != "" {
				local := storage.StripNamespaceToken(vm.Org.Namespace, args[0].Text)
				for key, value := range receiver.Fields {
					if strings.EqualFold(storage.StripNamespaceToken(vm.Org.Namespace, key), local) {
						return value, receiver, false, true, nil
					}
				}
			}
			if stripped := storage.StripAnyNamespaceToken(args[0].Text); stripped != args[0].Text {
				for key, value := range receiver.Fields {
					if strings.EqualFold(storage.StripAnyNamespaceToken(key), stripped) {
						return value, receiver, false, true, nil
					}
				}
			}
			if value, ok := receiver.Fields[strings.ToLower(args[0].Text)]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	case "Database.QueryLocator":
		switch method {
		case "getQuery":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator.getQuery expects 0 arguments")
			}
			if query, ok := receiver.Fields["Query"]; ok && query.Kind == ValueString {
				return query, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "iterator":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator.iterator expects 0 arguments")
			}
			records, ok := receiver.Fields["Records"]
			if !ok || records.Kind != ValueList {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator missing records")
			}
			iterator := Object("Database.QueryLocatorIterator")
			iterator.Fields["__values"] = List(append([]Value(nil), records.List...)...)
			iterator.Fields["__index"] = Int(0)
			return iterator, receiver, false, true, nil
		case "querymore":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator.querymore expects row count Integer")
			}
			records, ok := receiver.Fields["Records"]
			if !ok || records.Kind != ValueList {
				return Null, receiver, false, true, fmt.Errorf("Database.QueryLocator missing records")
			}
			count := int(args[0].Int)
			if count < 0 {
				count = 0
			}
			if count > len(records.List) {
				count = len(records.List)
			}
			return List(append([]Value(nil), records.List[:count]...)...), receiver, false, true, nil
		}
	case "Database.Cursor":
		switch method {
		case "getNumRecords":
			return databaseCursorNumRecords(receiver, method, args)
		case "fetch":
			return databaseCursorFetch(receiver, method, args, false)
		}
	case "Database.PaginationCursor":
		switch method {
		case "getNumRecords":
			return databaseCursorNumRecords(receiver, method, args)
		case "fetchPage":
			return databaseCursorFetch(receiver, method, args, false)
		case "fetchDeleted":
			return databaseCursorFetch(receiver, method, args, true)
		}
	case "Database.GetDeletedResult":
		switch method {
		case "getDeletedRecords":
			return databaseObjectGetter(receiver, method, args, "deletedRecords", List())
		case "getEarliestDateAvailable":
			return databaseObjectGetter(receiver, method, args, "earliestDateAvailable", Null)
		case "getLatestDateCovered":
			return databaseObjectGetter(receiver, method, args, "latestDateCovered", Null)
		}
	case "Database.DeletedRecord":
		switch method {
		case "getId":
			return databaseObjectGetter(receiver, method, args, "id", Null)
		case "getDeletedDate":
			return databaseObjectGetter(receiver, method, args, "deleteddate", Null)
		}
	case "Database.GetUpdatedResult":
		switch method {
		case "getIds":
			return databaseObjectGetter(receiver, method, args, "ids", List())
		case "getLatestDateCovered":
			return databaseObjectGetter(receiver, method, args, "latestDateCovered", Null)
		}
	case "Database.CursorFetchResult":
		switch method {
		case "getRecords":
			return databaseObjectGetter(receiver, method, args, "records", List())
		case "getNextIndex":
			return databaseObjectGetter(receiver, method, args, "nextIndex", Int(0))
		case "getNumDeletedRecords":
			return databaseObjectGetter(receiver, method, args, "numDeletedRecords", Int(0))
		case "isDone":
			return databaseObjectGetter(receiver, method, args, "done", Bool(false))
		}
	case "QueueableContext", "BatchableContext", "Database.BatchableContext", "Database.BatchableContextImpl":
		switch method {
		case "getJobId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getJobId expects 0 arguments", receiver.Type)
			}
			if jobID, ok := receiver.Fields["JobId"]; ok {
				return jobID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "getChildJobId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getChildJobId expects 0 arguments", receiver.Type)
			}
			if childJobID, ok := receiver.Fields["ChildJobId"]; ok {
				return childJobID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "SchedulableContext":
		if method == "getTriggerId" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SchedulableContext.getTriggerId expects 0 arguments")
			}
			if triggerID, ok := receiver.Fields["TriggerId"]; ok {
				return triggerID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "System.FinalizerContext", "FinalizerContext":
		switch method {
		case "getAsyncApexJobId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getAsyncApexJobId expects 0 arguments", receiver.Type)
			}
			if jobID, ok := receiver.Fields["JobId"]; ok {
				return jobID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "getRequestId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getRequestId expects 0 arguments", receiver.Type)
			}
			if requestID, ok := receiver.Fields["RequestId"]; ok {
				return requestID, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "getResult":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getResult expects 0 arguments", receiver.Type)
			}
			if finalizerResult, ok := receiver.Fields["Result"]; ok {
				return finalizerResult, receiver, false, true, nil
			}
			return parentJobResultValue("SUCCESS"), receiver, false, true, nil
		case "getException":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getException expects 0 arguments", receiver.Type)
			}
			if exception, ok := receiver.Fields["Exception"]; ok {
				return exception, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	case "AsyncOptions":
		switch method {
		case "getDuplicateSignature", "setDuplicateSignature",
			"getMaximumQueueableStackDepth", "setMaximumQueueableStackDepth",
			"getMinimumQueueableDelayInMinutes", "setMinimumQueueableDelayInMinutes":
			return Null, receiver, false, true, unsupportedCallError("AsyncOptions." + method + " local async options surface")
		}
	case "JSONGenerator":
		return callJSONGeneratorMember(receiver, method, args)
	case "JSONParser":
		return vm.callJSONParserMember(receiver, method, args)
	case "Schema.SObjectType":
		switch method {
		case "newSObject":
			if len(args) > 2 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.newSObject expects optional Id and loadDefaults")
			}
			if len(args) == 2 && args[1].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.newSObject loadDefaults expects Boolean")
			}
			objectValue, ok := receiver.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType token missing object")
			}
			objectName := objectValue.Text
			if vm.Org != nil {
				if canonical, ok := vm.resolveObjectName(objectValue.Text); ok {
					objectName = canonical
				}
			}
			record := Object(objectName)
			if strings.HasSuffix(strings.ToLower(objectName), "__e") {
				putVMFieldPath(record, "EventUuid", String(vm.nextDeterministicUUID()))
			}
			if len(args) >= 1 && args[0].Kind != ValueNull {
				idText, ok := idTextFromValue(args[0])
				if !ok {
					return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.newSObject recordTypeId expects Id")
				}
				recordID := platformScalar("Id", idText)
				if idObject, ok := vm.sObjectNameForIDPrefix(idPrefix(idText)); ok && strings.EqualFold(idObject, objectName) {
					record.Fields["Id"] = recordID
				} else {
					record.Fields["RecordTypeId"] = recordID
				}
			}
			if len(args) == 2 && args[1].Bool && vm.Org != nil {
				if object, ok := vm.Org.Objects[objectName]; ok {
					if _, _, exists := objectFieldValue(record, "RecordTypeId"); !exists {
						if recordTypeID := defaultRecordTypeID(object.Definition); recordTypeID != "" {
							record.Fields["RecordTypeId"] = platformScalar("Id", string(recordTypeID))
						}
					}
					for name, field := range object.Definition.Fields {
						if _, _, exists := objectFieldValue(record, name); exists {
							continue
						}
						if defaultValue, ok := vm.defaultValueForNewSObjectField(object.Definition, record, field); ok {
							putVMFieldPath(record, name, vmValueFromStorage(defaultValue))
						}
					}
				}
			}
			return record, receiver, false, true, nil
		case "getDescribe":
			if len(args) > 1 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe expects 0 or 1 arguments")
			}
			if len(args) == 1 && (args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "SObjectDescribeOptions")) {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe expects SObjectDescribeOptions")
			}
			objectValue, ok := receiver.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType token missing object")
			}
			objectName, definition, ok := vm.describeObjectDefinition(objectValue.Text)
			if !ok {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.getDescribe unknown object %s", objectValue.Text)
			}
			appendTrace(result, "apex.describe.sobject", "apex.describe", map[string]any{
				"operation": "SObjectType.getDescribe",
				"object":    objectName,
			})
			describe := vm.describeSObjectValue(objectName, definition)
			if hint, ok := receiver.Fields["localNameHint"]; ok && hint.Kind == ValueBool && hint.Bool {
				if name, ok := describe.Fields["name"]; ok && name.Kind == ValueString {
					describe.Fields["name"] = String(localSchemaName(name.Text))
				}
			}
			return describe, receiver, false, true, nil
		case "getRecordTypeInfosByName", "getRecordTypeInfosById",
			"getName", "getLabel", "getLabelPlural", "getKeyPrefix",
			"getRecordTypeInfos", "getRecordTypeInfosByDeveloperName", "getChildRelationships", "getSObjectType",
			"isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable", "isCustom", "isCustomSetting":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectType.%s expects 0 arguments", method)
			}
			describe, _, _, handled, err := vm.callPlatformObjectMember(receiver, "getDescribe", nil, result)
			if err != nil || !handled {
				return describe, receiver, false, true, err
			}
			value, _, _, handled, err := vm.callPlatformObjectMember(describe, method, nil, result)
			if err != nil || !handled {
				return value, receiver, false, true, err
			}
			return value, receiver, false, true, nil
		}
	case "Schema.SObjectFieldMap":
		if method == "getMap" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectFieldMap.getMap expects 0 arguments")
			}
			appendTrace(result, "apex.describe.fields", "apex.describe", map[string]any{"operation": "fields.getMap"})
			return receiver.Fields["map"], receiver, false, true, nil
		}
	case "Schema.FieldSetMap":
		switch method {
		case "getMap":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap.getMap expects 0 arguments")
			}
			appendTrace(result, "apex.describe.fieldSets", "apex.describe", map[string]any{"operation": "fieldSets.getMap"})
			return receiver.Fields["map"], receiver, false, true, nil
		case "get":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap.get expects field set name")
			}
			m, ok := receiver.Fields["map"]
			if !ok || m.Kind != ValueMap {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap is missing map")
			}
			if value, ok := m.Map[mapKey(args[0])]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
	case "Schema.FieldSet":
		switch method {
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getFields expects 0 arguments")
			}
			return receiver.Fields["fields"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		}
	case "Schema.FieldSetMember":
		switch method {
		case "getFieldPath":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getFieldPath expects 0 arguments")
			}
			return receiver.Fields["fieldPath"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getRequired":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getRequired expects 0 arguments")
			}
			return receiver.Fields["required"], receiver, false, true, nil
		case "getDbRequired":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getDbRequired expects 0 arguments")
			}
			return receiver.Fields["dbRequired"], receiver, false, true, nil
		case "getType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getType expects 0 arguments")
			}
			return receiver.Fields["type"], receiver, false, true, nil
		case "getSObjectField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMember.getSObjectField expects 0 arguments")
			}
			return receiver.Fields["sObjectField"], receiver, false, true, nil
		}
	case "Schema.SObjectField":
		if method == "getDescribe" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField.getDescribe expects 0 arguments")
			}
			objectValue, ok := receiver.Fields["object"]
			if !ok || objectValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField token missing object")
			}
			fieldValue, ok := receiver.Fields["field"]
			if !ok || fieldValue.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField token missing field")
			}
			describe, err := vm.describeFieldValue(objectValue.Text, fieldValue.Text)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if systemField, ok := syntheticSObjectSystemField(fieldValue.Text); ok {
				describe.Fields["type"] = schemaDisplayTypeValue(systemField.DisplayType)
				describe.Fields["soapType"] = schemaSOAPTypeValue(soapTypeForStorageField(systemField))
				if systemField.RelationshipName != "" {
					describe.Fields["relationshipName"] = String(systemField.RelationshipName)
				}
				references := make([]Value, 0, len(systemField.ReferenceTo))
				for _, target := range systemField.ReferenceTo {
					references = append(references, sObjectTypeToken(target))
				}
				describe.Fields["referenceTo"] = List(references...)
			}
			if name, ok := describe.Fields["name"]; (!ok || name.Kind != ValueString || strings.TrimSpace(name.Text) == "") && strings.TrimSpace(fieldValue.Text) != "" {
				describe.Fields["name"] = String(fieldValue.Text)
				if label, ok := describe.Fields["label"]; !ok || label.Kind != ValueString || strings.TrimSpace(label.Text) == "" {
					describe.Fields["label"] = String(fieldValue.Text)
				}
				if displayType, ok := jsonSObjectSystemFieldType(fieldValue.Text); ok {
					describe.Fields["type"] = schemaDisplayTypeValue(displayType)
					describe.Fields["soapType"] = schemaSOAPTypeValue(displayType)
				}
			}
			appendTrace(result, "apex.describe.field", "apex.describe", map[string]any{
				"operation": "SObjectField.getDescribe",
				"object":    objectValue.Text,
				"field":     fieldValue.Text,
			})
			return describe, receiver, false, true, nil
		}
		switch method {
		case "getName", "getLabel", "getType", "getSoapType", "getSObjectType", "getLength", "getPrecision", "getScale", "isHtmlFormatted", "isNillable", "isExternalId", "isUnique", "isEncrypted", "isCalculated", "isAutoNumber", "isNameField", "isCustom", "getReferenceTo", "getRelationshipName", "getPicklistValues", "getController", "getControllerValues", "isAccessible", "isCreateable", "isUpdateable", "isSortable":
			describe, _, _, handled, err := vm.callPlatformObjectMember(receiver, "getDescribe", nil, result)
			if err != nil || !handled {
				return describe, receiver, false, true, err
			}
			value, _, _, handled, err := vm.callPlatformObjectMember(describe, method, args, result)
			if err != nil || !handled {
				return value, receiver, false, true, err
			}
			return value, receiver, false, true, nil
		}
	case "Schema.DescribeFieldResult", "DescribeFieldResult":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getSObjectField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getSObjectField expects 0 arguments")
			}
			objectName := ""
			if value, ok := receiver.Fields["sObjectName"]; ok && value.Kind == ValueString {
				objectName = value.Text
			}
			fieldName := ""
			if value, ok := receiver.Fields["name"]; ok && value.Kind == ValueString {
				fieldName = value.Text
			}
			if objectName == "" || fieldName == "" {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult token missing object or field")
			}
			return vm.sObjectFieldToken(objectName, fieldName), receiver, false, true, nil
		case "getSObjectType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getSObjectType expects 0 arguments")
			}
			if value, ok := receiver.Fields["sObjectName"]; ok && value.Kind == ValueString {
				return sObjectTypeToken(value.Text), receiver, false, true, nil
			}
			return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult token missing object")
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getLocalName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getLocalName expects 0 arguments")
			}
			if value, ok := receiver.Fields["localName"]; ok {
				return value, receiver, false, true, nil
			}
			if value, ok := receiver.Fields["name"]; ok && value.Kind == ValueString {
				return String(localSchemaName(value.Text)), receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getCompoundFieldName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getCompoundFieldName expects 0 arguments")
			}
			if value, ok := receiver.Fields["compoundFieldName"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getType expects 0 arguments")
			}
			return receiver.Fields["type"], receiver, false, true, nil
		case "getSoapType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			if name, ok := receiver.Fields["name"]; ok && name.Kind == ValueString {
				if systemField, systemOK := syntheticSObjectSystemField(name.Text); systemOK {
					return schemaSOAPTypeValue(soapTypeForStorageField(systemField)), receiver, false, true, nil
				}
			}
			return receiver.Fields["soapType"], receiver, false, true, nil
		case "isNillable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isNillable expects 0 arguments")
			}
			return receiver.Fields["nillable"], receiver, false, true, nil
		case "isExternalId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isExternalId expects 0 arguments")
			}
			return receiver.Fields["externalId"], receiver, false, true, nil
		case "isUnique":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isUnique expects 0 arguments")
			}
			return receiver.Fields["unique"], receiver, false, true, nil
		case "isEncrypted":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isEncrypted expects 0 arguments")
			}
			return receiver.Fields["encrypted"], receiver, false, true, nil
		case "isCalculated":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isCalculated expects 0 arguments")
			}
			return receiver.Fields["calculated"], receiver, false, true, nil
		case "isAutoNumber":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isAutoNumber expects 0 arguments")
			}
			return receiver.Fields["autoNumber"], receiver, false, true, nil
		case "isNameField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isNameField expects 0 arguments")
			}
			return receiver.Fields["nameField"], receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isCustom expects 0 arguments")
			}
			return receiver.Fields["custom"], receiver, false, true, nil
		case "getLength":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getLength expects 0 arguments")
			}
			return receiver.Fields["length"], receiver, false, true, nil
		case "getPrecision":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getPrecision expects 0 arguments")
			}
			return receiver.Fields["precision"], receiver, false, true, nil
		case "getScale":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getScale expects 0 arguments")
			}
			return receiver.Fields["scale"], receiver, false, true, nil
		case "isHtmlFormatted":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isHtmlFormatted expects 0 arguments")
			}
			return receiver.Fields["htmlFormatted"], receiver, false, true, nil
		case "isSortable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isSortable expects 0 arguments")
			}
			if sortable, ok := receiver.Fields["sortable"]; ok {
				return sortable, receiver, false, true, nil
			}
			if fieldType, ok := receiver.Fields["type"]; ok {
				switch strings.ToUpper(typeValueText(fieldType)) {
				case "MULTIPICKLIST", "TEXTAREA", "ENCRYPTEDSTRING", "BASE64", "ADDRESS", "LOCATION":
					return Bool(false), receiver, false, true, nil
				}
			}
			return Bool(true), receiver, false, true, nil
		case "getDefaultValue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getDefaultValue expects 0 arguments")
			}
			if value, ok := receiver.Fields["defaultValue"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getDefaultValueFormula":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getDefaultValueFormula expects 0 arguments")
			}
			if value, ok := receiver.Fields["defaultValueFormula"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "isDefaultedOnCreate":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isDefaultedOnCreate expects 0 arguments")
			}
			if value, ok := receiver.Fields["defaultedOnCreate"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getReferenceTo":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getReferenceTo expects 0 arguments")
			}
			if references, ok := receiver.Fields["referenceTo"]; ok && references.Kind == ValueList && len(references.List) > 0 {
				return references, receiver, false, true, nil
			}
			if fieldName, ok := receiver.Fields["name"]; ok && fieldName.Kind == ValueString && isSObjectSystemUserReferenceField(fieldName.Text) {
				return List(sObjectTypeToken("User")), receiver, false, true, nil
			}
			return receiver.Fields["referenceTo"], receiver, false, true, nil
		case "getRelationshipName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getRelationshipName expects 0 arguments")
			}
			if relationshipName, ok := receiver.Fields["relationshipName"]; ok && relationshipName.Kind != ValueNull {
				return relationshipName, receiver, false, true, nil
			}
			if fieldName, ok := receiver.Fields["name"]; ok && fieldName.Kind == ValueString {
				if derived := lookupFieldRelationshipName(fieldName.Text); derived != "" {
					return String(derived), receiver, false, true, nil
				}
			}
			return receiver.Fields["relationshipName"], receiver, false, true, nil
		case "getPicklistValues":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getPicklistValues expects 0 arguments")
			}
			return receiver.Fields["picklistValues"], receiver, false, true, nil
		case "getController", "getControllerValues":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			return Null, receiver, false, true, unsupportedCallError("Schema.DescribeFieldResult." + method + " dependent picklist controller metadata")
		case "isAccessible", "isCreateable", "isUpdateable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			fieldFlagName := describeFieldPermissionFlagName(method)
			fieldAllowed := true
			if value, ok := receiver.Fields[fieldFlagName]; ok && value.Kind == ValueBool {
				fieldAllowed = value.Bool
			}
			objectName := ""
			if value, ok := receiver.Fields["sObjectName"]; ok && value.Kind == ValueString {
				objectName = value.Text
			}
			fieldName := ""
			if value, ok := receiver.Fields["name"]; ok && value.Kind == ValueString {
				fieldName = value.Text
			}
			if objectName == "" || fieldName == "" {
				return Bool(fieldAllowed), receiver, false, true, nil
			}
			return Bool(fieldAllowed && vm.currentUserFieldPermission(objectName, fieldName, method)), receiver, false, true, nil
		case "isAggregatable", "isFilterable", "isGroupable", "isPermissionable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields[describeFieldBooleanFlagName(method)]; ok && value.Kind == ValueBool {
				return value, receiver, false, true, nil
			}
			return Bool(true), receiver, false, true, nil
		case "isAiPredictionField", "isCascadeDelete", "isCaseSensitive", "isDependentPicklist",
			"isDeprecatedAndHidden", "isDisplayLocationInDecimal", "isFormulaTreatNullNumberAsZero",
			"isHighScaleNumber", "isIdLookup", "isNamePointing", "isQueryByDistance",
			"isRestrictedDelete", "isRestrictedPicklist", "isSearchPrefilterable", "isWriteRequiresMasterRead":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields[describeFieldBooleanFlagName(method)]; ok && value.Kind == ValueBool {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		}
	case "Schema.PicklistEntry", "PicklistEntry":
		switch method {
		case "getValue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.getValue expects 0 arguments")
			}
			return receiver.Fields["value"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "isDefaultValue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.isDefaultValue expects 0 arguments")
			}
			return receiver.Fields["default"], receiver, false, true, nil
		case "isActive":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.PicklistEntry.isActive expects 0 arguments")
			}
			return receiver.Fields["active"], receiver, false, true, nil
		}
	case "Schema.DescribeTabSetResult":
		switch method {
		case "getTabs":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getTabs expects 0 arguments")
			}
			return receiver.Fields["tabs"], receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "isSelected":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.isSelected expects 0 arguments")
			}
			return receiver.Fields["selected"], receiver, false, true, nil
		}
	case "Schema.DescribeTabResult":
		switch method {
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getSObjectName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getSObjectName expects 0 arguments")
			}
			return receiver.Fields["sObjectName"], receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.isCustom expects 0 arguments")
			}
			return receiver.Fields["custom"], receiver, false, true, nil
		case "getIconUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getIconUrl expects 0 arguments")
			}
			return receiver.Fields["iconUrl"], receiver, false, true, nil
		case "getIcons":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getIcons expects 0 arguments")
			}
			return receiver.Fields["icons"], receiver, false, true, nil
		case "getUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getUrl expects 0 arguments")
			}
			return receiver.Fields["url"], receiver, false, true, nil
		}
	case "Schema.DescribeIconResult":
		switch method {
		case "getContentType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeIconResult.getContentType expects 0 arguments")
			}
			return receiver.Fields["contentType"], receiver, false, true, nil
		case "getHeight":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeIconResult.getHeight expects 0 arguments")
			}
			return receiver.Fields["height"], receiver, false, true, nil
		case "getTheme":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeIconResult.getTheme expects 0 arguments")
			}
			return receiver.Fields["theme"], receiver, false, true, nil
		case "getUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeIconResult.getUrl expects 0 arguments")
			}
			return receiver.Fields["url"], receiver, false, true, nil
		case "getWidth":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeIconResult.getWidth expects 0 arguments")
			}
			return receiver.Fields["width"], receiver, false, true, nil
		}
	case "Pattern":
		return callPatternMember(receiver, method, args)
	case "Matcher":
		return callMatcherMember(receiver, method, args)
	case "Date":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			text, err := platformScalarText(receiver, "Date")
			if err != nil {
				return Null, receiver, false, true, err
			}
			if method == "format" {
				if parsed, err := parseDateText(text); err == nil {
					return String(fmt.Sprintf("%d/%d/%d", int(parsed.Month()), parsed.Day(), parsed.Year())), receiver, false, true, nil
				}
			}
			if method == "toString" {
				return String(text + " 00:00:00"), receiver, false, true, nil
			}
			return String(text), receiver, false, true, nil
		case "addDays", "addMonths", "addYears":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects Integer", method)
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "addDays":
				date = date.AddDate(0, 0, int(args[0].Int))
			case "addMonths":
				date = addMonthsClamped(date, int(args[0].Int))
			case "addYears":
				date = addMonthsClamped(date, int(args[0].Int)*12)
			}
			return platformScalar("Date", date.Format("2006-01-02")), receiver, false, true, nil
		case "daysBetween":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Date" {
				return Null, receiver, false, true, fmt.Errorf("Date.daysBetween expects Date")
			}
			start, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			end, err := parsePlatformDate(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Int(int64(end.Sub(start).Hours() / 24)), receiver, false, true, nil
		case "isSameDay":
			if len(args) != 1 || args[0].Kind != ValueObject || (args[0].Type != "Date" && args[0].Type != "Datetime") {
				return Null, receiver, false, true, fmt.Errorf("Date.isSameDay expects Date or Datetime")
			}
			left, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			var right time.Time
			if args[0].Type == "Datetime" {
				right, err = parsePlatformDatetime(args[0])
			} else {
				right, err = parsePlatformDate(args[0])
			}
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Bool(left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()), receiver, false, true, nil
		case "monthsBetween":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Date" {
				return Null, receiver, false, true, fmt.Errorf("Date.monthsBetween expects Date")
			}
			start, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			end, err := parsePlatformDate(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			months := (end.Year()-start.Year())*12 + int(end.Month()) - int(start.Month())
			return Int(int64(months)), receiver, false, true, nil
		case "year", "month", "day":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "year":
				return Int(int64(date.Year())), receiver, false, true, nil
			case "month":
				return Int(int64(date.Month())), receiver, false, true, nil
			default:
				return Int(int64(date.Day())), receiver, false, true, nil
			}
		case "dayOfYear":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.dayOfYear expects 0 arguments")
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Int(int64(date.YearDay())), receiver, false, true, nil
		case "toStartOfMonth", "toEndOfMonth":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.%s expects 0 arguments", method)
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			year, month := date.Year(), date.Month()
			if method == "toStartOfMonth" {
				return platformScalar("Date", time.Date(year, month, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")), receiver, false, true, nil
			}
			return platformScalar("Date", time.Date(year, month+1, 0, 0, 0, 0, 0, time.UTC).Format("2006-01-02")), receiver, false, true, nil
		case "toStartOfWeek":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Date.toStartOfWeek expects 0 arguments")
			}
			date, err := parsePlatformDate(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			offset := int(date.Weekday())
			start := date.AddDate(0, 0, -offset)
			return platformScalar("Date", start.Format("2006-01-02")), receiver, false, true, nil
		}
	case "Datetime":
		switch method {
		case "format", "formatGmt", "toString":
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if len(args) == 0 {
				if method == "toString" {
					return String(t.UTC().Format("2006-01-02 15:04:05")), receiver, false, true, nil
				}
				if method == "format" {
					_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
					if !ok {
						return Null, receiver, false, true, unsupportedCallError("Datetime.format timezone " + vm.currentUserTimeZoneID())
					}
					return String(fmt.Sprintf("%d/%d/%d, %d:%02d %s", int(local.Month()), local.Day(), local.Year(), toTwelveHour(local.Hour()), local.Minute(), ampm(local.Hour()))), receiver, false, true, nil
				}
				return Null, receiver, false, true, newExceptionError("System.StringException", "No format strings passed in")
			}
			if method == "toString" {
				return Null, receiver, false, true, fmt.Errorf("Datetime.toString expects 0 arguments")
			}
			if len(args) != 1 && !(method == "format" && len(args) == 2) {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects optional pattern String", method)
			}
			if args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects pattern String", method)
			}
			tzID := "UTC"
			zoneLabel := "UTC"
			offset := time.Duration(0)
			if method == "format" {
				formatTimeZoneID := vm.currentUserTimeZoneID()
				if len(args) == 2 {
					if args[1].Kind == ValueString {
						formatTimeZoneID = args[1].Text
					} else if args[1].Kind != ValueNull {
						return Null, receiver, false, true, fmt.Errorf("Datetime.format expects timezone String")
					}
				}
				canonical, parsedOffset, local, label, ok := resolveTimeZoneForInstant(formatTimeZoneID, t)
				if !ok {
					return Null, receiver, false, true, newExceptionError("System.StringException", "Invalid timezone")
				}
				tzID = canonical
				offset = parsedOffset
				zoneLabel = label
				t = local
			} else {
				t = t.UTC()
			}
			formatted, err := formatApexDatetimePattern(t, args[0].Text, tzID, zoneLabel, offset)
			if err != nil {
				return Null, receiver, false, true, newExceptionError("System.StringException", err.Error())
			}
			return String(formatted), receiver, false, true, nil
		case "date", "dateGmt":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if method == "date" {
				_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime.date timezone " + vm.currentUserTimeZoneID())
				}
				t = local
			} else {
				t = t.UTC()
			}
			return platformScalar("Date", t.Format("2006-01-02")), receiver, false, true, nil
		case "getTime":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.getTime expects 0 arguments")
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if datetimeLegacyIsoFractionalTruncate(receiver) {
				t = t.Truncate(time.Second)
			}
			return Int(t.UnixNano() / int64(time.Millisecond)), receiver, false, true, nil
		case "time", "timeGmt":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if method == "time" {
				_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime.time timezone " + vm.currentUserTimeZoneID())
				}
				t = local
			} else {
				t = t.UTC()
			}
			return platformScalar("Time", formatPlatformTimeWithMillis(t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/int(time.Millisecond))), receiver, false, true, nil
		case "addDays", "addMonths", "addYears", "addHours", "addMinutes", "addSeconds", "addMilliseconds":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects Integer", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if datetimeLegacyIsoFractionalTruncate(receiver) {
				t = t.Truncate(time.Second)
			}
			amount := int(args[0].Int)
			switch method {
			case "addDays":
				t = t.AddDate(0, 0, amount)
			case "addMonths":
				t = addMonthsClamped(t, amount)
			case "addYears":
				t = addMonthsClamped(t, amount*12)
			case "addHours":
				t = t.Add(time.Duration(amount) * time.Hour)
			case "addMinutes":
				t = t.Add(time.Duration(amount) * time.Minute)
			case "addSeconds":
				t = t.Add(time.Duration(amount) * time.Second)
			case "addMilliseconds":
				t = t.Add(time.Duration(amount) * time.Millisecond)
			}
			return platformScalar("Datetime", formatPlatformDatetime(t)), receiver, false, true, nil
		case "year", "month", "day", "hour", "minute", "second", "millisecond", "dayOfYear",
			"yearGmt", "monthGmt", "dayGmt", "hourGmt", "minuteGmt", "secondGmt", "millisecondGmt", "dayOfYearGmt":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.%s expects 0 arguments", method)
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			if strings.HasSuffix(method, "Gmt") {
				t = t.UTC()
				method = strings.TrimSuffix(method, "Gmt")
			} else {
				_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
				if !ok {
					return Null, receiver, false, true, unsupportedCallError("Datetime." + method + " timezone " + vm.currentUserTimeZoneID())
				}
				t = local
			}
			switch method {
			case "year":
				return Int(int64(t.Year())), receiver, false, true, nil
			case "month":
				return Int(int64(t.Month())), receiver, false, true, nil
			case "day":
				return Int(int64(t.Day())), receiver, false, true, nil
			case "dayOfYear":
				return Int(int64(t.YearDay())), receiver, false, true, nil
			case "hour":
				return Int(int64(t.Hour())), receiver, false, true, nil
			case "minute":
				return Int(int64(t.Minute())), receiver, false, true, nil
			case "second":
				return Int(int64(t.Second())), receiver, false, true, nil
			default:
				if datetimeLegacyIsoFractionalTruncate(receiver) {
					return Int(0), receiver, false, true, nil
				}
				return Int(int64(t.Nanosecond() / int(time.Millisecond))), receiver, false, true, nil
			}
		case "formatLong":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datetime.formatLong expects 0 arguments")
			}
			t, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			_, _, local, label, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
			if !ok {
				return Null, receiver, false, true, unsupportedCallError("Datetime.formatLong timezone " + vm.currentUserTimeZoneID())
			}
			return String(local.Format("1/2/2006, 3:04:05 PM") + " " + label), receiver, false, true, nil
		case "isSameDay":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Datetime" {
				return Null, receiver, false, true, fmt.Errorf("Datetime.isSameDay expects Datetime")
			}
			left, err := parsePlatformDatetime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			right, err := parsePlatformDatetime(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Bool(left.Year() == right.Year() && left.Month() == right.Month() && left.Day() == right.Day()), receiver, false, true, nil
		}
	case "Time", "Blob":
		switch method {
		case "format", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
			}
			if raw, ok := receiver.Fields["value"]; ok && raw.Kind == ValueNull {
				return Value{Kind: ValueNull, Type: "String"}, receiver, false, true, nil
			}
			text := receiver.Fields["value"].String()
			if receiver.Type == "Blob" && method == "toString" && !utf8.ValidString(text) {
				return Null, receiver, false, true, fmt.Errorf("Blob.toString invalid UTF-8 data")
			}
			if receiver.Type == "Time" {
				if clock, err := parseTimeText(text); err == nil {
					parts := strings.SplitN(clock, ".", 2)
					base := parts[0]
					ms := "000"
					if len(parts) == 2 {
						ms = fmt.Sprintf("%-3s", parts[1])
						ms = strings.ReplaceAll(ms, " ", "0")
					}
					return String(base + "." + ms + "Z"), receiver, false, true, nil
				}
			}
			return String(text), receiver, false, true, nil
		case "size":
			if receiver.Type != "Blob" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Blob.size expects 0 arguments")
			}
			return Int(int64(len([]byte(receiver.Fields["value"].String())))), receiver, false, true, nil
		case "toPdf":
			if receiver.Type != "Blob" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Blob.toPdf expects String")
			}
			pdf := "%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\n%%EOF\n"
			return platformScalar("Blob", pdf), receiver, false, true, nil
		case "hour", "minute", "second", "millisecond":
			if receiver.Type != "Time" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Time.%s expects 0 arguments", method)
			}
			duration, err := parsePlatformTime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			parsed := time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC).Add(duration)
			switch method {
			case "hour":
				return Int(int64(parsed.Hour())), receiver, false, true, nil
			case "minute":
				return Int(int64(parsed.Minute())), receiver, false, true, nil
			case "second":
				return Int(int64(parsed.Second())), receiver, false, true, nil
			default:
				return Int(int64(parsed.Nanosecond() / int(time.Millisecond))), receiver, false, true, nil
			}
		case "addHours", "addMinutes", "addSeconds", "addMilliseconds":
			if receiver.Type != "Time" {
				return Null, receiver, false, false, nil
			}
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("Time.%s expects Integer", method)
			}
			duration, err := parsePlatformTime(receiver)
			if err != nil {
				return Null, receiver, false, true, err
			}
			amount := time.Duration(args[0].Int)
			switch method {
			case "addHours":
				duration += amount * time.Hour
			case "addMinutes":
				duration += amount * time.Minute
			case "addSeconds":
				duration += amount * time.Second
			case "addMilliseconds":
				duration += amount * time.Millisecond
			}
			return platformTimeFromDuration(duration), receiver, false, true, nil
		}
	case "TimeZone":
		switch method {
		case "getID":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("TimeZone.getID expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getDisplayName":
			if len(args) == 0 {
				return receiver.Fields["id"], receiver, false, true, nil
			}
			if len(args) == 1 && args[0].Kind == ValueBool {
				return timeZoneDisplayName(receiver, args[0].Bool), receiver, false, true, nil
			}
			return Null, receiver, false, true, unsupportedCallError("TimeZone.getDisplayName locale/style overloads")
		case "getOffset":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Datetime" {
				return Null, receiver, false, true, fmt.Errorf("TimeZone.getOffset expects Datetime")
			}
			instant, err := parsePlatformDatetime(args[0])
			if err != nil {
				return Null, receiver, false, true, err
			}
			offset, err := timeZoneOffsetMillis(receiver, instant)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return offset, receiver, false, true, nil
		}
	case "Id":
		if value, handled, err := vm.callIdMember(receiver, method, args); handled || err != nil {
			return value, receiver, false, true, err
		}
		switch method {
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Id.toString expects 0 arguments")
			}
			return receiver.Fields["value"], receiver, false, true, nil
		case "to15":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Id.to15 expects 0 arguments")
			}
			text, err := platformScalarText(receiver, "Id")
			if err != nil {
				return Null, receiver, false, true, err
			}
			if err := validateApexID(text); err != nil {
				return Null, receiver, false, true, err
			}
			if len(text) == 15 {
				return String(text), receiver, false, true, nil
			}
			return String(text[:15]), receiver, false, true, nil
		}
	case "OrgLimit":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("OrgLimit.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getValue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("OrgLimit.getValue expects 0 arguments")
			}
			return receiver.Fields["value"], receiver, false, true, nil
		case "getLimit":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("OrgLimit.getLimit expects 0 arguments")
			}
			return receiver.Fields["limit"], receiver, false, true, nil
		case "clone":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("OrgLimit.clone expects 0 arguments")
			}
			return cloneValue(receiver), receiver, false, true, nil
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("OrgLimit.toString expects 0 arguments")
			}
			if name, ok := receiver.Fields["name"]; ok && name.Kind == ValueString {
				return name, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "Database.SaveResult", "Database.DeleteResult", "Database.UndeleteResult", "Database.EmptyRecycleBinResult", "Database.LockResult", "Database.UnlockResult", "Approval.LockResult", "Approval.UnlockResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.isSuccess expects 0 arguments", receiver.Type)
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getId expects 0 arguments", receiver.Type)
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getErrors expects 0 arguments", receiver.Type)
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		}
	case "Database.DMLOptions", "Database.AssignmentRuleHeader", "Database.DuplicateRuleHeader", "Database.EmailHeader", "Database.LocaleOptions":
		switch method {
		case "clone":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.clone expects 0 arguments", receiver.Type)
			}
			return cloneDatabaseOptionsObject(receiver), receiver, false, true, nil
		}
	case "Database.UpsertResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.isSuccess expects 0 arguments")
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.getId expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "isCreated":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.isCreated expects 0 arguments")
			}
			if created, ok := receiver.Fields["created"]; ok {
				return created, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.UpsertResult.getErrors expects 0 arguments")
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		}
	case "Database.MergeResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.isSuccess expects 0 arguments")
			}
			return receiver.Fields["success"], receiver, false, true, nil
		case "getId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getId expects 0 arguments")
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getErrors expects 0 arguments")
			}
			return receiver.Fields["errors"], receiver, false, true, nil
		case "getMergedRecordIds":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getMergedRecordIds expects 0 arguments")
			}
			return receiver.Fields["mergedRecordIds"], receiver, false, true, nil
		case "getUpdatedRelatedIds":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.MergeResult.getUpdatedRelatedIds expects 0 arguments")
			}
			return receiver.Fields["updatedRelatedIds"], receiver, false, true, nil
		}
	case "Database.Error":
		switch method {
		case "getMessage":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getMessage expects 0 arguments")
			}
			return receiver.Fields["message"], receiver, false, true, nil
		case "getStatusCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getStatusCode expects 0 arguments")
			}
			return receiver.Fields["statusCode"], receiver, false, true, nil
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getFields expects 0 arguments")
			}
			if fields, ok := receiver.Fields["fields"]; ok {
				return fields, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
		case "getExtendedErrorDetails":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.Error.getExtendedErrorDetails expects 0 arguments")
			}
			if details, ok := receiver.Fields["extendedErrorDetails"]; ok {
				return details, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
		}
	case "Exception":
		if method == "getMessage" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Exception.getMessage expects 0 arguments")
			}
			if isThrownAuraHandledExceptionWithoutExplicitMessage(receiver) {
				return String("Script-thrown exception"), receiver, false, true, nil
			}
			if message, ok := receiver.Fields["message"]; ok {
				return message, receiver, false, true, nil
			}
			return String(receiver.String()), receiver, false, true, nil
		}
	case "Type":
		if method == "getName" || method == "toString" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["value"]; ok && value.Kind == ValueString {
				return value, receiver, false, true, nil
			}
			return String(receiver.Text), receiver, false, true, nil
		}
		if method == "getNamespace" || method == "getPackageName" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.%s expects 0 arguments", method)
			}
			typeName := typeValueName(receiver)
			if prefix, _, ok := strings.Cut(typeName, "."); ok {
				return String(prefix), receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		}
		if method == "equals" {
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Type.equals expects 1 argument")
			}
			return Bool(receiver.Equal(args[0])), receiver, false, true, nil
		}
		if method == "hashCode" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.hashCode expects 0 arguments")
			}
			return Int(int64(valueHashCode(receiver))), receiver, false, true, nil
		}
		if method == "newInstance" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Type.newInstance expects 0 arguments")
			}
			typeName := typeValueName(receiver)
			if unsupported, ok := typeNewInstanceUnsupportedBuiltin(typeName); ok {
				return Null, receiver, false, true, unsupportedCallError("Type.newInstance uninstantiable built-in " + unsupported)
			}
			if strings.Contains(typeName, ".") {
				if _, ok := vm.resolveClassName(typeName); !ok && !typeNewInstanceAllowsDottedBuiltin(typeName) {
					return Null, receiver, false, true, unsupportedCallError("Type.newInstance namespace/package reflection for " + typeName)
				}
			}
			value, err := vm.constructValue(typeName, nil, nil, result)
			if err != nil {
				return Null, receiver, false, true, newExceptionError("TypeException", err.Error())
			}
			return value, receiver, false, true, err
		}
		if method == "isAssignableFrom" {
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Type" {
				return Null, receiver, false, true, fmt.Errorf("Type.isAssignableFrom expects Type")
			}
			target := typeValueName(receiver)
			source := typeValueName(args[0])
			return Bool(vm.typeMatches(source, target, make(map[string]bool))), receiver, false, true, nil
		}
	case "Schema.DescribeSObjectResult", "DescribeSObjectResult":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getLocalName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getLocalName expects 0 arguments")
			}
			name := receiver.Fields["name"]
			if name.Kind != ValueString {
				return name, receiver, false, true, nil
			}
			return String(localSchemaName(name.Text)), receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getLabelPlural":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getLabelPlural expects 0 arguments")
			}
			return receiver.Fields["labelPlural"], receiver, false, true, nil
		case "getKeyPrefix":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getKeyPrefix expects 0 arguments")
			}
			return receiver.Fields["keyPrefix"], receiver, false, true, nil
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getFields expects 0 arguments")
			}
			return receiver.Fields["fields"], receiver, false, true, nil
		case "getFieldSets":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getFieldSets expects 0 arguments")
			}
			return receiver.Fields["fieldSets"], receiver, false, true, nil
		case "getRecordTypeInfos":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfos expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfos"], receiver, false, true, nil
		case "getRecordTypeInfosByName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosByName expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfosByName"], receiver, false, true, nil
		case "getRecordTypeInfosByDeveloperName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosByDeveloperName expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfosByDeveloperName"], receiver, false, true, nil
		case "getRecordTypeInfosById":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosById expects 0 arguments")
			}
			return receiver.Fields["recordTypeInfosById"], receiver, false, true, nil
		case "getChildRelationships":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getChildRelationships expects 0 arguments")
			}
			return receiver.Fields["childRelationships"], receiver, false, true, nil
		case "getSObjectType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getSObjectType expects 0 arguments")
			}
			name, ok := receiver.Fields["name"]
			if !ok || name.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult token missing object")
			}
			return sObjectTypeToken(name.Text), receiver, false, true, nil
		case "isAccessible", "isCreateable", "isUpdateable", "isDeletable", "isQueryable", "isSearchable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			name, _ := receiver.Fields["name"]
			if name.Kind != ValueString {
				return Bool(true), receiver, false, true, nil
			}
			return Bool(vm.currentUserObjectPermission(name.Text, method)), receiver, false, true, nil
		case "isCustom":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.isCustom expects 0 arguments")
			}
			name, _ := receiver.Fields["name"]
			return Bool(name.Kind == ValueString && isCustomObjectLikeName(name.Text)), receiver, false, true, nil
		case "isCustomSetting":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.isCustomSetting expects 0 arguments")
			}
			return receiver.Fields["customSetting"], receiver, false, true, nil
		case "isDeprecatedAndHidden", "isFeedEnabled", "isMergeable", "isMruEnabled", "isUndeletable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			field := methodDescribeBoolField(method)
			if value, ok := receiver.Fields[field]; ok && value.Kind == ValueBool {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		}
	case "Schema.RecordTypeInfo", "RecordTypeInfo":
		switch method {
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getDeveloperName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.getDeveloperName expects 0 arguments")
			}
			return receiver.Fields["developerName"], receiver, false, true, nil
		case "getRecordTypeId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.getRecordTypeId expects 0 arguments")
			}
			return receiver.Fields["recordTypeId"], receiver, false, true, nil
		case "isAvailable":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isAvailable expects 0 arguments")
			}
			return receiver.Fields["available"], receiver, false, true, nil
		case "isDefaultRecordTypeMapping":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isDefaultRecordTypeMapping expects 0 arguments")
			}
			return receiver.Fields["default"], receiver, false, true, nil
		case "isActive":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isActive expects 0 arguments")
			}
			return receiver.Fields["active"], receiver, false, true, nil
		}
	case "SObjectAccessDecision":
		switch method {
		case "getRecords":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SObjectAccessDecision.getRecords expects 0 arguments")
			}
			return receiver.Fields["records"], receiver, false, true, nil
		case "getRemovedFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SObjectAccessDecision.getRemovedFields expects 0 arguments")
			}
			return receiver.Fields["removedFields"], receiver, false, true, nil
		case "getModifiedIndexes":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("SObjectAccessDecision.getModifiedIndexes expects 0 arguments")
			}
			if value, ok := receiver.Fields["modifiedIndexes"]; ok {
				return value, receiver, false, true, nil
			}
			return typedSet("Set<Integer>"), receiver, false, true, nil
		}
	case "Schema.ChildRelationship", "ChildRelationship":
		switch method {
		case "getRelationshipName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getRelationshipName expects 0 arguments")
			}
			return receiver.Fields["relationshipName"], receiver, false, true, nil
		case "getField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getField expects 0 arguments")
			}
			return receiver.Fields["field"], receiver, false, true, nil
		case "getChildSObject":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getChildSObject expects 0 arguments")
			}
			return receiver.Fields["childSObject"], receiver, false, true, nil
		case "isCascadeDelete":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.isCascadeDelete expects 0 arguments")
			}
			return receiver.Fields["cascadeDelete"], receiver, false, true, nil
		case "isRestrictedDelete":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.isRestrictedDelete expects 0 arguments")
			}
			return receiver.Fields["restrictedDelete"], receiver, false, true, nil
		}
	case "HttpRequest":
		method = canonicalStdlibMemberName(method, "setEndpoint", "getEndpoint", "setMethod", "getMethod", "setBody", "setBodyAsBlob", "setBodyDocument", "getBodyDocument", "setClientCertificateName", "setClientCertificate", "setHeader", "getHeaderKeys", "getHeader", "setCompressed", "getCompressed", "setTimeout", "getTimeout", "getBody", "getBodyAsBlob")
		switch method {
		case "setEndpoint":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setEndpoint expects String")
			}
			if err := validateHttpEndpoint(args[0].Text); err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["endpoint"] = args[0]
			return Null, receiver, true, true, nil
		case "getEndpoint":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getEndpoint expects 0 arguments")
			}
			return receiver.Fields["endpoint"], receiver, false, true, nil
		case "setMethod":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setMethod expects String")
			}
			method, err := normalizeHttpMethod(args[0].Text)
			if err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["method"] = String(method)
			return Null, receiver, true, true, nil
		case "getMethod":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getMethod expects 0 arguments")
			}
			return receiver.Fields["method"], receiver, false, true, nil
		case "setBody":
			if len(args) == 1 && args[0].Kind == ValueNull {
				return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
			}
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setBody expects String")
			}
			receiver.Fields["body"] = args[0]
			return Null, receiver, true, true, nil
		case "setBodyAsBlob":
			if len(args) == 1 && args[0].Kind == ValueNull {
				return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
			}
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setBodyAsBlob expects Blob")
			}
			receiver.Fields["body"] = args[0].Fields["value"]
			return Null, receiver, true, true, nil
		case "setBodyDocument":
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Dom.Document" {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setBodyDocument expects Dom.Document")
			}
			receiver.Fields["body"] = String(domDocumentXMLString(args[0]))
			return Null, receiver, true, true, nil
		case "setClientCertificateName":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setClientCertificateName expects String")
			}
			return Null, receiver, false, true, unsupportedCallError("HttpRequest.setClientCertificateName local client certificate callout surface")
		case "setClientCertificate":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setClientCertificate expects certificate and password Strings")
			}
			return Null, receiver, false, true, unsupportedCallError("HttpRequest.setClientCertificate local client certificate callout surface")
		case "setHeader":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setHeader expects name and value Strings")
			}
			httpSetHeader(receiver, args[0].Text, args[1])
			return Null, receiver, true, true, nil
		case "getHeaderKeys":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getHeaderKeys expects 0 arguments")
			}
			return httpHeaderKeys(receiver), receiver, false, true, nil
		case "getHeader":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getHeader expects name String")
			}
			return httpGetHeader(receiver, args[0].Text), receiver, false, true, nil
		case "setCompressed":
			if len(args) != 1 || args[0].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setCompressed expects Boolean")
			}
			receiver.Fields["compressed"] = args[0]
			return Null, receiver, true, true, nil
		case "getCompressed":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getCompressed expects 0 arguments")
			}
			if value, ok := receiver.Fields["compressed"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "setTimeout":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setTimeout expects Integer")
			}
			if err := validateHttpTimeout(args[0].Int); err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["timeout"] = args[0]
			return Null, receiver, true, true, nil
		case "getTimeout":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getTimeout expects 0 arguments")
			}
			if value, ok := receiver.Fields["timeout"]; ok {
				return value, receiver, false, true, nil
			}
			return Int(defaultHttpTimeoutMillis), receiver, false, true, nil
		case "getBody":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getBody expects 0 arguments")
			}
			return receiver.Fields["body"], receiver, false, true, nil
		case "getBodyDocument":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getBodyDocument expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			doc, err := parseDomDocument(body)
			if err != nil {
				return Null, receiver, false, true, newExceptionError("XmlException", err.Error())
			}
			return doc, receiver, false, true, nil
		case "getBodyAsBlob":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.getBodyAsBlob expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			return platformScalar("Blob", body), receiver, false, true, nil
		}
	case "HttpResponse":
		method = canonicalStdlibMemberName(method, "setBody", "setBodyAsBlob", "getBody", "getBodyAsBlob", "getBodyDocument", "getXmlStreamReader", "setStatusCode", "setStatus", "getStatus", "setHeader", "getHeaderKeys", "getHeader", "getStatusCode")
		switch method {
		case "setBody":
			if len(args) == 1 && args[0].Kind == ValueNull {
				return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
			}
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setBody expects String")
			}
			receiver.Fields["body"] = args[0]
			return Null, receiver, true, true, nil
		case "setBodyAsBlob":
			if len(args) == 1 && args[0].Kind == ValueNull {
				return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
			}
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "Blob" {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setBodyAsBlob expects Blob")
			}
			receiver.Fields["body"] = args[0].Fields["value"]
			return Null, receiver, true, true, nil
		case "getBody":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getBody expects 0 arguments")
			}
			return receiver.Fields["body"], receiver, false, true, nil
		case "getBodyAsBlob":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getBodyAsBlob expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			return platformScalar("Blob", body), receiver, false, true, nil
		case "getBodyDocument":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getBodyDocument expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			doc, err := parseDomDocument(body)
			if err != nil {
				return Null, receiver, false, true, newExceptionError("XmlException", err.Error())
			}
			return doc, receiver, false, true, nil
		case "getXmlStreamReader":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getXmlStreamReader expects 0 arguments")
			}
			body := ""
			if value, ok := receiver.Fields["body"]; ok && value.Kind == ValueString {
				body = value.Text
			}
			reader, err := newXmlStreamReader(body)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return reader, receiver, false, true, nil
		case "setStatusCode":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setStatusCode expects Integer")
			}
			receiver.Fields["statusCode"] = args[0]
			return Null, receiver, true, true, nil
		case "setStatus":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setStatus expects String")
			}
			receiver.Fields["status"] = args[0]
			return Null, receiver, true, true, nil
		case "getStatus":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getStatus expects 0 arguments")
			}
			if value, ok := receiver.Fields["status"]; ok {
				return value, receiver, false, true, nil
			}
			return String("OK"), receiver, false, true, nil
		case "setHeader":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.setHeader expects name and value Strings")
			}
			httpSetHeader(receiver, args[0].Text, args[1])
			return Null, receiver, true, true, nil
		case "getHeaderKeys":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getHeaderKeys expects 0 arguments")
			}
			return httpHeaderKeys(receiver), receiver, false, true, nil
		case "getHeader":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getHeader expects name String")
			}
			return httpGetHeader(receiver, args[0].Text), receiver, false, true, nil
		case "getStatusCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("HttpResponse.getStatusCode expects 0 arguments")
			}
			if value, ok := receiver.Fields["statusCode"]; ok {
				return value, receiver, false, true, nil
			}
			return Int(200), receiver, false, true, nil
		}
	case "Http":
		if method == "send" {
			if len(args) != 1 || args[0].Kind != ValueObject || args[0].Type != "HttpRequest" {
				return Null, receiver, false, true, fmt.Errorf("Http.send expects HttpRequest")
			}
			request := args[0]
			hasMock := vm.testContext != nil && vm.testContext.HTTPMock.Kind == ValueObject
			if !hasMock {
				if err := validateHttpRequest(request); err != nil {
					return Null, receiver, false, true, err
				}
			}
			if endpoint, ok := request.Fields["endpoint"]; ok && endpoint.Kind == ValueString && vm.Org != nil {
				if resolved, ok := resource.ResolveEndpoint(vm.Org.Metadata, endpoint.Text); ok {
					request.Fields["resolvedEndpoint"] = String(resolved)
				}
			}
			if err := vm.incrementLimit("callouts", 1); err != nil {
				return Null, receiver, false, true, err
			}
			appendTrace(result, "apex.callout.http", "apex.callout", map[string]any{"operation": "Http.send"})
			if hasMock {
				if target, ok := vm.resolveInstanceMethod(vm.testContext.HTTPMock.Type, "respond"); ok {
					value, err := vm.callMethodWithReceiver(target, vm.testContext.HTTPMock, []Value{request}, &Result{})
					if err != nil {
						return Null, receiver, false, true, err
					}
					if value.Kind == ValueObject && value.Type == "HttpResponse" {
						return value, receiver, false, true, nil
					}
					return Null, receiver, false, true, fmt.Errorf("HttpCalloutMock.respond must return HttpResponse")
				}
				value, err := vm.localHTTPMockResponse(vm.testContext.HTTPMock, request)
				if err != nil {
					return Null, receiver, false, true, err
				}
				return value, receiver, false, true, nil
			}
			if vm.testContext != nil {
				response := newHttpResponse()
				response.Fields["body"] = String("{}")
				response.Fields["status"] = String("OK")
				response.Fields["statusCode"] = Int(200)
				return response, receiver, false, true, nil
			}
			return Null, receiver, false, true, unsupportedCallError("Http.send real network transport")
		}
	case "Cache.Partition", "Cache.OrgPartition", "Cache.SessionPartition", "cache.Partition", "cache.OrgPartition", "cache.SessionPartition":
		value, updatedReceiver, err := vm.callCachePartitionMember(receiver, method, args)
		return value, updatedReceiver, true, true, err
	case "Cache.SecondaryKeyApi", "cache.SecondaryKeyApi":
		value, updatedReceiver, err := vm.callCacheSecondaryKeyMember(receiver, method, args)
		return value, updatedReceiver, true, true, err
	case "Auth.AuthConfiguration":
		switch method {
		case "getAuthProviders":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getAuthProviders expects 0 arguments")
			}
			return List(), receiver, false, true, nil
		case "getAuthConfig":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getAuthConfig expects 0 arguments")
			}
			if value, ok := receiver.Fields["authConfig"]; ok {
				return value, receiver, false, true, nil
			}
			config := Object("Auth.AuthConfig")
			config.Fields["Url"] = receiver.Fields["communityUrl"]
			return config, receiver, false, true, nil
		case "getStartUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getStartUrl expects 0 arguments")
			}
			if value, ok := receiver.Fields["startUrl"]; ok {
				return value, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "Auth.JWT":
		switch method {
		case "setIss":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Auth.JWT.setIss expects 1 argument")
			}
			receiver.Fields["iss"] = args[0]
			return Null, receiver, true, true, nil
		case "toJSONString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.JWT.toJSONString expects 0 arguments")
			}
			fields := make(map[string]any, len(receiver.Fields))
			for field, value := range receiver.Fields {
				if strings.HasPrefix(field, "__") || value.Kind == ValueNull {
					continue
				}
				fields[field] = jsonFromValue(value, true)
			}
			data, err := json.Marshal(fields)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return String(string(data)), receiver, false, true, nil
		}
	case "Metadata.DeployContainer":
		switch method {
		case "addMetadata":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.addMetadata expects metadata")
			}
			values := receiver.Fields["metadata"]
			if values.Kind != ValueList {
				values = List()
			}
			values.List = append(values.List, args[0])
			receiver.Fields["metadata"] = values
			return Null, receiver, true, true, nil
		case "getMetadata":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.getMetadata expects 0 arguments")
			}
			values := receiver.Fields["metadata"]
			if values.Kind != ValueList {
				values = typedList("List<Metadata.Metadata>")
				receiver.Fields["metadata"] = values
			}
			return values, receiver, false, true, nil
		case "removeMetadata":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.removeMetadata expects metadata")
			}
			values := receiver.Fields["metadata"]
			if values.Kind != ValueList {
				return Bool(false), receiver, false, true, nil
			}
			removed := false
			filtered := values
			filtered.List = filtered.List[:0]
			for _, item := range values.List {
				if !removed && item.Equal(args[0]) {
					removed = true
					continue
				}
				filtered.List = append(filtered.List, item)
			}
			receiver.Fields["metadata"] = filtered
			return Bool(removed), receiver, removed, true, nil
		case "removeMetadataByFullName":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.removeMetadataByFullName expects fullName String")
			}
			values := receiver.Fields["metadata"]
			if values.Kind != ValueList {
				return Bool(false), receiver, false, true, nil
			}
			removed := false
			filtered := values
			filtered.List = filtered.List[:0]
			for _, item := range values.List {
				fullName, ok := metadataStringField(item, "fullName")
				if !removed && ok && strings.EqualFold(fullName, args[0].Text) {
					removed = true
					continue
				}
				filtered.List = append(filtered.List, item)
			}
			receiver.Fields["metadata"] = filtered
			return Bool(removed), receiver, removed, true, nil
		}
	case "Messaging.SendEmailResult":
		switch method {
		case "isSuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Messaging.SendEmailResult.isSuccess expects 0 arguments")
			}
			if value, ok := receiver.Fields["success"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(true), receiver, false, true, nil
		case "getErrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Messaging.SendEmailResult.getErrors expects 0 arguments")
			}
			if value, ok := receiver.Fields["errors"]; ok {
				return value, receiver, false, true, nil
			}
			return List(), receiver, false, true, nil
		}
	case "Messaging.SendEmailError":
		return callMessagingDTOGetter(receiver, method, args)
	case "Messaging.RenderEmailTemplateBodyResult":
		return callMessagingDTOGetter(receiver, method, args)
	case "Messaging.RenderEmailTemplateError":
		return callMessagingDTOGetter(receiver, method, args)
	case "Messaging.ActionResult":
		return callMessagingActionResultMember(receiver, method, args)
	case "Messaging.ActionResult.Builder", "Messaging.ActionableNotification.Builder", "Messaging.Builder":
		return callMessagingBuilderMember(receiver, method, args)
	case "Messaging.ActionableNotification":
		return callMessagingDTOGetter(receiver, method, args)
	case "Messaging.CustomNotification":
		return vm.callCustomNotificationMember(receiver, method, args, result)
	case "Messaging.PushNotification":
		return vm.callPushNotificationMember(receiver, method, args, result)
	case "Messaging.EmailFileAttachment":
		return callEmailFileAttachmentMember(receiver, method, args)
	case "Messaging.SingleEmailMessage":
		return callSingleEmailMessageMember(receiver, method, args)
	case "Messaging.MassEmailMessage":
		return callMassEmailMessageMember(receiver, method, args)
	case "Messaging.SendEmailOptions":
		return Null, receiver, false, true, unsupportedCallError("Messaging.SendEmailOptions." + method + " local messaging send-options surface")
	case "Request", "System.Request":
		return vm.callRequestMember(receiver, method, args)
	case "formulaeval.FormulaBuilder":
		return vm.callFormulaBuilderMember(receiver, method, args)
	case "formulaeval.FormulaInstance":
		return vm.callFormulaInstanceMember(receiver, method, args)
	case "FormulaRecalcResult", "System.FormulaRecalcResult":
		return callFormulaRecalcResultMember(receiver, method, args)
	case "FormulaRecalcFieldError", "System.FormulaRecalcFieldError":
		return callFormulaRecalcFieldErrorMember(receiver, method, args)
	case "Flow.Interview":
		switch {
		case strings.EqualFold(method, "start"):
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Flow.Interview.start expects 0 arguments")
			}
			receiver.Fields["started"] = Bool(true)
			return Null, receiver, true, true, nil
		case strings.EqualFold(method, "getVariableValue"):
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Flow.Interview.getVariableValue expects variable name String")
			}
			if variables, ok := receiver.Fields["variables"]; ok && variables.Kind == ValueMap {
				if value, ok := variables.Map[mapKey(args[0])]; ok {
					return value, receiver, false, true, nil
				}
			}
			return Null, receiver, false, true, nil
		}
	case "VisualEditor.DataRow":
		return callVisualEditorDataRowMember(receiver, method, args)
	case "VisualEditor.DynamicPickListRows":
		return callVisualEditorDynamicPickListRowsMember(receiver, method, args)
	case "SelectOption":
		return callSelectOptionMember(receiver, method, args)
	case "Apex.Stack":
		return callApexStackMember(receiver, method, args)
	case "ApexPages.Action":
		return callApexPagesActionMember(receiver, method, args)
	case "ApexPages.Component", "ApexPages.ComponentIteration":
		return callApexPagesComponentMember(receiver, method, args)
	case "ApexPages.IdeaStandardController":
		return callApexPagesIdeaStandardControllerMember(receiver, method, args)
	case "ApexPages.IdeaStandardSetController":
		return callApexPagesIdeaStandardSetControllerMember(receiver, method, args)
	case "ApexPages.KnowledgeArticleVersionStandardController":
		return callApexPagesKnowledgeArticleVersionStandardControllerMember(receiver, method, args)
	case "ApexPages.StandardController":
		return vm.callStandardControllerMember(receiver, method, args, result)
	case "ApexPages.StandardSetController":
		return vm.callStandardSetControllerMember(receiver, method, args, result)
	case "ApexPages.Message":
		switch method {
		case "getSeverity":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getSeverity expects 0 arguments")
			}
			return receiver.Fields["severity"], receiver, false, true, nil
		case "getSummary":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getSummary expects 0 arguments")
			}
			return receiver.Fields["summary"], receiver, false, true, nil
		case "getDetail":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getDetail expects 0 arguments")
			}
			if value, ok := receiver.Fields["detail"]; ok {
				return value, receiver, false, true, nil
			}
			return receiver.Fields["summary"], receiver, false, true, nil
		}
	case "Dom.Document":
		return callDomDocumentMember(receiver, method, args)
	case "Dom.XmlNode":
		return callDomXmlNodeMember(receiver, method, args)
	case "Continuation":
		return callContinuationMember(receiver, method, args)
	case "StaticResourceCalloutMock":
		return callStaticResourceCalloutMockMember(receiver, method, args)
	case "MultiStaticResourceCalloutMock":
		return callMultiStaticResourceCalloutMockMember(receiver, method, args)
	case "PageReference":
		method = canonicalStdlibMemberName(method, "getContent", "getContentAsPDF", "getUrl", "setRedirect", "getRedirect", "getParameters", "getHeaders", "getCookies", "setCookies")
		switch method {
		case "getContent", "getContentAsPDF":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.%s expects 0 arguments", method)
			}
			return Null, receiver, false, true, unsupportedCallError("PageReference." + method + " local Visualforce page rendering surface")
		case "getUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getUrl expects 0 arguments")
			}
			return pageReferenceURL(receiver), receiver, false, true, nil
		case "setRedirect":
			if len(args) != 1 || args[0].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("PageReference.setRedirect expects Boolean")
			}
			receiver.Fields["redirect"] = args[0]
			return Null, receiver, true, true, nil
		case "getRedirect":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getRedirect expects 0 arguments")
			}
			if value, ok := receiver.Fields["redirect"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getParameters":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getParameters expects 0 arguments")
			}
			if value, ok := receiver.Fields["parameters"]; ok {
				return value, receiver, false, true, nil
			}
			params := typedMap("Map<String,String>")
			receiver.Fields["parameters"] = params
			return params, receiver, true, true, nil
		case "getHeaders":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getHeaders expects 0 arguments")
			}
			if value, ok := receiver.Fields["headers"]; ok {
				return value, receiver, false, true, nil
			}
			headers := typedMap("Map<String,String>")
			receiver.Fields["headers"] = headers
			return headers, receiver, true, true, nil
		case "getCookies":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getCookies expects 0 arguments")
			}
			if value, ok := receiver.Fields["cookies"]; ok {
				return value, receiver, false, true, nil
			}
			cookies := typedMap("Map<String,Cookie>")
			receiver.Fields["cookies"] = cookies
			return cookies, receiver, true, true, nil
		case "setCookies":
			if len(args) != 1 || args[0].Kind != ValueList {
				return Null, receiver, false, true, fmt.Errorf("PageReference.setCookies expects List<Cookie>")
			}
			cookies := typedMap("Map<String,Cookie>")
			for _, cookie := range args[0].List {
				if cookie.Kind != ValueObject || !strings.EqualFold(cookie.Type, "Cookie") {
					return Null, receiver, false, true, fmt.Errorf("PageReference.setCookies expects List<Cookie>")
				}
				_, name, ok := objectFieldValue(cookie, "name")
				if !ok || name.Kind != ValueString || name.Text == "" {
					continue
				}
				key := mapKey(name)
				cookies.Map[key] = cookie
				cookies.MapKeys[key] = name
			}
			receiver.Fields["cookies"] = cookies
			return Null, receiver, true, true, nil
		}
	case "Cookie":
		method = canonicalStdlibMemberName(method, "getName", "getValue", "getPath", "getDomain", "getMaxAge", "isSecure", "getSameSite", "isHttpOnly", "toString")
		return callCookieMember(receiver, method, args)
	case "Domain":
		return callDomainMember(receiver, method, args)
	case "Address", "System.Address":
		return callAddressMember(receiver, method, args)
	case "Location", "System.Location":
		return callLocationMember(receiver, method, args)
	case "QueueableDuplicateSignature":
		if strings.EqualFold(method, "toString") {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.toString expects 0 arguments")
			}
			if _, value, ok := objectFieldValue(receiver, "value"); ok {
				return value, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		}
	case "QueueableDuplicateSignature.Builder", "Builder":
		return callQueueableDuplicateSignatureBuilderMember(receiver, method, args)
	case "CURRENCY":
		method = canonicalStdlibMemberName(method, "format", "formatAmount", "toString")
		switch method {
		case "format":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("CURRENCY.format expects 0 arguments")
			}
			return String(currencyISOCode(receiver) + " " + currencyAmountText(receiver)), receiver, false, true, nil
		case "formatAmount":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("CURRENCY.formatAmount expects 0 arguments")
			}
			return String(currencyAmountText(receiver)), receiver, false, true, nil
		case "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("CURRENCY.toString expects 0 arguments")
			}
			return String(currencyISOCode(receiver) + " " + currencyAmountText(receiver)), receiver, false, true, nil
		}
	case "Collator":
		method = canonicalStdlibMemberName(method, "compare")
		if method == "compare" {
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Collator.compare expects two Strings")
			}
			return Int(collatorCompare(args[0].Text, args[1].Text)), receiver, false, true, nil
		}
	case "Search.KnowledgeSuggestionFilter", "Search.QuestionSuggestionFilter":
		return callSearchSuggestionFilterMember(receiver, method, args)
	case "Search.SuggestionOption":
		return callSearchSuggestionOptionMember(receiver, method, args)
	case "Search.SearchResult":
		return callSearchResultMember(receiver, method, args)
	case "Search.SearchResults":
		return callSearchResultsMember(receiver, method, args)
	case "Search.SuggestionResult":
		return callSearchSuggestionResultMember(receiver, method, args)
	case "Search.SuggestionResults":
		return callSearchSuggestionResultsMember(receiver, method, args)
	case "CartExtension.CartCalculateExecutorMock":
		return vm.callVoidMockMember(receiver, method, args)
	case "CartExtension.SplitShipmentServiceMock":
		return vm.callVoidMockMember(receiver, method, args)
	case "CartExtension.AbstractCartCalculator", "CartExtension.CartCalculate", "CartExtension.InventoryCartCalculator",
		"CartExtension.PricingCartCalculator", "CartExtension.PromotionsCartCalculator", "CartExtension.ShippingCartCalculator",
		"CartExtension.TaxCartCalculator":
		return vm.callCartExtensionMockBackedCalculator(receiver, method, args)
	case "CartExtension.SplitShipmentService":
		return vm.callCartExtensionMockBackedSplitShipment(receiver, method, args)
	case "URL":
		if method == "toExternalForm" || method == "toString" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("URL.%s expects 0 arguments", method)
			}
			return receiver.Fields["value"], receiver, false, true, nil
		}
		if method == "getProtocol" || method == "getHost" || method == "getAuthority" ||
			method == "getPath" || method == "getQuery" || method == "getRef" ||
			method == "getFile" || method == "getPort" || method == "getDefaultPort" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("URL.%s expects 0 arguments", method)
			}
			raw, err := platformScalarText(receiver, "URL")
			if err != nil {
				return Null, receiver, false, true, err
			}
			parsed, err := url.Parse(raw)
			if err != nil {
				return Null, receiver, false, true, err
			}
			switch method {
			case "getProtocol":
				return String(parsed.Scheme), receiver, false, true, nil
			case "getHost":
				return String(parsed.Hostname()), receiver, false, true, nil
			case "getAuthority":
				authority := parsed.Host
				if parsed.User != nil {
					authority = parsed.User.String() + "@" + authority
				}
				return String(authority), receiver, false, true, nil
			case "getPath":
				return String(parsed.Path), receiver, false, true, nil
			case "getQuery":
				return String(parsed.RawQuery), receiver, false, true, nil
			case "getRef":
				return String(parsed.Fragment), receiver, false, true, nil
			case "getFile":
				file := parsed.Path
				if parsed.RawQuery != "" {
					file += "?" + parsed.RawQuery
				}
				return String(file), receiver, false, true, nil
			case "getPort":
				if parsed.Port() == "" {
					return Int(-1), receiver, false, true, nil
				}
				port, err := strconv.ParseInt(parsed.Port(), 10, 64)
				if err != nil {
					return Null, receiver, false, true, err
				}
				return Int(port), receiver, false, true, nil
			case "getDefaultPort":
				return Int(defaultURLPort(parsed.Scheme)), receiver, false, true, nil
			}
		}
	}
	if strings.HasPrefix(receiver.Type, "DataWeaveScriptResource.") {
		return callDataWeaveScriptMember(receiver, method, args)
	}
	if value, updated, mutated, handled, err := vm.callPassivePlatformDTOObjectMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) callPassivePlatformDTOObjectMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !vm.isPassivePlatformDTOObject(receiver) {
		return Null, receiver, false, false, nil
	}
	if value, handled, err := callObjectMember(receiver, method, args); handled || err != nil {
		return value, receiver, false, true, err
	}
	if value, updated, mutated, handled, err := callPrefCenterLoadFormDataMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	if value, updated, mutated, handled, err := vm.callPassivePlatformDTOFluentMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	if strings.EqualFold(method, "getAsMap") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getAsMap expects 0 arguments", receiver.Type)
		}
		values := typedMap("Map<String,Object>")
		for field, value := range receiver.Fields {
			if strings.HasPrefix(field, "__") {
				continue
			}
			key := String(field)
			encodedKey := mapKey(key)
			values.Map[encodedKey] = value
			values.MapKeys[encodedKey] = key
		}
		return values, receiver, false, true, nil
	}
	if strings.EqualFold(method, "setCustomField") {
		if len(args) != 2 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("%s.setCustomField expects field name String and value", receiver.Type)
		}
		receiver.Fields[args[0].Text] = args[1]
		return Null, receiver, true, true, nil
	}
	if strings.EqualFold(method, "getCustomField") {
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("%s.getCustomField expects field name String", receiver.Type)
		}
		if _, value, ok := objectFieldValue(receiver, args[0].Text); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	}
	if strings.EqualFold(method, "setError") {
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("%s.setError expects error message and localized error message Strings", receiver.Type)
		}
		receiver.Fields["errorMessage"] = args[0]
		receiver.Fields["localizedErrorMessage"] = args[1]
		receiver.Fields["success"] = Bool(false)
		return Null, receiver, true, true, nil
	}
	if value, updated, mutated, handled, err := vm.callPassivePlatformDTOCollectionMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	if vm.receiverHasRegisteredInstanceMethod(receiver, method, args) {
		return Null, receiver, false, false, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "set"); ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
		}
		field := passiveAccessorFieldName(receiver, suffix)
		receiver.Fields[field] = args[0]
		return Null, receiver, true, true, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "add"); ok {
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 or 2 arguments", receiver.Type, method)
		}
		if len(args) == 2 {
			field := passiveAccessorFieldName(receiver, suffix)
			actual, mapValue, ok := objectFieldValue(receiver, field)
			if !ok {
				actual = field
			}
			if mapValue.Kind != ValueMap {
				mapValue = typedMap("Map<Object,Object>")
			}
			key := mapKey(args[0])
			mapValue.Map[key] = args[1]
			mapValue.MapKeys[key] = args[0]
			receiver.Fields[actual] = mapValue
			return Null, receiver, true, true, nil
		}
		field := passiveAccessorFieldName(receiver, suffix+"s")
		actual, listValue, ok := objectFieldValue(receiver, field)
		if !ok {
			actual = field
		}
		if listValue.Kind != ValueList {
			listValue = List()
			listValue.Type = "List<Object>"
		}
		listValue.List = append(listValue.List, args[0])
		receiver.Fields[actual] = listValue
		return Null, receiver, true, true, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "remove"); ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
		}
		field := passiveAccessorFieldName(receiver, suffix+"s")
		actual, listValue, ok := objectFieldValue(receiver, field)
		if !ok {
			mapField := passiveAccessorFieldName(receiver, suffix)
			if actualMap, mapValue, mapOK := objectFieldValue(receiver, mapField); mapOK && mapValue.Kind == ValueMap {
				key := mapKey(args[0])
				delete(mapValue.Map, key)
				delete(mapValue.MapKeys, key)
				if len(mapValue.MapOrder) > 0 {
					filtered := mapValue.MapOrder[:0]
					for _, orderedKey := range mapValue.MapOrder {
						if orderedKey != key {
							filtered = append(filtered, orderedKey)
						}
					}
					mapValue.MapOrder = filtered
				}
				receiver.Fields[actualMap] = mapValue
				return Null, receiver, true, true, nil
			}
		}
		if ok && listValue.Kind == ValueList {
			filtered := listValue
			filtered.List = filtered.List[:0]
			for _, item := range listValue.List {
				if !item.Equal(args[0]) {
					filtered.List = append(filtered.List, item)
				}
			}
			receiver.Fields[actual] = filtered
		}
		return Null, receiver, true, true, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "get"); ok {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		field := passiveAccessorFieldName(receiver, suffix)
		if _, value, ok := objectFieldValue(receiver, field); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "is"); ok {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		field := passiveAccessorFieldName(receiver, suffix)
		if _, value, ok := objectFieldValue(receiver, field); ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) receiverHasRegisteredInstanceMethod(receiver Value, method string, args []Value) bool {
	if receiver.Kind != ValueObject {
		return false
	}
	candidates := []string{runtimeObjectType(receiver), receiver.Type, receiver.Static}
	seen := make(map[string]bool, len(candidates))
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		key := strings.ToLower(candidate)
		if seen[key] {
			continue
		}
		seen[key] = true
		if _, ok, ambiguous := vm.resolveInstanceMethodForArgs(candidate, method, args); ok || ambiguous {
			return true
		}
	}
	return false
}

func callPrefCenterLoadFormDataMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "pref_center.LoadFormData") {
		return Null, receiver, false, false, nil
	}
	if len(args) == 0 || args[0].Kind != ValueString {
		return Null, receiver, false, false, nil
	}
	fieldID := args[0]
	switch strings.ToLower(method) {
	case "addoption":
		if len(args) != 2 && len(args) != 3 {
			return Null, receiver, false, true, fmt.Errorf("pref_center.LoadFormData.addOption expects fieldId and option or value/label")
		}
		option := args[1]
		if len(args) == 3 {
			option = newSelectOption(args[1], args[2], Bool(false), Bool(true))
		}
		appendLoadFormDataListField(&receiver, "options", fieldID, option, "List<SelectOption>")
		return Null, receiver, true, true, nil
	case "addselectedoption":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("pref_center.LoadFormData.addSelectedOption expects fieldId and option")
		}
		appendLoadFormDataListField(&receiver, "selectedOptions", fieldID, args[1], "List<String>")
		return Null, receiver, true, true, nil
	case "setbuttonlabel":
		return setLoadFormDataMapField(&receiver, method, args, "buttonLabels")
	case "setoptions":
		return setLoadFormDataMapField(&receiver, method, args, "options")
	case "setselectedoption":
		return setLoadFormDataMapField(&receiver, method, args, "selectedOption")
	case "setselectedoptions":
		return setLoadFormDataMapField(&receiver, method, args, "selectedOptions")
	case "settexthint":
		return setLoadFormDataMapField(&receiver, method, args, "textHints")
	case "settextvalue":
		return setLoadFormDataMapField(&receiver, method, args, "textValues")
	default:
		return Null, receiver, false, false, nil
	}
}

func setLoadFormDataMapField(receiver *Value, method string, args []Value, field string) (Value, Value, bool, bool, error) {
	if len(args) != 2 || args[0].Kind != ValueString {
		return Null, *receiver, false, true, fmt.Errorf("pref_center.LoadFormData.%s expects fieldId and value", method)
	}
	setObjectMapValue(receiver, field, args[0], args[1], "Map<String,Object>")
	return Null, *receiver, true, true, nil
}

func appendLoadFormDataListField(receiver *Value, field string, key Value, item Value, listType string) {
	actual, mapValue, ok := objectFieldValue(*receiver, field)
	if !ok {
		actual = field
	}
	if mapValue.Kind != ValueMap {
		mapValue = typedMap("Map<String,Object>")
	}
	mapKeyValue := mapKey(key)
	listValue, ok := mapValue.Map[mapKeyValue]
	if !ok || listValue.Kind != ValueList {
		listValue = typedList(listType)
	}
	listValue.List = append(listValue.List, item)
	mapValue.Map[mapKeyValue] = listValue
	mapValue.MapKeys[mapKeyValue] = key
	receiver.Fields[actual] = mapValue
}

func setObjectMapValue(receiver *Value, field string, key Value, value Value, mapType string) {
	actual, mapValue, ok := objectFieldValue(*receiver, field)
	if !ok {
		actual = field
	}
	if mapValue.Kind != ValueMap {
		mapValue = typedMap(mapType)
	}
	mapKeyValue := mapKey(key)
	mapValue.Map[mapKeyValue] = value
	mapValue.MapKeys[mapKeyValue] = key
	receiver.Fields[actual] = mapValue
}

func (vm *VM) callPassivePlatformDTOCollectionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	_, ok := vm.passivePlatformDTOCollectionMethod(receiver.Type, method, args)
	if !ok {
		return Null, receiver, false, false, nil
	}
	items := receiver.Fields["__items"]
	if items.Kind != ValueList {
		items = typedList("List<Object>")
	}
	switch strings.ToLower(method) {
	case "add":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.add expects 1 argument", receiver.Type)
		}
		items.List = append(items.List, args[0])
		receiver.Fields["__items"] = items
		return Null, receiver, true, true, nil
	case "clear":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.clear expects 0 arguments", receiver.Type)
		}
		items.List = nil
		receiver.Fields["__items"] = items
		return Null, receiver, true, true, nil
	case "size":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.size expects 0 arguments", receiver.Type)
		}
		return Int(int64(len(items.List))), receiver, false, true, nil
	case "isempty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.isEmpty expects 0 arguments", receiver.Type)
		}
		return Bool(len(items.List) == 0), receiver, false, true, nil
	case "iterator", "getiterator":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		return collectionIterator(items), receiver, false, true, nil
	case "get", "getfromlist":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects Integer index", receiver.Type, method)
		}
		index := int(args[0].Int)
		if index < 0 || index >= len(items.List) {
			return Null, receiver, false, true, newExceptionError("ListException", "List index out of bounds: "+strconv.Itoa(index))
		}
		return items.List[index], receiver, false, true, nil
	case "indexof", "getindexof":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
		}
		for i, item := range items.List {
			if item.Equal(args[0]) {
				return Int(int64(i)), receiver, false, true, nil
			}
		}
		return Int(-1), receiver, false, true, nil
	case "remove":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.remove expects 1 argument", receiver.Type)
		}
		filtered := items.List[:0]
		for _, item := range items.List {
			if !item.Equal(args[0]) {
				filtered = append(filtered, item)
			}
		}
		items.List = filtered
		receiver.Fields["__items"] = items
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) passivePlatformDTOCollectionMethod(typeName, method string, args []Value) (Method, bool) {
	spec, ok := vm.passivePlatformDTOMethod(typeName, method, args)
	if ok && generatedPlatformPassiveCollectionMethod(spec) {
		return spec, true
	}
	if !vm.isPassivePlatformDTOType(typeName) {
		return Method{}, false
	}
	methodsByName := generatedPlatformMethodIndex[strings.ToLower(typeName)]
	candidates := methodsByName[strings.ToLower(method)]
	for _, candidate := range candidates {
		if candidate.IsStatic || len(candidate.Params) != len(args) || !generatedPlatformPassiveCollectionMethod(candidate) {
			continue
		}
		return candidate, true
	}
	return Method{}, false
}

func (vm *VM) callPassivePlatformDTOFluentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	spec, ok := vm.passivePlatformDTOMethod(receiver.Type, method, args)
	if !ok {
		return Null, receiver, false, false, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "with"); ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
		}
		target := receiver
		returnType := vm.resolveTypeNameInClass(spec.ClassName, spec.ReturnType)
		if returnType != "" && !strings.EqualFold(returnType, receiver.Type) {
			target = Object(returnType)
			vm.initializeFields(&target, returnType)
			for field, value := range receiver.Fields {
				target.Fields[field] = value
			}
		}
		target.Fields[passiveAccessorFieldName(target, suffix)] = args[0]
		if len(spec.Params) == 1 && !passiveGeneratedPlaceholderParam(spec.Params[0].Name) {
			target.Fields[passiveAccessorFieldName(target, spec.Params[0].Name)] = args[0]
		}
		return target, target, true, true, nil
	}
	if len(args) == 1 && strings.EqualFold(vm.resolveTypeNameInClass(spec.ClassName, spec.ReturnType), receiver.Type) {
		target := receiver
		target.Fields[passiveAccessorFieldName(target, method)] = args[0]
		if len(spec.Params) == 1 && !passiveGeneratedPlaceholderParam(spec.Params[0].Name) {
			target.Fields[passiveAccessorFieldName(target, spec.Params[0].Name)] = args[0]
		}
		return target, target, true, true, nil
	}
	if strings.EqualFold(method, "build") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.build expects 0 arguments", receiver.Type)
		}
		returnType := vm.resolveTypeNameInClass(spec.ClassName, spec.ReturnType)
		if returnType == "" || strings.EqualFold(returnType, "void") {
			return Null, receiver, false, true, nil
		}
		built := Object(returnType)
		vm.initializeFields(&built, returnType)
		for field, value := range receiver.Fields {
			if strings.HasPrefix(field, "__") {
				continue
			}
			built.Fields[field] = value
		}
		vm.normalizeBuiltPassivePlatformDTO(&built)
		return built, receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func (vm *VM) normalizeBuiltPassivePlatformDTO(object *Value) {
	if object == nil || object.Kind != ValueObject {
		return
	}
	if strings.EqualFold(object.Type, "CartExtension.ItemArrange") {
		address := Object("Address")
		for _, field := range []string{"street", "city", "state", "stateCode", "postalCode", "country", "countryCode", "latitude", "longitude", "geocodeAccuracy"} {
			source := "deliverTo" + strings.ToUpper(field[:1]) + field[1:]
			if _, value, ok := objectFieldValue(*object, source); ok {
				address.Fields[field] = value
			}
		}
		if len(address.Fields) != 0 {
			object.Fields["deliveryAddress"] = address
		}
	}
}

func (vm *VM) passivePlatformDTOMethod(typeName, method string, args []Value) (Method, bool) {
	if !vm.isPassivePlatformDTOType(typeName) {
		return Method{}, false
	}
	spec, ok := vm.generatedPlatformMethodForArgs(typeName, method, args, false)
	if !ok {
		return Method{}, false
	}
	return spec, true
}

func (vm *VM) isPassivePlatformDTOObject(receiver Value) bool {
	if receiver.Kind != ValueObject {
		return false
	}
	return vm.isPassivePlatformDTOType(receiver.Type)
}

func (vm *VM) isPassivePlatformDTOType(typeName string) bool {
	if typeName == "" || !strings.Contains(typeName, ".") {
		return false
	}
	if generated, ok := generatedPlatformTypeIndex[strings.ToLower(typeName)]; ok {
		if safeSchemaPassiveDTOTypeName(generated.Name) {
			return true
		}
		return vm.generatedPlatformPassiveDTOShape(generated) || slackGeneratedPlatformPassiveDTOType(generated)
	}
	if class, ok := vm.lookupClass(typeName); ok && !passiveRuntimeClass(class) {
		return false
	}
	if vm.isSObjectLikeType(typeName) {
		return false
	}
	namespace := typeName[:strings.IndexByte(typeName, '.')]
	if namespace == "" {
		return false
	}
	switch strings.ToLower(namespace) {
	case "schema", "apexpages", "messaging", "dom", "system", "database", "test",
		"userinfo", "site", "network", "search", "approval", "security", "eventbus",
		"restcontext", "restrequest", "restresponse":
		return false
	}
	return true
}

func safeSchemaPassiveDTOTypeName(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "schema.datacategory",
		"schema.datacategorygroupsobjecttypepair",
		"schema.describecolorresult",
		"schema.describedatacategorygroupresult",
		"schema.describedatacategorygroupstructureresult":
		return true
	default:
		return false
	}
}

func (vm *VM) generatedPlatformPassiveDTOShape(generated generatedPlatformType) bool {
	if generated.Kind != apexast.DeclarationClass || generatedExecutionSurfaceRuntimeType(generated.Name) {
		return false
	}
	if generatedPlatformPassiveCollectionShape(generated) {
		return true
	}
	if strings.EqualFold(generated.Name, "Invocable.Action") {
		return true
	}
	if strings.EqualFold(generated.Name, "Flow.Interview") {
		return true
	}
	hasDataShape := len(generated.Fields) != 0 || len(generated.Constructors) != 0 || strings.HasSuffix(generated.Name, ".Builder")
	for _, overloads := range generatedPlatformMethodIndex[strings.ToLower(generated.Name)] {
		for _, method := range overloads {
			if hasTypePrefixFold(generated.Name, "ConnectApi") && method.IsStatic && !generatedConnectAPIPassiveStaticMethod(generated.Name, method) {
				return false
			}
			if method.IsStatic && strings.EqualFold(method.Name, "builder") && strings.HasSuffix(method.ReturnType, ".Builder") {
				continue
			}
			if !method.IsStatic && (strings.HasPrefix(strings.ToLower(method.Name), "get") || strings.HasPrefix(strings.ToLower(method.Name), "is")) {
				hasDataShape = true
			}
			if generatedPlatformPassiveDTOMethod(method) {
				continue
			}
			return false
		}
	}
	return hasDataShape
}

func generatedPlatformPassiveCollectionShape(generated generatedPlatformType) bool {
	short := generated.Name
	if dot := strings.LastIndex(short, "."); dot >= 0 {
		short = short[dot+1:]
	}
	if !(strings.HasSuffix(short, "Collection") || strings.HasSuffix(short, "List")) {
		return false
	}
	hasCollectionMethod := false
	for _, overloads := range generatedPlatformMethodIndex[strings.ToLower(generated.Name)] {
		for _, method := range overloads {
			if !generatedPlatformPassiveCollectionMethod(method) {
				return false
			}
			if !genericObjectRuntimeMethod(method) {
				hasCollectionMethod = true
			}
		}
	}
	return hasCollectionMethod
}

func generatedPlatformPassiveCollectionMethod(method Method) bool {
	name := strings.ToLower(method.Name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	if genericObjectRuntimeMethod(method) {
		return true
	}
	switch name {
	case "clear", "size", "isempty", "iterator", "getiterator":
		return len(method.Params) == 0
	case "get", "getfromlist", "indexof", "getindexof":
		return len(method.Params) == 1
	case "add", "remove":
		return len(method.Params) == 1 && strings.EqualFold(method.ReturnType, "void")
	default:
		return false
	}
}

func generatedConnectAPIPassiveStaticMethod(typeName string, method Method) bool {
	return len(method.Params) == 0 &&
		(strings.EqualFold(method.ReturnType, typeName) ||
			(strings.EqualFold(method.Name, "builder") && strings.HasSuffix(method.ReturnType, ".Builder")))
}

func generatedExecutionSurfaceRuntimeType(typeName string) bool {
	for _, prefix := range []string{
		"Cache.",
		"Continuation.",
		"ExternalService.",
		"ExternalServiceTest.",
	} {
		if len(typeName) >= len(prefix) && strings.EqualFold(typeName[:len(prefix)], prefix) {
			return true
		}
	}
	return false
}

func generatedPlatformPassiveDTOMethod(method Method) bool {
	name := strings.ToLower(method.Name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	switch name {
	case "clone", "equals", "hashcode", "tostring", "getbuildversion", "getasmap", "setcustomfield", "getcustomfield":
		return true
	case "build":
		return true
	default:
		if len(method.Params) == 1 && strings.EqualFold(method.ReturnType, "void") &&
			(strings.HasPrefix(name, "add") || strings.HasPrefix(name, "remove")) {
			return true
		}
		if len(method.Params) == 2 && strings.EqualFold(method.ReturnType, "void") && strings.HasPrefix(name, "add") {
			return true
		}
		if len(method.Params) == 3 && strings.EqualFold(method.ReturnType, "void") && strings.HasPrefix(name, "add") {
			return true
		}
		if len(method.Params) == 1 && strings.EqualFold(method.ReturnType, method.ClassName) {
			return true
		}
		return strings.HasPrefix(name, "get") ||
			strings.HasPrefix(name, "set") ||
			strings.HasPrefix(name, "is") ||
			strings.HasPrefix(name, "with")
	}
}

func slackGeneratedPlatformPassiveDTOType(generated generatedPlatformType) bool {
	if generated.Kind != apexast.DeclarationClass || generatedExecutionSurfaceRuntimeType(generated.Name) {
		return false
	}
	return slackGeneratedPlatformPassiveDTOTypeName(generated.Name)
}

func slackGeneratedPlatformPassiveDTOTypeName(typeName string) bool {
	if !strings.HasPrefix(typeName, "Slack.") {
		return false
	}
	short := typeName[strings.LastIndex(typeName, ".")+1:]
	if strings.HasSuffix(short, "Client") && !strings.HasSuffix(short, "ClientMock") {
		return false
	}
	return !strings.HasSuffix(short, "Dispatcher") && !strings.HasSuffix(short, "Provider")
}

func slackGeneratedPlatformPassiveDTOMethod(method Method) bool {
	name := strings.ToLower(method.Name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	if method.IsStatic && name == "builder" && strings.HasSuffix(method.ReturnType, ".Builder") {
		return true
	}
	if strings.HasPrefix(name, "get") || strings.HasPrefix(name, "set") || strings.HasPrefix(name, "is") {
		return true
	}
	if strings.HasSuffix(method.ClassName, ".Builder") && (name == "build" || strings.EqualFold(method.ReturnType, method.ClassName)) {
		return true
	}
	if method.ClassName == "Slack.Builder" && strings.HasSuffix(method.ReturnType, ".Builder") {
		return true
	}
	if strings.HasSuffix(method.ClassName, "ClientMock") && strings.HasPrefix(method.ReturnType, "Slack.") {
		return true
	}
	if strings.EqualFold(method.ReturnType, method.ClassName) {
		return true
	}
	return collectionBase(method.ReturnType) == "List" || collectionBase(method.ReturnType) == "Set" || isMapType(method.ReturnType) ||
		len(method.Params) == 0 && strings.HasPrefix(method.ReturnType, "Slack.")
}

func (vm *VM) callSlackLocalHarnessMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if receiver.Kind != ValueObject {
		return Null, receiver, false, false, nil
	}
	receiverType := slackTestHarnessRuntimeType(receiver.Type)
	name := strings.ToLower(method)
	switch receiverType {
	case "Slack.ActionDispatcher", "Slack.EventDispatcher", "Slack.ShortcutDispatcher", "Slack.SlashCommandDispatcher":
		switch name {
		case "allowunauthenticatedusers":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.allowUnauthenticatedUsers expects 0 arguments", receiverType)
			}
			return Bool(false), receiver, false, true, nil
		case "invoke":
			if len(args) != 2 {
				return Null, receiver, false, true, fmt.Errorf("%s.invoke expects 2 arguments", receiverType)
			}
			return Object("Slack.ActionHandler"), receiver, false, true, nil
		}
	case "Slack.UserMappingUrlServiceProvider":
		switch name {
		case "generatepartnerauthorizationurl":
			if len(args) != 2 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserMappingUrlServiceProvider.generatePartnerAuthorizationUrl expects 2 arguments")
			}
			return String(""), receiver, false, true, nil
		case "generateslackauthorizationurl":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserMappingUrlServiceProvider.generateSlackAuthorizationUrl expects 1 argument")
			}
			return String(""), receiver, false, true, nil
		}
	case "Slack.UserProvisioningProvider":
		switch name {
		case "importusers", "revokeusersbysalesforceid":
			if len(args) != 2 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserProvisioningProvider.%s expects 2 arguments", method)
			}
			return Object("Slack.UserProvisioningResult"), receiver, false, true, nil
		case "revokeusersbyslackid":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserProvisioningProvider.revokeUsersBySlackId expects 1 argument")
			}
			return Object("Slack.UserProvisioningResult"), receiver, false, true, nil
		}
	case "Slack.RunnableHandler":
		if name == "run" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.RunnableHandler.run expects 0 arguments")
			}
			return Null, receiver, false, true, nil
		}
	case "Slack.Button":
		if name == "click" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Button.click expects 0 arguments")
			}
			return Null, receiver, false, true, nil
		}
	case "Slack.Channel":
		switch name {
		case "adduser":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Channel.addUser expects 1 argument")
			}
			members := slackStringListField(receiver, "members")
			if userID := slackUserID(args[0]); userID != "" && !stringListContains(members, userID) {
				members.List = append(members.List, String(userID))
			}
			receiver.Fields["members"] = members
			return Null, receiver, true, true, nil
		case "removeuser":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Channel.removeUser expects 1 argument")
			}
			userID := slackUserID(args[0])
			members := slackStringListField(receiver, "members")
			if userID != "" {
				filtered := members.List[:0]
				for _, item := range members.List {
					if item.Kind != ValueString || item.Text != userID {
						filtered = append(filtered, item)
					}
				}
				members.List = filtered
			}
			receiver.Fields["members"] = members
			return Null, receiver, true, true, nil
		case "canbeopenedbyuser":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Channel.canBeOpenedByUser expects 1 argument")
			}
			return Bool(true), receiver, false, true, nil
		case "sendmessage":
			if len(args) != 2 || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Slack.Channel.sendMessage expects UserSession, String")
			}
			message := slackMessageForText(args[1].Text)
			message.Fields["channel"] = receiver
			if len(args) > 0 && args[0].Kind == ValueObject {
				session := args[0]
				messages := typedList("List<Slack.TestHarness.Message>")
				if existing, ok := session.Fields["messages"]; ok && existing.Kind == ValueList {
					messages = existing
				}
				messages.List = append(messages.List, message)
				session.Fields["messages"] = messages
			}
			return message, receiver, false, true, nil
		}
	case "Slack.Checkbox":
		switch name {
		case "togglevalue":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Checkbox.toggleValue expects 0 arguments")
			}
			current := false
			if value, ok := receiver.Fields["value"]; ok && value.Kind == ValueBool {
				current = value.Bool
			}
			receiver.Fields["value"] = Bool(!current)
			return Null, receiver, true, true, nil
		}
	case "Slack.CheckboxGroup":
		switch name {
		case "togglevalue":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.CheckboxGroup.toggleValue expects 1 argument")
			}
			id := ""
			if args[0].Kind == ValueString {
				id = args[0].Text
			} else if args[0].Kind == ValueObject {
				if value, ok := args[0].Fields["value"]; ok && value.Kind == ValueString {
					id = value.Text
				}
			}
			values := slackStringListField(receiver, "value")
			if id != "" {
				if stringListContains(values, id) {
					filtered := values.List[:0]
					for _, item := range values.List {
						if item.Kind != ValueString || item.Text != id {
							filtered = append(filtered, item)
						}
					}
					values.List = filtered
				} else {
					values.List = append(values.List, String(id))
				}
			}
			receiver.Fields["value"] = values
			return Null, receiver, true, true, nil
		}
	case "Slack.ExternalSelect":
		if name == "query" {
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Slack.ExternalSelect.query expects String")
			}
			receiver.Fields["lastQuery"] = args[0]
			return Null, receiver, true, true, nil
		}
	case "Slack.Message":
		if name == "canbeseenbyuser" {
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Message.canBeSeenByUser expects 1 argument")
			}
			return Bool(true), receiver, false, true, nil
		}
	case "Slack.Modal":
		switch name {
		case "close":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Modal.close expects 0 arguments")
			}
			receiver.Fields["closed"] = Bool(true)
			return Null, receiver, true, true, nil
		case "hasinputerrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Modal.hasInputErrors expects 0 arguments")
			}
			if blocks, ok := receiver.Fields["inputErrorBlocks"]; ok && blocks.Kind == ValueList {
				return Bool(len(blocks.List) > 0), receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "submit":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Modal.submit expects 0 arguments")
			}
			return Bool(true), receiver, false, true, nil
		}
	case "Slack.Overflow":
		if name == "clickoption" {
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Overflow.clickOption expects 1 argument")
			}
			receiver.Fields["selectedOption"] = args[0]
			return Null, receiver, true, true, nil
		}
	}
	return Null, receiver, false, false, nil
}

func slackStringListField(receiver Value, field string) Value {
	if value, ok := receiver.Fields[field]; ok && value.Kind == ValueList {
		return value
	}
	return typedList("List<String>")
}

func slackUserID(value Value) string {
	if value.Kind != ValueObject {
		return ""
	}
	for _, field := range []string{"id", "userId", "name"} {
		if raw, ok := value.Fields[field]; ok && raw.Kind == ValueString {
			return raw.Text
		}
	}
	return ""
}

func stringListContains(list Value, text string) bool {
	if list.Kind != ValueList {
		return false
	}
	for _, item := range list.List {
		if item.Kind == ValueString && item.Text == text {
			return true
		}
	}
	return false
}

func slackMessageForText(text string) Value {
	message := Object("Slack.TestHarness.Message")
	message.Fields["text"] = String(text)
	return message
}

func slackLocalClientHarnessMethod(method Method) bool {
	if method.ReturnType == "" || strings.EqualFold(method.ReturnType, "void") {
		return false
	}
	if !slackLocalClientHarnessType(method.ClassName) {
		return false
	}
	name := strings.ToLower(method.Name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
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

func slackLocalClientHarnessType(typeName string) bool {
	return strings.EqualFold(typeName, "Slack.AppClient") ||
		strings.EqualFold(typeName, "Slack.BotClient") ||
		strings.EqualFold(typeName, "Slack.UserClient")
}

func passiveRuntimeClass(class Class) bool {
	return !class.IsTest &&
		!class.IsInterface &&
		passiveRuntimeMethods(class.Methods) &&
		passiveRuntimeConstructors(class.Constructors) &&
		len(class.StaticInitializers) == 0 &&
		len(class.InstanceInitializers) == 0
}

func genericObjectRuntimeMethod(method Method) bool {
	name := strings.ToLower(method.Name)
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	switch name {
	case "clone", "equals", "hashcode", "tostring":
		return true
	default:
		return false
	}
}

func passiveRuntimeMethods(methods map[string]Method) bool {
	for _, method := range methods {
		if !passiveGeneratedMethod(method) {
			return false
		}
	}
	return true
}

func passiveRuntimeConstructors(constructors []Method) bool {
	for _, ctor := range constructors {
		if len(ctor.Program.Instructions) != 0 {
			return false
		}
	}
	return true
}

func bindPassiveConstructorArgs(object *Value, ctor Method, args []Value) {
	for i, arg := range args {
		if i >= len(ctor.Params) {
			return
		}
		field := strings.TrimSpace(ctor.Params[i].Name)
		if field == "" || passiveGeneratedPlaceholderParam(field) {
			continue
		}
		object.Fields[passiveAccessorFieldName(*object, field)] = arg
	}
}

func bindPassiveMethodArgs(object *Value, method Method, frame map[string]Value) {
	for _, param := range method.Params {
		field := strings.TrimSpace(param.Name)
		if field == "" || strings.HasPrefix(field, "arg") {
			continue
		}
		value, ok := frame[field]
		if !ok {
			continue
		}
		object.Fields[passiveAccessorFieldName(*object, field)] = value
	}
}

func passiveAccessorSuffix(method, prefix string) (string, bool) {
	if len(method) <= len(prefix) || !strings.EqualFold(method[:len(prefix)], prefix) {
		return "", false
	}
	return method[len(prefix):], true
}

func passivePropertyAccessorField(method, accessor string) (string, bool) {
	suffix := "." + accessor
	if len(method) <= len(suffix) || !strings.EqualFold(method[len(method)-len(suffix):], suffix) {
		return "", false
	}
	name := strings.TrimSuffix(method, method[len(method)-len(suffix):])
	if dot := strings.LastIndex(name, "."); dot >= 0 {
		name = name[dot+1:]
	}
	return name, name != ""
}

func passiveAccessorFieldName(receiver Value, suffix string) string {
	if suffix == "" {
		return suffix
	}
	field := strings.ToLower(suffix[:1]) + suffix[1:]
	if actual, _, ok := objectFieldValue(receiver, field); ok {
		return actual
	}
	return field
}

func callCookieMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("Cookie.%s expects 0 arguments", method)
	}
	switch method {
	case "getName", "getValue", "getPath", "getDomain", "getSameSite":
		field := passiveAccessorFieldName(receiver, strings.TrimPrefix(method, "get"))
		if _, value, ok := objectFieldValue(receiver, field); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getMaxAge":
		if _, value, ok := objectFieldValue(receiver, "maxAge"); ok {
			return value, receiver, false, true, nil
		}
		return Int(0), receiver, false, true, nil
	case "isSecure":
		if _, value, ok := objectFieldValue(receiver, "secure"); ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case "isHttpOnly":
		if _, value, ok := objectFieldValue(receiver, "httpOnly"); ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case "toString":
		if _, value, ok := objectFieldValue(receiver, "value"); ok && value.Kind == ValueString {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callDataWeaveScriptMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "execute")
	if method != "execute" {
		return Null, receiver, false, false, nil
	}
	inputs := typedMap("Map<String,Object>")
	switch {
	case len(args) == 0:
	case len(args) == 1 && args[0].Kind == ValueMap:
		inputs = args[0]
	default:
		return Null, receiver, false, true, fmt.Errorf("DataWeave.Script.execute expects optional Map<String,Object>")
	}
	scriptName := dataWeaveScriptName(receiver)
	if scriptName == "" {
		scriptName = "anonymous"
	}
	lower := strings.ToLower(scriptName)
	if lower == "exceloutputerror" {
		return Null, receiver, false, true, newExceptionError("DataWeaveScriptException", "Unknown content type `application/xlsx`")
	}
	if lower == "error" || strings.Contains(lower, "error") {
		return Null, receiver, false, true, newExceptionError("DataWeaveScriptException", "Division by zero")
	}
	return newDataWeaveResult(scriptName, inputs), receiver, false, true, nil
}

func callDataWeaveResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getValue", "getValueAsString", "getMimeType")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("DataWeave.Result.%s expects 0 arguments", method)
	}
	switch method {
	case "getValue":
		if _, value, ok := objectFieldValue(receiver, "value"); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getValueAsString":
		if _, value, ok := objectFieldValue(receiver, "valueAsString"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getMimeType":
		if _, value, ok := objectFieldValue(receiver, "mimeType"); ok {
			return value, receiver, false, true, nil
		}
		return String("application/apex"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callDomainMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getDomainType", "getMyDomainName", "getPackageName", "getSandboxName", "getSitesSubdomainName", "clone", "toString")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("Domain.%s expects 0 arguments", method)
	}
	switch method {
	case "getDomainType":
		if _, value, ok := objectFieldValue(receiver, "domainType"); ok {
			return value, receiver, false, true, nil
		}
		return domainTypeForHostname(""), receiver, false, true, nil
	case "getMyDomainName":
		if _, value, ok := objectFieldValue(receiver, "myDomainName"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getPackageName":
		if _, value, ok := objectFieldValue(receiver, "packageName"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getSandboxName":
		if _, value, ok := objectFieldValue(receiver, "sandboxName"); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getSitesSubdomainName":
		if _, value, ok := objectFieldValue(receiver, "sitesSubdomainName"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "clone":
		clone := Object("Domain")
		for field, value := range receiver.Fields {
			clone.Fields[field] = value
		}
		return clone, receiver, false, true, nil
	case "toString":
		if _, value, ok := objectFieldValue(receiver, "hostname"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callAddressMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getDistance")
	if method == "getDistance" {
		if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Address.getDistance expects Location and unit String")
		}
		value, err := locationDistance(receiver, args[0], args[1].Text)
		return value, receiver, false, true, err
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("Address.%s expects 0 arguments", method)
	}
	if suffix, ok := passiveAccessorSuffix(method, "get"); ok {
		field := passiveAccessorFieldName(receiver, suffix)
		if _, value, ok := objectFieldValue(receiver, field); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	}
	return Null, receiver, false, false, nil
}

func callLocationMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getLatitude", "getLongitude", "getDistance", "toString")
	switch method {
	case "getLatitude", "getLongitude":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Location.%s expects 0 arguments", method)
		}
		field := passiveAccessorFieldName(receiver, strings.TrimPrefix(method, "get"))
		if _, value, ok := objectFieldValue(receiver, field); ok {
			return value, receiver, false, true, nil
		}
		return Decimal(0), receiver, false, true, nil
	case "getDistance":
		if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Location.getDistance expects Location and unit String")
		}
		value, err := locationDistance(receiver, args[0], args[1].Text)
		return value, receiver, false, true, err
	case "toString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Location.toString expects 0 arguments")
		}
		lat, _ := locationCoordinate(receiver, "latitude")
		lon, _ := locationCoordinate(receiver, "longitude")
		return String(fmt.Sprintf("Location[%g,%g]", lat, lon)), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callQueueableDuplicateSignatureBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "addId", "addInteger", "addString", "build")
	switch method {
	case "addId", "addInteger", "addString":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.Builder.%s expects 1 argument", method)
		}
		parts, ok := receiver.Fields["parts"]
		if !ok || parts.Kind != ValueList {
			parts = typedList("List<String>")
		}
		kind := strings.TrimPrefix(method, "add")
		parts.List = append(parts.List, String(kind+":"+args[0].String()))
		receiver.Fields["parts"] = parts
		return receiver, receiver, true, true, nil
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.Builder.build expects 0 arguments")
		}
		parts, _ := receiver.Fields["parts"]
		textParts := make([]string, 0, len(parts.List))
		if parts.Kind == ValueList {
			for _, part := range parts.List {
				textParts = append(textParts, scalarText(part))
			}
		}
		signature := Object("QueueableDuplicateSignature")
		signature.Fields["value"] = String(strings.Join(textParts, "|"))
		return signature, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSearchSuggestionFilterMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if suffix, ok := passiveAccessorSuffix(method, "set"); ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 1 argument", receiver.Type, method)
		}
		receiver.Fields[passiveAccessorFieldName(receiver, suffix)] = args[0]
		return Null, receiver, true, true, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "add"); ok {
		if len(args) == 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects arguments", receiver.Type, method)
		}
		field := passiveAccessorFieldName(receiver, suffix+"s")
		list, ok := receiver.Fields[field]
		if !ok || list.Kind != ValueList {
			list = typedList("List<Object>")
		}
		if len(args) == 1 {
			list.List = append(list.List, args[0])
		} else {
			list.List = append(list.List, List(args...))
		}
		receiver.Fields[field] = list
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func callSearchSuggestionOptionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "setfilter":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Search.SuggestionOption.setFilter expects filter")
		}
		receiver.Fields["filter"] = args[0]
		return Null, receiver, true, true, nil
	case "setlimit":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("Search.SuggestionOption.setLimit expects Integer")
		}
		receiver.Fields["limit"] = args[0]
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callVoidMockMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	m, ok := vm.generatedPlatformMethodForArgs(receiver.Type, method, args, false)
	if !ok || !strings.EqualFold(m.ReturnType, "void") {
		return Null, receiver, false, false, nil
	}
	return Null, receiver, false, true, nil
}

func (vm *VM) callCartExtensionMockBackedCalculator(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	mock, ok := receiver.Fields["mockExecutor"]
	if !ok || mock.Kind != ValueObject || !strings.EqualFold(mock.Type, "CartExtension.CartCalculateExecutorMock") {
		return Null, receiver, false, false, nil
	}
	method = cartExtensionMockExecutorMethod(receiver.Type, method)
	value, updatedMock, mutated, handled, err := vm.callVoidMockMember(mock, method, args)
	if mutated {
		receiver.Fields["mockExecutor"] = updatedMock
	}
	return value, receiver, mutated, handled, err
}

func cartExtensionMockExecutorMethod(receiverType, method string) string {
	if !strings.EqualFold(method, "calculate") {
		return method
	}
	switch receiverType {
	case "CartExtension.InventoryCartCalculator":
		return "inventory"
	case "CartExtension.PricingCartCalculator":
		return "prices"
	case "CartExtension.PromotionsCartCalculator":
		return "promotions"
	case "CartExtension.ShippingCartCalculator":
		return "shipping"
	case "CartExtension.TaxCartCalculator":
		return "tax"
	default:
		return method
	}
}

func (vm *VM) callCartExtensionMockBackedSplitShipment(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	mock, ok := receiver.Fields["mockService"]
	if !ok || mock.Kind != ValueObject || !strings.EqualFold(mock.Type, "CartExtension.SplitShipmentServiceMock") {
		return Null, receiver, false, false, nil
	}
	value, updatedMock, mutated, handled, err := vm.callVoidMockMember(mock, method, args)
	if mutated {
		receiver.Fields["mockService"] = updatedMock
	}
	return value, receiver, mutated, handled, err
}

func callSfsqlquerySqlQueueableMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch method {
	case "cancel", "processDataChunk":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlQueueable.%s expects 0 arguments", method)
		}
		return Null, receiver, false, true, nil
	case "chainNextJob":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "sfsqlquery.QueryHandle") {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlQueueable.chainNextJob expects QueryHandle")
		}
		receiver.Fields["nextJob"] = args[0]
		return Null, receiver, true, true, nil
	case "getColumnNames":
		return databaseObjectGetter(receiver, method, args, "columnNames", typedList("List<String>"))
	case "getMetadata":
		return databaseObjectGetter(receiver, method, args, "metadata", typedList("List<ConnectApi.QuerySqlMetadataItem>"))
	case "getPageOutput":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("sfsqlquery.SqlQueueable.getPageOutput expects 0 arguments")
		}
		page := Object("ConnectApi.QuerySqlPageOutput")
		page.Fields["rows"] = receiver.Fields["rows"]
		page.Fields["metadata"] = receiver.Fields["metadata"]
		return page, receiver, false, true, nil
	case "getQueryId":
		return databaseObjectGetter(receiver, method, args, "queryId", String(""))
	case "getRows":
		return databaseObjectGetter(receiver, method, args, "rows", typedList("List<sfsqlquery.Row>"))
	default:
		return Null, receiver, false, false, nil
	}
}

func callSearchResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getSObject", "getSnippet")
	switch method {
	case "getSObject":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Search.SearchResult.getSObject expects 0 arguments")
		}
		if _, value, ok := objectFieldValue(receiver, "sObject"); ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getSnippet":
		if len(args) > 1 {
			return Null, receiver, false, true, fmt.Errorf("Search.SearchResult.getSnippet expects optional field")
		}
		if len(args) == 1 {
			if args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Search.SearchResult.getSnippet field expects String")
			}
			if _, snippets, ok := objectFieldValue(receiver, "snippets"); ok && snippets.Kind == ValueMap {
				if value, ok := snippets.Map[mapKey(args[0])]; ok {
					return value, receiver, false, true, nil
				}
			}
		}
		if _, value, ok := objectFieldValue(receiver, "snippet"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSearchResultsMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(method, "get") {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, receiver, false, true, fmt.Errorf("Search.SearchResults.get expects sObjectType String")
	}
	if _, results, ok := objectFieldValue(receiver, "results"); ok && results.Kind == ValueMap {
		if value, ok := results.Map[mapKey(args[0])]; ok && value.Kind == ValueList {
			return value, receiver, false, true, nil
		}
	}
	return typedList("List<Search.SearchResult>"), receiver, false, true, nil
}

func callSearchSuggestionResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(method, "getSObject") {
		return Null, receiver, false, false, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("Search.SuggestionResult.getSObject expects 0 arguments")
	}
	if _, value, ok := objectFieldValue(receiver, "sObject"); ok {
		return value, receiver, false, true, nil
	}
	return Null, receiver, false, true, nil
}

func callSearchSuggestionResultsMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getSuggestionResults", "hasMoreResults")
	switch method {
	case "getSuggestionResults":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Search.SuggestionResults.getSuggestionResults expects 0 arguments")
		}
		if _, value, ok := objectFieldValue(receiver, "suggestionResults"); ok {
			return value, receiver, false, true, nil
		}
		return typedList("List<Search.SuggestionResult>"), receiver, false, true, nil
	case "hasMoreResults":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Search.SuggestionResults.hasMoreResults expects 0 arguments")
		}
		if _, value, ok := objectFieldValue(receiver, "hasMoreResults"); ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callRestRequestMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "addHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.addHeader expects name and value Strings")
		}
		restMapPut(&receiver, "headers", args[0].Text, args[1], true)
		return Null, receiver, true, true, nil
	case "getHeader":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.getHeader expects name String")
		}
		return restMapGet(receiver, "headers", args[0].Text), receiver, false, true, nil
	case "getHeaderKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.getHeaderKeys expects 0 arguments")
		}
		return restMapKeys(receiver, "headers"), receiver, false, true, nil
	case "addParameter", "addParam":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects name and value Strings", method)
		}
		restMapPut(&receiver, "params", args[0].Text, args[1], false)
		return Null, receiver, true, true, nil
	case "getParameter", "getParam":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects name String", method)
		}
		return restMapGet(receiver, "params", args[0].Text), receiver, false, true, nil
	case "getParameterKeys", "getParamKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects 0 arguments", method)
		}
		return restMapKeys(receiver, "params"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callStaticResourceCalloutMockMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setStaticResource":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setStaticResource expects String")
		}
		receiver.Fields["staticResource"] = args[0]
		return Null, receiver, true, true, nil
	case "setStatusCode":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setStatusCode expects Integer")
		}
		receiver.Fields["statusCode"] = args[0]
		return Null, receiver, true, true, nil
	case "setHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setHeader expects name and value Strings")
		}
		httpSetHeader(receiver, args[0].Text, args[1])
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callMultiStaticResourceCalloutMockMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "setStaticResource":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setStaticResource expects endpoint and static resource Strings")
		}
		resources, ok := receiver.Fields["staticResources"]
		if !ok || resources.Kind != ValueMap {
			resources = typedMap("Map<String,String>")
		}
		resources.Map[mapKey(args[0])] = args[1]
		receiver.Fields["staticResources"] = resources
		return Null, receiver, true, true, nil
	case "setStatusCode":
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setStatusCode expects Integer")
		}
		receiver.Fields["statusCode"] = args[0]
		return Null, receiver, true, true, nil
	case "setHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setHeader expects name and value Strings")
		}
		httpSetHeader(receiver, args[0].Text, args[1])
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) localHTTPMockResponse(mock Value, request Value) (Value, error) {
	switch mock.Type {
	case "StaticResourceCalloutMock":
		resource, ok := mock.Fields["staticResource"]
		if !ok || resource.Kind != ValueString || strings.TrimSpace(resource.Text) == "" {
			return Null, fmt.Errorf("StaticResourceCalloutMock static resource is required before Http.send")
		}
		return vm.staticResourceMockResponse(mock, resource.Text), nil
	case "MultiStaticResourceCalloutMock":
		endpoint, ok := request.Fields["endpoint"]
		if !ok || endpoint.Kind != ValueString {
			return Null, fmt.Errorf("MultiStaticResourceCalloutMock request endpoint is missing")
		}
		resources, ok := mock.Fields["staticResources"]
		if !ok || resources.Kind != ValueMap {
			return Null, fmt.Errorf("MultiStaticResourceCalloutMock has no static resource for endpoint %s", endpoint.Text)
		}
		resource, ok := resources.Map[mapKey(endpoint)]
		if !ok {
			if resolved, hasResolved := request.Fields["resolvedEndpoint"]; hasResolved && resolved.Kind == ValueString {
				resource, ok = resources.Map[mapKey(resolved)]
			}
		}
		if !ok || resource.Kind != ValueString || strings.TrimSpace(resource.Text) == "" {
			return Null, fmt.Errorf("MultiStaticResourceCalloutMock has no static resource for endpoint %s", endpoint.Text)
		}
		return vm.staticResourceMockResponse(mock, resource.Text), nil
	default:
		response := newHttpResponse()
		if body, ok := mock.Fields["body"]; ok {
			response.Fields["body"] = body
		}
		if status, ok := mock.Fields["statusCode"]; ok {
			response.Fields["statusCode"] = status
		}
		if headers, ok := mock.Fields["headers"]; ok {
			response.Fields["headers"] = headers
		}
		return response, nil
	}
}

func (vm *VM) callCachePartitionMember(receiver Value, method string, args []Value) (Value, Value, error) {
	name, ok := receiver.Fields["name"]
	if !ok || name.Kind != ValueString || strings.TrimSpace(name.Text) == "" {
		name = String("default")
		receiver.Fields["name"] = name
	}
	partitionName := cachePartitionKey(receiver.Type, name.Text)
	method = strings.ToLower(method)
	switch method {
	case "get":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, fmt.Errorf("%s.get expects key or CacheBuilder type and key", receiver.Type)
		}
		if len(args) == 1 && args[0].Kind == ValueSet {
			out := typedMap("Map<String,Object>")
			for _, key := range args[0].Set {
				if key.Kind != ValueString {
					return Null, receiver, fmt.Errorf("%s.get keys expects Set<String>", receiver.Type)
				}
				if value, ok := vm.cacheGet(partitionName, key.Text); ok {
					out.Map[mapKey(key)] = value
					out.MapKeys[mapKey(key)] = key
				}
			}
			return out, receiver, nil
		}
		keyArg := args[0]
		if len(args) == 2 {
			keyArg = args[1]
		}
		if keyArg.Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.get key expects String", receiver.Type)
		}
		value, err := vm.cacheGetOrLoad(partitionName, cacheKeyForArgs(args, keyArg.Text), cacheBuilderArg(args), keyArg.Text)
		return value, receiver, err
	case "put":
		if len(args) < 2 || len(args) > 5 || args[0].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.put expects String key, value[, ttlSeconds[, visibility[, immutable]]]", receiver.Type)
		}
		ttl, err := cachePutTTL(receiver.Type+".put", args)
		if err != nil {
			return Null, receiver, err
		}
		vm.cachePut(partitionName, args[0].Text, args[1], ttl)
		return Null, receiver, nil
	case "remove":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, fmt.Errorf("%s.remove expects key or CacheBuilder type and key", receiver.Type)
		}
		keyArg := args[0]
		if len(args) == 2 {
			keyArg = args[1]
		}
		if keyArg.Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.remove key expects String", receiver.Type)
		}
		removed, ok := vm.cacheRemove(partitionName, cacheKeyForArgs(args, keyArg.Text))
		if !ok {
			return Null, receiver, nil
		}
		return removed, receiver, nil
	case "contains":
		if len(args) != 1 || (args[0].Kind != ValueString && args[0].Kind != ValueSet) {
			return Null, receiver, fmt.Errorf("%s.contains expects String key or Set<String>", receiver.Type)
		}
		if args[0].Kind == ValueSet {
			out := typedMap("Map<String,Boolean>")
			for _, key := range args[0].Set {
				if key.Kind != ValueString {
					return Null, receiver, fmt.Errorf("%s.contains keys expects Set<String>", receiver.Type)
				}
				_, ok := vm.cacheGet(partitionName, key.Text)
				out.Map[mapKey(key)] = Bool(ok)
				out.MapKeys[mapKey(key)] = key
			}
			return out, receiver, nil
		}
		_, ok := vm.cacheGet(partitionName, args[0].Text)
		return Bool(ok), receiver, nil
	case "getkeys":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getKeys expects no arguments", receiver.Type)
		}
		keys := vm.cacheKeys(partitionName)
		out := Set()
		out.Type = "Set<String>"
		for _, key := range keys {
			out.Set = append(out.Set, String(key))
		}
		return out, receiver, nil
	case "getnumkeys":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getNumKeys expects no arguments", receiver.Type)
		}
		return Int(int64(len(vm.cacheKeys(partitionName)))), receiver, nil
	case "getcapacity":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getCapacity expects no arguments", receiver.Type)
		}
		return Decimal(100), receiver, nil
	case "isavailable":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.isAvailable expects no arguments", receiver.Type)
		}
		return Bool(true), receiver, nil
	case "getname":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getName expects no arguments", receiver.Type)
		}
		return String(cacheNormalizePartitionName(name.Text)), receiver, nil
	case "getavggetsize", "getavggettime", "getavgvaluesize", "getmaxgetsize", "getmaxgettime", "getmaxvaluesize":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.%s expects no arguments", receiver.Type, method)
		}
		return Int(0), receiver, nil
	case "getmissrate":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getMissRate expects no arguments", receiver.Type)
		}
		return Decimal(0), receiver, nil
	case "createfullyqualifiedpartition":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.createFullyQualifiedPartition expects namespace and partition Strings", receiver.Type)
		}
		return String(cacheFullyQualifiedPartition(args[0].Text, args[1].Text)), receiver, nil
	case "createfullyqualifiedkey":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.createFullyQualifiedKey expects namespace, partition, and key Strings", receiver.Type)
		}
		return String(cacheFullyQualifiedPartition(args[0].Text, args[1].Text) + "." + args[2].Text), receiver, nil
	case "validatecachebuilder", "validatekey", "validatekeyvalue", "validatekeys", "validatepartitionname":
		return Null, receiver, nil
	default:
		return Null, receiver, unsupportedCallError(receiver.Type + "." + method)
	}
}

func (vm *VM) callCacheSecondaryKeyMember(receiver Value, method string, args []Value) (Value, Value, error) {
	feature, ok := receiver.Fields["featureName"]
	if !ok || feature.Kind != ValueString || strings.TrimSpace(feature.Text) == "" {
		feature = String("default")
		receiver.Fields["featureName"] = feature
	}
	partition := cacheSecondaryKeyPartition(feature.Text)
	method = strings.ToLower(method)
	switch method {
	case "putimmediate":
		if len(args) != 3 || args[0].Kind != ValueString || args[2].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.putImmediate expects key String, value, and secondaryKey String", receiver.Type)
		}
		vm.cachePutSecondary(partition, args[0].Text, args[1], args[2].Text)
		return Null, receiver, nil
	case "remove":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.remove expects key String", receiver.Type)
		}
		_, removed := vm.cacheRemove(partition, args[0].Text)
		return Bool(removed), receiver, nil
	case "scanforcount":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, fmt.Errorf("%s.scanForCount expects startKey and endKey Strings", receiver.Type)
		}
		return Int(int64(len(vm.cacheSecondaryScan(partition, args[0].Text, args[1].Text)))), receiver, nil
	case "scanforkeyvalues":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueInt {
			return Null, receiver, fmt.Errorf("%s.scanForKeyValues expects startKey String, endKey String, and batchSize Integer", receiver.Type)
		}
		items := vm.cacheSecondaryScan(partition, args[0].Text, args[1].Text)
		return vm.cacheScanResult(items, int(args[2].Int)), receiver, nil
	case "scanformorekeyvalues":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueInt {
			return Null, receiver, fmt.Errorf("%s.scanForMoreKeyValues expects scanLocator String and batchSize Integer", receiver.Type)
		}
		items := vm.cacheScanLocators[args[0].Text]
		delete(vm.cacheScanLocators, args[0].Text)
		return vm.cacheScanResult(items, int(args[1].Int)), receiver, nil
	default:
		return Null, receiver, unsupportedCallError(receiver.Type + "." + method)
	}
}

func cacheFullyQualifiedPartition(namespace, partition string) string {
	namespace = strings.TrimSpace(namespace)
	partition = cacheNormalizePartitionName(partition)
	if namespace == "" {
		return partition
	}
	if partition == "" {
		return namespace
	}
	if strings.HasPrefix(strings.ToLower(partition), strings.ToLower(namespace)+".") {
		return partition
	}
	return namespace + "." + partition
}

func (vm *VM) cacheStaticDefaultGet(callee string, args []Value) (Value, error) {
	partition := cacheDefaultPartitionKey(callee)
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("%s expects key or CacheBuilder type and key", callee)
	}
	keyArg := args[0]
	if len(args) == 2 {
		keyArg = args[1]
	}
	if keyArg.Kind != ValueString {
		return Null, fmt.Errorf("%s key expects String", callee)
	}
	return vm.cacheGetOrLoad(partition, cacheKeyForArgs(args, keyArg.Text), cacheBuilderArg(args), keyArg.Text)
}

func (vm *VM) cacheStaticDefaultPut(callee string, args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 5 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("%s expects String key, value[, ttlSeconds[, visibility[, immutable]]]", callee)
	}
	ttl, err := cachePutTTL(callee, args)
	if err != nil {
		return Null, err
	}
	vm.cachePut(cacheDefaultPartitionKey(callee), args[0].Text, args[1], ttl)
	return Null, nil
}

func cachePutTTL(callee string, args []Value) (int64, error) {
	if len(args) < 3 {
		return 0, nil
	}
	if args[2].Kind == ValueInt {
		return args[2].Int, nil
	}
	if len(args) == 3 && cacheVisibilityValue(args[2]) {
		return 0, nil
	}
	return 0, fmt.Errorf("%s ttl expects Integer seconds", callee)
}

func cacheVisibilityValue(value Value) bool {
	return value.Kind == ValueObject && strings.EqualFold(value.Type, "Cache.Visibility")
}

func (vm *VM) cacheStaticDefaultRemove(callee string, args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("%s expects key or CacheBuilder type and key", callee)
	}
	keyArg := args[0]
	if len(args) == 2 {
		keyArg = args[1]
	}
	if keyArg.Kind != ValueString {
		return Null, fmt.Errorf("%s key expects String", callee)
	}
	removed, ok := vm.cacheRemove(cacheDefaultPartitionKey(callee), cacheKeyForArgs(args, keyArg.Text))
	if !ok {
		return Null, nil
	}
	return removed, nil
}

func (vm *VM) cacheStaticDefaultContains(callee string, args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, fmt.Errorf("%s expects String key", callee)
	}
	_, ok := vm.cacheGet(cacheDefaultPartitionKey(callee), args[0].Text)
	return Bool(ok), nil
}

func (vm *VM) cacheStaticDefaultKeys(callee string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s expects 0 arguments", callee)
	}
	out := Set()
	out.Type = "Set<String>"
	for _, key := range vm.cacheKeys(cacheDefaultPartitionKey(callee)) {
		out.Set = append(out.Set, String(key))
	}
	return out, nil
}

func (vm *VM) cacheStaticDefaultNumKeys(callee string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s expects 0 arguments", callee)
	}
	return Int(int64(len(vm.cacheKeys(cacheDefaultPartitionKey(callee))))), nil
}

func cacheBuilderArg(args []Value) Value {
	if len(args) == 2 && args[0].Kind == ValueObject && args[0].Type == "Type" {
		return args[0]
	}
	return Null
}

func cacheKeyForArgs(args []Value, key string) string {
	builderType := cacheBuilderArg(args)
	if builderType.Kind == ValueObject && builderType.Type == "Type" && strings.TrimSpace(builderType.Text) != "" {
		return builderType.Text + "." + key
	}
	return key
}

func (vm *VM) cacheGetOrLoad(partition, cacheKey string, builderType Value, loadKey string) (Value, error) {
	if value, ok := vm.cacheGet(partition, cacheKey); ok {
		return value, nil
	}
	if builderType.Kind != ValueObject || builderType.Type != "Type" || strings.TrimSpace(builderType.Text) == "" {
		return Null, nil
	}
	if !vm.typeMatches(builderType.Text, "Cache.CacheBuilder", make(map[string]bool)) {
		return Null, nil
	}
	builder, err := vm.constructValue(builderType.Text, nil, nil, &Result{})
	if err != nil {
		return Null, err
	}
	method, ok, ambiguous := vm.resolveInstanceMethodForArgs(builder.Type, "doLoad", []Value{String(loadKey)})
	if ambiguous {
		return Null, vm.ambiguousOverloadError(builder.Type+".doLoad", []Value{String(loadKey)})
	}
	if !ok {
		return Null, fmt.Errorf("%s must implement Cache.CacheBuilder.doLoad(String)", builder.Type)
	}
	value, err := vm.callMethodWithReceiver(method, builder, []Value{String(loadKey)}, &Result{})
	if err != nil {
		return Null, err
	}
	vm.cachePut(partition, cacheKey, value, 0)
	return value, nil
}

func cacheDefaultPartitionKey(callee string) string {
	return cachePartitionKey(cachePartitionTypeFromCallee(callee), cacheDefaultPartitionName(callee))
}

func cacheDefaultPartitionName(callee string) string {
	return "local.default"
}

func cachePartitionTypeFromCallee(callee string) string {
	if strings.HasPrefix(callee, "Cache.Session.") {
		return "Cache.SessionPartition"
	}
	return "Cache.OrgPartition"
}

func cachePartitionKey(partitionType, name string) string {
	return strings.ToLower(partitionType + ":" + cacheNormalizePartitionName(name))
}

func cacheNormalizePartitionName(name string) string {
	name = strings.TrimSpace(name)
	if !strings.Contains(name, ".") {
		return "local." + name
	}
	return name
}

func (vm *VM) cacheGet(partition, key string) (Value, bool) {
	entries := vm.platformCache[partition]
	if entries == nil {
		return Null, false
	}
	entry, ok := entries[key]
	if !ok {
		return Null, false
	}
	if !entry.ExpireAt.IsZero() && !entry.ExpireAt.After(vm.fakeNow) {
		delete(entries, key)
		return Null, false
	}
	return entry.Value, true
}

func (vm *VM) cacheKeys(partition string) []string {
	entries := vm.platformCache[partition]
	if entries == nil {
		return nil
	}
	keys := make([]string, 0, len(entries))
	for key, entry := range entries {
		if !entry.ExpireAt.IsZero() && !entry.ExpireAt.After(vm.fakeNow) {
			delete(entries, key)
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func (vm *VM) cachePut(partition, key string, value Value, ttlSeconds int64) {
	if vm.platformCache == nil {
		vm.platformCache = make(map[string]map[string]cacheEntry)
	}
	entries := vm.platformCache[partition]
	if entries == nil {
		entries = make(map[string]cacheEntry)
		vm.platformCache[partition] = entries
	}
	entry := cacheEntry{Value: value}
	if ttlSeconds > 0 {
		entry.ExpireAt = vm.fakeNow.Add(time.Duration(ttlSeconds) * time.Second)
	}
	entries[key] = entry
}

func (vm *VM) cacheRemove(partition, key string) (Value, bool) {
	value, ok := vm.cacheGet(partition, key)
	if !ok {
		return Null, false
	}
	delete(vm.platformCache[partition], key)
	return value, true
}

func cacheSecondaryKeyPartition(feature string) string {
	feature = strings.TrimSpace(feature)
	if feature == "" {
		feature = "default"
	}
	return strings.ToLower("Cache.SecondaryKeyApi:" + feature)
}

func (vm *VM) cachePutSecondary(partition, key string, value Value, secondaryKey string) {
	if vm.platformCache == nil {
		vm.platformCache = make(map[string]map[string]cacheEntry)
	}
	entries := vm.platformCache[partition]
	if entries == nil {
		entries = make(map[string]cacheEntry)
		vm.platformCache[partition] = entries
	}
	entries[key] = cacheEntry{Value: value, SecondaryKey: secondaryKey}
}

func (vm *VM) cacheSecondaryScan(partition, startKey, endKey string) []cacheScanItem {
	entries := vm.platformCache[partition]
	if entries == nil {
		return nil
	}
	items := make([]cacheScanItem, 0, len(entries))
	for key, entry := range entries {
		if !entry.ExpireAt.IsZero() && !entry.ExpireAt.After(vm.fakeNow) {
			delete(entries, key)
			continue
		}
		secondary := entry.SecondaryKey
		if secondary == "" {
			secondary = key
		}
		if (startKey == "" || secondary >= startKey) && (endKey == "" || secondary <= endKey) {
			items = append(items, cacheScanItem{Key: key, Value: entry.Value})
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Key == items[j].Key {
			return items[i].Value.String() < items[j].Value.String()
		}
		return items[i].Key < items[j].Key
	})
	return items
}

func (vm *VM) cacheScanResult(items []cacheScanItem, batchSize int) Value {
	if batchSize <= 0 || batchSize > len(items) {
		batchSize = len(items)
	}
	result := typedMap("Map<String,Object>")
	for _, item := range items[:batchSize] {
		key := String(item.Key)
		result.Map[mapKey(key)] = item.Value
		result.MapKeys[mapKey(key)] = key
	}
	scan := Object("Cache.ScanResult")
	scan.Fields["result"] = result
	scan.Fields["isDone"] = Bool(batchSize == len(items))
	scan.Fields["scanLocator"] = String("")
	if batchSize < len(items) {
		if vm.cacheScanLocators == nil {
			vm.cacheScanLocators = make(map[string][]cacheScanItem)
		}
		vm.cacheScanSeq++
		locator := fmt.Sprintf("cache-scan-%d", vm.cacheScanSeq)
		vm.cacheScanLocators[locator] = append([]cacheScanItem(nil), items[batchSize:]...)
		scan.Fields["scanLocator"] = String(locator)
	}
	return scan
}

func (vm *VM) staticResourceMockResponse(mock Value, resourceName string) Value {
	response := newHttpResponse()
	response.Fields["body"] = String(vm.staticResourceBody(resourceName))
	if status, ok := mock.Fields["statusCode"]; ok {
		response.Fields["statusCode"] = status
	}
	if headers, ok := mock.Fields["headers"]; ok {
		response.Fields["headers"] = headers
	}
	return response
}

func (vm *VM) staticResourceBody(resourceName string) string {
	if vm.Org == nil {
		return resourceName
	}
	for _, resource := range vm.Org.Metadata.StaticResources {
		if strings.EqualFold(resource.Name, resourceName) {
			if resource.Content != "" {
				return resource.Content
			}
			break
		}
	}
	for _, asset := range vm.Org.Metadata.ContentAssets {
		if strings.EqualFold(asset.Name, resourceName) {
			if asset.Content != "" {
				return asset.Content
			}
			break
		}
	}
	object, ok := vm.Org.Objects["StaticResource"]
	if !ok {
		return resourceName
	}
	for _, record := range object.Records {
		if !staticResourceNameMatches(record, resourceName) {
			continue
		}
		for _, field := range []string{"Body", "Content"} {
			if value, ok := record.GetField(field); ok {
				if body, ok := staticResourceBodyValue(value); ok {
					return body
				}
			}
		}
	}
	return resourceName
}

func (vm *VM) lookupLabel(name string) (Value, bool) {
	label := strings.TrimPrefix(name, "Label.")
	if label == "" {
		return Null, false
	}
	namespace := ""
	if before, after, ok := strings.Cut(label, "."); ok {
		namespace = before
		label = after
	}
	if vm.Org != nil {
		if value, status := resource.ResolveLabel(vm.Org.Metadata, vm.Org.Namespace, namespace, label); status != resource.LabelLookupMissing {
			return String(value), true
		}
	}
	if namespace == "" && !strings.Contains(label, ".") {
		return String(label), true
	}
	return Null, false
}

func staticResourceNameMatches(record storage.Record, resourceName string) bool {
	if strings.EqualFold(string(record.ID), resourceName) {
		return true
	}
	for _, field := range []string{"Name", "DeveloperName"} {
		value, ok := record.GetField(field)
		if ok && value.Kind == storage.ValueString && strings.EqualFold(value.String, resourceName) {
			return true
		}
	}
	return false
}

func staticResourceBodyValue(value storage.Value) (string, bool) {
	switch value.Kind {
	case storage.ValueString, storage.ValueBlob:
		return value.String, true
	default:
		return "", false
	}
}

func callRestResponseMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	switch method {
	case "addHeader":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.addHeader expects name and value Strings")
		}
		restMapPut(&receiver, "headers", args[0].Text, args[1], true)
		return Null, receiver, true, true, nil
	case "getHeader":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.getHeader expects name String")
		}
		return restMapGet(receiver, "headers", args[0].Text), receiver, false, true, nil
	case "getHeaderKeys":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("RestResponse.getHeaderKeys expects 0 arguments")
		}
		return restMapKeys(receiver, "headers"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callSelectOptionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	fieldForGetter := map[string]string{
		"getValue":      "value",
		"getLabel":      "label",
		"getDisabled":   "disabled",
		"getEscapeItem": "escapeItem",
	}
	if field, ok := fieldForGetter[method]; ok {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects 0 arguments", method)
		}
		return receiver.Fields[field], receiver, false, true, nil
	}
	fieldForSetter := map[string]string{
		"setValue":      "value",
		"setLabel":      "label",
		"setDisabled":   "disabled",
		"setEscapeItem": "escapeItem",
	}
	if field, ok := fieldForSetter[method]; ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects 1 argument", method)
		}
		if (field == "value" || field == "label") && args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects String", method)
		}
		if (field == "disabled" || field == "escapeItem") && args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("SelectOption.%s expects Boolean", method)
		}
		receiver.Fields[field] = args[0]
		return Null, receiver, true, true, nil
	}
	return Null, receiver, false, false, nil
}

func callVisualEditorDataRowMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch {
	case strings.EqualFold(method, "getLabel"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.getLabel expects 0 arguments")
		}
		return receiver.Fields["label"], receiver, false, true, nil
	case strings.EqualFold(method, "getValue"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.getValue expects 0 arguments")
		}
		return receiver.Fields["value"], receiver, false, true, nil
	case strings.EqualFold(method, "isSelected"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.isSelected expects 0 arguments")
		}
		if value, ok := receiver.Fields["selected"]; ok && value.Kind == ValueBool {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case strings.EqualFold(method, "compareTo"):
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "VisualEditor.DataRow") {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.compareTo expects DataRow")
		}
		left := scalarText(receiver.Fields["label"])
		right := scalarText(args[0].Fields["label"])
		if cmp := strings.Compare(left, right); cmp != 0 {
			return Int(int64(cmp)), receiver, false, true, nil
		}
		return Int(int64(strings.Compare(scalarText(receiver.Fields["value"]), scalarText(args[0].Fields["value"])))), receiver, false, true, nil
	case strings.EqualFold(method, "setLabel"):
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.setLabel expects String")
		}
		receiver.Fields["label"] = args[0]
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "setValue"):
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DataRow.setValue expects value")
		}
		receiver.Fields["value"] = args[0]
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callVisualEditorDynamicPickListRowsMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	rows := receiver.Fields["rows"]
	if rows.Kind != ValueList {
		rows = typedList("List<VisualEditor.DataRow>")
	}
	switch {
	case strings.EqualFold(method, "addRow"):
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "VisualEditor.DataRow") {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.addRow expects VisualEditor.DataRow")
		}
		rows.List = append(rows.List, args[0])
		receiver.Fields["rows"] = rows
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "addAllRows"):
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.addAllRows expects List<VisualEditor.DataRow>")
		}
		rows.List = append(rows.List, args[0].List...)
		receiver.Fields["rows"] = rows
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "size"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.size expects 0 arguments")
		}
		return Int(int64(len(rows.List))), receiver, false, true, nil
	case strings.EqualFold(method, "containsAllRows"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.containsAllRows expects 0 arguments")
		}
		if _, value, ok := objectFieldValue(receiver, "containsAllRows"); ok && value.Kind == ValueBool {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case strings.EqualFold(method, "get"):
		if len(args) != 1 || args[0].Kind != ValueInt {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.get expects Integer index")
		}
		index := int(args[0].Int)
		if index < 0 || index >= len(rows.List) {
			return Null, receiver, false, true, listIndexException(index)
		}
		return rows.List[index], receiver, false, true, nil
	case strings.EqualFold(method, "getRows"), strings.EqualFold(method, "getDataRows"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.getRows expects 0 arguments")
		}
		return rows, receiver, false, true, nil
	case strings.EqualFold(method, "setContainsAllRows"):
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.setContainsAllRows expects Boolean")
		}
		receiver.Fields["containsAllRows"] = args[0]
		return Null, receiver, true, true, nil
	case strings.EqualFold(method, "sort"):
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("VisualEditor.DynamicPickListRows.sort expects 0 arguments")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func newCompressionZipWriter() Value {
	writer := Object("compression.ZipWriter")
	writer.Fields["entries"] = typedList("List<compression.ZipEntry>")
	writer.Fields["level"] = compressionEnumValue("compression.Level", "DEFAULT_LEVEL")
	writer.Fields["method"] = compressionEnumValue("compression.Method", "DEFLATED")
	return writer
}

func newCompressionZipReader(archive Value) (Value, error) {
	reader := Object("compression.ZipReader")
	reader.Fields["archive"] = archive
	entries, err := readCompressionZipEntries(blobText(archive))
	if err != nil {
		return Null, err
	}
	reader.Fields["entries"] = entries
	return reader, nil
}

func callCompressionZipMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Type {
	case "compression.ZipWriter":
		return callCompressionZipWriterMember(receiver, method, args)
	case "compression.ZipReader":
		return callCompressionZipReaderMember(receiver, method, args)
	case "compression.ZipEntry":
		return callCompressionZipEntryMember(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipWriterMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	entries := compressionZipEntries(receiver)
	switch strings.ToLower(method) {
	case "addentry":
		entry, err := compressionZipEntryFromAddArgs(args)
		if err != nil {
			return Null, receiver, false, true, err
		}
		entries.List = append(entries.List, entry)
		receiver.Fields["entries"] = entries
		return entry, receiver, true, true, nil
	case "getarchive":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getArchive expects 0 arguments")
		}
		archive, err := writeCompressionZipArchive(entries.List)
		if err != nil {
			return Null, receiver, false, true, err
		}
		return platformScalar("Blob", archive), receiver, false, true, nil
	case "getentries":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntries expects 0 arguments")
		}
		return entries, receiver, false, true, nil
	case "getentry":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntry expects String name")
		}
		return compressionZipFindEntry(entries, args[0].Text), receiver, false, true, nil
	case "getentrynames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getEntryNames expects 0 arguments")
		}
		names := Set()
		names.Type = "Set<String>"
		for _, entry := range entries.List {
			if name := compressionZipEntryName(entry); name != "" {
				names.Set = append(names.Set, String(name))
			}
		}
		return names, receiver, false, true, nil
	case "removeentry":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.removeEntry expects String name")
		}
		filtered := typedList("List<compression.ZipEntry>")
		for _, entry := range entries.List {
			if !strings.EqualFold(compressionZipEntryName(entry), args[0].Text) {
				filtered.List = append(filtered.List, entry)
			}
		}
		receiver.Fields["entries"] = filtered
		return Null, receiver, true, true, nil
	case "getlevel":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getLevel expects 0 arguments")
		}
		if value, ok := receiver.Fields["level"]; ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Level", "DEFAULT_LEVEL"), receiver, false, true, nil
	case "getmethod":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.getMethod expects 0 arguments")
		}
		if value, ok := receiver.Fields["method"]; ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Method", "DEFLATED"), receiver, false, true, nil
	case "setlevel":
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Level") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.setLevel expects compression.Level")
		}
		receiver.Fields["level"] = args[0]
		return receiver, receiver, true, true, nil
	case "setmethod":
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Method") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipWriter.setMethod expects compression.Method")
		}
		receiver.Fields["method"] = args[0]
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipReaderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	entries := compressionZipEntries(receiver)
	switch strings.ToLower(method) {
	case "extract":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.extract expects String name or ZipEntry")
		}
		var name string
		if args[0].Kind == ValueString {
			name = args[0].Text
		} else if args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "compression.ZipEntry") {
			name = compressionZipEntryName(args[0])
		} else {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.extract expects String name or ZipEntry")
		}
		entry := compressionZipFindEntry(entries, name)
		if entry.Kind == ValueNull {
			return Null, receiver, false, true, nil
		}
		return compressionZipEntryContent(entry), receiver, false, true, nil
	case "getentries":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntries expects 0 arguments")
		}
		return entries, receiver, false, true, nil
	case "getentriesmap":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntriesMap expects 0 arguments")
		}
		out := typedMap("Map<String,compression.ZipEntry>")
		for _, entry := range entries.List {
			name := compressionZipEntryName(entry)
			key := mapKey(String(name))
			out.Map[key] = entry
			out.MapKeys[key] = String(name)
		}
		return out, receiver, false, true, nil
	case "getentry":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntry expects String name")
		}
		return compressionZipFindEntry(entries, args[0].Text), receiver, false, true, nil
	case "getentrynames":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipReader.getEntryNames expects 0 arguments")
		}
		names := typedList("List<String>")
		for _, entry := range entries.List {
			names.List = append(names.List, String(compressionZipEntryName(entry)))
		}
		return names, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callCompressionZipEntryMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "getname":
		return String(compressionZipEntryName(receiver)), receiver, false, true, nil
	case "getcomment":
		if _, value, ok := objectFieldValue(receiver, "comment"); ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getcontent":
		return compressionZipEntryContent(receiver), receiver, false, true, nil
	case "getcompressedsize", "getuncompressedsize":
		return Int(int64(len([]byte(blobText(compressionZipEntryContent(receiver)))))), receiver, false, true, nil
	case "getcrc":
		return Int(0), receiver, false, true, nil
	case "getmethod":
		if _, value, ok := objectFieldValue(receiver, "method"); ok {
			return value, receiver, false, true, nil
		}
		return compressionEnumValue("compression.Method", "DEFLATED"), receiver, false, true, nil
	case "setcomment":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setComment expects String")
		}
		receiver.Fields["comment"] = args[0]
		return receiver, receiver, true, true, nil
	case "setcontent":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Blob") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setContent expects Blob")
		}
		receiver.Fields["content"] = args[0]
		return receiver, receiver, true, true, nil
	case "setmethod":
		if len(args) != 1 || !strings.EqualFold(args[0].Type, "compression.Method") {
			return Null, receiver, false, true, fmt.Errorf("compression.ZipEntry.setMethod expects compression.Method")
		}
		receiver.Fields["method"] = args[0]
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func compressionZipEntries(receiver Value) Value {
	if _, entries, ok := objectFieldValue(receiver, "entries"); ok && entries.Kind == ValueList {
		return entries
	}
	return typedList("List<compression.ZipEntry>")
}

func compressionZipEntryFromAddArgs(args []Value) (Value, error) {
	if len(args) == 1 && args[0].Kind == ValueObject && strings.EqualFold(args[0].Type, "compression.ZipEntry") {
		return args[0], nil
	}
	if len(args) == 2 && args[0].Kind == ValueString && args[1].Kind == ValueObject && strings.EqualFold(args[1].Type, "Blob") {
		return newCompressionZipEntry(args[0].Text, String(""), args[1], compressionEnumValue("compression.Method", "DEFLATED")), nil
	}
	if len(args) == 5 && args[0].Kind == ValueString && args[1].Kind == ValueString && args[4].Kind == ValueObject && strings.EqualFold(args[4].Type, "Blob") {
		method := args[3]
		if !strings.EqualFold(method.Type, "compression.Method") {
			method = compressionEnumValue("compression.Method", "DEFLATED")
		}
		return newCompressionZipEntry(args[0].Text, args[1], args[4], method), nil
	}
	return Null, fmt.Errorf("compression.ZipWriter.addEntry expects entry or name/data arguments")
}

func newCompressionZipEntry(name string, comment Value, content Value, method Value) Value {
	entry := Object("compression.ZipEntry")
	entry.Fields["name"] = String(name)
	entry.Fields["comment"] = comment
	entry.Fields["content"] = content
	entry.Fields["method"] = method
	return entry
}

func compressionZipFindEntry(entries Value, name string) Value {
	for _, entry := range entries.List {
		if strings.EqualFold(compressionZipEntryName(entry), name) {
			return entry
		}
	}
	return Null
}

func compressionZipEntryName(entry Value) string {
	if _, value, ok := objectFieldValue(entry, "name"); ok && value.Kind == ValueString {
		return value.Text
	}
	return ""
}

func compressionZipEntryContent(entry Value) Value {
	if _, value, ok := objectFieldValue(entry, "content"); ok && value.Kind == ValueObject && strings.EqualFold(value.Type, "Blob") {
		return value
	}
	return platformScalar("Blob", "")
}

func writeCompressionZipArchive(entries []Value) (string, error) {
	var buf bytes.Buffer
	writer := zip.NewWriter(&buf)
	for _, entry := range entries {
		name := compressionZipEntryName(entry)
		if name == "" {
			continue
		}
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		if _, value, ok := objectFieldValue(entry, "comment"); ok && value.Kind == ValueString {
			header.Comment = value.Text
		}
		file, err := writer.CreateHeader(header)
		if err != nil {
			_ = writer.Close()
			return "", err
		}
		if _, err := file.Write([]byte(blobText(compressionZipEntryContent(entry)))); err != nil {
			_ = writer.Close()
			return "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func readCompressionZipEntries(data string) (Value, error) {
	reader, err := zip.NewReader(bytes.NewReader([]byte(data)), int64(len([]byte(data))))
	if err != nil {
		return Null, fmt.Errorf("compression.ZipReader invalid archive: %w", err)
	}
	entries := typedList("List<compression.ZipEntry>")
	for _, file := range reader.File {
		handle, err := file.Open()
		if err != nil {
			return Null, err
		}
		content, err := io.ReadAll(handle)
		_ = handle.Close()
		if err != nil {
			return Null, err
		}
		entry := newCompressionZipEntry(file.Name, String(file.Comment), platformScalar("Blob", string(content)), compressionEnumValue("compression.Method", "DEFLATED"))
		entries.List = append(entries.List, entry)
	}
	return entries, nil
}

func blobText(value Value) string {
	if value.Kind != ValueObject || !strings.EqualFold(value.Type, "Blob") {
		return ""
	}
	if _, raw, ok := objectFieldValue(value, "value"); ok {
		return raw.String()
	}
	return ""
}

func compressionEnumValue(typeName, name string) Value {
	return Value{Kind: ValueObject, Type: typeName, Text: name}
}

func callWaveQueryBuilderStaticDefault(method string, args []Value) (Value, bool) {
	switch strings.ToLower(method) {
	case "load":
		if len(args) != 2 {
			return Null, false
		}
		node := newWaveQueryNode("load")
		node.Fields["datasetId"] = args[0]
		node.Fields["datasetVersionId"] = args[1]
		return node, true
	case "loadbydevelopername":
		if len(args) != 1 {
			return Null, false
		}
		node := newWaveQueryNode("load")
		node.Fields["developerName"] = args[0]
		return node, true
	case "union":
		if len(args) != 1 {
			return Null, false
		}
		node := newWaveQueryNode("union")
		node.Fields["nodes"] = args[0]
		return node, true
	case "cogroup":
		if len(args) != 2 {
			return Null, false
		}
		node := newWaveQueryNode("cogroup")
		node.Fields["nodes"] = args[0]
		node.Fields["groups"] = args[1]
		return node, true
	case "count":
		if len(args) != 0 {
			return Null, false
		}
		projection := newWaveProjectionNode("count")
		return projection, true
	case "get":
		if len(args) != 1 {
			return Null, false
		}
		projection := newWaveProjectionNode("get")
		projection.Fields["projection"] = args[0]
		return projection, true
	default:
		return Null, false
	}
}

func callWaveQueryMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch receiver.Type {
	case "wave.QueryNode":
		return callWaveQueryNodeMember(receiver, method, args)
	case "wave.ProjectionNode":
		return callWaveProjectionNodeMember(receiver, method, args)
	default:
		return Null, receiver, false, false, nil
	}
}

func callWaveQueryNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	switch strings.ToLower(method) {
	case "build":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.build expects String streamName")
		}
		return String(waveQueryNodeBuild(receiver, args[0].Text)), receiver, false, true, nil
	case "execute":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.execute expects String streamName")
		}
		out := Object("ConnectApi.LiteralJson")
		out.Fields["json"] = typedList("List<Object>")
		return out, receiver, false, true, nil
	case "cap", "filter", "foreach", "group", "order":
		if strings.EqualFold(method, "group") && len(args) > 1 {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.group expects 0 or 1 argument")
		}
		if !strings.EqualFold(method, "group") && len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("wave.QueryNode.%s expects 1 argument", method)
		}
		waveAppendStep(&receiver, method, args)
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callWaveProjectionNodeMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	name := strings.ToLower(method)
	switch name {
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("wave.ProjectionNode.build expects 0 arguments")
		}
		return String(waveProjectionNodeBuild(receiver)), receiver, false, true, nil
	case "alias":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("wave.ProjectionNode.alias expects String")
		}
		receiver.Fields["alias"] = args[0]
		return receiver, receiver, true, true, nil
	case "avg", "count", "max", "min", "sum", "unique":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("wave.ProjectionNode.%s expects 0 arguments", method)
		}
		receiver.Fields["aggregate"] = String(name)
		return receiver, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func newWaveQueryNode(kind string) Value {
	node := Object("wave.QueryNode")
	node.Fields["kind"] = String(kind)
	node.Fields["steps"] = typedList("List<String>")
	return node
}

func newWaveProjectionNode(kind string) Value {
	projection := Object("wave.ProjectionNode")
	projection.Fields["kind"] = String(kind)
	return projection
}

func waveAppendStep(receiver *Value, method string, args []Value) {
	steps := typedList("List<String>")
	if _, existing, ok := objectFieldValue(*receiver, "steps"); ok && existing.Kind == ValueList {
		steps = existing
	}
	step := strings.ToLower(method)
	if len(args) > 0 {
		step += "(" + args[0].String() + ")"
	} else {
		step += "()"
	}
	steps.List = append(steps.List, String(step))
	receiver.Fields["steps"] = steps
}

func waveQueryNodeBuild(node Value, streamName string) string {
	parts := []string{streamName}
	if _, kind, ok := objectFieldValue(node, "kind"); ok && kind.Kind == ValueString && kind.Text != "" {
		parts = append(parts, kind.Text)
	}
	if _, value, ok := objectFieldValue(node, "developerName"); ok && value.Kind == ValueString {
		parts = append(parts, value.Text)
	} else if _, value, ok := objectFieldValue(node, "datasetId"); ok && value.Kind == ValueString {
		parts = append(parts, value.Text)
	}
	if _, steps, ok := objectFieldValue(node, "steps"); ok && steps.Kind == ValueList {
		for _, step := range steps.List {
			if step.Kind == ValueString {
				parts = append(parts, step.Text)
			}
		}
	}
	return strings.Join(parts, " | ")
}

func waveProjectionNodeBuild(projection Value) string {
	kind := "projection"
	if _, value, ok := objectFieldValue(projection, "kind"); ok && value.Kind == ValueString && value.Text != "" {
		kind = value.Text
	}
	if _, value, ok := objectFieldValue(projection, "projection"); ok && value.Kind == ValueString && value.Text != "" {
		kind = value.Text
	}
	if _, aggregate, ok := objectFieldValue(projection, "aggregate"); ok && aggregate.Kind == ValueString && aggregate.Text != "" {
		kind = aggregate.Text + "(" + kind + ")"
	}
	if _, alias, ok := objectFieldValue(projection, "alias"); ok && alias.Kind == ValueString && alias.Text != "" {
		kind += " as " + alias.Text
	}
	return kind
}

func callContextIndustriesContextMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "Context.IndustriesContext") {
		return Null, receiver, false, false, nil
	}
	name := strings.ToLower(method)
	if len(args) != 1 || args[0].Kind != ValueMap {
		return Null, receiver, false, true, fmt.Errorf("Context.IndustriesContext.%s expects Map<String,Object>", method)
	}
	switch name {
	case "deletecontext", "evictcontextdefinition":
		return Null, receiver, false, true, nil
	case "addrecordstocontext", "buildcontext", "filteringcontext", "getcontext",
		"getcontexttranslation", "leanerquerytags", "persistcontext",
		"querycontextrecordsandchildren", "queryrecordstatus", "querytags",
		"updatecontextattributes":
		return args[0], receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callOrgInstrumentationOperationMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "OrgInstrumentationOperation") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "createnewspan":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.createNewSpan expects 0 arguments")
		}
		return Object("TracerSpan"), receiver, false, true, nil
	case "start":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.start expects publish type")
		}
		context := Object("OrgInstrumentationContext")
		context.Fields["publishType"] = args[0]
		context.Fields["started"] = Bool(true)
		context.Fields["duration"] = Int(0)
		return context, receiver, false, true, nil
	case "end":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.end expects context")
		}
		return Null, receiver, false, true, nil
	case "endwithstatus":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationOperation.endWithStatus expects context and status code")
		}
		return Null, receiver, false, true, nil
	case "publishcustomhistogramvalues", "publishcustomincrementalvalue", "publishcustompercentileset",
		"publishincrementalvalue", "publishpercentileset", "publishrequestcountandduration":
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callOrgInstrumentationContextMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "OrgInstrumentationContext") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "starttime":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationContext.startTime expects 0 arguments")
		}
		receiver.Fields["started"] = Bool(true)
		receiver.Fields["duration"] = Int(0)
		return Null, receiver, true, true, nil
	case "end":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationContext.end expects 0 arguments")
		}
		receiver.Fields["ended"] = Bool(true)
		return Null, receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callOrgInstrumentationServiceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !strings.EqualFold(receiver.Type, "OrgInstrumentationService") {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "propagatecontext":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
			return Null, receiver, false, true, fmt.Errorf("OrgInstrumentationService.propagateContext expects HttpRequest")
		}
		httpSetHeader(args[0], "x-glade-instrumentation-context", String("local"))
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callUserProvisioningBatchableMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if !userProvisioningBatchableType(receiver.Type) {
		return Null, receiver, false, false, nil
	}
	switch strings.ToLower(method) {
	case "start":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.start expects BatchableContext", receiver.Type)
		}
		locator := Object("Database.QueryLocator")
		locator.Fields["Records"] = typedList("List<SObject>")
		locator.Fields["Query"] = String("")
		return locator, receiver, false, true, nil
	case "execute":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("%s.execute expects BatchableContext and scope", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "finish":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.finish expects BatchableContext", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "flowinputpreprocessing":
		if len(args) != 1 || args[0].Kind != ValueMap {
			return Null, receiver, false, true, fmt.Errorf("%s.flowInputPreprocessing expects Map<String,Object>", receiver.Type)
		}
		return args[0], receiver, false, true, nil
	case "flowpostprocessing":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("%s.flowPostProcessing expects handler output and SObject", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "postbatchprocessing":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.postBatchProcessing expects 0 arguments", receiver.Type)
		}
		return Null, receiver, false, true, nil
	case "geteventprefix", "getflowname", "getflownamespace":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		return String(""), receiver, false, true, nil
	case "getperbatchupl":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getPerBatchUPL expects 0 arguments", receiver.Type)
		}
		return typedList("List<SObject>"), receiver, false, true, nil
	case "getperbatchupr":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getPerBatchUPR expects 0 arguments", receiver.Type)
		}
		return typedList("List<UserProvisioningRequest>"), receiver, false, true, nil
	case "getuprtonewuplmap":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getUprToNewUplMap expects 0 arguments", receiver.Type)
		}
		return typedMap("Map<Id,SObject>"), receiver, false, true, nil
	case "hasflow", "hasfloworapex":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
		}
		return Bool(false), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func userProvisioningBatchableType(typeName string) bool {
	switch typeName {
	case "UserProvisioning.ProvisioningBatchable", "UserProvisioning.CollectingBatchable",
		"UserProvisioning.PluginBatchable", "UserProvisioning.LinkingBatchable",
		"UserProvisioning.CommittingBatchable", "UserProvisioning.DeletingBatchable",
		"UserProvisioning.RequestingBatchable", "UserProvisioning.UPASCleaningBatchable":
		return true
	default:
		return false
	}
}

func callPlatformCallbackDefaultMember(receiver Value, method string, args []Value) (Value, bool, error) {
	typeName := receiver.Type
	lowerMethod := strings.ToLower(method)
	switch typeName {
	case "workflow.Action":
		if lowerMethod != "invoke" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("workflow.Action.invoke expects Context")
		}
		return typedList("List<workflow.ActionDml>"), true, nil
	case "workflow.ActionDml":
		if lowerMethod != "invoke" {
			return Null, false, nil
		}
		if len(args) != 0 {
			return Null, true, fmt.Errorf("workflow.ActionDml.invoke expects 0 arguments")
		}
		return Null, true, nil
	case "eventbus.EventPublishFailureCallback":
		if lowerMethod != "onfailure" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("eventbus.EventPublishFailureCallback.onFailure expects FailureResult")
		}
		return Null, true, nil
	case "eventbus.EventPublishSuccessCallback":
		if lowerMethod != "onsuccess" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("eventbus.EventPublishSuccessCallback.onSuccess expects SuccessResult")
		}
		return Null, true, nil
	case "TxnSecurity.EventCondition", "TxnSecurity.PolicyCondition":
		if lowerMethod != "evaluate" {
			return Null, false, nil
		}
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.evaluate expects event input", typeName)
		}
		return Bool(false), true, nil
	case "Social.DefaultInboundSocialPostHandler", "Social.InboundSocialPostHandlerImpl":
		return socialInboundHandlerDefault(typeName, lowerMethod, args)
	case "Social.InboundSocialPostHandler":
		if lowerMethod != "handleinboundsocialpost" {
			return Null, false, nil
		}
		if len(args) != 3 {
			return Null, true, fmt.Errorf("Social.InboundSocialPostHandler.handleInboundSocialPost expects post, persona, and rawData")
		}
		return Object("Social.InboundSocialPostResult"), true, nil
	default:
		return Null, false, nil
	}
}

func socialInboundHandlerDefault(typeName, method string, args []Value) (Value, bool, error) {
	switch method {
	case "createpersonaparent":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.createPersonaParent expects SocialPersona", typeName)
		}
		return Null, true, nil
	case "getcasesubject", "getpersonafirstname", "getpersonalastname":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("%s.%s expects one argument", typeName, method)
		}
		return String(""), true, nil
	case "getdefaultaccountid":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getDefaultAccountId expects 0 arguments", typeName)
		}
		return String(""), true, nil
	case "getmaxnumberofdaysclosedtoreopencase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getMaxNumberOfDaysClosedToReopenCase expects 0 arguments", typeName)
		}
		return Int(0), true, nil
	case "getposttagsthatcreatecase":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getPostTagsThatCreateCase expects 0 arguments", typeName)
		}
		return typedSet("Set<String>"), true, nil
	case "getusingcaseassignmentrule":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("%s.getUsingCaseAssignmentRule expects 0 arguments", typeName)
		}
		return Bool(false), true, nil
	case "handleinboundsocialpost":
		if len(args) != 3 {
			return Null, true, fmt.Errorf("%s.handleInboundSocialPost expects post, persona, and rawData", typeName)
		}
		return Object("Social.InboundSocialPostResult"), true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) callStandardControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	record, ok := receiver.Fields["record"]
	if !ok || record.Kind != ValueObject {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController has no SObject record")
	}
	switch method {
	case "getId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getId expects 0 arguments")
		}
		if _, id, ok := objectFieldValue(record, "Id"); ok {
			return id, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.getRecord expects 0 arguments")
		}
		return record, receiver, false, true, nil
	case "save", "quickSave":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		op := "insert"
		if _, id, ok := objectFieldValue(record, "Id"); ok {
			if idText, ok := idValueText(id); ok && idText != "" {
				op = "update"
			}
		}
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": op})
		results, err := vm.applyDML(op, record, true, "", dml.Options{}, result)
		if err != nil {
			appendStandardControllerErrorTrace(result, method, record, op, err)
			return Null, receiver, false, true, err
		}
		if len(results) > 0 && results[0].ID != "" {
			record.Fields["Id"] = String(string(results[0].ID))
			receiver.Fields["record"] = record
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  op,
			"pageReference": tracePageReference(page),
		})
		return page, receiver, true, true, nil
	case "delete":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.delete expects 0 arguments")
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "start", method, record, map[string]any{"dmlOperation": "delete"})
		if _, err := vm.applyDML("delete", record, true, "", dml.Options{}, result); err != nil {
			appendStandardControllerErrorTrace(result, method, record, "delete", err)
			return Null, receiver, false, true, err
		}
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"dmlOperation":  "delete",
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "view", "edit", "cancel", "reset":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.%s expects 0 arguments", method)
		}
		page := standardControllerPage(record)
		appendStandardControllerActionTrace(result, "start", method, record, nil)
		appendStandardControllerActionTrace(result, "complete", method, record, map[string]any{
			"pageReference": tracePageReference(page),
		})
		return page, receiver, false, true, nil
	case "addFields":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardController.addFields expects List")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexStackMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "empty", "peek", "pop", "push")
	values := receiver.Fields["values"]
	if values.Kind != ValueList {
		values = List()
	}
	switch method {
	case "empty":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.empty expects 0 arguments")
		}
		return Bool(len(values.List) == 0), receiver, false, true, nil
	case "peek":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.peek expects 0 arguments")
		}
		if len(values.List) == 0 {
			return Null, receiver, false, true, newExceptionError("Apex.EmptyStackException", "Stack is empty")
		}
		return values.List[len(values.List)-1], receiver, false, true, nil
	case "pop":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.pop expects 0 arguments")
		}
		if len(values.List) == 0 {
			return Null, receiver, false, true, newExceptionError("Apex.EmptyStackException", "Stack is empty")
		}
		value := values.List[len(values.List)-1]
		values.List = values.List[:len(values.List)-1]
		receiver.Fields["values"] = values
		return value, receiver, true, true, nil
	case "push":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Apex.Stack.push expects 1 argument")
		}
		values.List = append(values.List, args[0])
		receiver.Fields["values"] = values
		return args[0], receiver, true, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesActionMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getExpression", "invoke")
	switch method {
	case "getExpression":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.Action.getExpression expects 0 arguments")
		}
		if value, ok := receiver.Fields["expression"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "invoke":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.Action.invoke expects 0 arguments")
		}
		expression := ""
		if value, ok := receiver.Fields["expression"]; ok && value.Kind == ValueString {
			expression = strings.TrimSpace(value.Text)
		}
		if expression == "" || strings.EqualFold(expression, "null") || strings.EqualFold(expression, "{!null}") || strings.EqualFold(expression, "{!}") {
			return Null, receiver, false, true, nil
		}
		return Null, receiver, false, true, unsupportedCallError("ApexPages.Action.invoke requires bound Visualforce controller lifecycle")
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callFormulaBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "withFormula", "withReturnType", "withType", "withGlobalVariables", "treatNumericNullAsZero", "parseAsTemplate", "build")
	switch method {
	case "withFormula":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withFormula expects formula String")
		}
		receiver.Fields["formula"] = args[0]
		return receiver, receiver, true, true, nil
	case "withReturnType":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withReturnType expects return type")
		}
		receiver.Fields["returnType"] = args[0]
		return receiver, receiver, true, true, nil
	case "withType":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withType expects context type")
		}
		receiver.Fields["contextType"] = args[0]
		return receiver, receiver, true, true, nil
	case "withGlobalVariables":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.withGlobalVariables expects FormulaGlobal list")
		}
		receiver.Fields["globalVariables"] = args[0]
		return receiver, receiver, true, true, nil
	case "treatNumericNullAsZero":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.treatNumericNullAsZero expects Boolean")
		}
		receiver.Fields["treatNumericNullAsZero"] = args[0]
		return receiver, receiver, true, true, nil
	case "parseAsTemplate":
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.parseAsTemplate expects Boolean")
		}
		receiver.Fields["templateMode"] = args[0]
		return receiver, receiver, true, true, nil
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaBuilder.build expects 0 arguments")
		}
		if value, ok := receiver.Fields["templateMode"]; ok && value.Kind == ValueBool && value.Bool {
			return Null, receiver, false, true, unsupportedCallError("formulaeval.FormulaBuilder.parseAsTemplate template evaluation")
		}
		instance := Object("formulaeval.FormulaInstance")
		for field, value := range receiver.Fields {
			instance.Fields[field] = value
		}
		return instance, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) callFormulaInstanceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "evaluate", "getReferencedFields")
	switch method {
	case "evaluate":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaInstance.evaluate expects context object")
		}
		formula, _ := formulaInstanceText(receiver)
		if formula == "" {
			return Null, receiver, false, true, newExceptionError("FormulaEvaluationException", "formula text is required")
		}
		value, ok := vm.evaluateFormulaInstanceValue(receiver, args[0], formula)
		if !ok {
			return Null, receiver, false, true, unsupportedCallError("formulaeval.FormulaInstance.evaluate unsupported local formula expression")
		}
		return value, receiver, false, true, nil
	case "getReferencedFields":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("formulaeval.FormulaInstance.getReferencedFields expects 0 arguments")
		}
		formula, _ := formulaInstanceText(receiver)
		out := Set()
		out.Type = "Set<String>"
		for _, field := range formulaReferencedFields(formula) {
			out.Set = append(out.Set, String(field))
		}
		return out, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callFormulaRecalcResultMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "isSuccess", "getSObject", "getErrors")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "isSuccess":
		if value, ok := receiver.Fields["success"]; ok {
			return value, receiver, false, true, nil
		}
		return Bool(false), receiver, false, true, nil
	case "getSObject":
		if value, ok := receiver.Fields["sobject"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "getErrors":
		if value, ok := receiver.Fields["errors"]; ok {
			return value, receiver, false, true, nil
		}
		return typedList("List<FormulaRecalcFieldError>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callFormulaRecalcFieldErrorMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getFieldName", "getFieldError")
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	switch method {
	case "getFieldName":
		if value, ok := receiver.Fields["fieldName"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	case "getFieldError":
		if value, ok := receiver.Fields["fieldError"]; ok {
			return value, receiver, false, true, nil
		}
		return String(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func (vm *VM) recalculateFormulaList(args []Value) (Value, error) {
	if len(args) != 1 || args[0].Kind != ValueList {
		return Null, fmt.Errorf("Formula.recalculateFormulas expects SObject list")
	}
	out := typedList("List<FormulaRecalcResult>")
	for _, item := range args[0].List {
		if item.Kind != ValueObject || !vm.isSObjectLikeType(item.Type) {
			return Null, fmt.Errorf("Formula.recalculateFormulas expects SObject list")
		}
		out.List = append(out.List, vm.recalculateFormulaSObject(item))
	}
	return out, nil
}

func (vm *VM) recalculateFormulaSObject(item Value) Value {
	result := Object("FormulaRecalcResult")
	result.Fields["sobject"] = item
	result.Fields["success"] = Bool(true)
	errors := typedList("List<FormulaRecalcFieldError>")
	if vm.Org == nil {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "org metadata is required"))
		result.Fields["errors"] = errors
		return result
	}
	objectName, ok := vm.resolveObjectName(item.Type)
	if !ok {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "object metadata is required"))
		result.Fields["errors"] = errors
		return result
	}
	definition := vm.Org.Objects[objectName].Definition
	record, ok := vm.formulaRecordFromSObject(item)
	if !ok {
		result.Fields["success"] = Bool(false)
		errors.List = append(errors.List, formulaRecalcFieldError("", "SObject value cannot be converted to a formula context"))
		result.Fields["errors"] = errors
		return result
	}
	for fieldName, field := range definition.Fields {
		if field.Type != storage.FieldCalculated || strings.TrimSpace(field.Formula) == "" {
			continue
		}
		value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrg(field.Formula, field, vm.Org, definition, record)
		if !ok {
			result.Fields["success"] = Bool(false)
			errors.List = append(errors.List, formulaRecalcFieldError(fieldName, "unsupported local formula expression"))
			continue
		}
		if explicitNull {
			item.Fields[fieldName] = Null
			continue
		}
		item.Fields[fieldName] = vmValueFromStorage(value)
	}
	result.Fields["sobject"] = item
	result.Fields["errors"] = errors
	return result
}

func formulaRecalcFieldError(fieldName, message string) Value {
	err := Object("FormulaRecalcFieldError")
	err.Fields["fieldName"] = String(fieldName)
	err.Fields["fieldError"] = String(message)
	return err
}

func (vm *VM) evaluateFormulaInstanceValue(instance Value, context Value, formula string) (Value, bool) {
	if context.Kind != ValueObject || !vm.isSObjectLikeType(context.Type) || vm.Org == nil {
		return Null, false
	}
	objectName, ok := vm.resolveObjectName(context.Type)
	if !ok {
		return Null, false
	}
	definition := vm.Org.Objects[objectName].Definition
	record, ok := vm.formulaRecordFromSObject(context)
	if !ok {
		return Null, false
	}
	field := storage.Field{APIName: "__formula", Type: formulaReturnFieldType(instance), Formula: formula}
	value, explicitNull, ok := dml.EvaluateRecordFormulaValueInOrg(formula, field, vm.Org, definition, record)
	if !ok {
		return Null, false
	}
	if explicitNull {
		return Null, true
	}
	return vmValueFromStorage(value), true
}

func formulaInstanceText(instance Value) (string, bool) {
	if value, ok := instance.Fields["formula"]; ok && value.Kind == ValueString {
		return value.Text, true
	}
	if value, ok := instance.Fields["formulaText"]; ok && value.Kind == ValueString {
		return value.Text, true
	}
	return "", false
}

func formulaReturnFieldType(instance Value) storage.FieldType {
	if value, ok := instance.Fields["returnType"]; ok {
		name := strings.ToUpper(strings.TrimSpace(value.Text))
		if name == "" && value.Kind == ValueObject {
			name = strings.ToUpper(strings.TrimSpace(value.Text))
		}
		switch name {
		case "BOOLEAN":
			return storage.FieldBoolean
		case "INTEGER", "LONG":
			return storage.FieldInteger
		case "DECIMAL", "DOUBLE":
			return storage.FieldDecimal
		case "DATE":
			return storage.FieldDate
		case "DATETIME":
			return storage.FieldDateTime
		case "ID":
			return storage.FieldID
		}
	}
	return storage.FieldString
}

func formulaReferencedFields(formula string) []string {
	matches := regexp.MustCompile(`\b[A-Za-z_][A-Za-z0-9_]*(?:__c|__r)?(?:\.[A-Za-z_][A-Za-z0-9_]*(?:__c|__r)?)*\b`).FindAllString(formula, -1)
	seen := map[string]bool{}
	out := make([]string, 0, len(matches))
	for _, match := range matches {
		upper := strings.ToUpper(match)
		switch upper {
		case "AND", "OR", "NOT", "IF", "CASE", "ISBLANK", "ISNULL", "NULL", "TRUE", "FALSE", "TODAY", "NOW", "DATE", "DATETIMEVALUE", "TEXT", "VALUE", "LOWER", "UPPER", "FLOOR", "MOD", "REGEX", "CONTAINS":
			continue
		}
		if seen[strings.ToLower(match)] {
			continue
		}
		seen[strings.ToLower(match)] = true
		out = append(out, match)
	}
	sort.Strings(out)
	return out
}

func callContinuationMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "addHttpRequest", "getRequests")
	switch method {
	case "addHttpRequest":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "HttpRequest") {
			return Null, receiver, false, true, fmt.Errorf("Continuation.addHttpRequest expects HttpRequest")
		}
		requests, ok := receiver.Fields["requests"]
		if !ok || requests.Kind != ValueMap {
			requests = typedMap("Map<String,HttpRequest>")
		}
		label := fmt.Sprintf("request-%d", len(requests.Map)+1)
		key := mapKey(String(label))
		requests.Map[key] = args[0]
		requests.MapKeys[key] = String(label)
		receiver.Fields["requests"] = requests
		return String(label), receiver, true, true, nil
	case "getRequests":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Continuation.getRequests expects 0 arguments")
		}
		if requests, ok := receiver.Fields["requests"]; ok && requests.Kind == ValueMap {
			return requests, receiver, false, true, nil
		}
		return typedMap("Map<String,HttpRequest>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesComponentMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getComponentById")
	if method != "getComponentById" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 1 || args[0].Kind != ValueString {
		return Null, receiver, false, true, fmt.Errorf("%s.getComponentById expects id String", receiver.Type)
	}
	return Null, receiver, false, true, nil
}

func callApexPagesIdeaStandardControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getCommentList")
	if method != "getCommentList" {
		return Null, receiver, false, false, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardController.getCommentList expects 0 arguments")
	}
	return typedList("List<IdeaComment>"), receiver, false, true, nil
}

func callApexPagesIdeaStandardSetControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getIdeaList", "getListViewOptions")
	switch method {
	case "getIdeaList":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardSetController.getIdeaList expects 0 arguments")
		}
		return typedList("List<Idea>"), receiver, false, true, nil
	case "getListViewOptions":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.IdeaStandardSetController.getListViewOptions expects 0 arguments")
		}
		return typedList("List<SelectOption>"), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func callApexPagesKnowledgeArticleVersionStandardControllerMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "getSourceId", "selectDataCategory")
	switch method {
	case "getSourceId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.getSourceId expects 0 arguments")
		}
		return Null, receiver, false, true, nil
	case "selectDataCategory":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.KnowledgeArticleVersionStandardController.selectDataCategory expects group and category Strings")
		}
		return Null, receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

func appendStandardControllerActionTrace(result *Result, phase, method string, record Value, extra map[string]any) {
	args := standardControllerTraceArgs(method, record)
	for key, value := range extra {
		args[key] = value
	}
	appendTrace(result, "apex.visualforce.standard_controller.action."+phase, "apex.visualforce.standard_controller", args)
}

func appendStandardControllerErrorTrace(result *Result, method string, record Value, dmlOperation string, err error) {
	actionErr := uiInvocationError(err)
	appendStandardControllerActionTrace(result, "error", method, record, map[string]any{
		"dmlOperation": dmlOperation,
		"error":        actionErr.Message,
		"errorType":    actionErr.Type,
	})
}

func standardControllerTraceArgs(method string, record Value) map[string]any {
	args := map[string]any{"method": method}
	if record.Kind == ValueObject {
		args["objectType"] = record.Type
		if _, id, ok := objectFieldValue(record, "Id"); ok && id.Kind == ValueString && id.Text != "" {
			args["recordId"] = id.Text
		}
	}
	return args
}

func (vm *VM) callStandardSetControllerMember(receiver Value, method string, args []Value, result *Result) (Value, Value, bool, bool, error) {
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	records := receiver.Fields["records"]
	switch method {
	case "getRecords":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecords expects 0 arguments")
		}
		return standardSetCurrentPage(receiver, records), receiver, false, true, nil
	case "getRecord":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getRecord expects 0 arguments")
		}
		page := standardSetCurrentPage(receiver, records)
		if len(page.List) == 0 {
			return Null, receiver, false, true, nil
		}
		return page.List[0], receiver, false, true, nil
	case "getResultSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getResultSize expects 0 arguments")
		}
		if records.Kind != ValueList {
			return Int(0), receiver, false, true, nil
		}
		return Int(int64(len(records.List))), receiver, false, true, nil
	case "getSelected":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getSelected expects 0 arguments")
		}
		return receiver.Fields["selected"], receiver, false, true, nil
	case "setSelected":
		if len(args) != 1 || args[0].Kind != ValueList {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setSelected expects List")
		}
		receiver.Fields["selected"] = args[0]
		return Null, receiver, true, true, nil
	case "getPageSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageSize expects 0 arguments")
		}
		return receiver.Fields["pageSize"], receiver, false, true, nil
	case "setPageSize":
		if len(args) != 1 || args[0].Kind != ValueInt || args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageSize expects positive Integer")
		}
		receiver.Fields["pageSize"] = args[0]
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "getPageNumber":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getPageNumber expects 0 arguments")
		}
		return receiver.Fields["pageNumber"], receiver, false, true, nil
	case "setPageNumber":
		if len(args) != 1 || args[0].Kind != ValueInt || args[0].Int <= 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setPageNumber expects positive Integer")
		}
		page := int(args[0].Int)
		pageCount := standardSetPageCount(receiver, records)
		if page > pageCount {
			page = pageCount
		}
		receiver.Fields["pageNumber"] = Int(int64(page))
		return Null, receiver, true, true, nil
	case "getListViewOptions":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getListViewOptions expects 0 arguments")
		}
		return typedList("List<SelectOption>"), receiver, false, true, nil
	case "setFilterId":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.setFilterId expects String")
		}
		receiver.Fields["filterId"] = args[0]
		return Null, receiver, true, true, nil
	case "getFilterId":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("ApexPages.StandardSetController.getFilterId expects 0 arguments")
		}
		if value, ok := receiver.Fields["filterId"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	case "first":
		receiver.Fields["pageNumber"] = Int(1)
		return Null, receiver, true, true, nil
	case "last":
		receiver.Fields["pageNumber"] = Int(int64(standardSetPageCount(receiver, records)))
		return Null, receiver, true, true, nil
	case "next":
		page := int(receiver.Fields["pageNumber"].Int)
		if page < standardSetPageCount(receiver, records) {
			receiver.Fields["pageNumber"] = Int(int64(page + 1))
		}
		return Null, receiver, true, true, nil
	case "previous":
		page := int(receiver.Fields["pageNumber"].Int)
		if page > 1 {
			receiver.Fields["pageNumber"] = Int(int64(page - 1))
		}
		return Null, receiver, true, true, nil
	case "getHasNext":
		return Bool(int(receiver.Fields["pageNumber"].Int) < standardSetPageCount(receiver, records)), receiver, false, true, nil
	case "getHasPrevious":
		return Bool(receiver.Fields["pageNumber"].Int > 1), receiver, false, true, nil
	case "getCompleteResult":
		return Bool(true), receiver, false, true, nil
	case "save":
		return vm.standardSetDML(receiver, "update", result)
	case "delete":
		return vm.standardSetDML(receiver, "delete", result)
	case "cancel":
		return newPageReference(""), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}
