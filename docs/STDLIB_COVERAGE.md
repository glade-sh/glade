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
| Id | `Id.to15` | `supported` | Converts validated 18-character IDs to their 15-character prefix. |
| Id | `Id.valueOf` | `supported` | Validates 15- and 18-character alphanumeric IDs and restores 18-character casing from checksum suffixes. |
| JSON | `JSON.createGenerator` | `supported` | Creates deterministic local JSONGenerator instances. |
| JSON | `JSON.createParser` | `partial` | Creates deterministic local JSONParser token streams for valid JSON strings. |
| JSON | `JSON.deserialize` | `partial` | SObject, class, collection, and primitive shapes for supported subset. |
| JSON | `JSON.deserializeStrict` | `partial` | Rejects unknown fields for supported schema/class targets. |
| JSON | `JSON.deserializeUntyped` | `partial` | Maps JSON into local primitive/list/map values. |
| JSON | `JSON.serialize` | `partial` | Includes suppressApexObjectNulls overload for supported values. |
| JSON | `JSON.serializePretty` | `partial` | Pretty output for supported values. |
| JSON | `JSONGenerator` | `partial` | Object/array boundaries, field names, scalar string/number/Boolean/null, Date/Datetime/Time/Id/Blob, Object writers, getAsString, close, isClosed, and stable invalid-order errors. |
| JSON | `JSONParser` | `partial` | Token navigation, current token/name/text, numeric/Boolean/date/datetime/time/id/blob accessors, nextValue, and skipChildren for deterministic local JSON. |
| JSON | `JSONToken` | `partial` | Common parser token constants for object, array, field, string, number, Boolean, and null tokens. |
| Limits | `Limits.get*` | `partial` | SOQL, DML, heap, CPU, async, callout, and email counters. |
| List | `List.add` | `supported` | Adds typed local values, including indexed insertion. |
| List | `List.addAll` | `supported` | Appends typed values from local List or Set values. |
| List | `List.clear` | `supported` | Removes all local list elements. |
| List | `List.clone` | `supported` | Copies the local list container; elements keep local value identity. |
| List | `List.copyConstructor` | `supported` | Copies values from a local List or Set constructor argument. |
| List | `List.deepClone` | `partial` | No-argument local recursive clone; SObject preserve-option overloads are unsupported. |
| List | `List.get` | `supported` | Indexed lookup with stable bounds errors. |
| List | `List.indexOf` | `supported` | Local equality search with -1 for misses. |
| List | `List.isEmpty` | `supported` | Checks local list length. |
| List | `List.remove` | `supported` | Indexed removal returns the removed value. |
| List | `List.set` | `supported` | Indexed replacement with typed value coercion. |
| List | `List.size` | `supported` | Returns local list length. |
| List | `List.sort` | `partial` | Deterministic sort for local primitive comparable values only. |
| Map | `Map.clear` | `supported` | Removes all local map entries. |
| Map | `Map.clone` | `supported` | Copies the local map container; values keep local identity. |
| Map | `Map.containsKey` | `supported` | Checks local keys using deterministic key encoding. |
| Map | `Map.containsValue` | `supported` | Checks local values using local value equality. |
| Map | `Map.copyConstructor` | `supported` | Copies entries from a local Map constructor argument. |
| Map | `Map.deepClone` | `partial` | No-argument local recursive clone; SObject preserve-option overloads are unsupported. |
| Map | `Map.get` | `supported` | Returns local value or null for missing keys. |
| Map | `Map.isEmpty` | `supported` | Checks local map size. |
| Map | `Map.keySet` | `supported` | Returns deterministic local key Set. |
| Map | `Map.put` | `supported` | Stores typed local entries and returns the previous value. |
| Map | `Map.putAll` | `supported` | Copies local entries from another Map. |
| Map | `Map.remove` | `supported` | Removes a key and returns its previous local value or null. |
| Map | `Map.size` | `supported` | Returns local map size. |
| Map | `Map.toString` | `partial` | Deterministic local entry string form; exact platform formatting may differ. |
| Map | `Map.values` | `supported` | Returns deterministic local values List. |
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
| Object | `Object.equals` | `supported` | Uses local value equality for primitives, collections, platform scalars, and object identity. |
| Object | `Object.hashCode` | `supported` | Deterministic within local value equality; object identity hashes are request-local. |
| Object | `Object.toString` | `supported` | Returns local string forms for primitives, collections, platform scalars, and objects. |
| PageReference | `PageReference` | `partial` | Constructor, URL, redirect, parameters, headers, and string conversion basics. |
| Pattern | `Matcher.end` | `partial` | Go regexp-backed group end positions. |
| Pattern | `Matcher.find` | `partial` | Go regexp-backed matching with captured groups. |
| Pattern | `Matcher.group` | `partial` | Go regexp-backed group access. |
| Pattern | `Matcher.groupCount` | `partial` | Capturing group count from Go regexp. |
| Pattern | `Matcher.lookingAt` | `partial` | Beginning-of-input Go regexp match. |
| Pattern | `Matcher.matches` | `partial` | Whole-input Go regexp match. |
| Pattern | `Matcher.replaceAll` | `partial` | Go regexp-backed replacement with capture references. |
| Pattern | `Matcher.replaceFirst` | `partial` | Go regexp-backed first replacement with capture references. |
| Pattern | `Matcher.start` | `partial` | Go regexp-backed group start positions. |
| Pattern | `Pattern.compile` | `partial` | Go regexp syntax, not full Java Pattern syntax. |
| Pattern | `Pattern.matcher` | `partial` | Creates a Go regexp-backed Matcher. |
| Pattern | `Pattern.matches` | `partial` | Whole-string Go regexp match. |
| Pattern | `Pattern.pattern` | `partial` | Returns stored Go regexp source. |
| Schema | `DescribeFieldResult` | `partial` | Common field metadata and access booleans. |
| Schema | `DescribeSObjectResult` | `partial` | Common object metadata, fields, record types, and child relationships. |
| Schema | `Schema.describeSObjects` | `partial` | Object names and SObjectType tokens for local schema. |
| Schema | `Schema.getGlobalDescribe` | `partial` | Local schema-backed describe map. |
| Set | `Set.add` | `supported` | Adds typed local values and reports whether the Set changed. |
| Set | `Set.addAll` | `supported` | Adds typed values from local List or Set values. |
| Set | `Set.clear` | `supported` | Removes all local set elements. |
| Set | `Set.clone` | `supported` | Copies the local set container; elements keep local value identity. |
| Set | `Set.contains` | `supported` | Checks local values using local value equality. |
| Set | `Set.containsAll` | `supported` | Checks local List or Set membership. |
| Set | `Set.copyConstructor` | `supported` | Copies unique values from a local List or Set constructor argument. |
| Set | `Set.deepClone` | `partial` | No-argument local recursive clone; SObject preserve-option overloads are unsupported. |
| Set | `Set.isEmpty` | `supported` | Checks local set length. |
| Set | `Set.remove` | `supported` | Removes a local value and reports whether the Set changed. |
| Set | `Set.removeAll` | `supported` | Removes local List or Set members. |
| Set | `Set.retainAll` | `supported` | Retains only local List or Set members. |
| Set | `Set.size` | `supported` | Returns local set length. |
| String | `String.abbreviate` | `supported` | Apache-style abbreviation for one- and two-argument forms. |
| String | `String.charAt` | `supported` | Rune-indexed one-character String. |
| String | `String.codePointAt` | `supported` | Rune-indexed Unicode code point. |
| String | `String.codePointBefore` | `supported` | Rune-indexed previous Unicode code point. |
| String | `String.codePointCount` | `supported` | Counts runes between validated indexes. |
| String | `String.commonPrefix` | `supported` | Returns the shared rune prefix for two strings. |
| String | `String.contains` | `supported` | UTF-8 string contains. |
| String | `String.containsAny` | `supported` | Rune membership. |
| String | `String.containsNone` | `supported` | Rune exclusion. |
| String | `String.containsOnly` | `supported` | Rune allow-list. |
| String | `String.containsWhitespace` | `supported` | Unicode whitespace. |
| String | `String.countMatches` | `supported` | Non-overlapping literal substring count. |
| String | `String.difference` | `supported` | Returns the differing suffix from the comparison string. |
| String | `String.endsWith` | `supported` | UTF-8 string suffix. |
| String | `String.equalsIgnoreCase` | `supported` | Unicode simple fold. |
| String | `String.escapeCsv` | `supported` | RFC4180-style quoting and doubled quotes for local strings. |
| String | `String.escapeEcmaScript` | `partial` | JavaScript-style backslash escaping for common deterministic cases. |
| String | `String.escapeHtml4` | `partial` | Go HTML entity escaping for deterministic core entities. |
| String | `String.escapeJava` | `partial` | Java-style backslash and Unicode escaping for deterministic core cases. |
| String | `String.escapeSingleQuotes` | `supported` | Escapes single quotes with backslashes. |
| String | `String.escapeUnicode` | `partial` | Escapes non-ASCII and control runes as UTF-16 Unicode escapes. |
| String | `String.escapeXml` | `partial` | Escapes XML core entities; XML version-specific validity is not modeled. |
| String | `String.format` | `partial` | Deterministic {0}-style list substitution; full MessageFormat locale behavior is not modeled. |
| String | `String.fromCharArray` | `supported` | Builds a string from valid Unicode code point integers. |
| String | `String.getChars` | `supported` | Returns Unicode code point integers for each rune. |
| String | `String.getCommonPrefix` | `supported` | Returns the shared rune prefix for a list of strings. |
| String | `String.getLevenshteinDistance` | `supported` | Rune-based edit distance. |
| String | `String.hashCode` | `supported` | Java-compatible UTF-16 code-unit string hash for local values. |
| String | `String.indexOf` | `supported` | UTF-8 byte index behavior from Go strings. |
| String | `String.isAllLowerCase` | `supported` | All letters lowercase; non-letters ignored. |
| String | `String.isAllUpperCase` | `supported` | All letters uppercase; non-letters ignored. |
| String | `String.isAlpha` | `supported` | Unicode letters. |
| String | `String.isAlphaSpace` | `supported` | Unicode letters and space characters. |
| String | `String.isAlphanumeric` | `supported` | Unicode letters and decimal digits. |
| String | `String.isAlphanumericSpace` | `supported` | Unicode letters, decimal digits, and space characters. |
| String | `String.isAsciiPrintable` | `supported` | Checks ASCII printable range 0x20 through 0x7E. |
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
| String | `String.splitByCharacterType` | `supported` | Splits on coarse Unicode upper/lower/digit/space/other groups. |
| String | `String.splitByCharacterTypeCamelCase` | `supported` | Splits character types with camel-case upper-to-lower adjustment. |
| String | `String.startsWith` | `supported` | UTF-8 string prefix. |
| String | `String.substring` | `supported` | Rune-indexed substring. |
| String | `String.toLowerCase` | `supported` | Go Unicode lowercasing. |
| String | `String.toUpperCase` | `supported` | Go Unicode uppercasing. |
| String | `String.trim` | `supported` | Unicode whitespace trim. |
| String | `String.unescapeCsv` | `supported` | Unquotes doubled-quote CSV fields. |
| String | `String.unescapeEcmaScript` | `partial` | Unescapes common JavaScript-style backslash and Unicode escapes. |
| String | `String.unescapeHtml4` | `partial` | Go HTML entity unescaping for deterministic core entities. |
| String | `String.unescapeJava` | `partial` | Unescapes common Java-style backslash and Unicode escapes. |
| String | `String.unescapeUnicode` | `partial` | Unescapes UTF-16 Unicode escape sequences. |
| String | `String.unescapeXml` | `partial` | Unescapes XML core entities. |
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
| Type | `Type.equals` | `supported` | Compares local Type tokens by type name. |
| Type | `Type.forName` | `partial` | Local class/type token lookup. |
| Type | `Type.getName` | `supported` | Returns local type token name. |
| Type | `Type.hashCode` | `supported` | Matches the local String.hashCode of the type name. |
| Type | `Type.newInstance` | `partial` | Constructs local values and dispatches zero-arg constructors for registered classes; broader reflection is incomplete. |
| Type | `Type.toString` | `supported` | Returns the local type token name. |
| URL | `URL` | `partial` | Constructors for deterministic URL specs and protocol/host/file forms. |
| URL | `URL.getAuthority` | `supported` | Returns parsed authority for local URL values. |
| URL | `URL.getDefaultPort` | `supported` | Returns HTTP/HTTPS defaults or -1. |
| URL | `URL.getFile` | `supported` | Returns path plus query for local URL values. |
| URL | `URL.getHost` | `supported` | Returns parsed hostname for local URL values. |
| URL | `URL.getOrgDomainUrl` | `partial` | Deterministic local org URL. |
| URL | `URL.getPath` | `supported` | Returns parsed path for local URL values. |
| URL | `URL.getPort` | `supported` | Returns explicit port or -1. |
| URL | `URL.getProtocol` | `supported` | Returns parsed scheme for local URL values. |
| URL | `URL.getQuery` | `supported` | Returns parsed query for local URL values. |
| URL | `URL.getRef` | `supported` | Returns parsed fragment for local URL values. |
| URL | `URL.getSalesforceBaseUrl` | `partial` | Deterministic local base URL. |
| URL | `URL.toExternalForm` | `supported` | Returns the stored local URL string. |
| Unsupported | `unimplemented platform/stdlib calls` | `supported` | Typed UnsupportedFeature errors with stable message text. |
| UserInfo | `UserInfo.getLanguage` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getLocale` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getOrganizationId` | `partial` | Local org identity. |
| UserInfo | `UserInfo.getProfileId` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getSessionId` | `partial` | Empty local session value. |
| UserInfo | `UserInfo.getTimeZone` | `partial` | Deterministic UTC value. |
| UserInfo | `UserInfo.getUserId` | `partial` | Current runAs/default user. |
