# Standard Library Coverage

Generated from `internal/capability`.

Status values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`.

| Area | API | Status | Notes |
| --- | --- | --- | --- |
| ApexPages | `ApexPages.Message` | `partial` | Constructor plus severity, summary, and detail getters; no Visualforce rendering lifecycle. |
| ApexPages | `ApexPages.addMessage` | `supported` | Stores page messages on the VM instance. |
| ApexPages | `ApexPages.currentPage` | `supported` | Returns a deterministic local PageReference. |
| ApexPages | `ApexPages.getMessages` | `supported` | Returns VM-local page messages. |
| ApexPages | `ApexPages.hasMessages` | `supported` | Checks VM-local page messages. |
| Approval | `Approval process APIs` | `unsupported` | Approval.process and approval lock helpers return explicit UnsupportedFeature diagnostics; approval workflow side effects are not locally modeled. |
| Async | `AsyncApexJob / CronTrigger local records` | `partial` | Test-context enqueue/drain creates deterministic local AsyncApexJob rows and CronTrigger rows for supported future, queueable, batch, and scheduled jobs; broader platform lifecycle fields are not modeled. |
| Async | `AsyncInfo / AsyncOptions / finalizers` | `unsupported` | Queueable stack metadata, AsyncOptions mutators/accessors and enqueue overloads, System.attachFinalizer, and FinalizerContext getters return explicit UnsupportedFeature diagnostics. |
| Async | `BatchableContext.getJobId` | `partial` | Returns the deterministic local AsyncApexJob Id while supported batch start/execute/finish methods drain under Test.stopTest. |
| Async | `QueueableContext.getJobId` | `partial` | Returns the deterministic local AsyncApexJob Id while supported queueables drain under Test.stopTest. |
| Async | `SchedulableContext.getTriggerId` | `partial` | Returns the deterministic local CronTrigger Id while supported scheduled jobs drain under Test.stopTest. |
| Auth | `token/cloud APIs` | `unsupported` | Auth namespace token and session/cloud calls return explicit UnsupportedFeature diagnostics. |
| Blob | `Blob.size` | `supported` | Returns local Blob byte length. |
| Blob | `Blob.toString` | `supported` | Returns UTF-8 local Blob bytes as a string and rejects invalid UTF-8 data. |
| Blob | `Blob.valueOf` | `supported` | Stores the string bytes in a local Blob value. |
| Canvas | `Canvas namespace` | `unsupported` | Canvas app integration calls return explicit UnsupportedFeature diagnostics. |
| Continuation | `Continuation` | `unsupported` | Continuation construction and callback/response calls return explicit UnsupportedFeature diagnostics. |
| Crypto | `Crypto.areEqualConstantTime` | `supported` | Constant-time local Blob equality comparison. |
| Crypto | `Crypto.encrypt/decrypt/sign/verify` | `unsupported` | Org key, certificate, encryption, and random key-generation surfaces return explicit unsupported errors. |
| Crypto | `Crypto.generateDigest` | `supported` | MD5, SHA1, SHA-256, SHA-512, SHA3-256/384/512, with conservative algorithm normalization. |
| Crypto | `Crypto.generateMac` | `supported` | HMAC MD5, SHA1, SHA256, and SHA512 with local Blob keys and conservative algorithm normalization. |
| Data | `Custom metadata/custom settings getAll/getInstance` | `partial` | Fixture-backed local __mdt and list custom setting static access supports namespace-stripped object/field names and read-only returned records; hierarchy merge behavior and Metadata API mutation are not modeled. |
| Database | `Database.convertLead` | `unsupported` | Lead conversion returns an explicit UnsupportedFeature diagnostic until local lead/account/contact/opportunity side effects are modeled. |
| Database | `Database.delete` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.emptyRecycleBin` | `partial` | Permanently removes already-deleted local rows and returns EmptyRecycleBinResult-shaped objects; retention policy and related platform recycle-bin behavior are not modeled. |
| Database | `Database.getQueryLocator` | `partial` | Supported SOQL only; executes eagerly for local batch scopes and exposes getQuery()/iterator() over the local snapshot. |
| Database | `Database.insert` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.lock / Database.unlock` | `partial` | Toggles local storage row lock state and returns LockResult/UnlockResult-shaped objects; ownership, wait timing, and transaction-scoped lock release are not modeled. |
| Database | `Database.merge` | `partial` | Local merge behavior for supported account/contact-style data, including MergeResult merged and updated-related ID accessors. |
| Database | `Database.rollback` | `partial` | Local org-state savepoint rollback; no external side effects. |
| Database | `Database.setSavepoint` | `partial` | Local org-state snapshots with later-savepoint invalidation. |
| Database | `Database.undelete` | `partial` | Soft-delete restoration for supported local records with mixed-row result alignment, allOrNone rollback, ENTITY_IS_NOT_DELETED active-row errors, and ID/object mismatch errors. |
| Database | `Database.update` | `supported` | DML pipeline with result/error shapes for supported SObjects. |
| Database | `Database.upsert` | `partial` | Schema-backed external-ID matching for supported local records. |
| Date | `Date.addDays` | `supported` | Local Gregorian date arithmetic. |
| Date | `Date.addMonths` | `partial` | Local Gregorian arithmetic with month-end clamp; full Salesforce edge-case parity incomplete. |
| Date | `Date.addYears` | `partial` | Local Gregorian arithmetic with leap-day clamp; full Salesforce edge-case parity incomplete. |
| Date | `Date.day` | `supported` | Returns Gregorian day of month. |
| Date | `Date.daysBetween` | `supported` | Returns whole calendar days between local Date values. |
| Date | `Date.month` | `supported` | Returns Gregorian month number. |
| Date | `Date.newInstance` | `supported` | Validates date parts in the local year 1-9999 Gregorian slice. |
| Date | `Date.toEndOfMonth` | `supported` | Returns last day of the Date value's month. |
| Date | `Date.toStartOfMonth` | `supported` | Returns first day of the Date value's month. |
| Date | `Date.today` | `partial` | Deterministic VM clock date in UTC. |
| Date | `Date.valueOf` | `supported` | Parses strict yyyy-MM-dd strings plus yyyy-MM-dd time forms into local Date values with stable invalid-input errors. |
| Date | `Date.year` | `supported` | Returns Gregorian year. |
| Datetime | `Datetime.addDays` | `partial` | UTC-local arithmetic; no user timezone or DST model. |
| Datetime | `Datetime.addHours` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMilliseconds` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMinutes` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addMonths` | `partial` | UTC-local arithmetic with month-end clamp; no user timezone or DST model. |
| Datetime | `Datetime.addSeconds` | `partial` | UTC-local arithmetic. |
| Datetime | `Datetime.addYears` | `partial` | UTC-local arithmetic with leap-day clamp; no user timezone or DST model. |
| Datetime | `Datetime.date` | `partial` | Returns UTC-modeled Date component; no user timezone model. |
| Datetime | `Datetime.dateGmt` | `supported` | Returns the UTC Date component. |
| Datetime | `Datetime.day` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.format` | `partial` | Deterministic UTC/fixed-offset Java-pattern slice plus America/Los_Angeles DST formatting; format(pattern) and format() use the current user timezone for the modeled UTC/America/Los_Angeles slice; user locale is unsupported. |
| Datetime | `Datetime.formatGmt` | `partial` | Deterministic UTC Java-pattern slice with stable token errors; locale patterns are not modeled beyond pinned English names. |
| Datetime | `Datetime.hour` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.millisecond` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.minute` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.month` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.newInstance` | `supported` | Validates date and time parts in the local year 1-9999 UTC-modeled slice. |
| Datetime | `Datetime.newInstanceGmt` | `supported` | Constructs a UTC-modeled Datetime. |
| Datetime | `Datetime.now` | `partial` | Deterministic VM clock timestamp in UTC. |
| Datetime | `Datetime.second` | `partial` | UTC-modeled component getter. |
| Datetime | `Datetime.timeGmt` | `supported` | Returns the UTC Time component. |
| Datetime | `Datetime.valueOf` | `supported` | Parses supported strict datetime strings with stable invalid-input errors. |
| Datetime | `Datetime.valueOfGmt` | `supported` | Parses supported strict UTC/RFC3339 datetime strings with stable invalid-input errors. |
| Datetime | `Datetime.year` | `partial` | UTC-modeled component getter. |
| Decimal | `Decimal.abs` | `supported` | Absolute value for local Decimal values. |
| Decimal | `Decimal.doubleValue` | `supported` | Returns local Decimal value. |
| Decimal | `Decimal.format` | `partial` | Simple deterministic finite numeric formatting; locale and pattern overloads return explicit unsupported errors. |
| Decimal | `Decimal.intValue` | `supported` | Truncates to 32-bit Integer with overflow checks. |
| Decimal | `Decimal.longValue` | `supported` | Truncates to local Long representation with overflow checks. |
| Decimal | `Decimal.pow` | `supported` | Power with Integer exponent for finite local Decimal results. |
| Decimal | `Decimal.round` | `partial` | Default half-up plus local RoundingMode subset; exact Decimal scale parity is not modeled. |
| Decimal | `Decimal.setScale` | `partial` | Supports local scale 0-15 with UP, DOWN, CEILING, FLOOR, HALF_UP, HALF_DOWN, HALF_EVEN, and UNNECESSARY. |
| Decimal | `Decimal.valueOf` | `supported` | Parses finite decimal strings and numeric values, including trimmed signed strings. |
| Double | `Double.format` | `partial` | Simple deterministic finite numeric formatting; locale and pattern overloads return explicit unsupported errors. |
| Double | `Double.valueOf` | `supported` | Parses finite decimal strings and numeric values into the local numeric representation, including trimmed signed strings. |
| EncodingUtil | `EncodingUtil.base64Decode` | `supported` | Blob-shaped local value with stable invalid-input errors. |
| EncodingUtil | `EncodingUtil.base64Encode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertFromHex` | `supported` | Blob-shaped local value with stable odd-length and invalid-input errors. |
| EncodingUtil | `EncodingUtil.convertToHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.urlDecode` | `partial` | Uses query unescape with UTF-8/utf8/UTF_8 charset validation only; other charsets return UnsupportedFeature. |
| EncodingUtil | `EncodingUtil.urlEncode` | `partial` | Uses query escape with UTF-8/utf8/UTF_8 charset validation only; other charsets return UnsupportedFeature. |
| EventBus | `EventBus.publish` | `unsupported` | Platform event publish calls return explicit UnsupportedFeature diagnostics. |
| Exception | `Built-in exception types` | `partial` | Known public built-in exception tokens construct message-bearing local exceptions and assign to Exception; exact platform class catalog, line numbers, and stack text remain partial. |
| Exception | `Exception.getCause` | `partial` | Returns the locally initialized cause value; repeat/self-cause platform edge rules are not modeled. |
| Exception | `Exception.getLineNumber` | `partial` | Returns deterministic local throw-site line metadata when available; otherwise 0. |
| Exception | `Exception.getMessage` | `supported` | Returns the local exception message. |
| Exception | `Exception.getStackTraceString` | `partial` | Returns the local VM stack trace captured at throw time when available. |
| Exception | `Exception.getTypeName` | `supported` | Returns the local exception type name without System namespace prefix. |
| Exception | `Exception.initCause` | `partial` | Stores a local Exception cause or null for later getCause calls. |
| Exception | `Exception.toString` | `partial` | Returns System-prefixed built-in exception text for local exception values. |
| FeatureManagement | `FeatureManagement.checkPermission` | `partial` | Checks local runAs permission-list state. |
| HTTP | `Http.send` | `partial` | Mock-first local callouts with request validation and callout accounting; real network transport unsupported. |
| HTTP | `HttpRequest` | `partial` | Endpoint, method, case-insensitive headers, timeout validation/defaults, body, and blob body accessors. |
| HTTP | `HttpResponse` | `partial` | Status, status code, case-insensitive headers, body, and blob body accessors. |
| Id | `Id.getSObjectType` | `partial` | Resolves local schema key prefixes and a bounded common standard prefix table to Schema.SObjectType tokens; unknown shape-valid prefixes return a stable error. |
| Id | `Id.to15` | `supported` | Converts validated 18-character IDs to their 15-character prefix. |
| Id | `Id.to18` | `supported` | Adds or preserves the documented 3-character checksum for validated IDs. |
| Id | `Id.valueOf` | `supported` | Validates 15-character IDs and strict 18-character checksum suffixes; restoreCasing rebuilds casing from checksum suffixes. |
| Integer | `Integer.MAX_VALUE` | `supported` | Exposes the public 32-bit Integer maximum constant. |
| Integer | `Integer.MIN_VALUE` | `supported` | Exposes the public 32-bit Integer minimum constant. |
| Integer | `Integer.doubleValue` | `supported` | Converts local Integer values to the local numeric representation. |
| Integer | `Integer.format` | `partial` | Simple deterministic base-10 formatting; locale and pattern overloads return explicit unsupported errors. |
| Integer | `Integer.valueOf` | `supported` | Parses integer strings and numeric values with 32-bit overflow checks, including trimmed signed strings. |
| Iterator | `Iterator.hasNext` | `supported` | Checks remaining elements in a local collection snapshot. |
| Iterator | `Iterator.next` | `supported` | Returns the next element from a local collection snapshot and raises NoSuchElementException when exhausted. |
| Iterator | `Iterator.remove` | `unsupported` | Returns an explicit unsupported error; mutating collection iterators are not modeled. |
| JSON | `JSON.createGenerator` | `supported` | Creates deterministic local JSONGenerator instances. |
| JSON | `JSON.createParser` | `partial` | Creates deterministic local JSONParser token streams for valid JSON strings. |
| JSON | `JSON.deserialize` | `partial` | Primitive, platform scalar, List, Map<String,Object/value>, SObject, and class shapes for the supported local subset; stable mapping errors for mismatched typed shapes and unsupported local object targets. |
| JSON | `JSON.deserializeStrict` | `partial` | Rejects duplicate object fields and throws catchable JSONException for unknown fields on supported schema/class targets; otherwise shares the bounded typed local mapping subset. |
| JSON | `JSON.deserializeUntyped` | `partial` | Maps JSON into local primitive/list/map values with deterministic null and number handling. |
| JSON | `JSON.serialize` | `partial` | Includes suppressApexObjectNulls overload for supported object fields; map/list nulls are preserved for supported values. |
| JSON | `JSON.serializePretty` | `partial` | Pretty output for supported values with object-field null suppression and map/list null preservation. |
| JSON | `JSONGenerator` | `partial` | Object/array boundaries, field names, scalar string/number/Boolean/null, Date/Datetime/Time/Id/Blob, Object and validated raw value writers, getAsString, close, isClosed, stable invalid-order errors, and catchable JSONException for field names and object/array end mismatches inside containers. |
| JSON | `JSONParser` | `partial` | Token navigation, current token/name/text, integer/long/decimal/double/Boolean/date/datetime/time/id/blob accessors, nextValue, skipChildren current-name state, and clearCurrentToken for deterministic local JSON. |
| JSON | `JSONToken` | `partial` | Common parser token constants for object, array, field, string, number, Boolean, and null tokens. |
| Limits | `Limits.get*` | `partial` | SOQL, DML, heap, CPU, async, callout, and email counters; unmodeled documented getters return explicit unsupported diagnostics. |
| List | `List.add` | `supported` | Adds typed local values, including indexed insertion. |
| List | `List.addAll` | `supported` | Appends typed values from local List or Set values. |
| List | `List.clear` | `supported` | Removes all local list elements. |
| List | `List.clone` | `supported` | Copies the local list container; elements keep local value identity. |
| List | `List.contains` | `supported` | Checks local values, including null, using local value equality. |
| List | `List.copyConstructor` | `supported` | Copies values from a local List or Set constructor argument. |
| List | `List.deepClone` | `partial` | No-argument local recursive clone; SObject preserve-option overloads are unsupported. |
| List | `List.get` | `supported` | Indexed lookup with stable bounds errors. |
| List | `List.indexOf` | `supported` | Local equality search, including null elements, with -1 for misses. |
| List | `List.isEmpty` | `supported` | Checks local list length. |
| List | `List.iterator` | `supported` | Returns a deterministic snapshot Iterator for local List values. |
| List | `List.remove` | `supported` | Indexed removal returns the removed value. |
| List | `List.set` | `supported` | Indexed replacement with typed value coercion. |
| List | `List.size` | `supported` | Returns local list length. |
| List | `List.sort` | `partial` | Deterministic sort for local primitive comparable values; object/Comparable sorting returns an explicit unsupported error. |
| LoggingLevel | `LoggingLevel.name` | `supported` | Returns deterministic built-in enum member text. |
| LoggingLevel | `LoggingLevel.ordinal` | `supported` | Returns deterministic built-in enum order for the local logging level set. |
| LoggingLevel | `LoggingLevel.toString` | `supported` | Returns deterministic built-in enum member text. |
| LoggingLevel | `LoggingLevel.values` | `supported` | Returns NONE, ERROR, WARN, INFO, DEBUG, FINE, FINER, FINEST in deterministic order. |
| Long | `Long.MAX_VALUE` | `supported` | Exposes the public 64-bit Long maximum constant. |
| Long | `Long.MIN_VALUE` | `supported` | Exposes the public 64-bit Long minimum constant. |
| Long | `Long.format` | `partial` | Simple deterministic base-10 formatting; locale and pattern overloads return explicit unsupported errors. |
| Long | `Long.valueOf` | `supported` | Parses integer strings and numeric values with overflow checks, including trimmed signed strings. |
| Map | `Map.clear` | `supported` | Removes all local map entries. |
| Map | `Map.clone` | `supported` | Copies the local map container; values keep local identity. |
| Map | `Map.containsKey` | `supported` | Checks local keys, including null keys, using deterministic key encoding. |
| Map | `Map.containsValue` | `supported` | Checks local values, including null values, using local value equality. |
| Map | `Map.copyConstructor` | `supported` | Copies entries from a local Map and supports Map<Id,SObject> construction from List<SObject> with non-null unique Ids. |
| Map | `Map.deepClone` | `partial` | No-argument local recursive clone; SObject preserve-option overloads are unsupported. |
| Map | `Map.get` | `supported` | Returns local value or null for missing keys. |
| Map | `Map.isEmpty` | `supported` | Checks local map size. |
| Map | `Map.keySet` | `supported` | Returns deterministic local key Set, preserving null keys. |
| Map | `Map.put` | `supported` | Stores typed local entries and returns the previous value. |
| Map | `Map.putAll` | `supported` | Copies local entries from another Map or SObject rows into Map<Id,SObject> by non-null unique Id. |
| Map | `Map.remove` | `supported` | Removes a key and returns its previous local value or null. |
| Map | `Map.size` | `supported` | Returns local map size. |
| Map | `Map.toString` | `partial` | Deterministic local entry string form; exact platform formatting may differ. |
| Map | `Map.values` | `supported` | Returns deterministic local values List. |
| Math | `Math.E` | `supported` | Euler's number from Go's deterministic math constant. |
| Math | `Math.PI` | `supported` | Pi from Go's deterministic math constant. |
| Math | `Math.abs` | `supported` | Integer and Decimal values. |
| Math | `Math.acos` | `supported` | Finite deterministic result for inputs in [-1, 1]. |
| Math | `Math.asin` | `supported` | Finite deterministic result for inputs in [-1, 1]. |
| Math | `Math.atan` | `supported` | Finite deterministic result for numeric values. |
| Math | `Math.atan2` | `supported` | Finite deterministic result for two numeric values. |
| Math | `Math.ceil` | `supported` | Numeric values. |
| Math | `Math.cos` | `supported` | Finite deterministic result for numeric values. |
| Math | `Math.exp` | `supported` | Finite deterministic result; overflow is reported. |
| Math | `Math.floor` | `supported` | Numeric values. |
| Math | `Math.log` | `supported` | Finite deterministic result for positive numeric values. |
| Math | `Math.log10` | `supported` | Finite deterministic result for positive numeric values. |
| Math | `Math.max` | `supported` | Integer and Decimal values. |
| Math | `Math.min` | `supported` | Integer and Decimal values. |
| Math | `Math.mod` | `supported` | Integer remainder and Decimal modulus with zero-divisor errors. |
| Math | `Math.pow` | `supported` | Numeric values. |
| Math | `Math.round` | `supported` | Numeric values. |
| Math | `Math.roundToLong` | `supported` | Rounds to local Long representation with overflow checks. |
| Math | `Math.signum` | `supported` | Returns -1, 0, or 1 for local numeric values. |
| Math | `Math.sin` | `supported` | Finite deterministic result for numeric values. |
| Math | `Math.sqrt` | `supported` | Numeric values. |
| Math | `Math.tan` | `supported` | Finite deterministic result for numeric values. |
| Messaging | `Messaging.SendEmailResult` | `partial` | Local success result exposes isSuccess and getErrors getters. |
| Messaging | `Messaging.SingleEmailMessage` | `partial` | Common setters only; no delivery transport. |
| Messaging | `Messaging.sendEmail` | `partial` | Single-list overload returns local SendEmailResult and increments email limits; transport/options/template surfaces are unsupported. |
| Object | `Object.equals` | `supported` | Uses local value equality for primitives, collections, platform scalars, and object identity. |
| Object | `Object.hashCode` | `supported` | Deterministic within local value equality; object identity hashes are request-local. |
| Object | `Object.toString` | `supported` | Returns local string forms for primitives, collections, platform scalars, and objects. |
| PageReference | `PageReference` | `partial` | Constructor, URL, redirect, parameters, headers, and string conversion basics. |
| Pattern | `Matcher.appendReplacement/appendTail` | `unsupported` | Java StringBuffer append-position semantics return explicit unsupported errors. |
| Pattern | `Matcher.end` | `partial` | Go regexp-backed group end positions. |
| Pattern | `Matcher.find` | `partial` | Go regexp-backed matching with captured groups. |
| Pattern | `Matcher.group` | `partial` | Go regexp-backed group access. |
| Pattern | `Matcher.groupCount` | `partial` | Capturing group count from Go regexp. |
| Pattern | `Matcher.hasAnchoringBounds` | `partial` | Local bounds flag defaults to true; exact region-anchor interaction remains partial. |
| Pattern | `Matcher.hasTransparentBounds` | `partial` | Local bounds flag defaults to false; exact transparent-bound matching remains partial. |
| Pattern | `Matcher.lookingAt` | `partial` | Beginning-of-input Go regexp match. |
| Pattern | `Matcher.matches` | `partial` | Whole-input Go regexp match. |
| Pattern | `Matcher.region` | `partial` | Bounds matching to a local rune-indexed region; exact Java anchoring/transparent bounds interaction remains partial. |
| Pattern | `Matcher.regionStart/regionEnd` | `partial` | Returns local rune-indexed region bounds. |
| Pattern | `Matcher.replaceAll` | `partial` | Go regexp-backed replacement with capture references, region bounds, and basic Java escaped-dollar handling. |
| Pattern | `Matcher.replaceFirst` | `partial` | Go regexp-backed first replacement with capture references, region bounds, and basic Java escaped-dollar handling. |
| Pattern | `Matcher.reset` | `partial` | Clears local match state, resets the region to full input, and optionally replaces input. |
| Pattern | `Matcher.start` | `partial` | Go regexp-backed group start positions. |
| Pattern | `Matcher.useAnchoringBounds` | `partial` | Stores local bounds flag; exact region-anchor interaction remains partial. |
| Pattern | `Matcher.usePattern` | `partial` | Swaps the local Go regexp Pattern and resets search state within the current region. |
| Pattern | `Matcher.useTransparentBounds` | `partial` | Stores local bounds flag; exact transparent-bound matching remains partial. |
| Pattern | `Pattern.compile` | `partial` | Go regexp syntax, not full Java Pattern syntax. |
| Pattern | `Pattern.matcher` | `partial` | Creates a Go regexp-backed Matcher. |
| Pattern | `Pattern.matches` | `partial` | Whole-string Go regexp match. |
| Pattern | `Pattern.pattern` | `partial` | Returns stored Go regexp source. |
| Pattern | `Pattern.split` | `partial` | Go regexp-backed split with local limit semantics. |
| QuickAction | `QuickAction namespace` | `unsupported` | Quick action UI calls return explicit UnsupportedFeature diagnostics. |
| REST | `@RestResource local server dispatch` | `unsupported` | Custom Apex REST dispatch returns a stable unsupported error from the local server. |
| REST | `RestContext.request / RestContext.response` | `partial` | VM-local static slots support RestRequest assignment and lazy RestResponse creation; no platform request lifecycle dispatch is modeled. |
| REST | `RestRequest / RestResponse object shapes` | `partial` | Local request/response objects expose URI/path/method/address, params, headers, Blob body, status, and add/get helper methods covered by compatibility fixtures; broader platform lifecycle remains unsupported. |
| RoundingMode | `RoundingMode.valueOf` | `partial` | Constructs supported local Decimal rounding-mode tokens by exact name. |
| Schema | `DescribeFieldResult` | `partial` | Common field metadata and access booleans. |
| Schema | `DescribeSObjectResult` | `partial` | Common object metadata, fields, record types, and child relationships. |
| Schema | `Schema.describeSObjects` | `partial` | Object names and SObjectType tokens for local schema. |
| Schema | `Schema.getGlobalDescribe` | `partial` | Local schema-backed describe map. |
| Search | `Search.* / SOSL FIND` | `unsupported` | Cloud search and SOSL execution are not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| Set | `Set.add` | `supported` | Adds typed local values and reports whether the Set changed. |
| Set | `Set.addAll` | `supported` | Adds typed values from local List or Set values. |
| Set | `Set.clear` | `supported` | Removes all local set elements. |
| Set | `Set.clone` | `supported` | Copies the local set container; elements keep local value identity. |
| Set | `Set.contains` | `supported` | Checks local values, including null, using local value equality. |
| Set | `Set.containsAll` | `supported` | Checks local List or Set membership. |
| Set | `Set.copyConstructor` | `supported` | Copies unique values from a local List or Set constructor argument. |
| Set | `Set.deepClone` | `partial` | No-argument local recursive clone; SObject preserve-option overloads are unsupported. |
| Set | `Set.isEmpty` | `supported` | Checks local set length. |
| Set | `Set.iterator` | `supported` | Returns a deterministic snapshot Iterator for local Set values. |
| Set | `Set.remove` | `supported` | Removes a local value, including null, and reports whether the Set changed. |
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
| String | `String.escapeCsv` | `supported` | One-field RFC4180-style quoting for comma, quote, CR, and LF; doubles embedded quotes. |
| String | `String.escapeEcmaScript` | `partial` | JavaScript-style backslash escaping for common deterministic cases. |
| String | `String.escapeHtml3` | `partial` | Deterministic core HTML entity escaping only; full named-entity coverage is not modeled. |
| String | `String.escapeHtml4` | `partial` | Deterministic core HTML entity escaping only; full named-entity coverage is not modeled. |
| String | `String.escapeJava` | `partial` | Java-style backslash and Unicode escaping for deterministic core cases. |
| String | `String.escapeSingleQuotes` | `supported` | Escapes single quotes with backslashes. |
| String | `String.escapeUnicode` | `partial` | Escapes non-ASCII and control runes as UTF-16 Unicode escapes. |
| String | `String.escapeXml` | `partial` | Escapes XML core entities; XML version-specific validity is not modeled. |
| String | `String.escapeXml10` | `partial` | Escapes XML core entities, drops XML 1.0-invalid code points, and numeric-escapes restricted control ranges. |
| String | `String.escapeXml11` | `partial` | Escapes XML core entities, drops XML 1.1-invalid nulls, and numeric-escapes restricted control ranges. |
| String | `String.format` | `partial` | Deterministic {0}-style list substitution; full MessageFormat locale behavior is not modeled. |
| String | `String.fromCharArray` | `supported` | Builds a string from valid Unicode code point integers. |
| String | `String.getChars` | `supported` | Returns Unicode code point integers for each rune. |
| String | `String.getCommonPrefix` | `supported` | Returns the shared rune prefix for a list of strings. |
| String | `String.getLevenshteinDistance` | `supported` | Rune-based edit distance. |
| String | `String.hashCode` | `supported` | Java-compatible UTF-16 code-unit string hash for local values. |
| String | `String.indexOf` | `supported` | UTF-8 byte index behavior from Go strings. |
| String | `String.indexOfAny` | `supported` | Returns the first rune index whose character appears in the search set. |
| String | `String.indexOfAnyBut` | `supported` | Returns the first rune index whose character is outside the search set. |
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
| String | `String.lastIndexOfAny` | `supported` | Returns the last rune index whose character appears in the search set. |
| String | `String.lastOrdinalIndexOf` | `supported` | Finds the nth occurrence from the end using rune indexes. |
| String | `String.length` | `supported` | Counts runes. |
| String | `String.ordinalIndexOf` | `supported` | Finds the nth occurrence from the start using rune indexes. |
| String | `String.overlay` | `supported` | Overlays text between clamped rune indexes. |
| String | `String.remove` | `supported` | Literal non-overlapping removal; empty targets are a no-op. |
| String | `String.removeEnd` | `supported` | Removes a literal suffix when present. |
| String | `String.removeEndIgnoreCase` | `supported` | Removes a literal suffix with rune-wise case folding when present. |
| String | `String.removeIgnoreCase` | `supported` | Literal non-overlapping removal with rune-wise case folding. |
| String | `String.removeStart` | `supported` | Removes a literal prefix when present. |
| String | `String.removeStartIgnoreCase` | `supported` | Removes a literal prefix with rune-wise case folding when present. |
| String | `String.replace` | `supported` | Literal replacement; empty targets are a no-op. |
| String | `String.replaceAll` | `partial` | Go regexp-backed replacement. |
| String | `String.replaceFirst` | `partial` | Go regexp-backed first replacement. |
| String | `String.replaceIgnoreCase` | `supported` | Literal non-overlapping replacement with rune-wise case folding. |
| String | `String.replaceOnce` | `supported` | First literal replacement only. |
| String | `String.rotate` | `supported` | Rune-based rotation; positive shifts rotate right. |
| String | `String.split` | `partial` | Go regexp-backed split with Apex limit shape. |
| String | `String.splitByCharacterType` | `supported` | Splits on coarse Unicode upper/lower/digit/space/other groups. |
| String | `String.splitByCharacterTypeCamelCase` | `supported` | Splits character types with camel-case upper-to-lower adjustment. |
| String | `String.startsWith` | `supported` | UTF-8 string prefix. |
| String | `String.strip` | `supported` | Strips Unicode whitespace or a supplied rune set from both ends. |
| String | `String.stripAll` | `supported` | Strips each string in a list with optional strip-character set. |
| String | `String.stripEnd` | `supported` | Strips Unicode whitespace or a supplied rune set from the end. |
| String | `String.stripStart` | `supported` | Strips Unicode whitespace or a supplied rune set from the start. |
| String | `String.stripToEmpty` | `supported` | Strips whitespace and returns an empty string for all-blank values. |
| String | `String.stripToNull` | `supported` | Strips whitespace and returns null for all-blank values. |
| String | `String.substring` | `supported` | Rune-indexed substring. |
| String | `String.swapCase` | `supported` | Unicode upper/lower rune case swap. |
| String | `String.toLowerCase` | `supported` | Go Unicode lowercasing. |
| String | `String.toUpperCase` | `supported` | Go Unicode uppercasing. |
| String | `String.trim` | `supported` | Unicode whitespace trim. |
| String | `String.unescapeCsv` | `supported` | Unquotes one doubled-quote CSV field; plain strings are unchanged. |
| String | `String.unescapeEcmaScript` | `partial` | Unescapes common JavaScript-style backslash and Unicode escapes. |
| String | `String.unescapeHtml3` | `partial` | Unescapes core HTML entities, selected high-use HTML 3/4 named entities, and numeric references; remaining unknown named entities stay unchanged. |
| String | `String.unescapeHtml4` | `partial` | Unescapes core HTML entities, selected high-use HTML 3/4 named entities, and numeric references; remaining unknown named entities stay unchanged. |
| String | `String.unescapeJava` | `partial` | Unescapes common Java-style backslash and Unicode escapes. |
| String | `String.unescapeUnicode` | `partial` | Unescapes UTF-16 Unicode escape sequences. |
| String | `String.unescapeXml` | `partial` | Unescapes XML core entities and numeric references; XML version-specific validity is not modeled. |
| String | `String.unescapeXml10` | `partial` | Alias of local XML core/numeric entity unescaping; XML 1.0 validity is not modeled. |
| String | `String.unescapeXml11` | `partial` | Alias of local XML core/numeric entity unescaping; XML 1.1 validity is not modeled. |
| String | `String.valueOf` | `supported` | Local value string conversion. |
| System | `System.assert` | `supported` | Assertion failure returns runtime error; Object and null message values use deterministic local string conversion. |
| System | `System.assertEquals` | `supported` | Assertion failure returns runtime error with deterministic local expected/actual text and Object/null message conversion. |
| System | `System.assertNotEquals` | `supported` | Assertion failure returns runtime error with deterministic local value text and Object/null message conversion. |
| System | `System.asyncScheduling` | `partial` | System.abortJob removes queued local Queueable and Schedulable jobs before Test.stopTest; completed and unknown aborts plus scheduleBatch return explicit unsupported diagnostics. Broader async lifecycle control is not modeled. |
| System | `System.currentTimeMillis` | `partial` | Returns deterministic VM-clock epoch milliseconds. |
| System | `System.debug` | `supported` | One-argument and LoggingLevel overloads are collected in result debug output; null, LoggingLevel, and modeled Exception values use deterministic string forms; log framework text parity is not claimed. |
| System | `System.isBatch` | `partial` | Returns false in the local non-async VM context. |
| System | `System.isFuture` | `partial` | Returns false in the local non-async VM context. |
| System | `System.isQueueable` | `partial` | Returns false in the local non-async VM context. |
| System | `System.isScheduled` | `partial` | Returns false in the local non-async VM context. |
| System | `System.now` | `partial` | Returns deterministic VM-clock Datetime. |
| System | `System.today` | `partial` | Returns deterministic VM-clock Date. |
| Test | `Test.clearApexPageMessages` | `supported` | Clears VM-local ApexPages messages in test context. |
| Test | `Test.createSoqlStub` | `unsupported` | SOQL stub creation is not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| Test | `Test.createStub` | `unsupported` | Dynamic stub creation is not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| Test | `Test.getStandardPricebookId` | `partial` | Deterministic test-context-only ID. |
| Test | `Test.isRunningTest` | `supported` | Reflects local test context. |
| Test | `Test.setFixedSearchResults` | `unsupported` | Fixed SOSL search results are deferred with the local search surface; calls return explicit UnsupportedFeature diagnostics. |
| Test | `Test.setMock` | `partial` | HttpCalloutMock support for local tests; other mock interfaces return explicit unsupported diagnostics. |
| Test | `Test.startTest` | `partial` | Governor-window reset/restore for supported counters. |
| Test | `Test.stopTest` | `partial` | Drains supported async work. |
| Time | `Time.addHours` | `supported` | Local time arithmetic with 24-hour wrap. |
| Time | `Time.addMilliseconds` | `supported` | Local time arithmetic with 24-hour wrap. |
| Time | `Time.addMinutes` | `supported` | Local time arithmetic with 24-hour wrap. |
| Time | `Time.addSeconds` | `supported` | Local time arithmetic with 24-hour wrap. |
| Time | `Time.hour` | `supported` | Local time component. |
| Time | `Time.millisecond` | `supported` | Local time component. |
| Time | `Time.minute` | `supported` | Local time component. |
| Time | `Time.newInstance` | `supported` | Validates time parts including optional millisecond. |
| Time | `Time.second` | `supported` | Local time component. |
| Time | `Time.valueOf` | `supported` | Parses strict HH:mm:ss and HH:mm:ss.SSS strings with stable invalid-input errors. |
| TimeZone | `TimeZone.getDisplayName` | `partial` | Returns deterministic ID text for UTC, fixed GMT offsets, and America/Los_Angeles; DST/locale overloads are unsupported. |
| TimeZone | `TimeZone.getID` | `partial` | Returns canonical UTC, fixed GMT offset ID, or the supported America/Los_Angeles named zone. |
| TimeZone | `TimeZone.getOffset` | `partial` | Returns fixed offset milliseconds for the deterministic offset slice and America/Los_Angeles DST decisions; other named zones are unsupported. |
| TimeZone | `TimeZone.getTimeZone` | `partial` | Supports UTC/GMT, fixed GMT/UTC offsets through ±14:00, and America/Los_Angeles; other named zones and trimmed/invalid IDs are unsupported. |
| Type | `Type.equals` | `supported` | Compares local Type tokens by type name. |
| Type | `Type.forName` | `partial` | Local class/type token lookup, common local SObjects, built-in and generic collection type strings, and null for null/blank/unknown local names. |
| Type | `Type.getName` | `supported` | Returns local type token name. |
| Type | `Type.hashCode` | `supported` | Matches the local String.hashCode of the type name. |
| Type | `Type.isAssignableFrom` | `partial` | Uses the local class/interface and built-in exception hierarchy. |
| Type | `Type.newInstance` | `partial` | Constructs local values and dispatches zero-arg constructors for registered classes; unbacked namespace/package tokens return explicit unsupported errors. |
| Type | `Type.toString` | `supported` | Returns the local type token name. |
| URL | `URL` | `partial` | Constructors for deterministic absolute URL specs, context/spec resolution, and protocol/host/file forms with stable malformed input errors. |
| URL | `URL.getAuthority` | `supported` | Returns parsed authority for local URL values. |
| URL | `URL.getCurrentRequestUrl` | `unsupported` | Cloud request context is not modeled; returns an explicit unsupported error. |
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
| UserInfo | `UserInfo.getFirstName` | `partial` | Current runAs/server/default user. |
| UserInfo | `UserInfo.getLanguage` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getLastName` | `partial` | Current runAs/server/default user. |
| UserInfo | `UserInfo.getLocale` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getName` | `partial` | Current runAs/server/default user. |
| UserInfo | `UserInfo.getOrganizationId` | `partial` | Local org identity. |
| UserInfo | `UserInfo.getProfileId` | `partial` | Current runAs/server/default user. |
| UserInfo | `UserInfo.getSessionId` | `partial` | Empty local session value. |
| UserInfo | `UserInfo.getTimeZone` | `partial` | Returns a TimeZone object for the current user TimeZoneSidKey in the modeled UTC/America/Los_Angeles slice. |
| UserInfo | `UserInfo.getUserEmail` | `partial` | Current runAs/server/default user. |
| UserInfo | `UserInfo.getUserId` | `partial` | Current runAs/server/default user. |
