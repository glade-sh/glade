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
| Blob | `Blob.size` | `supported` | Returns local Blob byte length. |
| Blob | `Blob.toString` | `supported` | Returns the local Blob bytes as a string. |
| Blob | `Blob.valueOf` | `supported` | Stores the string bytes in a local Blob value. |
| Crypto | `Crypto.generateDigest` | `supported` | MD5, SHA1, SHA-256, SHA-512, and SHA3-256/384/512. |
| Crypto | `Crypto.generateMac` | `supported` | HMAC MD5, SHA1, SHA256, and SHA512 with local Blob keys. |
| Database | `Database.delete` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.getQueryLocator` | `partial` | Supported SOQL only; executes eagerly for local batch scopes. |
| Database | `Database.insert` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.merge` | `partial` | Local merge behavior for supported account/contact-style data. |
| Database | `Database.rollback` | `partial` | Local org-state savepoint rollback; no external side effects. |
| Database | `Database.setSavepoint` | `partial` | Local org-state snapshots with later-savepoint invalidation. |
| Database | `Database.undelete` | `partial` | Soft-delete restoration for supported local records. |
| Database | `Database.update` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.upsert` | `partial` | Schema-backed external-ID matching for supported local records. |
| Date | `Date.addDays` | `supported` | Local Gregorian date arithmetic. |
| Date | `Date.addMonths` | `partial` | Local Gregorian arithmetic with month-end clamp; full Salesforce edge-case parity incomplete. |
| Date | `Date.addYears` | `partial` | Local Gregorian arithmetic with leap-day clamp; full Salesforce edge-case parity incomplete. |
| Date | `Date.day` | `supported` | Returns Gregorian day of month. |
| Date | `Date.daysBetween` | `supported` | Returns whole calendar days between local Date values. |
| Date | `Date.month` | `supported` | Returns Gregorian month number. |
| Date | `Date.newInstance` | `supported` | Validates date parts. |
| Date | `Date.toEndOfMonth` | `supported` | Returns last day of the Date value's month. |
| Date | `Date.toStartOfMonth` | `supported` | Returns first day of the Date value's month. |
| Date | `Date.valueOf` | `supported` | Parses supported date strings. |
| Date | `Date.year` | `supported` | Returns Gregorian year. |
| Datetime | `Datetime.addDays` | `partial` | UTC-local arithmetic; no user timezone or DST model. |
| Datetime | `Datetime.addHours` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMinutes` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMonths` | `partial` | UTC-local arithmetic with month-end clamp; no user timezone or DST model. |
| Datetime | `Datetime.addSeconds` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addYears` | `partial` | UTC-local arithmetic with leap-day clamp; no user timezone or DST model. |
| Datetime | `Datetime.date` | `partial` | Returns UTC-modeled Date component; no user timezone model. |
| Datetime | `Datetime.day` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.hour` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.minute` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.month` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.newInstance` | `supported` | Validates date and time parts. |
| Datetime | `Datetime.second` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.valueOf` | `supported` | Parses supported datetime strings. |
| Datetime | `Datetime.year` | `partial` | UTC-modeled component getter. |
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
| HTTP | `Http.send` | `partial` | Mock-first local callouts; real network transport unsupported. |
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
| String | `String.contains` | `supported` | UTF-8 string contains. |
| String | `String.containsAny` | `supported` | Rune membership. |
| String | `String.containsNone` | `supported` | Rune exclusion. |
| String | `String.containsOnly` | `supported` | Rune allow-list. |
| String | `String.containsWhitespace` | `supported` | Unicode whitespace. |
| String | `String.countMatches` | `supported` | Non-overlapping literal substring count. |
| String | `String.endsWith` | `supported` | UTF-8 string suffix. |
| String | `String.equalsIgnoreCase` | `supported` | Unicode simple fold. |
| String | `String.indexOf` | `supported` | UTF-8 byte index behavior from Go strings. |
| String | `String.isAllLowerCase` | `supported` | All letters lowercase; non-letters ignored. |
| String | `String.isAllUpperCase` | `supported` | All letters uppercase; non-letters ignored. |
| String | `String.isAlpha` | `supported` | Unicode letters. |
| String | `String.isAlphaSpace` | `supported` | Unicode letters and space characters. |
| String | `String.isAlphanumeric` | `supported` | Unicode letters and decimal digits. |
| String | `String.isAlphanumericSpace` | `supported` | Unicode letters, decimal digits, and space characters. |
| String | `String.isBlank` | `supported` | Null and whitespace. |
| String | `String.isNotBlank` | `supported` | Null and whitespace. |
| String | `String.isNumeric` | `supported` | Unicode decimal digits. |
| String | `String.isNumericSpace` | `supported` | Unicode decimal digits and space characters. |
| String | `String.isWhitespace` | `supported` | Unicode whitespace; empty string is true. |
| String | `String.join` | `supported` | List values and separator. |
| String | `String.lastIndexOf` | `supported` | UTF-8 byte index behavior from Go strings. |
| String | `String.length` | `supported` | Counts runes. |
| String | `String.replace` | `supported` | Literal replacement. |
| String | `String.replaceAll` | `partial` | Go regexp-backed replacement. |
| String | `String.replaceFirst` | `partial` | Go regexp-backed first replacement. |
| String | `String.split` | `partial` | Go regexp-backed split with Apex limit shape. |
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
| UserInfo | `UserInfo.getTimeZone` | `partial` | Deterministic UTC value. |
| UserInfo | `UserInfo.getUserId` | `partial` | Current runAs/default user. |
