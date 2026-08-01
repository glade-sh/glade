package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

func semaCollectionMethodSignature(receiverType, method string) (semaCollectionSignature, bool) {
	receiverType = normalizeArrayType(receiverType)
	if strings.EqualFold(receiverType, "Database.QueryResult") {
		receiverType = "List<SObject>"
	}
	base, args := semaGenericBaseAndArgs(receiverType)
	method = normalizeName(method)
	switch normalizeName(base) {
	case "list":
		if len(args) == 0 {
			args = []string{"Object"}
		}
		if len(args) != 1 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "get":
			return semaCollectionSignature{returnType: args[0], params: [][]string{{"Integer"}}}, true
		case "getsobjecttype":
			if strings.EqualFold(args[0], "SObject") || isSemaSObjectLike(args[0], nil) {
				return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
			}
		case "add":
			return semaCollectionSignature{returnType: "void", params: [][]string{{args[0]}, {"Integer", args[0]}}}, true
		case "addall":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"List<" + args[0] + ">"}, {"Set<" + args[0] + ">"}, {"Iterable<" + args[0] + ">"}}}, true
		case "clear":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "sort":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}, {"Comparator<" + args[0] + ">"}, {"Comparator<Object>"}}}, true
		case "size", "hashcode":
			return semaCollectionSignature{returnType: "Integer"}, true
		case "indexof":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{args[0]}}}, true
		case "isempty":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "contains":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[0]}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"List<" + args[0] + ">"}}}, true
		case "remove":
			return semaCollectionSignature{returnType: args[0], params: [][]string{{"Integer"}}}, true
		case "set":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Integer", args[0]}}}, true
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "clone", "deepclone":
			if method == "deepclone" {
				return semaCollectionSignature{returnType: "List<" + args[0] + ">", params: [][]string{{}, {"Boolean"}, {"Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean"}}}, true
			}
			return semaCollectionSignature{returnType: "List<" + args[0] + ">"}, true
		case "iterator":
			return semaCollectionSignature{returnType: "Iterator<" + args[0] + ">", params: [][]string{{}}}, true
		}
	case "set":
		if len(args) == 0 {
			args = []string{"Object"}
		}
		if len(args) != 1 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "size", "hashcode":
			return semaCollectionSignature{returnType: "Integer"}, true
		case "add", "contains", "remove":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[0]}}}, true
		case "addall", "containsall", "removeall", "retainall":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"List<" + args[0] + ">"}, {"Set<" + args[0] + ">"}}}, true
		case "clear":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "isempty":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Set<" + args[0] + ">"}}}, true
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "clone":
			return semaCollectionSignature{returnType: "Set<" + args[0] + ">"}, true
		case "iterator":
			return semaCollectionSignature{returnType: "Iterator<" + args[0] + ">", params: [][]string{{}}}, true
		}
	case "iterator":
		if len(args) == 0 {
			args = []string{"Object"}
		}
		if len(args) != 1 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "hasnext":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "next":
			return semaCollectionSignature{returnType: args[0], params: [][]string{{}}}, true
		case "remove":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		}
	case "map":
		if len(args) == 0 {
			args = []string{"Object", "Object"}
		}
		if len(args) != 2 {
			return semaCollectionSignature{}, false
		}
		switch method {
		case "get":
			return semaCollectionSignature{returnType: args[1], params: [][]string{{args[0]}}}, true
		case "getsobjecttype":
			if strings.EqualFold(args[1], "SObject") || isSemaSObjectLike(args[1], nil) {
				return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
			}
		case "put":
			return semaCollectionSignature{returnType: args[1], params: [][]string{{args[0], args[1]}}}, true
		case "putall":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Map<" + args[0] + "," + args[1] + ">"}, {"List<" + args[1] + ">"}}}, true
		case "keyset":
			return semaCollectionSignature{returnType: "Set<" + args[0] + ">"}, true
		case "values":
			return semaCollectionSignature{returnType: "List<" + args[1] + ">"}, true
		case "size", "hashcode":
			return semaCollectionSignature{returnType: "Integer"}, true
		case "containskey", "containsvalue", "isempty":
			if method == "containsvalue" {
				return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[1]}}}, true
			}
			if method == "containskey" {
				return semaCollectionSignature{returnType: "Boolean", params: [][]string{{args[0]}}}, true
			}
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Map<" + args[0] + "," + args[1] + ">"}}}, true
		case "remove":
			return semaCollectionSignature{returnType: args[1], params: [][]string{{args[0]}}}, true
		case "clear":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "clone", "deepclone":
			return semaCollectionSignature{returnType: "Map<" + args[0] + "," + args[1] + ">"}, true
		}
	}
	return semaCollectionSignature{}, false
}

func semaPlatformConstructorSignatures(typeName string) ([][]string, bool) {
	lookup := normalizeName(semaCanonicalPlatformAlias(typeName))
	for _, typ := range typesys.StandardPlatformSymbolView() {
		if !semaPlatformSymbolMatchesConstructorType(typ, lookup) {
			continue
		}
		var params [][]string
		seen := make(map[string]bool)
		for _, member := range typ.Members {
			if member.Kind != apexast.DeclarationConstructor {
				continue
			}
			signature := make([]string, len(member.Parameters))
			for i, param := range member.Parameters {
				signature[i] = param.Type
			}
			key := strings.Join(signature, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			params = append(params, signature)
		}
		if len(params) == 0 {
			if typ.ConstructorsAuthoritative {
				return [][]string{}, true
			}
			return nil, false
		}
		return params, true
	}
	return nil, false
}

func semaPlatformSymbolMatchesConstructorType(typ typesys.TypeSymbol, lookup string) bool {
	if normalizeName(typ.Name) == lookup {
		return true
	}
	if typ.Namespace == "" {
		return false
	}
	return normalizeName(typ.Namespace+"."+typ.Name) == lookup
}

func checkSemaCollectionCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model *semaTypeMemberView) ([]diagnostic.Diagnostic, bool) {
	sig, ok := semaCollectionMethodSignature(receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if strings.EqualFold(method, "addError") && semaAddErrorArgsAccepted(argTypes, model) {
		return nil, true
	}
	if strings.EqualFold(method, "addAll") && len(args) == 1 && len(argTypes) == 1 && strings.EqualFold(argTypes[0], "Object") && strings.Contains(args[0].text, ".") {
		return nil, true
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	if semaCollectionFieldPathArgsMatch(sig.params, args, scope, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), start, end, source)}, true
}

func semaCollectionFieldPathArgsMatch(params [][]string, args []semaArg, scope map[string]string, model *semaTypeMemberView) bool {
	for _, candidate := range params {
		if len(candidate) != len(args) {
			continue
		}
		matched := true
		for i, param := range candidate {
			argType := inferSemaArgTypeWithModel(args[i].text, scope, model)
			if argType == "" || strings.EqualFold(argType, "null") || semaAssignableToType(param, argType, model) {
				continue
			}
			fieldType := semaSObjectFieldPathArgType(args[i].text, scope, model)
			if fieldType == "" || (!semaAssignableToType(param, fieldType, model) && !semaChildRelationshipListArgCompatible(param, fieldType, args[i].text)) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}

func semaSObjectFieldPathArgType(arg string, scope map[string]string, model *semaTypeMemberView) string {
	receiverExpr, field, ok := splitSemaMethodPath(strings.TrimSpace(arg))
	if !ok {
		return ""
	}
	receiverType := inferSemaArgTypeWithModel(receiverExpr, scope, model)
	if receiverType == "" || !isSemaSObjectLike(receiverType, model) {
		return ""
	}
	if target, ok := semaResolveFieldPath(model, receiverType, field); ok && target.member.Type != "" {
		return target.member.Type
	}
	return semaFallbackFieldPathType(field)
}

func semaChildRelationshipListArgCompatible(paramType, argType, argText string) bool {
	_, field, ok := splitSemaMethodPath(strings.TrimSpace(argText))
	field = strings.TrimSuffix(field, "__r")
	if !ok || !semaLooksLikeChildRelationship(normalizeName(field)) {
		return false
	}
	paramBase, paramArgs := semaGenericBaseAndArgs(paramType)
	argBase, argArgs := semaGenericBaseAndArgs(argType)
	if !strings.EqualFold(paramBase, "List") || !strings.EqualFold(argBase, "List") || len(paramArgs) != 1 || len(argArgs) != 1 {
		return false
	}
	return semaIsCustomSObjectTypeName(paramArgs[0]) && semaIsCustomSObjectTypeName(argArgs[0])
}

func semaIsCustomSObjectTypeName(typeName string) bool {
	key := normalizeName(typeName)
	return strings.HasSuffix(key, "__c") || strings.HasSuffix(key, "__e") || strings.HasSuffix(key, "__mdt")
}

func checkSemaEnumCall(typ typesys.TypeSymbol, member typesys.MemberSymbol, receiverType, method string, args []semaArg, start, end int, source string, scope map[string]string, model *semaTypeMemberView) ([]diagnostic.Diagnostic, bool) {
	sig, ok := semaEnumMethodSignature(model, receiverType, method)
	if !ok {
		return nil, false
	}
	argTypes := make([]string, len(args))
	for i, arg := range args {
		argTypes[i] = inferSemaArgTypeWithModel(arg.text, scope, model)
	}
	if semaArgsMatchAny(sig.params, argTypes, model) {
		return nil, true
	}
	return []diagnostic.Diagnostic{collectionCallDiagnostic(typ, member, method, len(args), start, end, source)}, true
}

func semaAddErrorArgsAccepted(argTypes []string, model *semaTypeMemberView) bool {
	switch len(argTypes) {
	case 1:
		return semaAssignableToType("String", argTypes[0], model) || semaAssignableToType("Exception", argTypes[0], model)
	case 2:
		return (semaAssignableToType("Schema.SObjectField", argTypes[0], model) && semaAssignableToType("String", argTypes[1], model)) ||
			(argTypes[0] == "" && semaAssignableToType("String", argTypes[1], model)) ||
			(strings.EqualFold(argTypes[0], "Object") && semaAssignableToType("String", argTypes[1], model)) ||
			(semaAssignableToType("String", argTypes[0], model) && semaAssignableToType("String", argTypes[1], model)) ||
			(semaAssignableToType("String", argTypes[0], model) && semaAssignableToType("Boolean", argTypes[1], model)) ||
			(semaAssignableToType("Exception", argTypes[0], model) && semaAssignableToType("Boolean", argTypes[1], model))
	case 3:
		return semaAssignableToType("Schema.SObjectField", argTypes[0], model) && semaAssignableToType("String", argTypes[1], model) && semaAssignableToType("Boolean", argTypes[2], model)
	default:
		return false
	}
}

func semaEnumMethodSignature(model *semaTypeMemberView, receiverType, method string) (semaCollectionSignature, bool) {
	originalType := strings.TrimSpace(receiverType)
	receiverType = semaCanonicalPlatformAlias(receiverType)
	members, ok := model.lookup(normalizeName(receiverType))
	if !ok || members.kind != apexast.DeclarationEnum {
		if !semaExplicitPlatformEnumType(originalType) {
			return semaCollectionSignature{}, false
		}
	}
	switch normalizeName(method) {
	case "name", "tostring":
		return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
	case "ordinal":
		return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
	case "values":
		return semaCollectionSignature{returnType: "List<" + receiverType + ">", params: [][]string{{}}}, true
	case "valueof":
		return semaCollectionSignature{returnType: receiverType, params: [][]string{{"String"}}}, true
	}
	return semaCollectionSignature{}, false
}

func semaPlatformMethodSignature(receiverType, method string) (semaCollectionSignature, bool) {
	receiverType = semaCanonicalPlatformAlias(receiverType)
	method = normalizeName(method)
	if semaDescribeSObjectResultChildRelationships(receiverType, method) {
		return semaCollectionSignature{returnType: "List<Schema.ChildRelationship>", params: [][]string{{}}}, true
	}
	if strings.EqualFold(receiverType, "System") {
		switch method {
		case "hashcode":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{"Object"}}}, true
		}
	}
	if strings.EqualFold(receiverType, "Auth.AuthToken") {
		switch method {
		case "getaccesstoken":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String"}}}, true
		case "getaccesstokenmap":
			return semaCollectionSignature{returnType: "Map<String,String>", params: [][]string{{"String", "String"}}}, true
		case "refreshaccesstoken":
			return semaCollectionSignature{returnType: "Map<String,String>", params: [][]string{{"String", "String", "String"}}}, true
		case "revokeaccess":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String", "String", "String", "String"}}}, true
		}
	}
	if strings.EqualFold(receiverType, "Auth.AuthConfiguration") {
		switch method {
		case "getauthproviderssodomainurl", "getauthproviderssourl", "getsamlssourl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String", "String"}}}, true
		case "getcertificateloginurl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String"}}}, true
		}
	}
	switch method {
	case "clone":
		return semaCollectionSignature{returnType: receiverType, params: [][]string{{}}}, true
	case "hashcode":
		return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
	case "tostring":
		return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
	case "equals":
		return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Object"}}}, true
	}
	receiverBase, receiverArgs := semaGenericBaseAndArgs(receiverType)
	if strings.EqualFold(receiverBase, "Database.Batchable") {
		itemType := "Object"
		if len(receiverArgs) == 1 && receiverArgs[0] != "" {
			itemType = receiverArgs[0]
		}
		switch method {
		case "start":
			return semaCollectionSignature{returnType: "Iterable<" + itemType + ">", params: [][]string{{"Database.BatchableContext"}}}, true
		case "execute":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Database.BatchableContext", "List<" + itemType + ">"}}}, true
		case "finish":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Database.BatchableContext"}}}, true
		}
	}
	if strings.EqualFold(receiverBase, "Iterable") {
		itemType := "Object"
		if len(receiverArgs) == 1 && receiverArgs[0] != "" {
			itemType = receiverArgs[0]
		}
		if method == "iterator" {
			return semaCollectionSignature{returnType: "Iterator<" + itemType + ">", params: [][]string{{}}}, true
		}
	}
	if strings.EqualFold(receiverBase, "Iterator") {
		itemType := "Object"
		if len(receiverArgs) == 1 && receiverArgs[0] != "" {
			itemType = receiverArgs[0]
		}
		switch method {
		case "hasnext":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "next":
			return semaCollectionSignature{returnType: itemType, params: [][]string{{}}}, true
		}
	}
	switch normalizeName(receiverType) {
	case "assert", "system.assert":
		switch method {
		case "areequal", "arenotequal":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Object", "Object"}, {"Object", "Object", "String"}}}, true
		case "istrue", "isfalse":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Boolean"}, {"Boolean", "String"}}}, true
		case "isnull", "isnotnull":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Object"}, {"Object", "String"}}}, true
		case "fail":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}, {"String"}}}, true
		case "isinstanceoftype":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Object", "Type"}, {"Object", "Type", "String"}}}, true
		}
	case "type":
		switch method {
		case "tostring", "getname":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "newinstance":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{}}}, true
		case "isassignablefrom":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Type"}}}, true
		}
	case "userinfo":
		switch method {
		case "getuserid", "getorganizationid":
			return semaCollectionSignature{returnType: "Id", params: [][]string{{}}}, true
		case "gettimezone":
			return semaCollectionSignature{returnType: "TimeZone", params: [][]string{{}}}, true
		case "getusername", "getname", "getfirstname", "getlastname", "getemail", "getorganizationname", "getprofileid", "getsessionid", "getlocale", "getlanguage":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "uuid":
		if method == "randomuuid" {
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "cookie":
		switch method {
		case "getmaxage":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "getname", "getvalue", "getpath", "getdomain", "getsamesite":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "issecure", "ishttponly":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "dmlexception":
		switch method {
		case "getnumdml":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "getdmlmessage", "getdmlstatuscode", "getdmlid":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Integer"}}}, true
		case "getdmltype":
			return semaCollectionSignature{returnType: "StatusCode", params: [][]string{{"Integer"}}}, true
		case "getdmlindex":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{"Integer"}}}, true
		}
	case "blob":
		switch method {
		case "tostring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "valueof":
			return semaCollectionSignature{returnType: "Blob", params: [][]string{{"String"}}}, true
		}
	case "encodingutil":
		switch method {
		case "base64encode", "converttohex":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Blob"}}}, true
		case "base64decode", "convertfromhex":
			return semaCollectionSignature{returnType: "Blob", params: [][]string{{"String"}}}, true
		case "urlencode", "urldecode":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String"}}}, true
		}
	case "address":
		switch method {
		case "getstreet", "getcity", "getstate", "getstatecode", "getpostalcode", "getcountry", "getcountrycode", "getgeocodeaccuracy":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getlatitude", "getlongitude":
			return semaCollectionSignature{returnType: "Double", params: [][]string{{}}}, true
		}
	case "id":
		switch method {
		case "getsobjecttype":
			return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
		case "adderror":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}, {"Exception"}, {"String", "Boolean"}, {"Exception", "Boolean"}, {"String", "String"}, {"String", "String", "Boolean"}, {"Schema.SObjectField", "String"}, {"Schema.SObjectField", "String", "Boolean"}}}, true
		}
	case "datetime":
		switch method {
		case "valueof", "valueofgmt":
			return semaCollectionSignature{returnType: "Datetime", params: [][]string{{"String"}, {"Object"}}}, true
		case "gettime":
			return semaCollectionSignature{returnType: "Long", params: [][]string{{}}}, true
		case "date", "dategmt":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{}}}, true
		case "format", "formatgmt":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}, {"String"}, {"String", "String"}}}, true
		}
	case "date":
		switch method {
		case "today":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{}}}, true
		case "newinstance":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{"Integer", "Integer", "Integer"}}}, true
		case "valueof":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{"String"}, {"Object"}}}, true
		case "parse":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{"String"}}}, true
		case "daysinmonth":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{"Integer", "Integer"}}}, true
		case "adddays", "addmonths", "addyears":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{"Integer"}}}, true
		case "day", "month", "year":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "daysbetween", "monthsbetween":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{"Date"}}}, true
		case "format":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}, {"String"}}}, true
		}
	case "integer", "long", "decimal", "double":
		switch method {
		case "intvalue":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "longvalue":
			return semaCollectionSignature{returnType: "Long", params: [][]string{{}}}, true
		case "doublevalue":
			return semaCollectionSignature{returnType: "Double", params: [][]string{{}}}, true
		case "decimalvalue":
			return semaCollectionSignature{returnType: "Decimal", params: [][]string{{}}}, true
		case "round":
			return semaCollectionSignature{returnType: "Long", params: [][]string{{}, {"System.RoundingMode"}, {"RoundingMode"}}}, true
		case "setscale":
			return semaCollectionSignature{returnType: "Decimal", params: [][]string{{"Integer"}, {"Integer", "System.RoundingMode"}}}, true
		case "toplainstring", "format":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "database":
		switch method {
		case "setsavepoint":
			return semaCollectionSignature{returnType: "Savepoint", params: [][]string{{}}}, true
		case "rollback":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Savepoint"}}}, true
		case "insert", "update":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"SObject"}, {"SObject", "Boolean"}, {"SObject", "Database.DMLOptions"}, {"SObject", "AccessLevel"}, {"SObject", "Boolean", "AccessLevel"}, {"List<SObject>"}, {"List<SObject>", "Boolean"}, {"List<SObject>", "Database.DMLOptions"}, {"List<SObject>", "AccessLevel"}, {"List<SObject>", "Boolean", "AccessLevel"}}}, true
		case "delete", "undelete":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"SObject"}, {"SObject", "Boolean"}, {"SObject", "AccessLevel"}, {"SObject", "Boolean", "AccessLevel"}, {"Id"}, {"Id", "Boolean"}, {"Id", "AccessLevel"}, {"Id", "Boolean", "AccessLevel"}, {"List<SObject>"}, {"List<SObject>", "Boolean"}, {"List<SObject>", "AccessLevel"}, {"List<SObject>", "Boolean", "AccessLevel"}, {"List<Id>"}, {"List<Id>", "Boolean"}, {"List<Id>", "AccessLevel"}, {"List<Id>", "Boolean", "AccessLevel"}}}, true
		case "upsert":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"SObject"}, {"SObject", "Boolean"}, {"SObject", "Database.DMLOptions"}, {"SObject", "AccessLevel"}, {"SObject", "Schema.SObjectField"}, {"SObject", "Schema.SObjectField", "Boolean"}, {"SObject", "Schema.SObjectField", "AccessLevel"}, {"SObject", "Boolean", "AccessLevel"}, {"SObject", "Schema.SObjectField", "Boolean", "AccessLevel"}, {"List<SObject>"}, {"List<SObject>", "Boolean"}, {"List<SObject>", "Database.DMLOptions"}, {"List<SObject>", "AccessLevel"}, {"List<SObject>", "Schema.SObjectField"}, {"List<SObject>", "Schema.SObjectField", "Boolean"}, {"List<SObject>", "Schema.SObjectField", "AccessLevel"}, {"List<SObject>", "Boolean", "AccessLevel"}, {"List<SObject>", "Schema.SObjectField", "Boolean", "AccessLevel"}}}, true
		}
	case "database.querylocator":
		switch method {
		case "getquery":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "iterator":
			return semaCollectionSignature{returnType: "Database.QueryLocatorIterator", params: [][]string{{}}}, true
		}
	case "database.querylocatoriterator":
		switch method {
		case "hasnext":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "next":
			return semaCollectionSignature{returnType: "SObject", params: [][]string{{}}}, true
		}
	case "jsonparser":
		switch method {
		case "nexttoken", "nextvalue", "getcurrenttoken":
			return semaCollectionSignature{returnType: "JSONToken", params: [][]string{{}}}, true
		case "gettext", "getcurrentname":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getintegervalue":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "getlongvalue":
			return semaCollectionSignature{returnType: "Long", params: [][]string{{}}}, true
		case "getdoublevalue":
			return semaCollectionSignature{returnType: "Double", params: [][]string{{}}}, true
		case "getdecimalvalue":
			return semaCollectionSignature{returnType: "Decimal", params: [][]string{{}}}, true
		case "getbooleanvalue":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "getdatevalue":
			return semaCollectionSignature{returnType: "Date", params: [][]string{{}}}, true
		case "getdatetimevalue":
			return semaCollectionSignature{returnType: "Datetime", params: [][]string{{}}}, true
		case "gettimevalue":
			return semaCollectionSignature{returnType: "Time", params: [][]string{{}}}, true
		case "getidvalue":
			return semaCollectionSignature{returnType: "Id", params: [][]string{{}}}, true
		case "getblobvalue":
			return semaCollectionSignature{returnType: "Blob", params: [][]string{{}}}, true
		case "skipchildren":
			return semaCollectionSignature{returnType: "JSONParser", params: [][]string{{}}}, true
		case "clearcurrenttoken":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		}
	case "exception":
		switch method {
		case "getmessage", "gettypename", "getstacktracestring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getlinenumber":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		}
	case "string":
		switch method {
		case "getchars":
			return semaCollectionSignature{returnType: "List<Integer>", params: [][]string{{}}}, true
		case "fromchararray":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"List<Integer>"}}}, true
		case "indexof", "lastindexof":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{"String"}, {"String", "Integer"}}}, true
		case "equals":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Object"}}}, true
		case "equalsignorecase", "startswith", "startswithignorecase", "endswith", "endswithignorecase", "contains", "containsignorecase":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String"}}}, true
		case "compareto", "comparetoignorecase":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{"String"}}}, true
		case "adderror":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}, {"Exception"}, {"String", "Boolean"}, {"Exception", "Boolean"}, {"String", "String"}, {"String", "String", "Boolean"}, {"Schema.SObjectField", "String"}, {"Schema.SObjectField", "String", "Boolean"}}}, true
		case "isnumeric", "isalpha", "isalphanumeric", "isblank", "isnotblank", "isempty", "isnotempty":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}, {"String"}}}, true
		case "valueof":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Object"}, {"String"}, {"Boolean"}, {"Integer"}, {"Long"}, {"Decimal"}, {"Double"}, {"Date"}, {"Datetime"}, {"Time"}, {"Id"}}}, true
		case "length":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "replace", "replaceall", "replacefirst":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String"}}}, true
		case "split":
			return semaCollectionSignature{returnType: "List<String>", params: [][]string{{"String"}, {"String", "Integer"}}}, true
		case "substring", "substringafter", "substringafterlast", "substringbefore", "substringbeforelast", "removeend", "removeendignorecase", "removestart", "removestartignorecase", "left", "right":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Integer"}, {"Integer", "Integer"}, {"String"}}}, true
		case "leftpad", "rightpad":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Integer"}, {"Integer", "String"}}}, true
		case "trim", "normalizespace", "deletewhitespace":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "tolowercase", "touppercase":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}, {"String"}}}, true
		case "capitalize", "uncapitalize":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "escapehtml3", "escapehtml4", "unescapehtml3", "unescapehtml4", "escapexml", "unescapexml":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "dom.document":
		switch method {
		case "toxmlstring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getrootelement":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{}}}, true
		case "load":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "createrootelement":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{"String", "String", "String"}}}, true
		}
	case "dom.xmlnode":
		switch method {
		case "gettext", "getname", "getnamespace", "getprefix", "getcommenttext":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getprefixfor", "getnamespacefor":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String"}}}, true
		case "getattribute", "getattributevalue", "getattributevaluens":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String"}}}, true
		case "getparent":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{}}}, true
		case "getchildelement":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{"String", "String"}}}, true
		case "getchildren", "getchildelements":
			return semaCollectionSignature{returnType: "List<Dom.XmlNode>", params: [][]string{{}}}, true
		case "getnodetype":
			return semaCollectionSignature{returnType: "Dom.XmlNodeType", params: [][]string{{}}}, true
		case "getattributecount":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "getattributekeyat", "getattributekeynsat":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Integer"}}}, true
		case "setattribute":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		case "setattributens":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String", "String", "String"}}}, true
		case "setnamespace":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		case "removeattribute":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String", "String"}}}, true
		case "removechild":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"Object"}}}, true
		case "insertbefore":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{"Object", "Object"}}}, true
		case "addtextnode", "addcommentnode":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{"String"}}}, true
		case "addchildelement":
			return semaCollectionSignature{returnType: "Dom.XmlNode", params: [][]string{{"String", "String", "String"}}}, true
		}
	case "sobject":
		switch method {
		case "get":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "put":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"String", "Object"}, {"Schema.SObjectField", "Object"}}}, true
		case "putsobject":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "SObject"}, {"Schema.SObjectField", "SObject"}}}, true
		case "getsobject":
			return semaCollectionSignature{returnType: "SObject", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "getsobjects":
			return semaCollectionSignature{returnType: "List<SObject>", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "isset":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String"}, {"Schema.SObjectField"}}}, true
		case "clear":
			return semaCollectionSignature{returnType: "void", params: [][]string{{}}}, true
		case "getpopulatedfieldsasmap":
			return semaCollectionSignature{returnType: "Map<String,Object>", params: [][]string{{}}}, true
		case "getsobjecttype":
			return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
		case "clone":
			return semaCollectionSignature{returnType: "SObject", params: [][]string{{}, {"Boolean"}, {"Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean"}, {"Boolean", "Boolean", "Boolean", "Boolean"}}}, true
		case "adderror":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}, {"Exception"}, {"String", "Boolean"}, {"Exception", "Boolean"}, {"String", "String"}, {"String", "String", "Boolean"}, {"Schema.SObjectField", "String"}, {"Schema.SObjectField", "String", "Boolean"}}}, true
		case "haserrors":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "geterrors":
			return semaCollectionSignature{returnType: "List<Database.Error>", params: [][]string{{}}}, true
		case "setoptions":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Database.DMLOptions"}}}, true
		case "getoptions":
			return semaCollectionSignature{returnType: "Database.DMLOptions", params: [][]string{{}}}, true
		}
	case "schema.recordtypeinfo":
		switch method {
		case "getname", "getdevelopername":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getrecordtypeid":
			return semaCollectionSignature{returnType: "Id", params: [][]string{{}}}, true
		case "isavailable", "isdefaultrecordtypemapping", "isactive":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "schema.describefieldresult":
		switch method {
		case "getname", "getlabel", "getrelationshipname", "getdefaultvalueformula":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "gettype":
			return semaCollectionSignature{returnType: "Schema.DisplayType", params: [][]string{{}}}, true
		case "getsobjecttype":
			return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
		case "getsoaptype":
			return semaCollectionSignature{returnType: "Schema.SoapType", params: [][]string{{}}}, true
		case "isnillable", "isexternalid", "isunique", "isencrypted", "isnamefield", "isaccessible", "iscreateable", "isupdateable":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "getreferenceto":
			return semaCollectionSignature{returnType: "List<Schema.SObjectType>", params: [][]string{{}}}, true
		case "getpicklistvalues":
			return semaCollectionSignature{returnType: "List<Schema.PicklistEntry>", params: [][]string{{}}}, true
		case "getsobjectfield":
			return semaCollectionSignature{returnType: "Schema.SObjectField", params: [][]string{{}}}, true
		}
	case "schema.fieldset":
		switch method {
		case "getfields":
			return semaCollectionSignature{returnType: "List<Schema.FieldSetMember>", params: [][]string{{}}}, true
		case "getlabel":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "schema.sobjecttypefieldsets":
		switch method {
		case "get":
			return semaCollectionSignature{returnType: "Schema.FieldSet", params: [][]string{{"String"}}}, true
		case "getmap":
			return semaCollectionSignature{returnType: "Map<String,Schema.FieldSet>", params: [][]string{{}}}, true
		}
	case "schema.fieldsetmember":
		switch method {
		case "getfieldpath", "getlabel":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getrequired", "getdbrequired":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "gettype":
			return semaCollectionSignature{returnType: "Schema.DisplayType", params: [][]string{{}}}, true
		}
	case "schema.sobjecttype":
		switch method {
		case "getdescribe":
			return semaCollectionSignature{returnType: "Schema.DescribeSObjectResult", params: [][]string{{}, {"SObjectDescribeOptions"}}}, true
		case "newsobject":
			return semaCollectionSignature{returnType: "SObject", params: [][]string{{}, {"Id"}, {"Id", "Boolean"}}}, true
		case "getrecordtypeinfosbyname":
			return semaCollectionSignature{returnType: "Map<String,Schema.RecordTypeInfo>", params: [][]string{{}}}, true
		case "getrecordtypeinfosbyid":
			return semaCollectionSignature{returnType: "Map<Id,Schema.RecordTypeInfo>", params: [][]string{{}}}, true
		}
	case "schema.describesobjectresult", "describesobjectresult":
		switch method {
		case "getname", "getlabel", "getlabelplural", "getkeyprefix":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getfields":
			return semaCollectionSignature{returnType: "Schema.SObjectTypeFields", params: [][]string{{}}}, true
		case "getfieldsets":
			return semaCollectionSignature{returnType: "Schema.SObjectTypeFieldSets", params: [][]string{{}}}, true
		case "getrecordtypeinfos":
			return semaCollectionSignature{returnType: "List<Schema.RecordTypeInfo>", params: [][]string{{}}}, true
		case "getrecordtypeinfosbyname", "getrecordtypeinfosbydevelopername":
			return semaCollectionSignature{returnType: "Map<String,Schema.RecordTypeInfo>", params: [][]string{{}}}, true
		case "getrecordtypeinfosbyid":
			return semaCollectionSignature{returnType: "Map<Id,Schema.RecordTypeInfo>", params: [][]string{{}}}, true
		case "getchildrelationships":
			return semaCollectionSignature{returnType: "List<Schema.ChildRelationship>", params: [][]string{{}}}, true
		case "getsobjecttype":
			return semaCollectionSignature{returnType: "Schema.SObjectType", params: [][]string{{}}}, true
		case "isaccessible", "iscreateable", "isupdateable", "isdeletable", "isqueryable", "issearchable":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "schema.sobjecttypefields", "schema.sobjectfields":
		switch method {
		case "getmap":
			return semaCollectionSignature{returnType: "Map<String,Schema.SObjectField>", params: [][]string{{}}}, true
		case "get":
			return semaCollectionSignature{returnType: "Schema.SObjectField", params: [][]string{{"String"}}}, true
		}
	case "schema.sobjectfield":
		switch method {
		case "getdescribe":
			return semaCollectionSignature{returnType: "Schema.DescribeFieldResult", params: [][]string{{}}}, true
		case "adderror":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}, {"Exception"}, {"String", "Boolean"}, {"Exception", "Boolean"}, {"Schema.SObjectField", "String"}, {"Schema.SObjectField", "String", "Boolean"}}}, true
		}
	case "auth.communitiesutil":
		if method == "isguestuser" {
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "auth.authtoken":
		if method == "revokeaccess" {
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String", "String", "String"}}}, true
		}
	case "auth.sessionmanagement":
		if method == "getcurrentsession" {
			return semaCollectionSignature{returnType: "Map<String,String>", params: [][]string{{}}}, true
		}
	case "auth.authconfiguration":
		switch method {
		case "getauthproviders":
			return semaCollectionSignature{returnType: "List<AuthProvider>", params: [][]string{{}}}, true
		case "getauthconfig":
			return semaCollectionSignature{returnType: "Auth.AuthConfig", params: [][]string{{}}}, true
		case "getstarturl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getauthproviderssourl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String", "String", "String"}}}, true
		}
	case "auth.jwt":
		switch method {
		case "setiss":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "tojsonstring":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		}
	case "messaging":
		switch method {
		case "sendemail":
			return semaCollectionSignature{returnType: "List<Messaging.SendEmailResult>", params: [][]string{{"List<Messaging.Email>"}, {"List<Messaging.Email>", "Boolean"}}}, true
		}
	case "messaging.singleemailmessage":
		switch method {
		case "setwhatid", "settargetobjectid", "setorgwideemailaddressid", "settemplateid":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Id"}}}, true
		case "setplaintextbody", "sethtmlbody", "setsubject", "setreplyto", "setsenderdisplayname", "setcharset":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "settoaddresses", "setccaddresses", "setbccaddresses":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"List<String>"}, {"String[]"}}}, true
		case "setsaveasactivity", "settreattargetobjectasrecipient", "setusesignature":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Boolean"}}}, true
		}
	case "site":
		switch method {
		case "getsiteid", "getbaseurl", "getpathprefix", "getadminemail", "getadminid", "getmasterlabel", "geterrormessage", "geterrordescription":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "isregistrationenabled", "isloginenabled":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "isvalidusername":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{"String"}}}, true
		case "setexperienceid":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "forgotpassword":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}}}, true
		case "login":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{"String", "String", "String"}}}, true
		case "changepassword":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{"String", "String"}, {"String", "String", "String"}}}, true
		case "validatepassword":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"User", "String", "String"}}}, true
		case "createexternaluser":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"User", "String", "String"}, {"User", "String", "String", "Boolean"}}}, true
		case "createportaluser":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"User", "String", "String"}}}, true
		}
	case "system":
		switch method {
		case "now":
			return semaCollectionSignature{returnType: "Datetime", params: [][]string{{}}}, true
		case "setpassword":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		case "currentpagereference":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{}}}, true
		}
	case "usermanagement":
		switch method {
		case "initselfregistration":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"Auth.VerificationMethod", "User"}}}, true
		case "verifyselfregistration":
			return semaCollectionSignature{returnType: "Auth.VerificationResult", params: [][]string{{"Auth.VerificationMethod", "String", "String", "String"}}}, true
		}
	case "network":
		switch method {
		case "getnetworkid":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getloginurl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String"}}}, true
		case "communitieslanding":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{}}}, true
		}
	case "test":
		switch method {
		case "isrunningtest":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		case "createstub":
			return semaCollectionSignature{returnType: "Object", params: [][]string{{"Type", "StubProvider"}}}, true
		case "calculatepermissionsetgroup":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String"}, {"Id"}, {"Object"}, {"List<String>"}}}, true
		case "setmock":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "Object"}, {"Type", "Object"}}}, true
		case "setcurrentpagereference", "setcurrentpage":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"PageReference"}}}, true
		}
	case "apexpages":
		switch method {
		case "currentpage":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{}}}, true
		case "addmessage":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"ApexPages.Message"}}}, true
		case "hasmessages":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}, {"ApexPages.Severity"}}}, true
		}
	case "webservicecallout":
		if method == "invoke" {
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Object", "Object", "Map<String,Object>", "List<String>"}}}, true
		}
	case "pagereference":
		switch method {
		case "getparameters":
			return semaCollectionSignature{returnType: "Map<String,String>", params: [][]string{{}}}, true
		case "geturl":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "setredirect":
			return semaCollectionSignature{returnType: "PageReference", params: [][]string{{"Boolean"}}}, true
		case "getredirect":
			return semaCollectionSignature{returnType: "Boolean", params: [][]string{{}}}, true
		}
	case "http":
		if method == "send" {
			return semaCollectionSignature{returnType: "HttpResponse", params: [][]string{{"HttpRequest"}}}, true
		}
	case "httpresponse":
		switch method {
		case "getbody":
			return semaCollectionSignature{returnType: "String", params: [][]string{{}}}, true
		case "getstatuscode":
			return semaCollectionSignature{returnType: "Integer", params: [][]string{{}}}, true
		case "getheader":
			return semaCollectionSignature{returnType: "String", params: [][]string{{"String"}}}, true
		}
	case "multistaticresourcecalloutmock":
		switch method {
		case "setstaticresource":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		case "setstatuscode":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"Integer"}}}, true
		case "setheader":
			return semaCollectionSignature{returnType: "void", params: [][]string{{"String", "String"}}}, true
		}
	}
	return semaCollectionSignature{}, false
}

func semaDescribeSObjectResultChildRelationships(receiverType, method string) bool {
	receiverType = semaCanonicalPlatformAlias(receiverType)
	return (strings.EqualFold(receiverType, "Schema.DescribeSObjectResult") || strings.EqualFold(receiverType, "DescribeSObjectResult")) &&
		normalizeName(method) == "getchildrelationships"
}

func semaObjectMethodName(method string) bool {
	switch normalizeName(method) {
	case "clone", "hashcode", "tostring", "equals":
		return true
	default:
		return false
	}
}
