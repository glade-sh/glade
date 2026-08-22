package vm

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/trace"
)

func (vm *VM) callCommerceInventoryServiceMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	if strings.EqualFold(receiver.Type, "commerce_inventory.InventoryLevelsResponse") {
		if !strings.EqualFold(method, "getItemsInventoryLevels") || len(args) != 0 {
			return Null, receiver, false, false, nil
		}
		return receiver.Fields["itemsInventoryLevels"], receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "commerce_inventory.InventoryReservation") {
		switch strings.ToLower(method) {
		case "getdurationinseconds":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
			}
			return receiver.Fields["durationInSeconds"], receiver, false, true, nil
		case "getitems":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
			}
			return receiver.Fields["items"], receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "commerce_inventory.InventoryCheckAvailability") && strings.EqualFold(method, "getInventoryCheckItemAvailability") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getInventoryCheckItemAvailability expects 0 arguments", receiver.Type)
		}
		return receiver.Fields["inventoryCheckItemAvailability"], receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "commerce_ordermanagement.ProductExpandResponse") && strings.EqualFold(method, "getSucceed") {
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("%s.getSucceed expects 0 arguments", receiver.Type)
		}
		if value, ok := receiver.Fields["succeed"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "commerce_ordermanagement.ProductExpandService") && strings.EqualFold(method, "returnReasons") {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("%s.returnReasons expects ProductExpandRequest", receiver.Type)
		}
		return Object("commerce_ordermanagement.ProductExpandResponse"), receiver, false, true, nil
	}
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
	raw, err := jsonMarshalNoEscape(values)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func (vm *VM) callPlatformObjectMember(receiver Value, method string, args []Value, result *Result) (value Value, updated Value, mutated bool, handled bool, err error) {
	defer func() {
		if mutated {
			vm.advanceAliasContainmentMutation()
		}
	}()
	method = canonicalPlatformObjectMemberName(receiver.Type, method)
	if strings.EqualFold(receiver.Type, "ApexPages.Message") {
		switch strings.ToLower(method) {
		case "equals":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.equals expects 1 argument")
			}
			return Bool(apexPagesMessagesEquivalent(receiver, args[0])), receiver, false, true, nil
		case "hashcode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.hashCode expects 0 arguments")
			}
			return Int(int64(apexPagesMessageHashCode(receiver))), receiver, false, true, nil
		case "tostring":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.toString expects 0 arguments")
			}
			summary := ""
			if value, ok := receiver.Fields["summary"]; ok {
				summary = value.String()
			}
			return String("ApexPages.Message[\"" + summary + "\"]"), receiver, false, true, nil
		}
	}
	if value, handled, err := vm.callTypeObjectMember(receiver, method, args, result); handled || err != nil {
		return value, receiver, false, true, err
	}
	if strings.EqualFold(receiver.Type, "Invocable.Action") {
		if value, handled := vm.callInvocableActionMember(receiver, method, args); handled {
			return value, receiver, true, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "Invocable.Action.Result") {
		if value, handled := callInvocableActionResultMember(receiver, method, args); handled {
			return value, receiver, false, true, nil
		}
	}
	if value, updated, mutated, handled, err := vm.callRegisteredPlatformObjectMemberPhase("early", receiver, method, args, result); handled || err != nil {
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
	if strings.EqualFold(receiver.Type, "Datacloud.FindDuplicatesResult") {
		switch strings.ToLower(method) {
		case "issuccess":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datacloud.FindDuplicatesResult.isSuccess expects 0 arguments")
			}
			if value, ok := receiver.Fields["success"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(true), receiver, false, true, nil
		case "getduplicateresults":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datacloud.FindDuplicatesResult.getDuplicateResults expects 0 arguments")
			}
			if value, ok := receiver.Fields["duplicateResults"]; ok {
				return value, receiver, false, true, nil
			}
			return typedList("List<Datacloud.DuplicateResult>"), receiver, false, true, nil
		case "geterrors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Datacloud.FindDuplicatesResult.getErrors expects 0 arguments")
			}
			if value, ok := receiver.Fields["errors"]; ok {
				return value, receiver, false, true, nil
			}
			return typedList("List<Database.Error>"), receiver, false, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "Support.EinsteinBots") && strings.EqualFold(method, "sendMessageToBot") {
		if len(args) != 3 {
			return Null, receiver, false, true, fmt.Errorf("Support.EinsteinBots.sendMessageToBot expects bot Id, bot version Id, and prompt")
		}
		return String(""), receiver, false, true, nil
	}
	if strings.EqualFold(receiver.Type, "Support.EmailTemplateSelector") && (strings.EqualFold(method, "getDefaultEmailTemplateId") || strings.EqualFold(method, "getDefaultTemplateId")) {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Support.EmailTemplateSelector.%s expects context Id", method)
		}
		return Null, receiver, false, true, nil
	}
	if value, updated, mutated, handled, err := vm.callRegisteredPlatformObjectMemberPhase("commerce", receiver, method, args, result); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if strings.HasPrefix(receiver.Type, "RichMessaging.") {
		switch receiver.Type {
		case "RichMessaging.AuthRequestHandler":
			if strings.EqualFold(method, "handleAuthRequest") {
				if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "RichMessaging.AuthRequestResponse") {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.AuthRequestHandler.handleAuthRequest expects AuthRequestResponse")
				}
				return vm.newRichMessagingAuthRequestResult(), receiver, false, true, nil
			}
		case "RichMessaging.ProcessCatalogOrderHandler":
			if strings.EqualFold(method, "processCatalogOrderRequest") {
				if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "RichMessaging.ProcessCatalogOrderRequest") {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.ProcessCatalogOrderHandler.processCatalogOrderRequest expects ProcessCatalogOrderRequest")
				}
				return newRichMessagingProcessCatalogOrderResult(), receiver, false, true, nil
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
				if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "RichMessaging.ProcessPaymentRequest") {
					return Null, receiver, false, true, fmt.Errorf("RichMessaging.ProcessPaymentHandler.processPaymentRequest expects ProcessPaymentRequest")
				}
				return newRichMessagingProcessPaymentResult(), receiver, false, true, nil
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
			if len(args) != 1 || args[0].Kind != ValueObject || !hasPrefixFold(args[0].Type, "nlppredictions.predictionrequestcontext") {
				return Null, receiver, false, true, fmt.Errorf("NLPPredictions.PredictionHandler.handlePredictionRequest expects PredictionRequestContext")
			}
			return Null, receiver, false, true, nil
		case "handlepredictionresponse":
			if len(args) != 1 || args[0].Kind != ValueObject || !hasPrefixFold(args[0].Type, "nlppredictions.predictionresponsecontext") {
				return Null, receiver, false, true, fmt.Errorf("NLPPredictions.PredictionHandler.handlePredictionResponse expects PredictionResponseContext")
			}
			return Null, receiver, false, true, nil
		}
	}
	if value, updated, mutated, handled, err := vm.callRegisteredPlatformObjectMemberPhase("user-provisioning", receiver, method, args, result); handled || err != nil {
		return value, updated, mutated, true, err
	}
	if vm.typeMatches(runtimeObjectType(receiver), "UserProvisioning.FlowProvisionBase", make(map[string]bool)) {
		switch strings.ToLower(method) {
		case "getflowname", "getflownamespace":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UserProvisioning.FlowProvisionBase.%s expects 0 arguments", method)
			}
			return Null, receiver, false, true, nil
		case "hasflow", "hasfloworapex":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("UserProvisioning.FlowProvisionBase.%s expects 0 arguments", method)
			}
			return Bool(false), receiver, false, true, nil
		}
	}
	if vm.typeMatches(runtimeObjectType(receiver), "UserProvisioning.UserProvisioningPlugin", make(map[string]bool)) {
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
			return String(shortTypeName(runtimeObjectType(receiver))), receiver, false, true, nil
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
	if value, updated, mutated, handled, err := vm.callRegisteredPlatformObjectMemberPhase("controller", receiver, method, args, result); handled || err != nil {
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
			if isCustomExceptionWithoutExplicitMessage(receiver) {
				return String("Script-thrown exception"), receiver, false, true, nil
			}
			if isBuiltinExceptionType(receiver.Type) {
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
			if !builtinExceptionTypeMatches(receiver.Type, "QueryException") && !builtinExceptionTypeMatches(receiver.Type, "InvalidParameterValueException") {
				return Null, receiver, false, true, newExceptionError("System.TypeException", "Procedure is only valid for System.QueryException")
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
			return Null, receiver, true, true, nil
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
				if stack.Kind == ValueString {
					if cause, ok := receiver.Fields["__cause"]; ok && cause.Kind == ValueObject {
						if causeStack, ok := cause.Fields["__stackTrace"]; ok && causeStack.Kind == ValueString && causeStack.Text != "" {
							return String(stack.Text + "\nCaused by\n" + causeStack.Text), receiver, false, true, nil
						}
					}
				}
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
	if value, updated, mutated, handled, err := callXmlStreamWriterMember(receiver, method, args); handled || err != nil {
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
		return vm.callDataWeaveScriptMember(receiver, method, args)
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
	if componentApexRuntimeType(receiver.Type) {
		if value, updated, mutated, handled, err := callApexPagesComponentMember(receiver, method, args); handled || err != nil {
			return value, updated, mutated, handled, err
		}
	}
	switch receiver.Type {
	case "eventbus.TriggerContext", "EventBus.TriggerContext":
		switch strings.ToLower(method) {
		case "getresumecheckpoint":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("%s.getResumeCheckpoint expects 0 arguments", receiver.Type)
			}
			if vm.eventBusTriggerContext == nil || !vm.eventBusTriggerContext.hasCheckpoint {
				return Null, receiver, false, true, nil
			}
			return String(vm.eventBusTriggerContext.checkpoint), receiver, false, true, nil
		case "setresumecheckpoint":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("%s.setResumeCheckpoint expects String", receiver.Type)
			}
			if args[0].Kind == ValueNull || args[0].Kind != ValueString || vm.eventBusTriggerContext == nil {
				return Null, receiver, false, true, newExceptionError("eventbus.InvalidReplayIdException", "The replay ID is invalid")
			}
			if _, ok := vm.eventBusTriggerContext.replayIDs[args[0].Text]; !ok {
				return Null, receiver, false, true, newExceptionError("eventbus.InvalidReplayIdException", "The replay ID is invalid")
			}
			vm.eventBusTriggerContext.checkpoint = args[0].Text
			vm.eventBusTriggerContext.hasCheckpoint = true
			return Null, receiver, true, true, nil
		}
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
		case "isUpgrade":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("InstallContext.isUpgrade expects 0 arguments")
			}
			if value, ok := receiver.Fields["PreviousVersion"]; ok && value.Kind != ValueNull {
				return Bool(true), receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
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
	case "UninstallContext":
		if method != "organizationId" {
			break
		}
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("UninstallContext.organizationId expects 0 arguments")
		}
		if value, ok := receiver.Fields["organizationId"]; ok {
			return value, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
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
			return vm.databaseCursorFetch(receiver, method, args, false)
		}
	case "Database.PaginationCursor":
		switch method {
		case "getNumRecords":
			return databaseCursorNumRecords(receiver, method, args)
		case "fetchPage":
			return vm.databaseCursorFetch(receiver, method, args, false)
		case "fetchDeleted":
			return vm.databaseCursorFetch(receiver, method, args, true)
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
	case "QueueableContext", "QueueableContextImpl", "System.QueueableContextImpl", "BatchableContext", "Database.BatchableContext", "Database.BatchableContextImpl":
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
			// Salesforce returns null for a live top-level BatchableContext with
			// no child batch. Keep the existing empty-string behavior for a
			// directly constructed passive context, whose JobId is unset.
			if receiver.Type == "BatchableContext" || receiver.Type == "Database.BatchableContext" || receiver.Type == "Database.BatchableContextImpl" {
				if _, live := receiver.Fields["JobId"]; live {
					return Null, receiver, false, true, nil
				}
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
	case "System.FinalizerContext", "FinalizerContext", "System.FinalizerContextImpl", "FinalizerContextImpl":
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
		case "getMaximumQueueableStackDepth":
			return Null, receiver, false, true, unsupportedCallError("AsyncOptions.getMaximumQueueableStackDepth")
		case "setMaximumQueueableStackDepth":
			if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueNull) {
				return Null, receiver, false, true, fmt.Errorf("AsyncOptions.setMaximumQueueableStackDepth expects Integer")
			}
			receiver.Fields["maximumQueueableStackDepth"] = args[0]
			return receiver, receiver, true, true, nil
		case "getMinimumQueueableDelayInMinutes":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("AsyncOptions.getMinimumQueueableDelayInMinutes expects 0 arguments")
			}
			if value, ok := receiver.Fields["minimumQueueableDelayInMinutes"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "setMinimumQueueableDelayInMinutes":
			if len(args) != 1 || (args[0].Kind != ValueInt && args[0].Kind != ValueNull) {
				return Null, receiver, false, true, fmt.Errorf("AsyncOptions.setMinimumQueueableDelayInMinutes expects Integer")
			}
			receiver.Fields["minimumQueueableDelayInMinutes"] = args[0]
			return receiver, receiver, true, true, nil
		case "getDuplicateSignature":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("AsyncOptions.getDuplicateSignature expects 0 arguments")
			}
			if value, ok := receiver.Fields["duplicateSignature"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "setDuplicateSignature":
			if len(args) != 1 || (args[0].Kind != ValueObject && args[0].Kind != ValueNull) {
				return Null, receiver, false, true, fmt.Errorf("AsyncOptions.setDuplicateSignature expects QueueableDuplicateSignature")
			}
			if args[0].Kind == ValueObject && !strings.EqualFold(args[0].Type, "QueueableDuplicateSignature") {
				return Null, receiver, false, true, fmt.Errorf("AsyncOptions.setDuplicateSignature expects QueueableDuplicateSignature")
			}
			receiver.Fields["duplicateSignature"] = args[0]
			return receiver, receiver, true, true, nil
		}
	case "JSONGenerator":
		return vm.callJSONGeneratorMember(receiver, method, args)
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
			if hasSuffixFold(objectName, "__e") {
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
			if len(args) == 1 && !isSchemaDescribeOptionValue(args[0], "SObjectDescribeOptions") {
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
			appendTraceLazy(result, "apex.describe.sobject", "apex.describe", func() map[string]any {
				return vm.traceDescribeArgs("SObjectType.getDescribe", map[string]any{
					trace.ArgObject: objectName,
				})
			})
			describe := vm.describeSObjectValue(objectName, definition)
			optionOverride := ""
			if len(args) == 1 && args[0].Text != "" {
				optionOverride = strings.ToUpper(args[0].Text)
			}
			nameOverride := ""
			if vm.sObjectTypeDescribeShouldUseLocalName(objectName) {
				if name, ok := describe.Fields["name"]; ok && name.Kind == ValueString {
					nameOverride = localSchemaName(name.Text)
				}
			}
			describe = overlaySObjectDescribe(describe, nameOverride, optionOverride)
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
			appendTraceLazy(result, "apex.describe.fields", "apex.describe", func() map[string]any {
				return vm.traceDescribeArgs("fields.getMap", nil)
			})
			return privateDescribeCollection(receiver.Fields["map"]), receiver, false, true, nil
		}
	case "Schema.SObjectTypeFieldSets", "Schema.FieldSetMap":
		switch method {
		case "getMap":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSetMap.getMap expects 0 arguments")
			}
			appendTraceLazy(result, "apex.describe.fieldSets", "apex.describe", func() map[string]any {
				return vm.traceDescribeArgs("fieldSets.getMap", nil)
			})
			return privateDescribeCollection(receiver.Fields["map"]), receiver, false, true, nil
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
		case "getDescription":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getDescription expects 0 arguments")
			}
			return receiver.Fields["description"], receiver, false, true, nil
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getFields expects 0 arguments")
			}
			return privateDescribeCollection(receiver.Fields["fields"]), receiver, false, true, nil
		case "getLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getLabel expects 0 arguments")
			}
			return receiver.Fields["label"], receiver, false, true, nil
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getNamespace", "getNameSpace":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getNamespace expects 0 arguments")
			}
			return receiver.Fields["namespace"], receiver, false, true, nil
		case "getSObjectType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FieldSet.getSObjectType expects 0 arguments")
			}
			return receiver.Fields["sObjectType"], receiver, false, true, nil
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
			if len(args) > 1 {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField.getDescribe expects 0 or 1 arguments")
			}
			if len(args) == 1 && !isSchemaDescribeOptionValue(args[0], "FieldDescribeOptions") {
				return Null, receiver, false, true, fmt.Errorf("Schema.SObjectField.getDescribe expects FieldDescribeOptions")
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
			describe = overlaySObjectDescribe(describe, "", "")
			if systemField, ok := syntheticSObjectSystemField(fieldValue.Text); ok {
				if _, ok := describe.Fields["type"]; !ok {
					describe.Fields["type"] = schemaDisplayTypeValue(systemField.DisplayType)
				}
				if _, ok := describe.Fields["soapType"]; !ok {
					describe.Fields["soapType"] = schemaSOAPTypeValue(soapTypeForStorageField(systemField))
				}
				if relationship, ok := describe.Fields["relationshipName"]; systemField.RelationshipName != "" && (!ok || relationship.Kind == ValueNull) {
					describe.Fields["relationshipName"] = String(systemField.RelationshipName)
				}
				if existing, ok := describe.Fields["referenceTo"]; !ok || existing.Kind != ValueList || len(existing.List) == 0 {
					references := make([]Value, 0, len(systemField.ReferenceTo))
					for _, target := range systemField.ReferenceTo {
						references = append(references, sObjectTypeToken(target))
					}
					describe.Fields["referenceTo"] = List(references...)
				}
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
			appendTraceLazy(result, "apex.describe.field", "apex.describe", func() map[string]any {
				return vm.traceDescribeArgs("SObjectField.getDescribe", map[string]any{
					trace.ArgObject: objectValue.Text,
					trace.ArgField:  fieldValue.Text,
				})
			})
			return describe, receiver, false, true, nil
		}
		switch method {
		case "getName", "getLabel", "getType", "getSoapType", "getSObjectType", "getLength", "getByteLength", "getPrecision", "getScale", "getDigits", "getCalculatedFormula", "isHtmlFormatted", "isNillable", "isExternalId", "isUnique", "isEncrypted", "isCalculated", "isAutoNumber", "isCaseSensitive", "isNameField", "isCustom", "getReferenceTo", "getRelationshipName", "getPicklistValues", "getController", "getControllerValues", "isAccessible", "isCreateable", "isUpdateable", "isSortable":
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
			if _, field, ok := vm.sObjectFieldDefinition(objectName, fieldName); ok {
				return vm.sObjectFieldTokenFromField(objectName, field), receiver, false, true, nil
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
		case "getInlineHelpText":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getInlineHelpText expects 0 arguments")
			}
			if value, ok := receiver.Fields["inlineHelpText"]; ok {
				return value, receiver, false, true, nil
			}
			return String(""), receiver, false, true, nil
		case "getLocalName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getLocalName expects 0 arguments")
			}
			if value, ok := receiver.Fields["localName"]; ok {
				return value, receiver, false, true, nil
			}
			if value, ok := receiver.Fields["name"]; ok && value.Kind == ValueString {
				return String(vm.localSchemaName(value.Text)), receiver, false, true, nil
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
		case "getCalculatedFormula":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getCalculatedFormula expects 0 arguments")
			}
			if value, ok := receiver.Fields["calculatedFormula"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
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
		case "getByteLength":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getByteLength expects 0 arguments")
			}
			return receiver.Fields["byteLength"], receiver, false, true, nil
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
		case "getDigits":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getDigits expects 0 arguments")
			}
			return receiver.Fields["digits"], receiver, false, true, nil
		case "isHtmlFormatted":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.isHtmlFormatted expects 0 arguments")
			}
			return receiver.Fields["htmlFormatted"], receiver, false, true, nil
		case "getDataTranslationEnabled":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getDataTranslationEnabled expects 0 arguments")
			}
			if value, ok := receiver.Fields["dataTranslationEnabled"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
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
				return privateDescribeCollection(references), receiver, false, true, nil
			}
			if fieldName, ok := receiver.Fields["name"]; ok && fieldName.Kind == ValueString && isSObjectSystemUserReferenceField(fieldName.Text) {
				return List(sObjectTypeToken("User")), receiver, false, true, nil
			}
			return privateDescribeCollection(receiver.Fields["referenceTo"]), receiver, false, true, nil
		case "getRelationshipName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getRelationshipName expects 0 arguments")
			}
			hasReferenceTarget := false
			if references, ok := receiver.Fields["referenceTo"]; ok && references.Kind == ValueList && len(references.List) > 0 {
				hasReferenceTarget = true
			}
			fieldName, hasFieldName := receiver.Fields["name"]
			if !hasReferenceTarget && hasFieldName && fieldName.Kind == ValueString && isSObjectSystemUserReferenceField(fieldName.Text) {
				hasReferenceTarget = true
			}
			if !hasReferenceTarget {
				return Null, receiver, false, true, nil
			}
			if relationshipName, ok := receiver.Fields["relationshipName"]; ok && relationshipName.Kind != ValueNull {
				return relationshipName, receiver, false, true, nil
			}
			if hasFieldName && fieldName.Kind == ValueString {
				if derived := lookupFieldRelationshipName(fieldName.Text); derived != "" {
					return String(derived), receiver, false, true, nil
				}
			}
			return Null, receiver, false, true, nil
		case "getReferenceTargetField":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getReferenceTargetField expects 0 arguments")
			}
			if value, ok := receiver.Fields["referenceTargetField"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getRelationshipOrder":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getRelationshipOrder expects 0 arguments")
			}
			if value, ok := receiver.Fields["relationshipOrder"]; ok {
				return value, receiver, false, true, nil
			}
			return Int(0), receiver, false, true, nil
		case "getPicklistValues":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.getPicklistValues expects 0 arguments")
			}
			return privateDescribeCollection(receiver.Fields["picklistValues"]), receiver, false, true, nil
		case "getController":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["controller"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getControllerValues":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeFieldResult.%s expects 0 arguments", method)
			}
			if value, ok := receiver.Fields["controllerValues"]; ok {
				return privateDescribeCollection(value), receiver, false, true, nil
			}
			return typedMap("Map<String,Integer>"), receiver, false, true, nil
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
	case "Schema.FilteredLookupInfo", "FilteredLookupInfo":
		switch method {
		case "getControllingFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FilteredLookupInfo.getControllingFields expects 0 arguments")
			}
			if fields, ok := receiver.Fields["controllingFields"]; ok {
				return privateDescribeCollection(fields), receiver, false, true, nil
			}
			return typedList("List<String>"), receiver, false, true, nil
		case "isDependent":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FilteredLookupInfo.isDependent expects 0 arguments")
			}
			if dependent, ok := receiver.Fields["dependent"]; ok {
				return dependent, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "isOptionalFilter":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.FilteredLookupInfo.isOptionalFilter expects 0 arguments")
			}
			if optional, ok := receiver.Fields["optionalFilter"]; ok {
				return optional, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		}
	case "Schema.DescribeTabSetResult":
		switch method {
		case "getDescription":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getDescription expects 0 arguments")
			}
			return receiver.Fields["description"], receiver, false, true, nil
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
		case "getLogoUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getLogoUrl expects 0 arguments")
			}
			return receiver.Fields["logoUrl"], receiver, false, true, nil
		case "getName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getName expects 0 arguments")
			}
			return receiver.Fields["name"], receiver, false, true, nil
		case "getNamespace":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getNamespace expects 0 arguments")
			}
			return receiver.Fields["namespace"], receiver, false, true, nil
		case "getTabSetId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.getTabSetId expects 0 arguments")
			}
			return receiver.Fields["tabSetId"], receiver, false, true, nil
		case "isSelected":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabSetResult.isSelected expects 0 arguments")
			}
			return receiver.Fields["selected"], receiver, false, true, nil
		}
	case "Schema.DescribeTabResult":
		switch method {
		case "getColors":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getColors expects 0 arguments")
			}
			return receiver.Fields["colors"], receiver, false, true, nil
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
		case "getMiniIconUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getMiniIconUrl expects 0 arguments")
			}
			return receiver.Fields["miniIconUrl"], receiver, false, true, nil
		case "getMobileUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getMobileUrl expects 0 arguments")
			}
			return receiver.Fields["mobileUrl"], receiver, false, true, nil
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
		case "getTabEnumOrId":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeTabResult.getTabEnumOrId expects 0 arguments")
			}
			return receiver.Fields["tabEnumOrId"], receiver, false, true, nil
		}
	case "Schema.DescribeColorResult":
		switch method {
		case "getColor":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeColorResult.getColor expects 0 arguments")
			}
			return receiver.Fields["color"], receiver, false, true, nil
		case "getContext":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeColorResult.getContext expects 0 arguments")
			}
			return receiver.Fields["context"], receiver, false, true, nil
		case "getTheme":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeColorResult.getTheme expects 0 arguments")
			}
			return receiver.Fields["theme"], receiver, false, true, nil
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
				if method == "formatGmt" {
					return Null, receiver, false, true, fmt.Errorf("Datetime.formatGmt expects pattern String")
				}
				if method == "format" {
					_, _, local, _, ok := resolveTimeZoneForInstant(vm.currentUserTimeZoneID(), t)
					if !ok {
						return Null, receiver, false, true, unsupportedCallError("Datetime.format timezone " + vm.currentUserTimeZoneID())
					}
					return String(fmt.Sprintf("%d/%d/%d, %d:%02d %s", int(local.Month()), local.Day(), local.Year(), toTwelveHour(local.Hour()), local.Minute(), ampm(local.Hour()))), receiver, false, true, nil
				}
				gmt := t.UTC()
				return String(fmt.Sprintf("%d/%d/%d, %d:%02d %s", int(gmt.Month()), gmt.Day(), gmt.Year(), toTwelveHour(gmt.Hour()), gmt.Minute(), ampm(gmt.Hour()))), receiver, false, true, nil
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
			return longIntValue(t.UnixNano() / int64(time.Millisecond)), receiver, false, true, nil
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
		case "addDays", "addMonths", "addYears", "addHours", "addMinutes", "addSeconds":
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
				if receiver.Type == "Blob" && method == "toString" {
					return Null, receiver, false, true, newExceptionError("System.NullPointerException", "Argument cannot be null.")
				}
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
			return Int(int64(len(receiver.Fields["value"].String()))), receiver, false, true, nil
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
		case "getID", "toString":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("TimeZone.%s expects 0 arguments", method)
			}
			return receiver.Fields["id"], receiver, false, true, nil
		case "getDisplayName":
			if len(args) == 0 {
				return vm.timeZoneDisplayName(receiver), receiver, false, true, nil
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
			return databaseErrorStatusCodeValue(receiver.Fields["statusCode"]), receiver, false, true, nil
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
	case "Database.DuplicateError":
		switch method {
		case "getDuplicateResult":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.DuplicateError.getDuplicateResult expects 0 arguments")
			}
			if dupResult, ok := receiver.Fields["duplicateresult"]; ok {
				return dupResult, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
		case "getMessage":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.DuplicateError.getMessage expects 0 arguments")
			}
			return receiver.Fields["message"], receiver, false, true, nil
		case "getStatusCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.DuplicateError.getStatusCode expects 0 arguments")
			}
			return databaseErrorStatusCodeValue(receiver.Fields["statusCode"]), receiver, false, true, nil
		case "getFields":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Database.DuplicateError.getFields expects 0 arguments")
			}
			if fields, ok := receiver.Fields["fields"]; ok {
				return fields, receiver, false, true, nil
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
			return String(vm.localSchemaName(name.Text)), receiver, false, true, nil
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
			return privateDescribeCollection(receiver.Fields["recordTypeInfos"]), receiver, false, true, nil
		case "getRecordTypeInfosByName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosByName expects 0 arguments")
			}
			return privateDescribeCollection(receiver.Fields["recordTypeInfosByName"]), receiver, false, true, nil
		case "getRecordTypeInfosByDeveloperName":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosByDeveloperName expects 0 arguments")
			}
			return privateDescribeCollection(receiver.Fields["recordTypeInfosByDeveloperName"]), receiver, false, true, nil
		case "getRecordTypeInfosById":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getRecordTypeInfosById expects 0 arguments")
			}
			return privateDescribeCollection(receiver.Fields["recordTypeInfosById"]), receiver, false, true, nil
		case "getChildRelationships":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getChildRelationships expects 0 arguments")
			}
			value, err := vm.describeChildRelationshipsForDescribe(&receiver)
			return privateDescribeCollection(value), receiver, false, true, err
		case "getSObjectType", "getSobjectType":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			name, ok := receiver.Fields["name"]
			if !ok || name.Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult token missing object")
			}
			return sObjectTypeToken(name.Text), receiver, false, true, nil
		case "getSObjectDescribeOption":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.getSObjectDescribeOption expects 0 arguments")
			}
			return receiver.Fields["sObjectDescribeOption"], receiver, false, true, nil
		case "getDataTranslationEnabled", "getHasSubtypes", "getIsInterface":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			field := methodDescribeBoolField(method)
			if value, ok := receiver.Fields[field]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getAssociateEntityType", "getAssociateParentEntity", "getDefaultImplementation", "getImplementedBy", "getImplementsInterfaces":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.DescribeSObjectResult.%s expects 0 arguments", method)
			}
			field := methodDescribeStringField(method)
			if value, ok := receiver.Fields[field]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
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
		case "isMaster":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.RecordTypeInfo.isMaster expects 0 arguments")
			}
			return receiver.Fields["master"], receiver, false, true, nil
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
		case "isDeprecatedAndHidden":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.isDeprecatedAndHidden expects 0 arguments")
			}
			return receiver.Fields["deprecatedAndHidden"], receiver, false, true, nil
		case "getJunctionIdListNames":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getJunctionIdListNames expects 0 arguments")
			}
			if value, ok := receiver.Fields["junctionIdListNames"]; ok {
				return privateDescribeCollection(value), receiver, false, true, nil
			}
			return typedList("List<String>"), receiver, false, true, nil
		case "getJunctionReferenceTo":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Schema.ChildRelationship.getJunctionReferenceTo expects 0 arguments")
			}
			if value, ok := receiver.Fields["junctionReferenceTo"]; ok {
				return privateDescribeCollection(value), receiver, false, true, nil
			}
			return typedList("List<Schema.SObjectType>"), receiver, false, true, nil
		}
	case "HttpRequest":
		method = canonicalStdlibMemberName(method, "setEndpoint", "getEndpoint", "setMethod", "getMethod", "setBody", "setBodyAsBlob", "setBodyDocument", "getBodyDocument", "setClientCertificateName", "setClientCertificate", "setHeader", "getHeaderKeys", "getHeader", "setCompressed", "getCompressed", "setTimeout", "getTimeout", "getBody", "getBodyAsBlob")
		switch method {
		case "setEndpoint":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setEndpoint expects String")
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
			trimmedMethod := strings.TrimSpace(args[0].Text)
			_, err := normalizeHttpMethod(trimmedMethod)
			if err != nil {
				return Null, receiver, false, true, err
			}
			receiver.Fields["method"] = String(trimmedMethod)
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
			name := strings.TrimSpace(args[0].Text)
			if !vm.hasLocalClientCertificate(name) {
				return Null, receiver, false, true, newExceptionError("CalloutException", fmt.Sprintf("HttpRequest client certificate %s was not found in local certificate metadata", name))
			}
			receiver.Fields["clientCertificateName"] = String(name)
			receiver.Fields["clientCertificateSource"] = String("named")
			receiver.Fields["clientCertificatePasswordPresent"] = Bool(false)
			return Null, receiver, true, true, nil
		case "setClientCertificate":
			if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("HttpRequest.setClientCertificate expects certificate and password Strings")
			}
			receiver.Fields["clientCertificate"] = args[0]
			receiver.Fields["clientCertificateSource"] = String("inline")
			receiver.Fields["clientCertificatePasswordPresent"] = Bool(args[1].Text != "")
			return Null, receiver, true, true, nil
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
				} else if name, ok := httpCalloutEndpointName(endpoint.Text); ok && (!hasMock || httpMockRequiresResolvedEndpoint(vm.testContext.HTTPMock)) {
					return Null, receiver, false, true, newExceptionError("CalloutException", fmt.Sprintf("Named Credential %s was not found or is inactive", name))
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
			return typedList("List<AuthProvider>"), receiver, false, true, nil
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
		case "getUsernamePasswordEnabled":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getUsernamePasswordEnabled expects 0 arguments")
			}
			return Bool(true), receiver, false, true, nil
		case "getSelfRegistrationEnabled":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getSelfRegistrationEnabled expects 0 arguments")
			}
			return Bool(false), receiver, false, true, nil
		case "getSelfRegistrationUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getSelfRegistrationUrl expects 0 arguments")
			}
			return Null, receiver, false, true, nil
		case "getForgotPasswordUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.getForgotPasswordUrl expects 0 arguments")
			}
			return String("/ForgotPassword"), receiver, false, true, nil
		case "isCommunityUsingSiteAsContainer":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Auth.AuthConfiguration.isCommunityUsingSiteAsContainer expects 0 arguments")
			}
			return Bool(false), receiver, false, true, nil
		}
	case "Auth.JWT":
		return vm.callAuthJWTMember(receiver, method, args)
	case "Metadata.DeployContainer":
		switch method {
		case "addMetadata":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.addMetadata expects metadata")
			}
			values := receiver.Fields["components"]
			if values.Kind != ValueList {
				values = List()
			}
			values.List = append(values.List, args[0])
			receiver.Fields["components"] = values
			return Null, receiver, true, true, nil
		case "getMetadata":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.getMetadata expects 0 arguments")
			}
			values := receiver.Fields["components"]
			if values.Kind != ValueList {
				values = typedList("List<Metadata.Metadata>")
				receiver.Fields["components"] = values
			}
			return values, receiver, false, true, nil
		case "removeMetadata":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.removeMetadata expects metadata")
			}
			values := receiver.Fields["components"]
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
			receiver.Fields["components"] = filtered
			return Bool(removed), receiver, removed, true, nil
		case "removeMetadataByFullName":
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Metadata.DeployContainer.removeMetadataByFullName expects fullName String")
			}
			values := receiver.Fields["components"]
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
			receiver.Fields["components"] = filtered
			return Bool(removed), receiver, removed, true, nil
		}
	case "Metadata.DeployCallbackContext":
		if method != "getCallbackJobId" {
			break
		}
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Metadata.DeployCallbackContext.getCallbackJobId expects 0 arguments")
		}
		if jobID, ok := receiver.Fields["__callbackJobId"]; ok {
			return jobID, receiver, false, true, nil
		}
		return Null, receiver, false, true, nil
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
		return callSendEmailOptionsMember(receiver, method, args)
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
			flowName := ""
			if f, ok := receiver.Fields["flowName"]; ok && f.Kind == ValueString {
				flowName = f.Text
			}
			if flowName != "" && vm.Org != nil {
				found := false
				for _, obj := range vm.Org.Objects {
					for _, rule := range obj.Definition.FlowRules {
						if rule.Active && strings.EqualFold(rule.Name, flowName) {
							found = true
						}
					}
				}
				if !found {
					return Null, receiver, false, true, newExceptionError("FlowException", fmt.Sprintf("Flow.Interview.start: flow %q not found", flowName))
				}
			}
			receiver.Fields["started"] = Bool(true)
			receiver.Fields["status"] = String("Completed")
			return Null, receiver, true, true, nil
		case strings.EqualFold(method, "getVariableValue"):
			if len(args) != 1 || args[0].Kind != ValueString {
				return Null, receiver, false, true, fmt.Errorf("Flow.Interview.getVariableValue expects variable name String")
			}
			if variables, ok := receiver.Fields["variables"]; ok && variables.Kind == ValueMap {
				if value, ok := variables.Map[mapKey(args[0])]; ok {
					return value, receiver, false, true, nil
				}
				for key, stored := range variables.MapKeys {
					if stored.Kind == ValueString && strings.EqualFold(stored.Text, args[0].Text) {
						return variables.Map[key], receiver, false, true, nil
					}
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
		return vm.callApexPagesActionMember(receiver, method, args)
	case "ApexPages.Component", "ApexPages.ComponentIteration":
		return callApexPagesComponentMember(receiver, method, args)
	case "ApexPages.IdeaStandardController":
		return vm.callApexPagesIdeaStandardControllerMember(receiver, method, args, result)
	case "ApexPages.IdeaStandardSetController":
		return vm.callApexPagesIdeaStandardSetControllerMember(receiver, method, args, result)
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
		case "getComponentLabel":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("ApexPages.Message.getComponentLabel expects 0 arguments")
			}
			if value, ok := receiver.Fields["componentLabel"]; ok {
				return value, receiver, false, true, nil
			}
			return Null, receiver, false, true, nil
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
		method = canonicalStdlibMemberName(method, "getContent", "getContentAsPDF", "getUrl", "getAnchor", "setAnchor", "setRedirect", "getRedirect", "getRedirectCode", "setRedirectCode", "getParameters", "getHeaders", "getCookies", "setCookies")
		switch method {
		case "getContent", "getContentAsPDF":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.%s expects 0 arguments", method)
			}
			content, err := renderPageContent(vm, pageReferenceURL(receiver).Text, method == "getContentAsPDF")
			if err != nil {
				return Null, receiver, false, true, err
			}
			return content, receiver, false, true, nil
		case "getUrl":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getUrl expects 0 arguments")
			}
			return pageReferenceURL(receiver), receiver, false, true, nil
		case "getAnchor":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getAnchor expects 0 arguments")
			}
			return pageReferenceAnchor(receiver), receiver, false, true, nil
		case "setAnchor":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.setAnchor expects String")
			}
			if err := setPageReferenceAnchor(&receiver, args[0]); err != nil {
				return Null, receiver, false, true, err
			}
			return receiver, receiver, true, true, nil
		case "setRedirect":
			if len(args) != 1 || args[0].Kind != ValueBool {
				return Null, receiver, false, true, fmt.Errorf("PageReference.setRedirect expects Boolean")
			}
			receiver.Fields["redirect"] = args[0]
			return receiver, receiver, true, true, nil
		case "getRedirect":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getRedirect expects 0 arguments")
			}
			if value, ok := receiver.Fields["redirect"]; ok {
				return value, receiver, false, true, nil
			}
			return Bool(false), receiver, false, true, nil
		case "getRedirectCode":
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("PageReference.getRedirectCode expects 0 arguments")
			}
			if value, ok := receiver.Fields["redirectCode"]; ok {
				return value, receiver, false, true, nil
			}
			return Int(0), receiver, false, true, nil
		case "setRedirectCode":
			if len(args) != 1 || args[0].Kind != ValueInt {
				return Null, receiver, false, true, fmt.Errorf("PageReference.setRedirectCode expects Integer")
			}
			receiver.Fields["redirectCode"] = args[0]
			return receiver, receiver, true, true, nil
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
			if existing, ok := receiver.Fields["cookies"]; ok && existing.Kind == ValueMap {
				cookies = existing
				if cookies.Type == "" {
					cookies.Type = "Map<String,Cookie>"
				}
			}
			for _, cookie := range args[0].List {
				if cookie.Kind != ValueObject || !strings.EqualFold(cookie.Type, "Cookie") {
					return Null, receiver, false, true, fmt.Errorf("PageReference.setCookies expects List<Cookie>")
				}
				_, name, ok := objectFieldValue(cookie, "name")
				if !ok || name.Kind != ValueString || name.Text == "" {
					continue
				}
				key := mapKey(name)
				if _, exists := cookies.Map[key]; !exists {
					cookies.MapOrder = append(cookies.MapOrder, key)
				}
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
		if method == "getProtocol" || method == "getHost" || method == "getAuthority" || method == "getUserInfo" ||
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
			case "getUserInfo":
				if parsed.User == nil {
					return Null, receiver, false, true, nil
				}
				return String(parsed.User.String()), receiver, false, true, nil
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
		if method == "sameFile" {
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("URL.sameFile expects URL")
			}
			raw, err := platformScalarText(receiver, "URL")
			if err != nil {
				return Null, receiver, false, true, err
			}
			otherRaw, err := platformScalarText(args[0], "URL")
			if err != nil {
				return Null, receiver, false, true, err
			}
			parsed, err := url.Parse(raw)
			if err != nil {
				return Null, receiver, false, true, err
			}
			other, err := url.Parse(otherRaw)
			if err != nil {
				return Null, receiver, false, true, err
			}
			return Bool(urlSameFile(parsed, other)), receiver, false, true, nil
		}
	}
	if strings.HasPrefix(receiver.Type, "DataWeaveScriptResource.") {
		return vm.callDataWeaveScriptMember(receiver, method, args)
	}
	if value, updated, mutated, handled, err := vm.callPassivePlatformDTOObjectMember(receiver, method, args); handled || err != nil {
		return value, updated, mutated, handled, err
	}
	return Null, receiver, false, false, nil
}

func callSendEmailOptionsMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method,
		"setTriggerUserEmail", "getTriggerUserEmail",
		"setTriggerOtherEmail", "getTriggerOtherEmail",
		"setTriggerAutoResponseEmail", "getTriggerAutoResponseEmail",
	)
	fields := map[string]string{
		"setTriggerUserEmail":         "triggerUserEmail",
		"getTriggerUserEmail":         "triggerUserEmail",
		"setTriggerOtherEmail":        "triggerOtherEmail",
		"getTriggerOtherEmail":        "triggerOtherEmail",
		"setTriggerAutoResponseEmail": "triggerAutoResponseEmail",
		"getTriggerAutoResponseEmail": "triggerAutoResponseEmail",
	}
	field, ok := fields[method]
	if !ok {
		return Null, receiver, false, false, nil
	}
	if strings.HasPrefix(method, "set") {
		if len(args) != 1 || args[0].Kind != ValueBool {
			return Null, receiver, false, true, fmt.Errorf("Messaging.SendEmailOptions.%s expects Boolean", method)
		}
		receiver.Fields[field] = args[0]
		return Null, receiver, true, true, nil
	}
	if len(args) != 0 {
		return Null, receiver, false, true, fmt.Errorf("Messaging.SendEmailOptions.%s expects 0 arguments", method)
	}
	if value, ok := receiver.Fields[field]; ok {
		return value, receiver, false, true, nil
	}
	return Bool(false), receiver, false, true, nil
}

func (vm *VM) hasLocalClientCertificate(name string) bool {
	if vm == nil || vm.Org == nil || strings.TrimSpace(name) == "" {
		return false
	}
	for _, endpoint := range vm.Org.Metadata.Endpoints {
		if !strings.EqualFold(strings.TrimSpace(endpoint.Name), name) {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(endpoint.Kind)) {
		case "clientcertificate", "certificate", "certificateandkey":
			return true
		}
	}
	return false
}

func httpCalloutEndpointName(endpoint string) (string, bool) {
	trimmed := strings.TrimSpace(endpoint)
	if !hasPrefixFold(trimmed, "callout:") {
		return "", false
	}
	rest := strings.TrimSpace(trimmed[len("callout:"):])
	name, _, _ := strings.Cut(rest, "/")
	name = strings.TrimSpace(name)
	return name, name != ""
}

func urlSameFile(left, right *url.URL) bool {
	return strings.EqualFold(left.Scheme, right.Scheme) &&
		strings.EqualFold(left.Hostname(), right.Hostname()) &&
		urlEffectivePort(left) == urlEffectivePort(right) &&
		left.Path == right.Path &&
		left.RawQuery == right.RawQuery
}

func urlEffectivePort(parsed *url.URL) int64 {
	if raw := parsed.Port(); raw != "" {
		port, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return port
		}
	}
	return defaultURLPort(parsed.Scheme)
}

func (vm *VM) newRichMessagingAuthRequestResult() Value {
	out := Object("RichMessaging.AuthRequestResult")
	out.Fields["redirectPageReference"] = vm.newPageReference("/richmessaging/authenticated")
	out.Fields["resultStatus"] = Value{Kind: ValueObject, Type: "RichMessaging.AuthRequestResultStatus", Text: "AUTHENTICATED"}
	out.Fields["expirationDateTime"] = platformScalar("Datetime", formatPlatformDatetime(vm.fakeNow.Add(time.Hour)))
	return out
}

func newRichMessagingProcessCatalogOrderResult() Value {
	out := Object("RichMessaging.ProcessCatalogOrderResult")
	out.Fields["resultStatus"] = Value{Kind: ValueObject, Type: "RichMessaging.ProcessCatalogOrderResultStatus", Text: "SUCCESS"}
	out.Fields["errorMessage"] = String("")
	out.Fields["catalogOrderReferenceId"] = String("local-catalog-order")
	return out
}

func newRichMessagingProcessPaymentResult() Value {
	out := Object("RichMessaging.ProcessPaymentResult")
	out.Fields["resultStatus"] = Value{Kind: ValueObject, Type: "RichMessaging.ProcessPaymentResultStatus", Text: "SUCCESS"}
	out.Fields["errorMessage"] = String("")
	return out
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
		return passiveDTOMapValue(receiver), receiver, false, true, nil
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
	if value, handled, err := callInvocableActionPassiveDTOMember(receiver, method, args); handled || err != nil {
		return value, receiver, false, true, err
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

func callInvocableActionPassiveDTOMember(receiver Value, method string, args []Value) (Value, bool, error) {
	if receiver.Kind != ValueObject || !hasQualifiedPrefixFold(receiver.Type, "Invocable.Action") {
		return Null, false, nil
	}
	if len(args) != 0 {
		return Null, true, fmt.Errorf("%s.%s expects 0 arguments", receiver.Type, method)
	}
	member := apexMethodMemberName(method)
	if suffix, ok := passiveAccessorSuffix(member, "get"); ok {
		if _, value, found := objectFieldValue(receiver, passiveAccessorFieldName(receiver, suffix)); found {
			return value, true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "Invocable.Action.AdditionalAttribute") {
		switch {
		case strings.EqualFold(member, "getIsCollection"):
			return Bool(false), true, nil
		case strings.EqualFold(member, "getValueAsBooleanList"):
			return typedList("List<Boolean>"), true, nil
		case strings.EqualFold(member, "getValueAsDateList"):
			return typedList("List<Date>"), true, nil
		case strings.EqualFold(member, "getValueAsDoubleList"):
			return typedList("List<Double>"), true, nil
		case strings.EqualFold(member, "getValueAsIntegerList"):
			return typedList("List<Integer>"), true, nil
		case strings.EqualFold(member, "getValueAsList"):
			return typedList("List<Object>"), true, nil
		case strings.EqualFold(member, "getValueAsLongList"):
			return typedList("List<Long>"), true, nil
		case strings.EqualFold(member, "getValueAsStringList"):
			return typedList("List<String>"), true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "Invocable.Action.OutputParameter") {
		switch {
		case strings.EqualFold(member, "getMaxOccurs"):
			return Int(0), true, nil
		case strings.EqualFold(member, "getAdditionalAttributes"):
			return typedList("List<Invocable.Action.AdditionalAttribute>"), true, nil
		case strings.EqualFold(member, "getPicklistValues"):
			return typedList("List<Invocable.Action.PicklistValue>"), true, nil
		}
	}
	if strings.EqualFold(receiver.Type, "Invocable.Action.PicklistValue") {
		switch {
		case strings.EqualFold(member, "getActive"), strings.EqualFold(member, "getDefaultValue"):
			return Bool(false), true, nil
		}
	}
	return Null, false, nil
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
	methodsByName := generatedPlatformMethods()[strings.ToLower(typeName)]
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

func (vm *VM) callTypeObjectMember(receiver Value, method string, args []Value, result *Result) (Value, bool, error) {
	if !strings.EqualFold(receiver.Type, "Type") {
		return Null, false, nil
	}
	switch method {
	case "getName", "toString":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Type.%s expects 0 arguments", method)
		}
		return String(vm.typeDisplayName(typeValueName(receiver))), true, nil
	case "getNamespace", "getPackageName":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Type.%s expects 0 arguments", method)
		}
		typeName := typeValueName(receiver)
		if prefix, _, ok := strings.Cut(typeName, "."); ok {
			return String(prefix), true, nil
		}
		return Null, true, nil
	case "equals":
		if len(args) != 1 {
			return Null, true, fmt.Errorf("Type.equals expects 1 argument")
		}
		return Bool(receiver.Equal(args[0])), true, nil
	case "hashCode":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Type.hashCode expects 0 arguments")
		}
		return Int(int64(valueHashCode(receiver))), true, nil
	case "newInstance":
		if len(args) != 0 {
			return Null, true, fmt.Errorf("Type.newInstance expects 0 arguments")
		}
		typeName := typeValueName(receiver)
		if unsupported, ok := typeNewInstanceUnsupportedBuiltin(typeName); ok {
			return Null, true, unsupportedCallError("Type.newInstance uninstantiable built-in " + unsupported)
		}
		if strings.Contains(typeName, ".") {
			if _, ok := vm.resolveClassName(typeName); !ok && !typeNewInstanceAllowsDottedBuiltin(typeName) {
				return Null, true, unsupportedCallError("Type.newInstance namespace/package reflection for " + typeName)
			}
		}
		previousReflectionType := vm.reflectionConstructType
		vm.reflectionConstructType = typeName
		defer func() {
			vm.reflectionConstructType = previousReflectionType
		}()
		value, err := vm.constructValue(typeName, nil, nil, result)
		if err != nil {
			return Null, true, newExceptionError("TypeException", err.Error())
		}
		return value, true, nil
	case "isAssignableFrom":
		if len(args) != 1 || args[0].Kind != ValueObject || !strings.EqualFold(args[0].Type, "Type") {
			return Null, true, fmt.Errorf("Type.isAssignableFrom expects Type")
		}
		target := typeValueName(receiver)
		source := typeValueName(args[0])
		return Bool(vm.typeMatches(source, target, make(map[string]bool))), true, nil
	}
	return Null, false, nil
}

func (vm *VM) typeDisplayName(typeName string) string {
	if strings.TrimSpace(typeName) == "" {
		return typeName
	}
	if objectName, ok := vm.resolveObjectName(typeName); ok {
		return vm.sObjectTypeDisplayName(objectName)
	}
	return typeName
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
	if generated, ok := generatedPlatformTypes()[strings.ToLower(typeName)]; ok {
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
	for _, overloads := range generatedPlatformMethods()[strings.ToLower(generated.Name)] {
		for _, method := range overloads {
			if hasTypePrefixFold(generated.Name, "ConnectApi") && method.IsStatic && !generatedConnectAPIPassiveStaticMethod(generated.Name, method) {
				return false
			}
			if method.IsStatic && strings.EqualFold(method.Name, "builder") && strings.HasSuffix(method.ReturnType, ".Builder") {
				continue
			}
			if !method.IsStatic && (hasPrefixFold(apexMethodMemberName(method.Name), "get") || hasPrefixFold(apexMethodMemberName(method.Name), "is")) {
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
	for _, overloads := range generatedPlatformMethods()[strings.ToLower(generated.Name)] {
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
			slackRecordInvocation(&receiver, method, args)
			handler := Object("Slack.ActionHandler")
			handler.Fields["dispatcherType"] = String(receiverType)
			handler.Fields["parameters"] = args[0]
			handler.Fields["requestContext"] = args[1]
			return handler, receiver, true, true, nil
		}
	case "Slack.UserMappingUrlServiceProvider":
		switch name {
		case "generatepartnerauthorizationurl":
			if len(args) != 2 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserMappingUrlServiceProvider.generatePartnerAuthorizationUrl expects 2 arguments")
			}
			return String(slackAuthorizationURL("partner", args)), receiver, false, true, nil
		case "generateslackauthorizationurl":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserMappingUrlServiceProvider.generateSlackAuthorizationUrl expects 1 argument")
			}
			return String(slackAuthorizationURL("slack", args)), receiver, false, true, nil
		}
	case "Slack.UserProvisioningProvider":
		switch name {
		case "importusers", "revokeusersbysalesforceid":
			if len(args) != 2 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserProvisioningProvider.%s expects 2 arguments", method)
			}
			return slackUserProvisioningResult(method, args), receiver, false, true, nil
		case "revokeusersbyslackid":
			if len(args) != 1 {
				return Null, receiver, false, true, fmt.Errorf("Slack.UserProvisioningProvider.revokeUsersBySlackId expects 1 argument")
			}
			return slackUserProvisioningResult(method, args), receiver, false, true, nil
		}
	case "Slack.RunnableHandler":
		if name == "run" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.RunnableHandler.run expects 0 arguments")
			}
			receiver.Fields["ran"] = Bool(true)
			return Null, receiver, true, true, nil
		}
	case "Slack.Button":
		if name == "click" {
			if len(args) != 0 {
				return Null, receiver, false, true, fmt.Errorf("Slack.Button.click expects 0 arguments")
			}
			receiver.Fields["clicked"] = Bool(true)
			return Null, receiver, true, true, nil
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
			channel, ok := receiver.Fields["channel"]
			if !ok || channel.Kind != ValueObject {
				return Bool(true), receiver, false, true, nil
			}
			userID := slackUserID(args[0])
			if userID == "" {
				return Bool(false), receiver, false, true, nil
			}
			members := slackStringListField(channel, "members")
			if len(members.List) == 0 {
				return Bool(true), receiver, false, true, nil
			}
			return Bool(stringListContains(members, userID)), receiver, false, true, nil
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
			receiver.Fields["submitted"] = Bool(true)
			return Bool(true), receiver, true, true, nil
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

func slackRecordInvocation(receiver *Value, method string, args []Value) {
	count := int64(0)
	if value, ok := receiver.Fields["invocationCount"]; ok && value.Kind == ValueInt {
		count = value.Int
	}
	receiver.Fields["invocationCount"] = Int(count + 1)
	receiver.Fields["lastMethod"] = String(method)
	receiver.Fields["lastArgs"] = List(args...)
	if len(args) > 0 {
		receiver.Fields["lastParameters"] = args[0]
	}
	if len(args) > 1 {
		receiver.Fields["lastRequestContext"] = args[1]
	}
}

func slackAuthorizationURL(kind string, args []Value) string {
	switch kind {
	case "partner":
		partner := ""
		state := ""
		if len(args) > 0 {
			partner = scalarText(args[0])
		}
		if len(args) > 1 {
			state = scalarText(args[1])
		}
		return "https://slack.local/authorize/partner?partner=" + url.QueryEscape(partner) + "&state=" + url.QueryEscape(state)
	default:
		state := ""
		if len(args) > 0 {
			state = scalarText(args[0])
		}
		return "https://slack.local/authorize/slack?state=" + url.QueryEscape(state)
	}
}

func slackUserProvisioningResult(method string, args []Value) Value {
	result := Object("Slack.UserProvisioningResult")
	result.Fields["success"] = Bool(true)
	result.Fields["action"] = String(method)
	if len(args) > 0 {
		result.Fields["users"] = args[0]
		if args[0].Kind == ValueList {
			result.Fields["userCount"] = Int(int64(len(args[0].List)))
		}
	}
	if len(args) > 1 {
		result.Fields["teamId"] = args[1]
	}
	return result
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

func passiveDTOMapValue(receiver Value) Value {
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
	return values
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

func databaseErrorStatusCodeValue(value Value) Value {
	if value.Kind == ValueNull {
		return Null
	}
	if value.Kind == ValueObject && strings.EqualFold(value.Type, "StatusCode") {
		return value
	}
	code := strings.TrimSpace(value.String())
	if code == "" {
		return value
	}
	if canonical, ok := canonicalStatusCodeName(code); ok {
		code = canonical
	}
	return Value{Kind: ValueObject, Type: "StatusCode", Text: code}
}

func isSchemaDescribeOptionValue(value Value, optionType string) bool {
	if value.Kind != ValueObject {
		return false
	}
	valueType := strings.TrimSpace(value.Type)
	if strings.EqualFold(valueType, optionType) {
		return true
	}
	suffix := "." + optionType
	if len(valueType) <= len(suffix) || !strings.EqualFold(valueType[len(valueType)-len(suffix):], suffix) {
		return false
	}
	return strings.EqualFold(valueType[:len(valueType)-len(suffix)], "Schema")
}

func (vm *VM) sObjectTypeDescribeShouldUseLocalName(typeName string) bool {
	if vm == nil || vm.Org == nil {
		return false
	}
	if !hasNamespaceTokenInSchemaName(typeName) {
		return false
	}
	namespace := namespaceTokenInSchemaName(typeName)
	if namespace == "" {
		return false
	}
	if _, ok := vm.Org.Objects[localSchemaName(typeName)]; !ok {
		return false
	}
	currentNamespace := strings.TrimSpace(vm.currentCallerNamespace())
	if currentNamespace != "" {
		return strings.EqualFold(namespace, currentNamespace)
	}
	return strings.EqualFold(namespace, strings.TrimSpace(vm.Org.Namespace))
}
