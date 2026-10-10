package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
	"github.com/glade-sh/glade/internal/storage"
)

// These are authoritative Salesforce API 67 negative contracts. Generated
// symbols cannot express every stale alias or member, and legacy namespace and
// runtime fallbacks would otherwise admit those names. Keep the boundary
// centralized and data-like; callers must still preserve project-defined types
// that shadow platform names.
var semaAPI67PatternFlags = map[string]struct{}{
	"case_insensitive":        {},
	"comments":                {},
	"multiline":               {},
	"literal":                 {},
	"dotall":                  {},
	"unicode_case":            {},
	"unix_lines":              {},
	"canon_eq":                {},
	"unicode_character_class": {},
}

var semaAPI67ReadOnlyPlatformFields = map[string]struct{}{
	// The maps can be mutated, but cannot be replaced.
	"restrequest.headers":                       {},
	"restrequest.params":                        {},
	"messaging.emailfileattachment.id":          {},
	"messaging.singleemailmessage.templatename": {},
	"messaging.singleemailmessage.usermail":     {},
}

func semaAPI67ReadOnlyPlatformField(path string) bool {
	path = strings.TrimSpace(path)
	dot := strings.LastIndexByte(path, '.')
	if dot <= 0 || dot >= len(path)-1 {
		return false
	}
	receiver := semaCanonicalPlatformAlias(strings.TrimSpace(path[:dot]))
	field := normalizeName(path[dot+1:])
	key := normalizeName(receiver + "." + field)
	if _, readOnly := semaAPI67ReadOnlyPlatformFields[key]; readOnly {
		return true
	}
	_, readOnly := semaAPI67ReadOnlyPlatformFields["messaging."+key]
	return readOnly
}

var semaAPI67RejectedPlatformConstructors = map[string]struct{}{
	// At API 62 and 67, CacheBuilder is an interface.
	"cache.cachebuilder": {},
	// These describe types reject construction at API 62 and 67.
	"schema.fieldset":              {},
	"schema.describesobjectresult": {},
	"schema.describefieldresult":   {},
	// Apex exposes these names as scalar value types, but Salesforce rejects
	// `new` construction for them. Keep the local compiler aligned with the
	// platform's "Type cannot be constructed" contract; use the documented
	// static factories (for example Date.today or Decimal.valueOf) instead.
	"date":                             {},
	"datetime":                         {},
	"decimal":                          {},
	"double":                           {},
	"approval":                         {},
	"messaging.actionablenotification": {},
	"messaging.actionresult":           {},
	"messaging.sendemailerror":         {},
	"messaging.sendemailresult":        {},
	"queueableduplicatesignature":      {},
	"site.urlrewriter":                 {},
	"visualeditor.dynamicpicklist":     {},
	"webservicecalloutfuture":          {},
	"aura":                             {},
	"flexqueue":                        {},
	"resetpasswordresult":              {},
	"system":                           {},
	"webservicecallout":                {},
	"txnsecurity.eventcondition":       {},
	"txnsecurity.policycondition":      {},
}

func semaAPI67RejectedPlatformConstructor(typeName string) bool {
	typeName = semaCanonicalPlatformAlias(strings.TrimSpace(typeName))
	// Compression C006: entries are obtained from a writer or reader.
	if strings.EqualFold(typeName, "compression.ZipEntry") {
		return true
	}
	_, rejected := semaAPI67RejectedPlatformConstructors[normalizeName(typeName)]
	return rejected
}

// semaAPI67RejectedPlatformType identifies names that are present only through
// permissive namespace or runtime fallbacks, not through the Salesforce API.
func semaAPI67RejectedPlatformType(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return false
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		for _, arg := range args {
			if semaAPI67RejectedPlatformType(arg) {
				return true
			}
		}
		typeName = base
	}
	normalized := normalizeName(typeName)
	switch normalized {
	// At API 62/67,
	// generated provider shapes are not visible to user Apex in the captured cases.
	case "commercetax.taxenginecontext", "functions.function":
		return true
	case "messaging.sendemailoptions",
		"system.messaging.sendemailoptions",
		"system.messaging.singleemailmessage",
		// Salesforce exposes these as the return shape of DescribeSObjectResult
		// members, but API 67 does not allow either name in user Apex type
		// declarations. Keep the synthetic return type usable for member access
		// while rejecting explicit local declarations.
		"schema.sobjecttypefieldsets",
		"schema.fieldsetmap",
		"database.allowcallouts",
		"system.database.allowcallouts",
		"database.lockresult",
		"database.unlockresult",
		"system.database.lockresult",
		"system.database.unlockresult",
		"canvas.lifecyclehandler",
		"system.pushupgradecustomizationrepository":
		return true
	default:
		return false
	}
}

// semaAPI67RejectedPlatformCall covers method names that must not be admitted
// by the platform fallback after generated symbol lookup finds no canonical
// Salesforce member. receiverType may be generic or explicitly qualified.
func semaAPI67RejectedPlatformCall(receiverType, method, receiverMode string) bool {
	return semaAPI67RejectedPlatformCallAtVersion("67.0", receiverType, method, receiverMode)
}

func semaAPI67RejectedPlatformCallAtVersion(version, receiverType, method, receiverMode string) bool {
	receiverType = strings.TrimSpace(receiverType)
	method = normalizeName(method)
	if strings.EqualFold(receiverType, "System.PushUpgradeCustomizationRepository") {
		return true
	}
	receiverType = semaCanonicalPlatformAlias(receiverType)
	base, _ := semaGenericBaseAndArgs(receiverType)
	base = normalizeName(base)
	switch base {
	case "httprequest":
		return method == "getheaderkeys" || method == "gettimeout"
	case "restrequest":
		return method == "getheader" || method == "getparameter"
	case "schema.describetabsetresult":
		return method == "getname"
	case "schema.recordtypeinfo":
		return method == "getnamespace"
	case "location":
		// API62/API67 T001: the coordinate surface has no altitude getter.
		// Reject it before permissive fluent/dependency call fallbacks.
		return method == "getaltitude"
	case "compression.zipwriter":
		return method == "getentriesmap" // Compression C020.
	case "compression.zipentry":
		return method == "setname" // Compression C023.
	case "url":
		return method == "getsalesforcebaseurl" && apexversion.AtLeast(version, 59)
	case "dataweave.result":
		return method == "getmimetype"
	case "date":
		// Native C006 (API62/67) rejects Date.toEndOfMonth.
		return method == "toendofmonth"
	case "id":
		return method == "to18"
	case "integer":
		return method == "doublevalue" || method == "intvalue" || method == "longvalue" || method == "decimalvalue"
	case "map":
		return method == "containsvalue"
	case "string":
		switch method {
		case "commonprefix", "escapexml10", "escapexml11", "lastindexofany", "lastordinalindexof", "ordinalindexof", "removeignorecase", "replaceignorecase", "replaceonce", "rotate", "strip", "stripall", "stripend", "stripstart", "striptoempty", "striptonull", "unescapexml10", "unescapexml11":
			return true
		}
		return false
	case "iterator":
		return method == "remove"
	case "matcher":
		return method == "appendreplacement" || method == "appendtail"
	case "asyncoptions":
		return method == "getminimumqueueabledelayinminutes"
	case "database":
		return method == "lock" || method == "unlock"
	case "quickaction":
		return method == "describeavailableactions"
	case "schema.sobjecttypefieldsets":
		return method == "get" && apexversion.AtLeast(version, 67)
	case "canvas.environmentcontext":
		return method == "getparameters" || (method == "getparametersasjson" && receiverMode == "class")
	case "canvas.lifecyclehandler":
		return true
	case "connectapi":
		if receiverMode != "class" {
			return false
		}
		switch method {
		case "geterror", "geterrormessage", "geterrortypename", "getresult", "issuccess":
			return true
		}
	case "site":
		// These legacy Site URL helpers were removed from Salesforce after
		// API version 29.0. Keep the generated legacy shape for evidence and
		// versioned catalogs, but reject calls in the current API boundary.
		switch method {
		case "getcurrentsiteurl", "getcustomwebaddress", "getprefix":
			return !apexversion.Enabled(version, apexversion.LegacySiteURLHelpers)
		}
	case "auth.authconfiguration":
		return method == "getrightframeurl"
	case "cache.org":
		// Cache.Org lost isAvailable, and the value-size stat methods were
		// removed after API version 49.0. Keep the generated legacy shapes for
		// evidence and versioned catalogs, but reject calls in the current API
		// boundary.
		switch method {
		case "isavailable", "getavgvaluesize", "getmaxvaluesize":
			return method == "isavailable" || !apexversion.Enabled(version, apexversion.LegacyCacheValueSize)
		}
	case "cache.session":
		switch method {
		case "getavgvaluesize", "getmaxvaluesize":
			return !apexversion.Enabled(version, apexversion.LegacyCacheValueSize)
		}
	case "cache.partition", "cache.orgpartition", "cache.sessionpartition":
		switch method {
		case "getavgvaluesize", "getmaxvaluesize":
			return !apexversion.Enabled(version, apexversion.LegacyCacheValueSize)
		case "createfullyqualifiedkey", "createfullyqualifiedpartition",
			"validatepartitionname", "validatekey", "validatekeyvalue":
			// Salesforce declares these partition helpers static; calling
			// them through an instance is a compile error in API 67.
			return receiverMode == "instance"
		}
	case "auth.authproviderpluginclass":
		switch method {
		case "getcustommetadatatype", "getuserinfo", "initiate":
			return true
		}
	case "auth.connectedappplugin":
		return method == "customattributes"
	}
	return false
}

func semaAPI67RejectedPlatformCallArgs(version, receiverType, method string, argTypes []string) bool {
	receiverBase, _ := semaGenericBaseAndArgs(semaCanonicalPlatformAlias(receiverType))
	// The Org List<String> get/remove overloads were
	// removed after API 54. Keep the Set overloads and legacy sources intact.
	if strings.EqualFold(receiverBase, "Cache.Org") && (strings.EqualFold(method, "get") || strings.EqualFold(method, "remove")) && len(argTypes) == 1 && !apexversion.Enabled(version, apexversion.LegacyCacheValidateKeys) {
		base, args := semaGenericBaseAndArgs(argTypes[0])
		if strings.EqualFold(base, "List") && len(args) == 1 && strings.EqualFold(semaCanonicalPlatformAlias(args[0]), "String") {
			return true
		}
	}
	// Permissive platform fallback must not accept send(String).
	// Keep null and unresolved expressions available to normal overload checks.
	if strings.EqualFold(receiverBase, "Http") && strings.EqualFold(method, "send") && len(argTypes) == 1 {
		argType := semaCanonicalPlatformAlias(argTypes[0])
		return argType != "" && !strings.EqualFold(argType, "null") && !strings.EqualFold(argType, "HttpRequest")
	}
	if apexversion.AtLeast(version, 67) && strings.EqualFold(receiverBase, "Database") && strings.EqualFold(method, "emptyRecycleBin") && len(argTypes) == 1 {
		return strings.EqualFold(semaCanonicalAssignableType(argTypes[0]), "Id")
	}
	if apexversion.AtLeast(version, 67) && strings.EqualFold(receiverBase, "Database") && strings.EqualFold(method, "update") && len(argTypes) == 2 {
		return strings.EqualFold(semaCanonicalAssignableType(argTypes[0]), "Object") && strings.EqualFold(semaCanonicalAssignableType(argTypes[1]), "Boolean")
	}
	if (strings.EqualFold(receiverBase, "Cache.Partition") || strings.EqualFold(receiverBase, "Cache.OrgPartition") || strings.EqualFold(receiverBase, "Cache.SessionPartition")) && strings.EqualFold(method, "validateKeys") {
		if len(argTypes) != 2 {
			return true
		}
		base, args := semaGenericBaseAndArgs(argTypes[1])
		if len(args) != 1 || !strings.EqualFold(semaCanonicalPlatformAlias(args[0]), "String") {
			return true
		}
		return !strings.EqualFold(base, "Set") && (!strings.EqualFold(base, "List") || !apexversion.Enabled(version, apexversion.LegacyCacheValidateKeys))
	}
	if strings.EqualFold(receiverType, "String") && strings.EqualFold(method, "join") {
		if len(argTypes) != 2 {
			return true
		}
		base, _ := semaGenericBaseAndArgs(argTypes[0])
		return !strings.EqualFold(base, "List") && !strings.EqualFold(base, "Set") && !strings.EqualFold(base, "Iterable")
	}
	if !strings.EqualFold(method, "pow") && !strings.EqualFold(method, "valueOf") {
		return false
	}
	if !strings.EqualFold(receiverType, "Math") && !strings.EqualFold(receiverType, "Decimal") {
		return false
	}
	for _, argType := range argTypes {
		if strings.EqualFold(argType, "Decimal") {
			return true
		}
	}
	return false
}

func semaAPI67RejectedPlatformField(path string) bool {
	path = strings.TrimSpace(path)
	dot := strings.LastIndexByte(path, '.')
	if dot <= 0 || dot >= len(path)-1 {
		return false
	}
	receiver := strings.TrimSpace(path[:dot])
	field := normalizeName(path[dot+1:])
	// Compression C003/C004 and C012/C013 define the complete enum surface.
	if strings.EqualFold(receiver, "compression.Level") {
		return field != "default_level" && field != "no_compression" && field != "best_speed" && field != "best_compression"
	}
	if strings.EqualFold(receiver, "compression.Method") {
		return field != "stored" && field != "deflated"
	}
	if strings.EqualFold(semaCanonicalPlatformAlias(receiver), "UninstallContext") {
		return field == "organizationid"
	}
	if strings.EqualFold(receiver, "Database.AllowCallouts") || strings.EqualFold(receiver, "System.Database.AllowCallouts") {
		return true
	}
	if strings.EqualFold(receiver, "Continuation") || strings.EqualFold(receiver, "System.Continuation") {
		return field == "state"
	}
	if strings.EqualFold(receiver, "System.Pattern") {
		receiver = "Pattern"
	}
	if !strings.EqualFold(receiver, "Pattern") {
		return false
	}
	_, rejected := semaAPI67PatternFlags[field]
	return rejected
}

// These two Result members were rejected by exact API65 Salesforce controls.
// Keep the supported getValue/getValueAsString carrier shape open to its
// existing type and overload checks.
func semaRejectedDataWeaveResultField(receiver, field string) bool {
	return strings.EqualFold(semaCanonicalPlatformAlias(receiver), "DataWeave.Result") && strings.EqualFold(field, "valueAsString")
}

func semaStandardFieldAssignmentReadOnly(model *semaTypeMemberView, receiver, field string) bool {
	// Location's readonly coordinates belong to the System class. Resolve the
	// receiver through the member model so Schema.Location and project classes
	// retain their own field rules, including when a member owner is Location.
	if strings.EqualFold(semaCanonicalPlatformAlias(receiver), "Location") &&
		(strings.EqualFold(field, "latitude") || strings.EqualFold(field, "longitude")) {
		// API62/API67 X001-X002 also reject the unqualified System type.
		// A standard Schema.Location entry can mask its platform members.
		if strings.EqualFold(receiver, "Location") && !semaProjectTypeShadowsPlatform(model, receiver) {
			return true
		}
		if members, _, found := semaLookupTypeMembers(model, receiver); found && members.platform && !members.sobject {
			return true
		}
	}
	// Address's getter-equivalent city property belongs to the System class.
	if strings.EqualFold(semaCanonicalPlatformAlias(receiver), "Address") && strings.EqualFold(field, "city") {
		if members, _, found := semaLookupTypeMembers(model, receiver); found && members.platform && !members.sobject {
			return true
		}
	}
	if members, found := model.lookup(normalizeName(receiver)); found && !members.sobject && !members.platform {
		return false
	}
	return storage.StandardFieldAssignmentReadOnly(receiver, field)
}
