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
	if method == "equals" {
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("Cookie.equals expects 1 argument")
		}
		return Bool(receiver.Equal(args[0])), receiver, false, true, nil
	}
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

func (vm *VM) callDataWeaveScriptMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
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
	value, err := vm.executeDataWeaveSource(receiver, inputs)
	return value, receiver, false, true, err
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
		if receiver.Fields["__gladeSourceDriven"].Bool {
			return Null, receiver, false, true, unsupportedCallError("DataWeave.Result.getValue output format " + receiver.Fields["mimeType"].Text)
		}
		return Null, receiver, false, true, nil
	case "getValueAsString":
		if receiver.Fields["__gladeTypedResult"].Bool {
			value := receiver.Fields["value"]
			switch value.Kind {
			case ValueString, ValueInt, ValueDecimal, ValueList:
				return String(apexCollectionString(value)), receiver, false, true, nil
			default:
				return Null, receiver, false, true, unsupportedCallError("DataWeave application/apex getValueAsString " + valueShape(value))
			}
		}
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
		return Null, receiver, false, true, nil
	case "getDistance":
		if len(args) != 2 {
			return Null, receiver, false, true, fmt.Errorf("Location.getDistance expects Location and unit String")
		}
		for i, arg := range args {
			if arg.Kind == ValueNull {
				return Null, receiver, false, true, newExceptionError("NullPointerException", fmt.Sprintf("Argument %d cannot be null", i+1))
			}
		}
		if args[0].Kind != ValueObject || args[1].Kind != ValueString {
			return Null, receiver, false, true, fmt.Errorf("Location.getDistance expects Location and unit String")
		}
		value, err := locationDistance(receiver, args[0], args[1].Text)
		return value, receiver, false, true, err
	case "toString":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("Location.toString expects 0 arguments")
		}
		return String(locationString(receiver)), receiver, false, true, nil
	default:
		return Null, receiver, false, false, nil
	}
}

// Native capacity counts payload bytes,
// independently of the quoted/underscore-separated display representation.
func queueableDuplicateSignatureSize(parts Value) int64 {
	var size int64
	for _, part := range parts.List {
		kind, text, _ := strings.Cut(scalarText(part), ":")
		switch kind {
		case "Integer":
			size += 4
		case "Id":
			size += 15
		case "String":
			size += int64(len(text))
		}
	}
	return size
}

func callQueueableDuplicateSignatureBuilderMember(receiver Value, method string, args []Value) (Value, Value, bool, bool, error) {
	method = canonicalStdlibMemberName(method, "addId", "addInteger", "addString", "build", "getMaxSize", "getRemainingSize", "getSize")
	switch method {
	case "addId", "addInteger", "addString":
		if len(args) != 1 {
			return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.Builder.%s expects 1 argument", method)
		}
		// T008/T013-T015: invalid additions fail before mutating the Builder.
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("InvalidParameterValueException", "Cannot add null to a deduplication signature")
		}
		text := args[0].String()
		if method == "addString" && text == "" {
			return Null, receiver, false, true, newExceptionError("InvalidParameterValueException", "Cannot add an empty string to a deduplication signature")
		}
		if method == "addId" && len(text) == 18 {
			text = text[:15]
		}
		parts, ok := receiver.Fields["parts"]
		if !ok || parts.Kind != ValueList {
			parts = typedList("List<String>")
		}
		kind := strings.TrimPrefix(method, "add")
		// Copy before appending so a rejected addition cannot modify an aliased list.
		parts.List = append(append([]Value(nil), parts.List...), String(kind+":"+text))
		// T016/T017/T028/T034/T035: the addition, not build(), enforces capacity.
		if queueableDuplicateSignatureSize(parts) > 32 {
			return Null, receiver, false, true, newExceptionError("DuplicateMessageException", "Deduplication signature exceeds the maximum length of 32 bytes")
		}
		receiver.Fields["parts"] = parts
		return receiver, receiver, true, true, nil
	case "build":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.Builder.build expects 0 arguments")
		}
		parts, _ := receiver.Fields["parts"]
		textParts := make([]string, 0, len(parts.List))
		for _, part := range parts.List {
			kind, text, _ := strings.Cut(scalarText(part), ":")
			if kind == "String" {
				// T007/T012/T024-T027: only string components are quoted and escaped.
				text = "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'").Replace(text) + "'"
			}
			textParts = append(textParts, text)
		}
		signature := Object("QueueableDuplicateSignature")
		signature.Fields["value"] = String(strings.Join(textParts, "_"))
		return signature, receiver, false, true, nil
	case "getMaxSize", "getRemainingSize", "getSize":
		if len(args) != 0 {
			return Null, receiver, false, true, fmt.Errorf("QueueableDuplicateSignature.Builder.%s expects 0 arguments", method)
		}
		parts, _ := receiver.Fields["parts"]
		size := queueableDuplicateSignatureSize(parts)
		switch method {
		case "getMaxSize":
			return Int(32), receiver, false, true, nil
		case "getRemainingSize":
			return Int(32 - size), receiver, false, true, nil
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
		if args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
		receiver.Fields["filter"] = args[0]
		return Null, receiver, true, true, nil
	case "setlimit":
		if len(args) == 1 && args[0].Kind == ValueNull {
			return Null, receiver, false, true, newExceptionError("NullPointerException", "Argument 1 cannot be null")
		}
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
	var callCount int64
	if existing, ok := receiver.Fields["callCount"]; ok && existing.Kind == ValueInt {
		callCount = existing.Int
	}
	receiver.Fields["callCount"] = Int(callCount + 1)
	receiver.Fields["lastMethod"] = String(method)
	receiver.Fields["lastArgs"] = List(args...)
	return Null, receiver, true, true, nil
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

func queueableDuplicateSignaturePlatformObjectType(typeName string) bool {
	return strings.EqualFold(typeName, "QueueableDuplicateSignature") ||
		strings.EqualFold(typeName, "QueueableDuplicateSignature.Builder") ||
		strings.EqualFold(typeName, "System.QueueableDuplicateSignature.Builder") ||
		strings.EqualFold(typeName, "Builder")
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
		for key, value := range results.Map {
			if name, ok := results.MapKeys[key]; ok && strings.EqualFold(name.Text, args[0].Text) && value.Kind == ValueList {
				return value, receiver, false, true, nil
			}
		}
	}
	return Null, receiver, false, true, newExceptionError("NoDataFoundException", "You are trying to retrieve a SObject type that was not part of the search query ["+args[0].Text+"]")
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
		return restAddHeader(receiver, args)
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
		if len(args) != 2 || (args[0].Kind != ValueString && args[0].Kind != ValueNull) || (args[1].Kind != ValueString && args[1].Kind != ValueNull) {
			return Null, receiver, false, true, fmt.Errorf("RestRequest.%s expects name and value Strings", method)
		}
		params := receiver.Fields["params"]
		if params.Kind != ValueMap {
			params = typedMap("Map<String,String>")
		}
		key := mapKey(args[0])
		params.Map[key] = args[1]
		params.MapKeys[key] = args[0]
		receiver.Fields["params"] = params
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

func httpMockRequiresResolvedEndpoint(mock Value) bool {
	return mock.Kind == ValueObject && mock.Type == "MultiStaticResourceCalloutMock"
}

func (vm *VM) callCachePartitionMember(receiver Value, method string, args []Value) (Value, Value, error) {
	name, ok := receiver.Fields["name"]
	if !ok || name.Kind != ValueString || name.Text == "" {
		name = String(vm.cacheDefaultName())
		receiver.Fields["name"] = name
	}
	partitionName := cachePartitionKey(receiver.Type, name.Text)
	scope := cacheScope(receiver.Type)
	method = strings.ToLower(method)
	switch method {
	case "clone":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.clone expects no arguments", receiver.Type)
		}
		cloned := cloneValue(receiver)
		cloned.Ref = newValueRef()
		return cloned, receiver, nil
	case "get":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, fmt.Errorf("%s.get expects key or CacheBuilder type and key", receiver.Type)
		}
		if len(args) == 1 && args[0].Kind == ValueSet {
			out := typedMap("Map<String,Object>")
			for _, key := range args[0].Set {
				partition, text, err := vm.cacheOperationKey(scope+".get", key, name.Text)
				if err != nil {
					return Null, receiver, err
				}
				if value, found := vm.cacheGet(partition, text); found {
					out.Map[mapKey(key)] = value
					out.MapKeys[mapKey(key)] = key
				}
			}
			return out, receiver, nil
		}
		value, err := vm.cacheGetValue(scope+".get", args, name.Text)
		return value, receiver, err
	case "put":
		if len(args) < 2 || len(args) > 5 {
			return Null, receiver, fmt.Errorf("%s.put expects String key, value[, ttlSeconds[, visibility[, immutable]]]", receiver.Type)
		}
		partition, key, err := vm.cacheOperationKey(scope+".put", args[0], name.Text)
		if err != nil {
			return Null, receiver, err
		}
		return Null, receiver, vm.cachePutValue(scope+".put", partition, key, args)
	case "remove":
		if len(args) != 1 && len(args) != 2 {
			return Null, receiver, fmt.Errorf("%s.remove expects key or CacheBuilder type and key", receiver.Type)
		}
		keyArg := args[len(args)-1]
		partition, key, err := vm.cacheOperationKey(scope+".remove", keyArg, name.Text)
		if err != nil {
			return Null, receiver, err
		}
		value, err := vm.cacheRemoveValue(scope+".remove", partition, cacheKeyForArgs(args, key), keyArg.Text)
		return value, receiver, err
	case "contains":
		if len(args) != 1 {
			return Null, receiver, fmt.Errorf("%s.contains expects String key or Set<String>", receiver.Type)
		}
		if args[0].Kind == ValueSet {
			out := typedMap("Map<String,Boolean>")
			for _, key := range args[0].Set {
				partition, text, err := vm.cacheOperationKey(scope+".contains", key, name.Text)
				if err != nil {
					return Null, receiver, err
				}
				_, found := vm.cacheGet(partition, text)
				out.Map[mapKey(key)] = Bool(found)
				out.MapKeys[mapKey(key)] = key
			}
			return out, receiver, nil
		}
		partition, key, err := vm.cacheOperationKey(scope+".contains", args[0], name.Text)
		if err != nil {
			return Null, receiver, err
		}
		_, found := vm.cacheGet(partition, key)
		return Bool(found), receiver, nil
	case "getkeys":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getKeys expects no arguments", receiver.Type)
		}
		out := Set()
		out.Type = "Set<String>"
		for _, key := range vm.cacheKeys(partitionName) {
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
	case "getavggetsize", "getavggettime", "getmaxgetsize", "getmaxgettime":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.%s expects no arguments", receiver.Type, method)
		}
		return Int(0), receiver, nil
	case "getmissrate":
		if len(args) != 0 {
			return Null, receiver, fmt.Errorf("%s.getMissRate expects no arguments", receiver.Type)
		}
		return Decimal(0), receiver, nil
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
		(queueableDuplicateSignaturePlatformObjectType(typeName) && !strings.EqualFold(typeName, "Builder")) ||
		strings.EqualFold(typeName, "compression.ZipWriter") ||
		strings.EqualFold(typeName, "compression.ZipReader") ||
		strings.EqualFold(typeName, "compression.ZipEntry") ||
		strings.EqualFold(typeName, "DataWeave.Result") ||
		strings.EqualFold(typeName, "FormulaEval.FormulaBuilder") ||
		strings.EqualFold(typeName, "FormulaEval.FormulaInstance") ||
		cachePartitionPlatformObjectType(typeName) ||
		strings.EqualFold(typeName, "UserProvisioning.FlowProvisionBase") ||
		strings.EqualFold(typeName, "UserProvisioning.UserProvisioningPlugin") ||
		strings.EqualFold(typeName, "UserProvisioning.UserProvisioningProcessHandler") ||
		strings.EqualFold(typeName, "UserProvisioning.DummyConnectorApexHandler") ||
		strings.EqualFold(typeName, "workflow.Action") ||
		strings.EqualFold(typeName, "workflow.ActionDml") ||
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

func (vm *VM) callCachePartitionStaticDefault(callee string, args []Value) (Value, bool, error) {
	className, methodName, ok := vm.splitClassMember(callee)
	if !ok || !cachePartitionPlatformObjectType(className) {
		return Null, false, nil
	}
	switch strings.ToLower(methodName) {
	case "validatekeys":
		if len(args) != 2 || args[0].Kind != ValueBool || (args[1].Kind != ValueList && args[1].Kind != ValueSet) {
			return Null, true, fmt.Errorf("%s expects Boolean and List<String> or Set<String>", callee)
		}
		return Null, true, nil
	case "validatecachebuilder", "validatekey", "validatekeyvalue", "validatepartitionname":
		return Null, true, nil
	case "createfullyqualifiedpartition":
		if len(args) != 2 || args[0].Kind != ValueString || args[1].Kind != ValueString {
			return Null, true, fmt.Errorf("%s expects namespace and partition Strings", callee)
		}
		return String(cacheFullyQualifiedPartition(args[0].Text, args[1].Text)), true, nil
	case "createfullyqualifiedkey":
		if len(args) != 3 || args[0].Kind != ValueString || args[1].Kind != ValueString || args[2].Kind != ValueString {
			return Null, true, fmt.Errorf("%s expects namespace, partition, and key Strings", callee)
		}
		return String(cacheFullyQualifiedPartition(args[0].Text, args[1].Text) + "." + args[2].Text), true, nil
	default:
		return Null, false, nil
	}
}

func (vm *VM) cacheStaticDefaultGet(callee string, args []Value) (Value, error) {
	if len(args) != 1 && len(args) != 2 {
		return Null, fmt.Errorf("%s expects key or CacheBuilder type and key", callee)
	}
	if len(args) == 1 && args[0].Kind == ValueList {
		out := List()
		out.Type = "List<Object>"
		for _, key := range args[0].List {
			if key.Kind != ValueString {
				return Null, fmt.Errorf("%s keys expects List<String>", callee)
			}
			if value, ok := vm.cacheGet(cacheDefaultPartitionKey(callee), key.Text); ok {
				out.List = append(out.List, value)
			} else {
				out.List = append(out.List, Null)
			}
		}
		return out, nil
	}
	if len(args) == 1 && args[0].Kind == ValueSet {
		out := typedMap("Map<String,Object>")
		for _, key := range args[0].Set {
			partition, text, err := vm.cacheOperationKey(callee, key, "")
			if err != nil {
				return Null, err
			}
			if value, found := vm.cacheGet(partition, text); found {
				encoded := mapKey(key)
				out.Map[encoded], out.MapKeys[encoded] = value, key
				out.MapOrder = append(out.MapOrder, encoded)
			}
		}
		return out, nil
	}
	return vm.cacheGetValue(callee, args, "")
}

func (vm *VM) cacheGetValue(callee string, args []Value, receiverPartition string) (Value, error) {
	keyArg := args[len(args)-1]
	partition, key, err := vm.cacheOperationKey(callee, keyArg, receiverPartition)
	if err != nil {
		return Null, err
	}
	if len(args) == 2 && args[0].Kind == ValueNull {
		return Null, newNullDereferenceError(callee)
	}
	return vm.cacheGetOrLoad(callee, keyArg.Text, partition, cacheKeyForArgs(args, key), cacheBuilderArg(args), key)
}

func (vm *VM) cacheStaticDefaultPut(callee string, args []Value) (Value, error) {
	if len(args) < 2 || len(args) > 5 {
		return Null, fmt.Errorf("%s expects String key, value[, ttlSeconds[, visibility[, immutable]]]", callee)
	}
	partition, key, err := vm.cacheOperationKey(callee, args[0], "")
	if err != nil {
		return Null, err
	}
	return Null, vm.cachePutValue(callee, partition, key, args)
}

func cachePutTTL(callee string, args []Value) (int64, error) {
	if len(args) < 3 || args[2].Kind == ValueNull {
		return 0, nil
	}
	if args[2].Kind == ValueInt {
		ttl := args[2].Int
		maximum, label := int64(172800), "Organization"
		if cacheScope(callee) == "Cache.Session" {
			maximum, label = 28800, "Session"
		}
		var message string
		if ttl > 0 && ttl < 300 {
			message = fmt.Sprintf("%s Cache TTL, %d, below minimum allowed: 300 secs", label, ttl)
		}
		if ttl > maximum {
			message = fmt.Sprintf("%s Cache TTL, %d, above maximum allowed: %d secs", label, ttl, maximum)
		}
		if message != "" {
			return 0, newExceptionError("cache.Org.OrgCacheException", fmt.Sprintf("Failed %s.put() for key '%s': %s", cacheScope(callee), args[0].Text, message))
		}
		return ttl, nil
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
	keyArg := args[len(args)-1]
	partition, key, err := vm.cacheOperationKey(callee, keyArg, "")
	if err != nil {
		return Null, err
	}
	return vm.cacheRemoveValue(callee, partition, cacheKeyForArgs(args, key), keyArg.Text)
}

func (vm *VM) cacheStaticDefaultContains(callee string, args []Value) (Value, error) {
	if len(args) != 1 {
		return Null, fmt.Errorf("%s expects String key", callee)
	}
	contains := func(key Value) (Value, error) {
		partition, text, err := vm.cacheOperationKey(callee, key, "")
		if err != nil {
			return Null, err
		}
		_, found := vm.cacheGet(partition, text)
		return Bool(found), nil
	}
	switch args[0].Kind {
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
			value, err := contains(key)
			if err != nil {
				return Null, err
			}
			encoded := mapKey(key)
			out.Map[encoded], out.MapKeys[encoded] = value, key
			out.MapOrder = append(out.MapOrder, encoded)
		}
		return out, nil
	default:
		return contains(args[0])
	}
}

func (vm *VM) cacheStaticDefaultKeys(callee string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s expects 0 arguments", callee)
	}
	out := Set()
	out.Type = "Set<String>"
	for _, key := range vm.cacheKeys(cachePartitionKey(cachePartitionTypeFromCallee(callee), vm.cacheDefaultName())) {
		out.Set = append(out.Set, String(key))
	}
	return out, nil
}

func (vm *VM) cacheStaticDefaultNumKeys(callee string, args []Value) (Value, error) {
	if len(args) != 0 {
		return Null, fmt.Errorf("%s expects 0 arguments", callee)
	}
	return Int(int64(len(vm.cacheKeys(cachePartitionKey(cachePartitionTypeFromCallee(callee), vm.cacheDefaultName()))))), nil
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

func (vm *VM) cacheGetOrLoad(callee, rawKey, partition, cacheKey string, builderType Value, loadKey string) (Value, error) {
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
		if builderType.Kind == ValueObject && builderType.Type == "Type" {
			return Null, newExceptionError("cache.InvalidCacheBuilderException", fmt.Sprintf("%s does not implement CacheBuilder", typeName))
		}
		return Null, nil
	}
	if !vm.typeMatches(typeName, "Cache.CacheBuilder", make(map[string]bool)) {
		return Null, newExceptionError("cache.InvalidCacheBuilderException", fmt.Sprintf("%s does not implement CacheBuilder", typeName))
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
		return Null, vm.cacheBuilderExecutionError(callee, rawKey, err)
	}
	if value.Kind != ValueNull && vm.cacheHasCapacity(partition) {
		vm.cachePut(partition, cacheKey, value, 0)
	}
	return value, nil
}

// List overloads retain their legacy storage path; they were removed after
// source API 54, before the supported API floor (controls N019/N021).
func cacheDefaultPartitionKey(callee string) string {
	return cachePartitionKey(cachePartitionTypeFromCallee(callee), "local.default")
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

// Cache put/get retain shared references within a transaction (PC001-PC006,
// PC101-PC106). Multi-get, builder and scan paths use the same store.
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
		vm.cacheDelete(partition, key)
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
			vm.cacheDelete(partition, key)
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
	// Retain the caller's reference, including collection slice headers.
	entry := cacheEntry{Value: value}
	if ttlSeconds > 0 {
		entry.ExpireAt = vm.fakeNow.Add(time.Duration(ttlSeconds) * time.Second)
	}
	entries[key] = entry
	vm.rememberCacheValueRefs(partition, key, value)
}

func (vm *VM) cacheRemove(partition, key string) (Value, bool) {
	value, ok := vm.cacheGet(partition, key)
	if !ok {
		return Null, false
	}
	vm.cacheDelete(partition, key)
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
	vm.rememberCacheValueRefs(partition, key, value)
}

func (vm *VM) cacheSecondaryScan(partition, startKey, endKey string) []cacheScanItem {
	entries := vm.platformCache[partition]
	if entries == nil {
		return nil
	}
	items := make([]cacheScanItem, 0, len(entries))
	for key, entry := range entries {
		if !entry.ExpireAt.IsZero() && !entry.ExpireAt.After(vm.fakeNow) {
			vm.cacheDelete(partition, key)
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
		registry := labelRegistryForNamespace(vm.Org.Metadata, namespace)
		if language := strings.TrimSpace(vm.currentUserInfoField("LanguageLocaleKey", "")); language != "" {
			filtered := registry
			filtered.Labels = labelsForLanguage(filtered.Labels, language)
			if value, status := resource.ResolveLabel(filtered, vm.Org.Namespace, namespace, label); status != resource.LabelLookupMissing {
				return String(value), true
			}
		}
		if value, status := resource.ResolveLabel(registry, vm.Org.Namespace, namespace, label); status != resource.LabelLookupMissing {
			return String(value), true
		}
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

// Cache partition identifiers contain a colon, which cannot occur in an Apex
// class name. Reuse static root locations without allocating a second index.
func (vm *VM) rememberCacheValueRefs(partition, key string, value Value) {
	vm.markCollectionRefsEscaped(value)
	vm.registerSObjectAliasRecord(value)
	vm.advanceAliasContainmentMutation()
	vm.rememberStaticValueRefsInField(value, staticFieldRef{ClassName: partition, FieldName: key})
}

func (vm *VM) cacheDelete(partition, key string) {
	delete(vm.platformCache[partition], key)
	vm.forgetStaticValueRefsInField(staticFieldRef{ClassName: partition, FieldName: key})
	vm.advanceAliasContainmentMutation()
	if len(vm.platformCache[partition]) == 0 {
		delete(vm.platformCache, partition)
	}
}

func (vm *VM) cacheAliasRoot(location staticFieldRef) (Value, bool) {
	if entry, ok := vm.platformCache[location.ClassName][location.FieldName]; ok {
		return entry.Value, true
	}
	return Null, false
}

func (vm *VM) storeCacheAliasRoot(location staticFieldRef, value Value) {
	if entry, ok := vm.platformCache[location.ClassName][location.FieldName]; ok {
		entry.Value = value
		vm.platformCache[location.ClassName][location.FieldName] = entry
	}
}

// Called only for indexed cache roots. Empty cache maps add no walk or
// allocation to ordinary mutations; the existing static-root index rejects them.
func (vm *VM) propagateAliasSnapshotToCacheEntry(location staticFieldRef, previous aliasSnapshot, updated Value) bool {
	old, ok := vm.cacheAliasRoot(location)
	if !ok {
		return false
	}
	seenPtr := aliasRefSetPool.Get().(*map[uint64]bool)
	seen := *seenPtr
	clear(seen)
	replaced, changed := replaceCacheAliasSnapshot(old, previous, updated, seen)
	aliasRefSetPool.Put(seenPtr)
	if changed {
		vm.storeCacheAliasRoot(location, replaced)
		if old.Ref == previous.ref && old.Kind == previous.kind {
			vm.rememberAdditionalStaticValueRefsInField(old, replaced, location)
		} else {
			vm.rememberStaticAliasUpdateRefs(previous, updated, location)
		}
	} else {
		vm.forgetStaticValueRefInField(previous.ref, location)
	}
	return true
}

// Indexed cache graphs can contain nested collections of the same kind. Walk
// those roots conservatively rather than using scope's flat-list exclusions.
// This keeps cache slice headers current without changing other alias routes.
func replaceCacheAliasSnapshot(value Value, previous aliasSnapshot, updated Value, seen map[uint64]bool) (Value, bool) {
	if value.Ref != 0 {
		if value.Ref == previous.ref && value.Kind == previous.kind {
			return updated, true
		}
		if seen[value.Ref] {
			return value, false
		}
		seen[value.Ref] = true
	}
	changed := false
	for _, children := range []map[string]Value{value.Fields, value.Map, value.MapKeys} {
		for key, child := range children {
			replaced, ok := replaceCacheAliasSnapshot(child, previous, updated, seen)
			if ok {
				children[key] = replaced
				changed = true
			}
		}
	}
	for _, children := range [][]Value{value.List, value.Set} {
		for i, child := range children {
			replaced, ok := replaceCacheAliasSnapshot(child, previous, updated, seen)
			if ok {
				children[i] = replaced
				changed = true
			}
		}
	}
	return value, changed
}
