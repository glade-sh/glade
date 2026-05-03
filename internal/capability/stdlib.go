package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
)

type StdlibEntry struct {
	Area   string `json:"area"`
	API    string `json:"api"`
	Status Status `json:"status"`
	Notes  string `json:"notes,omitempty"`
}

func StdlibMatrix() []StdlibEntry {
	entries := append([]StdlibEntry(nil), stdlibEntries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Area == entries[j].Area {
			return entries[i].API < entries[j].API
		}
		return entries[i].Area < entries[j].Area
	})
	return entries
}

func WriteStdlibJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(StdlibMatrix())
}

func WriteStdlibMarkdown(w io.Writer) error {
	if _, err := fmt.Fprintln(w, "# Standard Library Coverage"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\nGenerated from `internal/capability`."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\nStatus values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`."); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\n| Area | API | Status | Notes |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, entry := range StdlibMatrix() {
		if _, err := fmt.Fprintf(w, "| %s | `%s` | `%s` | %s |\n", entry.Area, entry.API, entry.Status, entry.Notes); err != nil {
			return err
		}
	}
	return nil
}

var stdlibEntries = []StdlibEntry{
	{Area: "ApexPages", API: "ApexPages.addMessage", Status: StatusSupported, Notes: "Stores page messages on the VM instance."},
	{Area: "ApexPages", API: "ApexPages.currentPage", Status: StatusSupported, Notes: "Returns a deterministic local PageReference."},
	{Area: "ApexPages", API: "ApexPages.getMessages", Status: StatusSupported, Notes: "Returns VM-local page messages."},
	{Area: "ApexPages", API: "ApexPages.hasMessages", Status: StatusSupported, Notes: "Checks VM-local page messages."},
	{Area: "ApexPages", API: "ApexPages.Message", Status: StatusPartial, Notes: "Constructor and getters; no Visualforce rendering lifecycle."},
	{Area: "Blob", API: "Blob.size", Status: StatusSupported, Notes: "Returns local Blob byte length."},
	{Area: "Blob", API: "Blob.toString", Status: StatusSupported, Notes: "Returns the local Blob bytes as a string."},
	{Area: "Blob", API: "Blob.valueOf", Status: StatusSupported, Notes: "Stores the string bytes in a local Blob value."},
	{Area: "Crypto", API: "Crypto.generateDigest", Status: StatusSupported, Notes: "MD5, SHA1, SHA-256, SHA-512, and SHA3-256/384/512."},
	{Area: "Crypto", API: "Crypto.generateMac", Status: StatusSupported, Notes: "HMAC MD5, SHA1, SHA256, and SHA512 with local Blob keys."},
	{Area: "Database", API: "Database.delete", Status: StatusSupported, Notes: "DML pipeline with result/error shapes for supported SObjects."},
	{Area: "Database", API: "Database.getQueryLocator", Status: StatusPartial, Notes: "Supported SOQL only; executes eagerly for local batch scopes."},
	{Area: "Database", API: "Database.insert", Status: StatusSupported, Notes: "DML pipeline with result/error shapes for supported SObjects."},
	{Area: "Database", API: "Database.merge", Status: StatusPartial, Notes: "Local merge behavior for supported account/contact-style data."},
	{Area: "Database", API: "Database.rollback", Status: StatusPartial, Notes: "Local org-state savepoint rollback; no external side effects."},
	{Area: "Database", API: "Database.setSavepoint", Status: StatusPartial, Notes: "Local org-state snapshots with later-savepoint invalidation."},
	{Area: "Database", API: "Database.undelete", Status: StatusPartial, Notes: "Soft-delete restoration for supported local records."},
	{Area: "Database", API: "Database.update", Status: StatusSupported, Notes: "DML pipeline with result/error shapes for supported SObjects."},
	{Area: "Database", API: "Database.upsert", Status: StatusPartial, Notes: "Schema-backed external-ID matching for supported local records."},
	{Area: "Date", API: "Date.addDays", Status: StatusSupported, Notes: "Local Gregorian date arithmetic."},
	{Area: "Date", API: "Date.addMonths", Status: StatusPartial, Notes: "Local Gregorian arithmetic with month-end clamp; full Salesforce edge-case parity incomplete."},
	{Area: "Date", API: "Date.addYears", Status: StatusPartial, Notes: "Local Gregorian arithmetic with leap-day clamp; full Salesforce edge-case parity incomplete."},
	{Area: "Date", API: "Date.day", Status: StatusSupported, Notes: "Returns Gregorian day of month."},
	{Area: "Date", API: "Date.daysBetween", Status: StatusSupported, Notes: "Returns whole calendar days between local Date values."},
	{Area: "Date", API: "Date.month", Status: StatusSupported, Notes: "Returns Gregorian month number."},
	{Area: "Date", API: "Date.newInstance", Status: StatusSupported, Notes: "Validates date parts."},
	{Area: "Date", API: "Date.toEndOfMonth", Status: StatusSupported, Notes: "Returns last day of the Date value's month."},
	{Area: "Date", API: "Date.toStartOfMonth", Status: StatusSupported, Notes: "Returns first day of the Date value's month."},
	{Area: "Date", API: "Date.valueOf", Status: StatusSupported, Notes: "Parses supported date strings."},
	{Area: "Date", API: "Date.year", Status: StatusSupported, Notes: "Returns Gregorian year."},
	{Area: "Datetime", API: "Datetime.addDays", Status: StatusPartial, Notes: "UTC-local arithmetic; no user timezone or DST model."},
	{Area: "Datetime", API: "Datetime.addHours", Status: StatusPartial, Notes: "UTC-local arithmetic."},
	{Area: "Datetime", API: "Datetime.addMinutes", Status: StatusPartial, Notes: "UTC-local arithmetic."},
	{Area: "Datetime", API: "Datetime.addMonths", Status: StatusPartial, Notes: "UTC-local arithmetic with month-end clamp; no user timezone or DST model."},
	{Area: "Datetime", API: "Datetime.addSeconds", Status: StatusPartial, Notes: "UTC-local arithmetic."},
	{Area: "Datetime", API: "Datetime.addYears", Status: StatusPartial, Notes: "UTC-local arithmetic with leap-day clamp; no user timezone or DST model."},
	{Area: "Datetime", API: "Datetime.date", Status: StatusPartial, Notes: "Returns UTC-modeled Date component; no user timezone model."},
	{Area: "Datetime", API: "Datetime.day", Status: StatusPartial, Notes: "UTC-modeled component getter."},
	{Area: "Datetime", API: "Datetime.hour", Status: StatusPartial, Notes: "UTC-modeled component getter."},
	{Area: "Datetime", API: "Datetime.minute", Status: StatusPartial, Notes: "UTC-modeled component getter."},
	{Area: "Datetime", API: "Datetime.month", Status: StatusPartial, Notes: "UTC-modeled component getter."},
	{Area: "Datetime", API: "Datetime.newInstance", Status: StatusSupported, Notes: "Validates date and time parts."},
	{Area: "Datetime", API: "Datetime.second", Status: StatusPartial, Notes: "UTC-modeled component getter."},
	{Area: "Datetime", API: "Datetime.valueOf", Status: StatusSupported, Notes: "Parses supported datetime strings."},
	{Area: "Datetime", API: "Datetime.year", Status: StatusPartial, Notes: "UTC-modeled component getter."},
	{Area: "Decimal", API: "Decimal.doubleValue", Status: StatusSupported, Notes: "Returns local Decimal value."},
	{Area: "Decimal", API: "Decimal.intValue", Status: StatusSupported, Notes: "Truncates to integer."},
	{Area: "Decimal", API: "Decimal.round", Status: StatusPartial, Notes: "Uses Go round-half-away behavior."},
	{Area: "Decimal", API: "Decimal.setScale", Status: StatusPartial, Notes: "Non-negative scale only; advanced rounding modes not modeled."},
	{Area: "EncodingUtil", API: "EncodingUtil.base64Decode", Status: StatusSupported, Notes: "Blob-shaped local value."},
	{Area: "EncodingUtil", API: "EncodingUtil.base64Encode", Status: StatusSupported, Notes: "Blob-shaped local value."},
	{Area: "EncodingUtil", API: "EncodingUtil.convertFromHex", Status: StatusSupported, Notes: "Blob-shaped local value."},
	{Area: "EncodingUtil", API: "EncodingUtil.convertToHex", Status: StatusSupported, Notes: "Blob-shaped local value."},
	{Area: "EncodingUtil", API: "EncodingUtil.urlDecode", Status: StatusPartial, Notes: "Uses query unescape; charset validation is not modeled."},
	{Area: "EncodingUtil", API: "EncodingUtil.urlEncode", Status: StatusPartial, Notes: "Uses query escape; charset validation is not modeled."},
	{Area: "FeatureManagement", API: "FeatureManagement.checkPermission", Status: StatusPartial, Notes: "Checks local runAs permission-list state."},
	{Area: "HTTP", API: "Http.send", Status: StatusPartial, Notes: "Mock-first local callouts; real network transport unsupported."},
	{Area: "HTTP", API: "HttpRequest", Status: StatusPartial, Notes: "Endpoint, method, headers, timeout, body, and blob body accessors."},
	{Area: "HTTP", API: "HttpResponse", Status: StatusPartial, Notes: "Status, status code, headers, body, and blob body accessors."},
	{Area: "JSON", API: "JSON.createGenerator", Status: StatusSupported, Notes: "Creates deterministic local JSONGenerator instances."},
	{Area: "JSON", API: "JSON.createParser", Status: StatusPartial, Notes: "Creates deterministic local JSONParser token streams for valid JSON strings."},
	{Area: "JSON", API: "JSON.deserialize", Status: StatusPartial, Notes: "SObject, class, collection, and primitive shapes for supported subset."},
	{Area: "JSON", API: "JSON.deserializeStrict", Status: StatusPartial, Notes: "Rejects unknown fields for supported schema/class targets."},
	{Area: "JSON", API: "JSON.deserializeUntyped", Status: StatusPartial, Notes: "Maps JSON into local primitive/list/map values."},
	{Area: "JSON", API: "JSONGenerator", Status: StatusPartial, Notes: "Object/array boundaries, field names, scalar string/number/Boolean/null, Date/Datetime/Time/Id/Blob, Object writers, getAsString, close, isClosed, and stable invalid-order errors."},
	{Area: "JSON", API: "JSONParser", Status: StatusPartial, Notes: "Token navigation, current token/name/text, numeric/Boolean/date/datetime/time/id/blob accessors, nextValue, and skipChildren for deterministic local JSON."},
	{Area: "JSON", API: "JSONToken", Status: StatusPartial, Notes: "Common parser token constants for object, array, field, string, number, Boolean, and null tokens."},
	{Area: "JSON", API: "JSON.serialize", Status: StatusPartial, Notes: "Includes suppressApexObjectNulls overload for supported values."},
	{Area: "JSON", API: "JSON.serializePretty", Status: StatusPartial, Notes: "Pretty output for supported values."},
	{Area: "Id", API: "Id.to15", Status: StatusSupported, Notes: "Converts validated 18-character IDs to their 15-character prefix."},
	{Area: "Id", API: "Id.valueOf", Status: StatusSupported, Notes: "Validates 15- and 18-character alphanumeric IDs and restores 18-character casing from checksum suffixes."},
	{Area: "Limits", API: "Limits.get*", Status: StatusPartial, Notes: "SOQL, DML, heap, CPU, async, callout, and email counters."},
	{Area: "Math", API: "Math.abs", Status: StatusSupported, Notes: "Integer and Decimal values."},
	{Area: "Math", API: "Math.ceil", Status: StatusSupported, Notes: "Numeric values."},
	{Area: "Math", API: "Math.floor", Status: StatusSupported, Notes: "Numeric values."},
	{Area: "Math", API: "Math.max", Status: StatusSupported, Notes: "Integer and Decimal values."},
	{Area: "Math", API: "Math.min", Status: StatusSupported, Notes: "Integer and Decimal values."},
	{Area: "Math", API: "Math.pow", Status: StatusSupported, Notes: "Numeric values."},
	{Area: "Math", API: "Math.round", Status: StatusSupported, Notes: "Numeric values."},
	{Area: "Math", API: "Math.sqrt", Status: StatusSupported, Notes: "Numeric values."},
	{Area: "Messaging", API: "Messaging.SingleEmailMessage", Status: StatusPartial, Notes: "Common setters only; no delivery transport."},
	{Area: "Messaging", API: "Messaging.sendEmail", Status: StatusPartial, Notes: "Returns local SendEmailResult and increments email limits."},
	{Area: "Object", API: "Object.equals", Status: StatusSupported, Notes: "Uses local value equality for primitives, collections, platform scalars, and object identity."},
	{Area: "Object", API: "Object.hashCode", Status: StatusSupported, Notes: "Deterministic within local value equality; object identity hashes are request-local."},
	{Area: "Object", API: "Object.toString", Status: StatusSupported, Notes: "Returns local string forms for primitives, collections, platform scalars, and objects."},
	{Area: "PageReference", API: "PageReference", Status: StatusPartial, Notes: "Constructor, URL, redirect, parameters, headers, and string conversion basics."},
	{Area: "Pattern", API: "Matcher.end", Status: StatusPartial, Notes: "Go regexp-backed group end positions."},
	{Area: "Pattern", API: "Matcher.find", Status: StatusPartial, Notes: "Go regexp-backed matching with captured groups."},
	{Area: "Pattern", API: "Matcher.group", Status: StatusPartial, Notes: "Go regexp-backed group access."},
	{Area: "Pattern", API: "Matcher.groupCount", Status: StatusPartial, Notes: "Capturing group count from Go regexp."},
	{Area: "Pattern", API: "Matcher.lookingAt", Status: StatusPartial, Notes: "Beginning-of-input Go regexp match."},
	{Area: "Pattern", API: "Matcher.matches", Status: StatusPartial, Notes: "Whole-input Go regexp match."},
	{Area: "Pattern", API: "Matcher.replaceAll", Status: StatusPartial, Notes: "Go regexp-backed replacement with capture references."},
	{Area: "Pattern", API: "Matcher.replaceFirst", Status: StatusPartial, Notes: "Go regexp-backed first replacement with capture references."},
	{Area: "Pattern", API: "Matcher.start", Status: StatusPartial, Notes: "Go regexp-backed group start positions."},
	{Area: "Pattern", API: "Pattern.compile", Status: StatusPartial, Notes: "Go regexp syntax, not full Java Pattern syntax."},
	{Area: "Pattern", API: "Pattern.matcher", Status: StatusPartial, Notes: "Creates a Go regexp-backed Matcher."},
	{Area: "Pattern", API: "Pattern.matches", Status: StatusPartial, Notes: "Whole-string Go regexp match."},
	{Area: "Pattern", API: "Pattern.pattern", Status: StatusPartial, Notes: "Returns stored Go regexp source."},
	{Area: "Schema", API: "Schema.describeSObjects", Status: StatusPartial, Notes: "Object names and SObjectType tokens for local schema."},
	{Area: "Schema", API: "Schema.getGlobalDescribe", Status: StatusPartial, Notes: "Local schema-backed describe map."},
	{Area: "Schema", API: "DescribeFieldResult", Status: StatusPartial, Notes: "Common field metadata and access booleans."},
	{Area: "Schema", API: "DescribeSObjectResult", Status: StatusPartial, Notes: "Common object metadata, fields, record types, and child relationships."},
	{Area: "String", API: "String.contains", Status: StatusSupported, Notes: "UTF-8 string contains."},
	{Area: "String", API: "String.containsAny", Status: StatusSupported, Notes: "Rune membership."},
	{Area: "String", API: "String.containsNone", Status: StatusSupported, Notes: "Rune exclusion."},
	{Area: "String", API: "String.containsOnly", Status: StatusSupported, Notes: "Rune allow-list."},
	{Area: "String", API: "String.containsWhitespace", Status: StatusSupported, Notes: "Unicode whitespace."},
	{Area: "String", API: "String.countMatches", Status: StatusSupported, Notes: "Non-overlapping literal substring count."},
	{Area: "String", API: "String.endsWith", Status: StatusSupported, Notes: "UTF-8 string suffix."},
	{Area: "String", API: "String.equalsIgnoreCase", Status: StatusSupported, Notes: "Unicode simple fold."},
	{Area: "String", API: "String.indexOf", Status: StatusSupported, Notes: "UTF-8 byte index behavior from Go strings."},
	{Area: "String", API: "String.isAllLowerCase", Status: StatusSupported, Notes: "All letters lowercase; non-letters ignored."},
	{Area: "String", API: "String.isAllUpperCase", Status: StatusSupported, Notes: "All letters uppercase; non-letters ignored."},
	{Area: "String", API: "String.isAlpha", Status: StatusSupported, Notes: "Unicode letters."},
	{Area: "String", API: "String.isAlphaSpace", Status: StatusSupported, Notes: "Unicode letters and space characters."},
	{Area: "String", API: "String.isAlphanumeric", Status: StatusSupported, Notes: "Unicode letters and decimal digits."},
	{Area: "String", API: "String.isAlphanumericSpace", Status: StatusSupported, Notes: "Unicode letters, decimal digits, and space characters."},
	{Area: "String", API: "String.isBlank", Status: StatusSupported, Notes: "Null and whitespace."},
	{Area: "String", API: "String.isNotBlank", Status: StatusSupported, Notes: "Null and whitespace."},
	{Area: "String", API: "String.isNumeric", Status: StatusSupported, Notes: "Unicode decimal digits."},
	{Area: "String", API: "String.isNumericSpace", Status: StatusSupported, Notes: "Unicode decimal digits and space characters."},
	{Area: "String", API: "String.isWhitespace", Status: StatusSupported, Notes: "Unicode whitespace; empty string is true."},
	{Area: "String", API: "String.join", Status: StatusSupported, Notes: "List values and separator."},
	{Area: "String", API: "String.hashCode", Status: StatusSupported, Notes: "Java-compatible UTF-16 code-unit string hash for local values."},
	{Area: "String", API: "String.lastIndexOf", Status: StatusSupported, Notes: "UTF-8 byte index behavior from Go strings."},
	{Area: "String", API: "String.length", Status: StatusSupported, Notes: "Counts runes."},
	{Area: "String", API: "String.replace", Status: StatusSupported, Notes: "Literal replacement."},
	{Area: "String", API: "String.replaceAll", Status: StatusPartial, Notes: "Go regexp-backed replacement."},
	{Area: "String", API: "String.replaceFirst", Status: StatusPartial, Notes: "Go regexp-backed first replacement."},
	{Area: "String", API: "String.split", Status: StatusPartial, Notes: "Go regexp-backed split with Apex limit shape."},
	{Area: "String", API: "String.startsWith", Status: StatusSupported, Notes: "UTF-8 string prefix."},
	{Area: "String", API: "String.substring", Status: StatusSupported, Notes: "Rune-indexed substring."},
	{Area: "String", API: "String.toLowerCase", Status: StatusSupported, Notes: "Go Unicode lowercasing."},
	{Area: "String", API: "String.toUpperCase", Status: StatusSupported, Notes: "Go Unicode uppercasing."},
	{Area: "String", API: "String.trim", Status: StatusSupported, Notes: "Unicode whitespace trim."},
	{Area: "String", API: "String.valueOf", Status: StatusSupported, Notes: "Local value string conversion."},
	{Area: "System", API: "System.assert", Status: StatusSupported, Notes: "Assertion failure returns runtime error."},
	{Area: "System", API: "System.assertEquals", Status: StatusSupported, Notes: "Assertion failure returns runtime error."},
	{Area: "System", API: "System.debug", Status: StatusSupported, Notes: "Collected in result debug output."},
	{Area: "Test", API: "Test.getStandardPricebookId", Status: StatusPartial, Notes: "Deterministic test-context-only ID."},
	{Area: "Test", API: "Test.isRunningTest", Status: StatusSupported, Notes: "Reflects local test context."},
	{Area: "Test", API: "Test.setMock", Status: StatusPartial, Notes: "HttpCalloutMock support for local tests."},
	{Area: "Test", API: "Test.startTest", Status: StatusPartial, Notes: "Governor-window reset/restore for supported counters."},
	{Area: "Test", API: "Test.stopTest", Status: StatusPartial, Notes: "Drains supported async work."},
	{Area: "Time", API: "Time.hour", Status: StatusSupported, Notes: "Local time component."},
	{Area: "Time", API: "Time.minute", Status: StatusSupported, Notes: "Local time component."},
	{Area: "Time", API: "Time.newInstance", Status: StatusSupported, Notes: "Validates time parts."},
	{Area: "Time", API: "Time.second", Status: StatusSupported, Notes: "Local time component."},
	{Area: "Time", API: "Time.valueOf", Status: StatusSupported, Notes: "Parses supported time strings."},
	{Area: "Type", API: "Type.forName", Status: StatusPartial, Notes: "Local class/type token lookup."},
	{Area: "Type", API: "Type.equals", Status: StatusSupported, Notes: "Compares local Type tokens by type name."},
	{Area: "Type", API: "Type.getName", Status: StatusSupported, Notes: "Returns local type token name."},
	{Area: "Type", API: "Type.hashCode", Status: StatusSupported, Notes: "Matches the local String.hashCode of the type name."},
	{Area: "Type", API: "Type.newInstance", Status: StatusPartial, Notes: "Constructs local values and dispatches zero-arg constructors for registered classes; broader reflection is incomplete."},
	{Area: "Type", API: "Type.toString", Status: StatusSupported, Notes: "Returns the local type token name."},
	{Area: "URL", API: "URL", Status: StatusPartial, Notes: "Constructors for deterministic URL specs and protocol/host/file forms."},
	{Area: "URL", API: "URL.getAuthority", Status: StatusSupported, Notes: "Returns parsed authority for local URL values."},
	{Area: "URL", API: "URL.getDefaultPort", Status: StatusSupported, Notes: "Returns HTTP/HTTPS defaults or -1."},
	{Area: "URL", API: "URL.getFile", Status: StatusSupported, Notes: "Returns path plus query for local URL values."},
	{Area: "URL", API: "URL.getHost", Status: StatusSupported, Notes: "Returns parsed hostname for local URL values."},
	{Area: "URL", API: "URL.getOrgDomainUrl", Status: StatusPartial, Notes: "Deterministic local org URL."},
	{Area: "URL", API: "URL.getPath", Status: StatusSupported, Notes: "Returns parsed path for local URL values."},
	{Area: "URL", API: "URL.getPort", Status: StatusSupported, Notes: "Returns explicit port or -1."},
	{Area: "URL", API: "URL.getProtocol", Status: StatusSupported, Notes: "Returns parsed scheme for local URL values."},
	{Area: "URL", API: "URL.getQuery", Status: StatusSupported, Notes: "Returns parsed query for local URL values."},
	{Area: "URL", API: "URL.getRef", Status: StatusSupported, Notes: "Returns parsed fragment for local URL values."},
	{Area: "URL", API: "URL.getSalesforceBaseUrl", Status: StatusPartial, Notes: "Deterministic local base URL."},
	{Area: "URL", API: "URL.toExternalForm", Status: StatusSupported, Notes: "Returns the stored local URL string."},
	{Area: "Unsupported", API: "unimplemented platform/stdlib calls", Status: StatusSupported, Notes: "Typed UnsupportedFeature errors with stable message text."},
	{Area: "UserInfo", API: "UserInfo.getLanguage", Status: StatusPartial, Notes: "Deterministic local value."},
	{Area: "UserInfo", API: "UserInfo.getLocale", Status: StatusPartial, Notes: "Deterministic local value."},
	{Area: "UserInfo", API: "UserInfo.getOrganizationId", Status: StatusPartial, Notes: "Local org identity."},
	{Area: "UserInfo", API: "UserInfo.getProfileId", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getSessionId", Status: StatusPartial, Notes: "Empty local session value."},
	{Area: "UserInfo", API: "UserInfo.getTimeZone", Status: StatusPartial, Notes: "Deterministic UTC value."},
	{Area: "UserInfo", API: "UserInfo.getUserId", Status: StatusPartial, Notes: "Current runAs/default user."},
}
