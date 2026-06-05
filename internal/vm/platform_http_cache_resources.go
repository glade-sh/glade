package vm

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

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
	method = canonicalStdlibMemberName(method, "getDistance", "equals", "hashCode", "toString")
	switch method {
	case "getDistance":
		if len(args) != 2 || args[0].Kind != ValueObject || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Address.getDistance expects Location and unit String")
		}
		value, err := locationDistance(receiver, args[0], args[1].Text)
		return value, receiver, false, true, err
	case "equals":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Address.equals expects 1 argument")
		}
		return Bool(receiver.Equal(args[0])), receiver, false, true, nil
	case "hashCode":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Address.hashCode expects 0 arguments")
		}
		return Int(int64(valueHashCode(receiver))), receiver, false, true, nil
	case "toString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Address.toString expects 0 arguments")
		}
		return String(receiver.String()), receiver, false, true, nil
	}
	if suffix, ok := passiveAccessorSuffix(method, "with"); ok {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Address.%s expects 1 argument", method)
		}
		receiver.Fields[passiveAccessorFieldName(receiver, suffix)] = args[0]
		return receiver, receiver, true, true, nil
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
	method = canonicalStdlibMemberName(method, "addId", "addInteger", "addString", "build", "getMaxSize", "getRemainingSize", "getSize")
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
	case "getMaxSize", "getRemainingSize", "getSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.Builder.%s expects 0 arguments", method)
		}
		parts, _ := receiver.Fields["parts"]
		size := int64(0)
		if parts.Kind == ValueList {
			size = int64(len(parts.List))
		}
		switch method {
		case "getMaxSize":
			return Int(10), receiver, false, true, nil
		case "getRemainingSize":
			remaining := int64(10) - size
			if remaining < 0 {
				remaining = 0
			}
			return Int(remaining), receiver, false, true, nil
		default:
			return Int(size), receiver, false, true, nil
		}
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
	case "setStatus":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("StaticResourceCalloutMock.setStatus expects String")
		}
		receiver.Fields["status"] = args[0]
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
	case "setStatus":
		if len(args) != 1 || args[0].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("MultiStaticResourceCalloutMock.setStatus expects String")
		}
		receiver.Fields["status"] = args[0]
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
		key, hasKey, ok := cacheStringKeyArg(keyArg)
		if !ok {
			return Null, receiver, fmt.Errorf("%s.get key expects String", receiver.Type)
		}
		if !hasKey {
			return Null, receiver, nil
		}
		value, err := vm.cacheGetOrLoad(partitionName, cacheKeyForArgs(args, key), cacheBuilderArg(args), key)
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
		key, hasKey, ok := cacheStringKeyArg(keyArg)
		if !ok {
			return Null, receiver, fmt.Errorf("%s.remove key expects String", receiver.Type)
		}
		if !hasKey {
			return Null, receiver, nil
		}
		removed, removedOK := vm.cacheRemove(partitionName, cacheKeyForArgs(args, key))
		if !removedOK {
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

func cachePartitionPlatformObjectType(typeName string) bool {
	return strings.EqualFold(typeName, "Cache.Partition") ||
		strings.EqualFold(typeName, "Cache.OrgPartition") ||
		strings.EqualFold(typeName, "Cache.SessionPartition")
}

func generatedPlatformObjectMemberReceiver(typeName string) bool {
	return isExceptionType(typeName) ||
		cachePartitionPlatformObjectType(typeName) ||
		strings.EqualFold(typeName, "Cache.SecondaryKeyApi")
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
	if hasPrefixFold(partition, strings.ToLower(namespace)+".") {
		return partition
	}
	return namespace + "." + partition
}

func (vm *VM) cacheStaticDefaultGet(callee string, args []Value) (Value, error) {
	partition := cacheDefaultPartitionKey(callee)
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("%s expects key or CacheBuilder type and key", callee)
	}
	if len(args) == 1 {
		switch args[0].Kind {
		case ValueList:
			out := List()
			out.Type = "List<Object>"
			for _, key := range args[0].List {
				if key.Kind != ValueString {
					return Null, fmt.Errorf("%s keys expects List<String>", callee)
				}
				if value, ok := vm.cacheGet(partition, key.Text); ok {
					out.List = append(out.List, value)
				} else {
					out.List = append(out.List, Null)
				}
			}
			return out, nil
		case ValueSet:
			out := typedMap("Map<String,Object>")
			for _, key := range args[0].Set {
				if key.Kind != ValueString {
					return Null, fmt.Errorf("%s keys expects Set<String>", callee)
				}
				if value, ok := vm.cacheGet(partition, key.Text); ok {
					encoded := mapKey(key)
					out.Map[encoded] = value
					out.MapKeys[encoded] = key
					out.MapOrder = append(out.MapOrder, encoded)
				}
			}
			return out, nil
		}
	}
	keyArg := args[0]
	if len(args) == 2 {
		keyArg = args[1]
	}
	key, hasKey, ok := cacheStringKeyArg(keyArg)
	if !ok {
		return Null, fmt.Errorf("%s key expects String", callee)
	}
	if !hasKey {
		return Null, nil
	}
	return vm.cacheGetOrLoad(partition, cacheKeyForArgs(args, key), cacheBuilderArg(args), key)
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
	if len(args) == 1 && args[0].Kind == ValueList {
		out := List()
		out.Type = "List<Boolean>"
		for _, key := range args[0].List {
			if key.Kind != ValueString {
				return Null, fmt.Errorf("%s keys expects List<String>", callee)
			}
			_, removed := vm.cacheRemove(cacheDefaultPartitionKey(callee), key.Text)
			out.List = append(out.List, Bool(removed))
		}
		return out, nil
	}
	keyArg := args[0]
	if len(args) == 2 {
		keyArg = args[1]
	}
	key, hasKey, ok := cacheStringKeyArg(keyArg)
	if !ok {
		return Null, fmt.Errorf("%s key expects String", callee)
	}
	if !hasKey {
		return Null, nil
	}
	removed, removedOK := vm.cacheRemove(cacheDefaultPartitionKey(callee), cacheKeyForArgs(args, key))
	if !removedOK {
		return Null, nil
	}
	return removed, nil
}

func (vm *VM) cacheStaticDefaultContains(callee string, args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("%s expects String key", callee)
	}
	switch args[0].Kind {
	case ValueString:
		_, ok := vm.cacheGet(cacheDefaultPartitionKey(callee), args[0].Text)
		return Bool(ok), nil
	case ValueList:
		out := List()
		out.Type = "List<Boolean>"
		for _, key := range args[0].List {
			if key.Kind != ValueString {
				return Null, fmt.Errorf("%s keys expects List<String>", callee)
			}
			_, ok := vm.cacheGet(cacheDefaultPartitionKey(callee), key.Text)
			out.List = append(out.List, Bool(ok))
		}
		return out, nil
	case ValueSet:
		out := typedMap("Map<String,Boolean>")
		for _, key := range args[0].Set {
			if key.Kind != ValueString {
				return Null, fmt.Errorf("%s keys expects Set<String>", callee)
			}
			_, ok := vm.cacheGet(cacheDefaultPartitionKey(callee), key.Text)
			encoded := mapKey(key)
			out.Map[encoded] = Bool(ok)
			out.MapKeys[encoded] = key
			out.MapOrder = append(out.MapOrder, encoded)
		}
		return out, nil
	default:
		return Null, fmt.Errorf("%s expects String key", callee)
	}
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

func cacheStringKeyArg(value Value) (string, bool, bool) {
	switch value.Kind {
	case ValueString:
		return value.Text, true, true
	case ValueNull:
		return "", false, true
	default:
		return "", false, false
	}
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
	typeName := strings.TrimSpace(builderType.Text)
	if typeName == "" {
		if text, err := platformScalarText(builderType, "Type"); err == nil {
			typeName = strings.TrimSpace(text)
		}
	}
	if builderType.Kind != ValueObject || builderType.Type != "Type" || typeName == "" {
		return Null, nil
	}
	if !vm.typeMatches(typeName, "Cache.CacheBuilder", make(map[string]bool)) {
		return Null, nil
	}
	builder, err := vm.constructValue(typeName, nil, nil, &Result{})
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
	if status, ok := mock.Fields["status"]; ok {
		response.Fields["status"] = status
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

func (vm *VM) resolveLabelMergeExpressions(text string) string {
	if !strings.Contains(text, "{!$Label.") {
		return text
	}
	var out strings.Builder
	for {
		start := strings.Index(text, "{!$Label.")
		if start < 0 {
			out.WriteString(text)
			return out.String()
		}
		out.WriteString(text[:start])
		rest := text[start+len("{!$Label."):]
		end := strings.IndexByte(rest, '}')
		if end < 0 {
			out.WriteString(text[start:])
			return out.String()
		}
		labelName := strings.TrimSpace(rest[:end])
		if value, ok := vm.lookupLabel("Label." + labelName); ok {
			out.WriteString(value.String())
		} else {
			out.WriteString(text[start : start+len("{!$Label.")+end+1])
		}
		text = rest[end+1:]
	}
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
