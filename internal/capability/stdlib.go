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
	{Area: "Assert", API: "Assert.areEqual", Status: StatusSupported, Notes: "Routes through local assertion failures with optional message text."},
	{Area: "Assert", API: "Assert.areNotEqual", Status: StatusSupported, Notes: "Routes through local assertion failures with optional message text."},
	{Area: "Assert", API: "Assert.fail", Status: StatusSupported, Notes: "Raises local System.AssertException with optional message text."},
	{Area: "Assert", API: "Assert.isFalse", Status: StatusSupported, Notes: "Routes through local assertion failures with optional message text."},
	{Area: "Assert", API: "Assert.isNotNull", Status: StatusSupported, Notes: "Routes through local assertion failures with optional message text."},
	{Area: "Assert", API: "Assert.isNull", Status: StatusSupported, Notes: "Routes through local assertion failures with optional message text."},
	{Area: "Assert", API: "Assert.isTrue", Status: StatusSupported, Notes: "Routes through local assertion failures with optional message text."},
	{Area: "Crypto", API: "Crypto.generateDigest", Status: StatusPartial, Notes: "MD5, SHA1, and SHA-256."},
	{Area: "Database", API: "Database.delete", Status: StatusSupported, Notes: "DML pipeline with result/error shapes for supported SObjects."},
	{Area: "Database", API: "Database.getAsyncLocator", Status: StatusSupported, Notes: "Returns deterministic VM-local locator strings for local result and locator objects; no external async service lookup."},
	{Area: "Database", API: "Database.getQueryLocator", Status: StatusSupported, Notes: "Supported SOQL executes eagerly for local batch scopes."},
	{Area: "Database", API: "Database.insert", Status: StatusSupported, Notes: "DML pipeline with result/error shapes for supported SObjects."},
	{Area: "Database", API: "Database.merge", Status: StatusSupported, Notes: "Local merge behavior for supported schema-backed data."},
	{Area: "Database", API: "Database.rollback", Status: StatusSupported, Notes: "Local org-state savepoint rollback with no external side effects."},
	{Area: "Database", API: "Database.setSavepoint", Status: StatusSupported, Notes: "Local org-state snapshots with later-savepoint invalidation."},
	{Area: "Database", API: "Database.UnitOfWork", Status: StatusSupported, Notes: "Queues local DML operations and applies them on commitWork; discardWork drops pending local work."},
	{Area: "Database", API: "Database.undelete", Status: StatusSupported, Notes: "Soft-delete restoration for supported local records."},
	{Area: "Database", API: "Database.update", Status: StatusSupported, Notes: "DML pipeline with result/error shapes for supported SObjects."},
	{Area: "Database", API: "Database.upsert", Status: StatusSupported, Notes: "Schema-backed external-ID matching for supported local records."},
	{Area: "Date", API: "Date.addMonths", Status: StatusSupported, Notes: "Local Gregorian month arithmetic with end-of-month clamping."},
	{Area: "Date", API: "Date.addYears", Status: StatusSupported, Notes: "Local Gregorian year arithmetic with leap-day clamping."},
	{Area: "Date", API: "Date.addDays", Status: StatusSupported, Notes: "Local Gregorian day arithmetic."},
	{Area: "Date", API: "Date.newInstance", Status: StatusSupported, Notes: "Validates date parts."},
	{Area: "Date", API: "Date.today", Status: StatusSupported, Notes: "Returns the deterministic local runtime date."},
	{Area: "Date", API: "Date.valueOf", Status: StatusSupported, Notes: "Parses supported date strings."},
	{Area: "Datetime", API: "Datetime.addDays", Status: StatusSupported, Notes: "UTC-local day arithmetic."},
	{Area: "Datetime", API: "Datetime.addHours", Status: StatusPartial, Notes: "UTC-local arithmetic."},
	{Area: "Datetime", API: "Datetime.addMinutes", Status: StatusPartial, Notes: "UTC-local arithmetic."},
	{Area: "Datetime", API: "Datetime.addMonths", Status: StatusSupported, Notes: "UTC-local month arithmetic with end-of-month clamping."},
	{Area: "Datetime", API: "Datetime.addSeconds", Status: StatusPartial, Notes: "UTC-local arithmetic."},
	{Area: "Datetime", API: "Datetime.addYears", Status: StatusSupported, Notes: "UTC-local year arithmetic with leap-day clamping."},
	{Area: "Datetime", API: "Datetime.format", Status: StatusSupported, Notes: "Formats using the deterministic local timezone model."},
	{Area: "Datetime", API: "Datetime.formatGmt", Status: StatusSupported, Notes: "Formats using GMT."},
	{Area: "Datetime", API: "Datetime.newInstance", Status: StatusSupported, Notes: "Validates date and time parts."},
	{Area: "Datetime", API: "Datetime.now", Status: StatusSupported, Notes: "Returns the deterministic local runtime datetime."},
	{Area: "Datetime", API: "Datetime.valueOf", Status: StatusSupported, Notes: "Parses supported datetime strings."},
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
	{Area: "HTTP", API: "Http.send local mock callouts", Status: StatusSupported, Notes: "Routes local callouts through registered HttpCalloutMock implementations."},
	{Area: "HTTP", API: "Http.send real network transport", Status: StatusUnsupported, Notes: "Outbound network callouts are intentionally not executed by the local runtime."},
	{Area: "HTTP", API: "HttpRequest", Status: StatusPartial, Notes: "Endpoint, method, headers, timeout, body, and blob body accessors."},
	{Area: "HTTP", API: "HttpResponse", Status: StatusPartial, Notes: "Status, status code, headers, body, and blob body accessors."},
	{Area: "JSON", API: "JSON.deserialize", Status: StatusPartial, Notes: "SObject, class, collection, and primitive shapes for supported subset."},
	{Area: "JSON", API: "JSON.deserializeStrict", Status: StatusPartial, Notes: "Rejects unknown fields for supported schema/class targets."},
	{Area: "JSON", API: "JSON.deserializeUntyped", Status: StatusPartial, Notes: "Maps JSON into local primitive/list/map values."},
	{Area: "JSON", API: "JSON.serialize", Status: StatusPartial, Notes: "Includes suppressApexObjectNulls overload for supported values."},
	{Area: "JSON", API: "JSON.serializePretty", Status: StatusPartial, Notes: "Pretty output for supported values."},
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
	{Area: "PageReference", API: "PageReference", Status: StatusPartial, Notes: "Constructor, URL, redirect, parameters, headers, and string conversion basics."},
	{Area: "Pattern", API: "Matcher.find", Status: StatusPartial, Notes: "Go regexp-backed matching."},
	{Area: "Pattern", API: "Matcher.group", Status: StatusPartial, Notes: "Latest matched group only."},
	{Area: "Pattern", API: "Matcher.matches", Status: StatusPartial, Notes: "Go regexp-backed matching."},
	{Area: "Pattern", API: "Pattern.compile", Status: StatusPartial, Notes: "Go regexp syntax."},
	{Area: "Pattern", API: "Pattern.matches", Status: StatusPartial, Notes: "Whole-string Go regexp match."},
	{Area: "Schema", API: "Schema.describeSObjects", Status: StatusPartial, Notes: "Object names and SObjectType tokens for local schema."},
	{Area: "Schema", API: "Schema.getGlobalDescribe", Status: StatusPartial, Notes: "Local schema-backed describe map."},
	{Area: "Schema", API: "DescribeFieldResult", Status: StatusPartial, Notes: "Common field metadata and access booleans."},
	{Area: "Schema", API: "DescribeSObjectResult", Status: StatusPartial, Notes: "Common object metadata, fields, record types, and child relationships."},
	{Area: "Search", API: "Search.* / SOSL FIND", Status: StatusUnsupported, Notes: "Cloud search and SOSL execution are not locally modeled; calls return explicit UnsupportedFeature diagnostics."},
	{Area: "String", API: "String.contains", Status: StatusSupported, Notes: "UTF-8 string contains."},
	{Area: "String", API: "String.endsWith", Status: StatusSupported, Notes: "UTF-8 string suffix."},
	{Area: "String", API: "String.equalsIgnoreCase", Status: StatusSupported, Notes: "Unicode simple fold."},
	{Area: "String", API: "String.indexOf", Status: StatusSupported, Notes: "UTF-8 byte index behavior from Go strings."},
	{Area: "String", API: "String.isBlank", Status: StatusSupported, Notes: "Null and whitespace."},
	{Area: "String", API: "String.isNotBlank", Status: StatusSupported, Notes: "Null and whitespace."},
	{Area: "String", API: "String.join", Status: StatusSupported, Notes: "List values and separator."},
	{Area: "String", API: "String.lastIndexOf", Status: StatusSupported, Notes: "UTF-8 byte index behavior from Go strings."},
	{Area: "String", API: "String.length", Status: StatusSupported, Notes: "Counts runes."},
	{Area: "String", API: "String.replace", Status: StatusSupported, Notes: "Literal replacement."},
	{Area: "String", API: "String.split", Status: StatusPartial, Notes: "Literal separator, not full Java regex split."},
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
	{Area: "Test", API: "Test.createStubQueryRow", Status: StatusPartial, Notes: "Builds local SObject rows from field maps for SOQL stub providers."},
	{Area: "Test", API: "Test.createStubQueryRows", Status: StatusPartial, Notes: "Builds local SObject row lists from field maps for SOQL stub providers."},
	{Area: "Test", API: "Test.isRunningTest", Status: StatusSupported, Notes: "Reflects local test context."},
	{Area: "Test", API: "Test.loadData", Status: StatusPartial, Notes: "Loads CSV static-resource content into local org storage through DML."},
	{Area: "Test", API: "Test.setMock", Status: StatusPartial, Notes: "HttpCalloutMock support for local tests."},
	{Area: "Test", API: "Test.startTest", Status: StatusPartial, Notes: "Governor-window reset/restore for supported counters."},
	{Area: "Test", API: "Test.stopTest", Status: StatusPartial, Notes: "Drains supported async work."},
	{Area: "Time", API: "Time.hour", Status: StatusSupported, Notes: "Local time component."},
	{Area: "Time", API: "Time.minute", Status: StatusSupported, Notes: "Local time component."},
	{Area: "Time", API: "Time.newInstance", Status: StatusSupported, Notes: "Validates time parts."},
	{Area: "Time", API: "Time.second", Status: StatusSupported, Notes: "Local time component."},
	{Area: "Time", API: "Time.valueOf", Status: StatusSupported, Notes: "Parses supported time strings."},
	{Area: "TimeZone", API: "TimeZone.getDisplayName", Status: StatusSupported, Notes: "Returns deterministic display names for local timezone values."},
	{Area: "TimeZone", API: "TimeZone.getID", Status: StatusSupported, Notes: "Returns local timezone IDs."},
	{Area: "TimeZone", API: "TimeZone.getOffset", Status: StatusSupported, Notes: "Returns offsets from the deterministic local timezone model."},
	{Area: "TimeZone", API: "TimeZone.getTimeZone", Status: StatusSupported, Notes: "Resolves timezone IDs into local timezone values."},
	{Area: "Type", API: "Type.forName", Status: StatusPartial, Notes: "Local class/type token lookup."},
	{Area: "Type", API: "Type.getName", Status: StatusSupported, Notes: "Returns local type token name."},
	{Area: "Type", API: "Type.newInstance", Status: StatusPartial, Notes: "Constructs local object values without constructor dispatch parity."},
	{Area: "URL", API: "URL.getOrgDomainUrl", Status: StatusPartial, Notes: "Deterministic local org URL."},
	{Area: "URL", API: "URL.getSalesforceBaseUrl", Status: StatusPartial, Notes: "Deterministic local base URL."},
	{Area: "Unsupported", API: "unimplemented platform/stdlib calls", Status: StatusSupported, Notes: "Typed UnsupportedFeature errors with stable message text."},
	{Area: "UserInfo", API: "UserInfo.getLanguage", Status: StatusPartial, Notes: "Deterministic local value."},
	{Area: "UserInfo", API: "UserInfo.getLocale", Status: StatusPartial, Notes: "Deterministic local value."},
	{Area: "UserInfo", API: "UserInfo.getFirstName", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getLastName", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getName", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getOrganizationId", Status: StatusPartial, Notes: "Local org identity."},
	{Area: "UserInfo", API: "UserInfo.getProfileId", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getSessionId", Status: StatusPartial, Notes: "Empty local session value."},
	{Area: "UserInfo", API: "UserInfo.getTimeZone", Status: StatusSupported, Notes: "Returns the deterministic local user timezone."},
	{Area: "UserInfo", API: "UserInfo.getUserEmail", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getUserId", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getUserName", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.getUserType", Status: StatusPartial, Notes: "Current runAs/default user."},
	{Area: "UserInfo", API: "UserInfo.isMultiCurrencyOrganization", Status: StatusPartial, Notes: "Local org metadata flag."},
}
