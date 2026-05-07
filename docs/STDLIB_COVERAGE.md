# Standard Library Coverage

Generated from `internal/capability`.

Status values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`.

| Area | API | Status | Notes |
| --- | --- | --- | --- |
| ApexPages | `ApexPages.Message` | `partial` | Constructor and getters; no Visualforce rendering lifecycle. |
| ApexPages | `ApexPages.addMessage` | `supported` | Stores page messages on the VM instance. |
| ApexPages | `ApexPages.currentPage` | `supported` | Returns a deterministic local PageReference. |
| ApexPages | `ApexPages.getMessages` | `supported` | Returns VM-local page messages. |
| ApexPages | `ApexPages.hasMessages` | `supported` | Checks VM-local page messages. |
| Assert | `Assert.areEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.areNotEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.fail` | `supported` | Raises local System.AssertException with optional message text. |
| Assert | `Assert.isFalse` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNotNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isTrue` | `supported` | Routes through local assertion failures with optional message text. |
| Crypto | `Crypto.generateDigest` | `partial` | MD5, SHA1, and SHA-256. |
| Database | `Database.delete` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.getQueryLocator` | `supported` | Supported SOQL executes eagerly for local batch scopes. |
| Database | `Database.insert` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.merge` | `supported` | Local merge behavior for supported schema-backed data. |
| Database | `Database.rollback` | `supported` | Local org-state savepoint rollback with no external side effects. |
| Database | `Database.setSavepoint` | `supported` | Local org-state snapshots with later-savepoint invalidation. |
| Database | `Database.undelete` | `supported` | Soft-delete restoration for supported local records. |
| Database | `Database.update` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.upsert` | `supported` | Schema-backed external-ID matching for supported local records. |
| Date | `Date.addMonths` | `supported` | Local Gregorian month arithmetic with end-of-month clamping. |
| Date | `Date.addYears` | `supported` | Local Gregorian year arithmetic with leap-day clamping. |
| Date | `Date.newInstance` | `supported` | Validates date parts. |
| Date | `Date.today` | `supported` | Returns the deterministic local runtime date. |
| Date | `Date.valueOf` | `supported` | Parses supported date strings. |
| Datetime | `Datetime.addDays` | `supported` | UTC-local day arithmetic. |
| Datetime | `Datetime.addHours` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMinutes` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMonths` | `supported` | UTC-local month arithmetic with end-of-month clamping. |
| Datetime | `Datetime.addSeconds` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addYears` | `supported` | UTC-local year arithmetic with leap-day clamping. |
| Datetime | `Datetime.format` | `supported` | Formats using the deterministic local timezone model. |
| Datetime | `Datetime.formatGmt` | `supported` | Formats using GMT. |
| Datetime | `Datetime.newInstance` | `supported` | Validates date and time parts. |
| Datetime | `Datetime.now` | `supported` | Returns the deterministic local runtime datetime. |
| Datetime | `Datetime.valueOf` | `supported` | Parses supported datetime strings. |
| Decimal | `Decimal.doubleValue` | `supported` | Returns local Decimal value. |
| Decimal | `Decimal.intValue` | `supported` | Truncates to integer. |
| Decimal | `Decimal.round` | `partial` | Uses Go round-half-away behavior. |
| Decimal | `Decimal.setScale` | `partial` | Non-negative scale only; advanced rounding modes not modeled. |
| EncodingUtil | `EncodingUtil.base64Decode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.base64Encode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertFromHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertToHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.urlDecode` | `partial` | Uses query unescape; charset validation is not modeled. |
| EncodingUtil | `EncodingUtil.urlEncode` | `partial` | Uses query escape; charset validation is not modeled. |
| FeatureManagement | `FeatureManagement.checkPermission` | `partial` | Checks local runAs permission-list state. |
| HTTP | `Http.send local mock callouts` | `supported` | Routes local callouts through registered HttpCalloutMock implementations. |
| HTTP | `Http.send real network transport` | `unsupported` | Outbound network callouts are intentionally not executed by the local runtime. |
| HTTP | `HttpRequest` | `partial` | Endpoint, method, headers, timeout, body, and blob body accessors. |
| HTTP | `HttpResponse` | `partial` | Status, status code, headers, body, and blob body accessors. |
| JSON | `JSON.deserialize` | `partial` | SObject, class, collection, and primitive shapes for supported subset. |
| JSON | `JSON.deserializeStrict` | `partial` | Rejects unknown fields for supported schema/class targets. |
| JSON | `JSON.deserializeUntyped` | `partial` | Maps JSON into local primitive/list/map values. |
| JSON | `JSON.serialize` | `partial` | Includes suppressApexObjectNulls overload for supported values. |
| JSON | `JSON.serializePretty` | `partial` | Pretty output for supported values. |
| Limits | `Limits.get*` | `partial` | SOQL, DML, heap, CPU, async, callout, and email counters. |
| Math | `Math.abs` | `supported` | Integer and Decimal values. |
| Math | `Math.ceil` | `supported` | Numeric values. |
| Math | `Math.floor` | `supported` | Numeric values. |
| Math | `Math.max` | `supported` | Integer and Decimal values. |
| Math | `Math.min` | `supported` | Integer and Decimal values. |
| Math | `Math.pow` | `supported` | Numeric values. |
| Math | `Math.round` | `supported` | Numeric values. |
| Math | `Math.sqrt` | `supported` | Numeric values. |
| Messaging | `Messaging.SingleEmailMessage` | `partial` | Common setters only; no delivery transport. |
| Messaging | `Messaging.sendEmail` | `partial` | Returns local SendEmailResult and increments email limits. |
| PageReference | `PageReference` | `partial` | Constructor, URL, redirect, parameters, headers, and string conversion basics. |
| Pattern | `Matcher.find` | `partial` | Go regexp-backed matching. |
| Pattern | `Matcher.group` | `partial` | Latest matched group only. |
| Pattern | `Matcher.matches` | `partial` | Go regexp-backed matching. |
| Pattern | `Pattern.compile` | `partial` | Go regexp syntax. |
| Pattern | `Pattern.matches` | `partial` | Whole-string Go regexp match. |
| Schema | `DescribeFieldResult` | `partial` | Common field metadata and access booleans. |
| Schema | `DescribeSObjectResult` | `partial` | Common object metadata, fields, record types, and child relationships. |
| Schema | `Schema.describeSObjects` | `partial` | Object names and SObjectType tokens for local schema. |
| Schema | `Schema.getGlobalDescribe` | `partial` | Local schema-backed describe map. |
| Search | `Search.* / SOSL FIND` | `unsupported` | Cloud search and SOSL execution are not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| String | `String.contains` | `supported` | UTF-8 string contains. |
| String | `String.endsWith` | `supported` | UTF-8 string suffix. |
| String | `String.equalsIgnoreCase` | `supported` | Unicode simple fold. |
| String | `String.indexOf` | `supported` | UTF-8 byte index behavior from Go strings. |
| String | `String.isBlank` | `supported` | Null and whitespace. |
| String | `String.isNotBlank` | `supported` | Null and whitespace. |
| String | `String.join` | `supported` | List values and separator. |
| String | `String.lastIndexOf` | `supported` | UTF-8 byte index behavior from Go strings. |
| String | `String.length` | `supported` | Counts runes. |
| String | `String.replace` | `supported` | Literal replacement. |
| String | `String.split` | `partial` | Literal separator, not full Java regex split. |
| String | `String.startsWith` | `supported` | UTF-8 string prefix. |
| String | `String.substring` | `supported` | Rune-indexed substring. |
| String | `String.toLowerCase` | `supported` | Go Unicode lowercasing. |
| String | `String.toUpperCase` | `supported` | Go Unicode uppercasing. |
| String | `String.trim` | `supported` | Unicode whitespace trim. |
| String | `String.valueOf` | `supported` | Local value string conversion. |
| System | `System.assert` | `supported` | Assertion failure returns runtime error. |
| System | `System.assertEquals` | `supported` | Assertion failure returns runtime error. |
| System | `System.debug` | `supported` | Collected in result debug output. |
| Test | `Test.getStandardPricebookId` | `partial` | Deterministic test-context-only ID. |
| Test | `Test.isRunningTest` | `supported` | Reflects local test context. |
| Test | `Test.setMock` | `partial` | HttpCalloutMock support for local tests. |
| Test | `Test.startTest` | `partial` | Governor-window reset/restore for supported counters. |
| Test | `Test.stopTest` | `partial` | Drains supported async work. |
| Time | `Time.hour` | `supported` | Local time component. |
| Time | `Time.minute` | `supported` | Local time component. |
| Time | `Time.newInstance` | `supported` | Validates time parts. |
| Time | `Time.second` | `supported` | Local time component. |
| Time | `Time.valueOf` | `supported` | Parses supported time strings. |
| TimeZone | `TimeZone.getDisplayName` | `supported` | Returns deterministic display names for local timezone values. |
| TimeZone | `TimeZone.getID` | `supported` | Returns local timezone IDs. |
| TimeZone | `TimeZone.getOffset` | `supported` | Returns offsets from the deterministic local timezone model. |
| TimeZone | `TimeZone.getTimeZone` | `supported` | Resolves timezone IDs into local timezone values. |
| Type | `Type.forName` | `partial` | Local class/type token lookup. |
| Type | `Type.getName` | `supported` | Returns local type token name. |
| Type | `Type.newInstance` | `partial` | Constructs local object values without constructor dispatch parity. |
| URL | `URL.getOrgDomainUrl` | `partial` | Deterministic local org URL. |
| URL | `URL.getSalesforceBaseUrl` | `partial` | Deterministic local base URL. |
| Unsupported | `unimplemented platform/stdlib calls` | `supported` | Typed UnsupportedFeature errors with stable message text. |
| UserInfo | `UserInfo.getLanguage` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getLocale` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getOrganizationId` | `partial` | Local org identity. |
| UserInfo | `UserInfo.getProfileId` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getSessionId` | `partial` | Empty local session value. |
| UserInfo | `UserInfo.getTimeZone` | `supported` | Returns the deterministic local user timezone. |
| UserInfo | `UserInfo.getUserId` | `partial` | Current runAs/default user. |
