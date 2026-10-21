package sema

import "strings"

// The owned source API 62 pagination observations establish these callable
// signatures independently of their query text. Other source profiles and
// overloads retain the generated availability ranges.
func observedPaginationSourceCallCandidate(version, receiver, method, mode string) bool {
	if version != "62.0" {
		return false
	}
	if strings.EqualFold(method, "getPaginationCursorWithBinds") && mode == "class" {
		return strings.EqualFold(receiver, "Database") || strings.EqualFold(receiver, "System.Database")
	}
	if mode != "instance" {
		return false
	}
	return ((strings.EqualFold(method, "getNumRecords") || strings.EqualFold(method, "fetchPage")) &&
		strings.EqualFold(semaCanonicalPlatformAlias(receiver), "Database.PaginationCursor")) ||
		(strings.EqualFold(method, "getRecords") && strings.EqualFold(semaCanonicalPlatformAlias(receiver), "Database.CursorFetchResult"))
}

func observedPaginationSourceCall(version, receiver, method string, argTypes []string, mode string, model *semaTypeMemberView) bool {
	if !observedPaginationSourceCallCandidate(version, receiver, method, mode) {
		return false
	}
	var surfaces []string
	if strings.EqualFold(method, "getPaginationCursorWithBinds") {
		if len(argTypes) != 3 || !strings.EqualFold(semaCanonicalPlatformAlias(argTypes[0]), "String") ||
			!strings.EqualFold(semaCanonicalPlatformAlias(argTypes[2]), "AccessLevel") ||
			semaProjectTypeShadowsPlatform(model, argTypes[2]) {
			return false
		}
		base, parameters := semaGenericBaseAndArgs(semaCanonicalPlatformAlias(argTypes[1]))
		if !strings.EqualFold(base, "Map") || len(parameters) != 2 ||
			!strings.EqualFold(parameters[0], "String") || !strings.EqualFold(parameters[1], "Object") {
			return false
		}
		// The generated catalog represents the AccessLevel parameter as Object;
		// the typed platform view also exposes the AccessLevel signature.
		for _, owner := range []string{"database", "system.database"} {
			surfaces = append(surfaces, "apex:"+owner,
				"apex:"+owner+".getpaginationcursorwithbinds(string,map,object)",
				"apex:"+owner+".getpaginationcursorwithbinds(string,map,accesslevel)")
		}
	} else if strings.EqualFold(method, "fetchPage") {
		if len(argTypes) != 2 || !strings.EqualFold(semaCanonicalPlatformAlias(argTypes[0]), "Integer") ||
			!strings.EqualFold(semaCanonicalPlatformAlias(argTypes[1]), "Integer") {
			return false
		}
		surfaces = []string{"apex:database.paginationcursor", "apex:database.paginationcursor.fetchpage(integer,integer)"}
	} else if strings.EqualFold(method, "getRecords") {
		if len(argTypes) != 0 {
			return false
		}
		// The captured chained fetch infers this receiver type. This call-site
		// exception does not admit explicit type references or other members.
		surfaces = []string{"apex:database.cursorfetchresult", "apex:database.cursorfetchresult.getrecords()"}
	} else {
		if len(argTypes) != 0 {
			return false
		}
		surfaces = []string{"apex:database.paginationcursor", "apex:database.paginationcursor.getnumrecords()"}
	}
	for _, surface := range surfaces {
		if semaPlatformExplicitlyUnsupported(surface) {
			return false
		}
	}
	return true
}
