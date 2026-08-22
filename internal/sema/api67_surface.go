package sema

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexversion"
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
	case "id":
		return method == "to18"
	case "integer":
		return method == "doublevalue"
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
