# Standard Library Coverage

Generated from `internal/capability`.

Status values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`.

| Area | API | Status | Notes |
| --- | --- | --- | --- |
| ApexPages | `ApexPages.Message` | `supported` | Constructor plus severity, summary, and detail getters, including local ApexPages.Severity enum values. |
| ApexPages | `ApexPages.addMessage` | `supported` | Stores page messages on the VM instance. |
| ApexPages | `ApexPages.currentPage` | `supported` | Returns a stable VM-local PageReference; Test.setCurrentPage can replace it in test context. |
| ApexPages | `ApexPages.getMessages` | `supported` | Returns VM-local page messages. |
| ApexPages | `ApexPages.hasMessages` | `supported` | Checks VM-local page messages. |
| Approval | `Approval process APIs` | `unsupported` | Approval namespace process, request, and lock helpers return fixture-backed UnsupportedFeature diagnostics; approval workflow side effects are not locally modeled. |
| Async | `AsyncApexJob / CronTrigger local records` | `supported` | Test-context enqueue/drain creates deterministic local AsyncApexJob rows and CronTrigger rows for supported future, queueable, batch, and scheduled jobs in the local model. |
| Async | `AsyncInfo / AsyncOptions / finalizers` | `unsupported` | Queueable stack metadata, AsyncOptions mutators/accessors and enqueue overloads, System.attachFinalizer, and FinalizerContext getters return fixture-backed UnsupportedFeature diagnostics. |
| Async | `BatchableContext.getJobId` | `supported` | Returns the deterministic local AsyncApexJob Id during supported batch start, execute, and finish drain phases. |
| Async | `QueueableContext.getJobId` | `supported` | Returns the deterministic local AsyncApexJob Id during supported queueable Test.stopTest drain. |
| Async | `SchedulableContext.getTriggerId` | `supported` | Returns the deterministic local CronTrigger Id during supported scheduled Test.stopTest drain. |
| Async | `platform async lifecycle controls` | `unsupported` | Completed or unknown abortJob targets, scheduleBatch deferral, broader cloud lifecycle fields, AsyncInfo, AsyncOptions, and finalizers return fixture-backed UnsupportedFeature diagnostics. |
| Auth | `token/cloud APIs` | `unsupported` | Auth namespace session, JWT, OAuth, token, and cloud calls return fixture-backed UnsupportedFeature diagnostics. |
| Blob | `Blob.size` | `supported` | Returns local Blob byte length. |
| Blob | `Blob.toString` | `supported` | Returns UTF-8 local Blob bytes as a string and rejects invalid UTF-8 data. |
| Blob | `Blob.valueOf` | `supported` | Stores the string bytes in a local Blob value. |
| Canvas | `Canvas namespace` | `unsupported` | Canvas app integration and lifecycle calls return fixture-backed UnsupportedFeature diagnostics. |
| Continuation | `Continuation` | `unsupported` | Continuation construction, request registration, and response calls return fixture-backed UnsupportedFeature diagnostics. |
| Crypto | `Crypto.areEqualConstantTime` | `supported` | Constant-time local Blob equality comparison. |
| Crypto | `Crypto.encrypt/decrypt/sign/verify` | `unsupported` | Encrypt/decrypt, managed-IV variants, signing, verification, org key, keystore, certificate, and random key-generation surfaces return explicit UnsupportedFeature diagnostics; no fake local key material is modeled. |
| Crypto | `Crypto.generateDigest` | `supported` | MD5, SHA1, SHA-256, SHA-512, SHA3-256/384/512, with conservative algorithm normalization. |
| Crypto | `Crypto.generateMac` | `supported` | HMAC MD5, SHA1, SHA256, and SHA512 with local Blob keys and conservative algorithm normalization. |
| Data | `Custom metadata/list custom setting getAll/getInstance` | `supported` | Fixture-backed local __mdt and list custom setting static access supports namespace-stripped object/field names and read-only returned records. |
| Data | `Hierarchy custom setting merge behavior` | `unsupported` | Hierarchy custom setting static accessors return fixture-backed UnsupportedFeature diagnostics until org/profile/user merge behavior is modeled. |
| Data | `Metadata mutation for custom metadata/settings` | `unsupported` | Local custom metadata and custom setting static rows are read-only; Metadata API mutation is outside the local runtime model. |
| Database | `Database.convertLead` | `unsupported` | Lead conversion returns an explicit UnsupportedFeature diagnostic until local lead/account/contact/opportunity side effects are modeled. |
| Database | `Database.delete` | `supported` | DML pipeline with DeleteResult id/success/errors accessors and structured status/message/fields details for supported SObjects. |
| Database | `Database.emptyRecycleBin` | `supported` | Permanently removes already-deleted local rows and returns EmptyRecycleBinResult id/success/errors accessors including active-row ENTITY_IS_NOT_IN_RECYCLE_BIN errors; retention policy and full platform recycle-bin lifecycle are outside the local model. |
| Database | `Database.getQueryLocator` | `supported` | Executes supported SOQL eagerly for local batch scopes and exposes getQuery()/iterator() over the local snapshot. |
| Database | `Database.insert` | `supported` | DML pipeline with SaveResult id/success/errors accessors, structured required/duplicate/validation error details, and DmlException detail methods for supported SObjects. |
| Database | `Database.lock / Database.unlock` | `supported` | Toggles the local storage row lock marker and returns LockResult/UnlockResult id/success/errors accessors; ownership, wait timing, and transaction-scoped release are outside the local model. |
| Database | `Database.merge` | `supported` | Local merge behavior for supported account/contact-style data, including duplicate soft delete, child lookup reparenting, and MergeResult merged/updated-related ID accessors. |
| Database | `Database.rollback` | `supported` | Restores local org-state savepoint snapshots and invalidates later savepoints; rollback-specific Limits getters remain explicit unsupported diagnostics. |
| Database | `Database.setSavepoint` | `supported` | Creates local org-state snapshots for Database.rollback; external side effects are outside the local model. |
| Database | `Database.undelete` | `supported` | Soft-delete restoration for supported local records with UndeleteResult accessors, mixed-row result alignment, allOrNone rollback, ENTITY_IS_NOT_DELETED active-row errors, and ID/object mismatch errors. |
| Database | `Database.update` | `supported` | DML pipeline with SaveResult id/success/errors accessors and structured status/message/fields details for supported SObjects. |
| Database | `Database.upsert` | `supported` | Schema-backed ID and external-ID insert/update matching with trigger dispatch and UpsertResult id/success/errors/isCreated accessors for supported local records. |
| Date | `Date.addDays` | `supported` | Local Gregorian date arithmetic. |
| Date | `Date.addMonths` | `supported` | Deterministic local Gregorian month arithmetic with month-end clamp for positive and negative deltas. |
| Date | `Date.addYears` | `supported` | Deterministic local Gregorian year arithmetic with leap-day clamp for positive and negative deltas. |
| Date | `Date.day` | `supported` | Returns Gregorian day of month. |
| Date | `Date.daysBetween` | `supported` | Returns whole calendar days between local Date values. |
| Date | `Date.month` | `supported` | Returns Gregorian month number. |
| Date | `Date.newInstance` | `supported` | Validates date parts in the local year 1-9999 Gregorian slice. |
| Date | `Date.toEndOfMonth` | `supported` | Returns last day of the Date value's month. |
| Date | `Date.toStartOfMonth` | `supported` | Returns first day of the Date value's month. |
| Date | `Date.today` | `supported` | Returns the deterministic VM clock date in the local UTC clock model. |
| Date | `Date.valueOf` | `supported` | Parses strict yyyy-MM-dd strings plus yyyy-MM-dd time forms into local Date values with stable invalid-input errors. |
| Date | `Date.year` | `supported` | Returns Gregorian year. |
| Datetime | `Datetime.addDays` | `supported` | Adds whole UTC calendar days to the Datetime instant in the deterministic local model; user-timezone calendar arithmetic is not claimed. |
| Datetime | `Datetime.addHours` | `supported` | Adds whole hours to the UTC instant. |
| Datetime | `Datetime.addMilliseconds` | `supported` | Adds whole milliseconds to the UTC instant. |
| Datetime | `Datetime.addMinutes` | `supported` | Adds whole minutes to the UTC instant. |
| Datetime | `Datetime.addMonths` | `supported` | Adds whole UTC calendar months with month-end clamp in the deterministic local model; user-timezone calendar arithmetic is not claimed. |
| Datetime | `Datetime.addSeconds` | `supported` | Adds whole seconds to the UTC instant. |
| Datetime | `Datetime.addYears` | `supported` | Adds whole UTC calendar years with leap-day clamp in the deterministic local model; user-timezone calendar arithmetic is not claimed. |
| Datetime | `Datetime.date` | `supported` | Returns Date component in the deterministic current-user timezone slice: UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.dateGmt` | `supported` | Returns the UTC Date component. |
| Datetime | `Datetime.day` | `supported` | Current-user timezone component getter for UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.dayGmt` | `supported` | Returns the UTC day component. |
| Datetime | `Datetime.format` | `supported` | Deterministic UTC/fixed-offset Java-pattern slice plus modeled named-zone DST formatting for America/Los_Angeles, America/New_York, America/Chicago, America/Denver, Europe/London, Europe/Berlin, Asia/Tokyo, and Australia/Sydney; format(pattern) and format() use the current user timezone inside that slice; locale-dependent or unknown pattern/timezone cases return UnsupportedFeature. |
| Datetime | `Datetime.formatGmt` | `supported` | Deterministic UTC Java-pattern slice with stable token errors; locale-dependent pattern tokens return UnsupportedFeature rather than localized text. |
| Datetime | `Datetime.hour` | `supported` | Current-user timezone component getter for UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.hourGmt` | `supported` | Returns the UTC hour component. |
| Datetime | `Datetime.millisecond` | `supported` | Returns the millisecond component of the Datetime instant. |
| Datetime | `Datetime.minute` | `supported` | Current-user timezone component getter for UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.minuteGmt` | `supported` | Returns the UTC minute component. |
| Datetime | `Datetime.month` | `supported` | Current-user timezone component getter for UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.monthGmt` | `supported` | Returns the UTC month component. |
| Datetime | `Datetime.newInstance` | `supported` | Validates integer and Date+Time parts, constructing through the current user timezone for UTC, fixed offsets, and the modeled named-zone slice with deterministic DST gap/overlap handling. |
| Datetime | `Datetime.newInstanceGmt` | `supported` | Constructs a UTC Datetime from integer or Date+Time parts. |
| Datetime | `Datetime.now` | `supported` | Returns the deterministic VM clock timestamp in the local UTC clock model. |
| Datetime | `Datetime.second` | `supported` | Current-user timezone component getter for UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.secondGmt` | `supported` | Returns the UTC second component. |
| Datetime | `Datetime.time` | `supported` | Returns Time component in the deterministic current-user timezone slice: UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.timeGmt` | `supported` | Returns the UTC Time component. |
| Datetime | `Datetime.valueOf` | `supported` | Parses supported strict datetime strings with stable invalid-input errors. |
| Datetime | `Datetime.valueOfGmt` | `supported` | Parses supported strict UTC/RFC3339 datetime strings with stable invalid-input errors. |
| Datetime | `Datetime.year` | `supported` | Current-user timezone component getter for UTC, fixed offsets, and modeled named zones; unsupported user zones return typed diagnostics. |
| Datetime | `Datetime.yearGmt` | `supported` | Returns the UTC year component. |
| Decimal | `Decimal.abs` | `supported` | Absolute value for local Decimal values. |
| Decimal | `Decimal.doubleValue` | `supported` | Returns local Decimal value. |
| Decimal | `Decimal.format()` | `supported` | Formats finite local Decimal values with deterministic base-10 output and no locale grouping. |
| Decimal | `Decimal.intValue` | `supported` | Truncates to 32-bit Integer with overflow checks. |
| Decimal | `Decimal.longValue` | `supported` | Truncates to local Long representation with overflow checks. |
| Decimal | `Decimal.pow` | `supported` | Power with Integer exponent for finite local Decimal results. |
| Decimal | `Decimal.round` | `supported` | Default half-up plus all local RoundingMode values for finite local Decimal values, using deterministic base-10 local value ties. |
| Decimal | `Decimal.setScale` | `supported` | Supports finite local Decimal scale 0-15 with UP, DOWN, CEILING, FLOOR, HALF_UP, HALF_DOWN, HALF_EVEN, and UNNECESSARY; larger arbitrary-precision scales return UnsupportedFeature. |
| Decimal | `Decimal.valueOf` | `supported` | Parses finite decimal strings and numeric values, including trimmed signed strings. |
| Decimal | `Decimal/Double.format overloads` | `unsupported` | Locale, grouping, and pattern overloads return explicit UnsupportedFeature diagnostics; localized numeric formatting is not modeled. |
| Double | `Double.format()` | `supported` | Formats finite local Double values with deterministic base-10 output using the local numeric representation. |
| Double | `Double.valueOf` | `supported` | Parses finite decimal strings and numeric values into the local numeric representation, including trimmed signed strings. |
| EncodingUtil | `EncodingUtil.base64Decode` | `supported` | Blob-shaped local value with stable invalid-input errors. |
| EncodingUtil | `EncodingUtil.base64Encode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertFromHex` | `supported` | Blob-shaped local value with stable odd-length and invalid-input errors. |
| EncodingUtil | `EncodingUtil.convertToHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.urlDecode UTF-8/ASCII/Latin-1` | `supported` | Decodes application/x-www-form-urlencoded text for bounded UTF-8, US-ASCII, and ISO-8859-1 charset aliases with stable invalid-byte errors. |
| EncodingUtil | `EncodingUtil.urlDecode other charsets` | `unsupported` | Charsets outside the local UTF-8, US-ASCII, and ISO-8859-1 slice return explicit UnsupportedFeature diagnostics. |
| EncodingUtil | `EncodingUtil.urlEncode UTF-8/ASCII/Latin-1` | `supported` | Encodes application/x-www-form-urlencoded text for bounded UTF-8, US-ASCII, and ISO-8859-1 charset aliases with stable unencodable-code-point errors. |
| EncodingUtil | `EncodingUtil.urlEncode other charsets` | `unsupported` | Charsets outside the local UTF-8, US-ASCII, and ISO-8859-1 slice return explicit UnsupportedFeature diagnostics. |
| EventBus | `EventBus.publish` | `unsupported` | Platform event publish and after-commit publish calls return fixture-backed UnsupportedFeature diagnostics. |
| Exception | `Built-in exception types` | `supported` | Known public built-in exception tokens construct message-bearing local exceptions, assign to Exception, and resolve through Type.forName/isAssignableFrom; unknown exception tokens return null from Type.forName. |
| Exception | `Exception.getCause` | `supported` | Returns the locally initialized cause value after one-shot initCause, including null causes. |
| Exception | `Exception.getLineNumber` | `supported` | Returns deterministic local throw-site line metadata for caught/thrown exceptions and 0 for constructed-only values. |
| Exception | `Exception.getMessage` | `supported` | Returns the local exception message. |
| Exception | `Exception.getStackTraceString` | `supported` | Returns the local VM stack trace captured at throw time and an empty string for constructed-only values. |
| Exception | `Exception.getTypeName` | `supported` | Returns the local exception type name without System namespace prefix, including System-prefixed constructed values. |
| Exception | `Exception.initCause` | `supported` | Stores one local Exception cause or null, returns the receiver, and throws catchable local exceptions for repeat initialization or self-causation. |
| Exception | `Exception.toString` | `supported` | Returns System-prefixed local exception type and message text. |
| FeatureManagement | `FeatureManagement.checkPermission` | `supported` | Checks local current-user and runAs permission-list state. |
| HTTP | `Http.send` | `partial` | Mock-first local callouts with request validation and callout accounting; real network transport remains explicitly unsupported. |
| HTTP | `HttpRequest` | `partial` | Deterministic constructor defaults plus endpoint, method, compressed flag, case-insensitive headers/header keys, timeout validation/defaults, body, and blob body accessors; client-certificate and static-resource callout surfaces remain explicit unsupported seams. |
| HTTP | `HttpResponse` | `supported` | Deterministic constructor defaults plus status, status code, case-insensitive headers/header keys, body, and blob body accessors for local mock responses. |
| Id | `Id.getSObjectType local prefixes` | `supported` | Resolves local schema key prefixes and the modeled standard prefix table to Schema.SObjectType tokens. |
| Id | `Id.getSObjectType unmodeled prefixes` | `unsupported` | Unknown shape-valid or unmodeled platform prefixes return a stable StringException rather than guessed object types. |
| Id | `Id.to15` | `supported` | Converts validated 18-character IDs to their 15-character prefix. |
| Id | `Id.to18` | `supported` | Adds or preserves the documented 3-character checksum for validated IDs. |
| Id | `Id.valueOf` | `supported` | Validates 15-character IDs and strict 18-character checksum suffixes; restoreCasing rebuilds casing from checksum suffixes. |
| Integer | `Integer.MAX_VALUE` | `supported` | Exposes the public 32-bit Integer maximum constant. |
| Integer | `Integer.MIN_VALUE` | `supported` | Exposes the public 32-bit Integer minimum constant. |
| Integer | `Integer.doubleValue` | `supported` | Converts local Integer values to the local numeric representation. |
| Integer | `Integer.format()` | `supported` | Formats local Integer values with deterministic base-10 output and no locale grouping. |
| Integer | `Integer.valueOf` | `supported` | Parses integer strings and numeric values with 32-bit overflow checks, including trimmed signed strings. |
| Integer | `Integer/Long.format overloads` | `unsupported` | Locale, grouping, and pattern overloads return explicit UnsupportedFeature diagnostics; localized numeric formatting is not modeled. |
| Iterator | `Iterator.hasNext` | `supported` | Checks remaining elements in a local collection snapshot. |
| Iterator | `Iterator.next` | `supported` | Returns the next element from a local collection snapshot and raises NoSuchElementException when exhausted. |
| Iterator | `Iterator.remove` | `unsupported` | Returns an explicit UnsupportedFeature diagnostic; mutating collection iterators are not modeled. |
| JSON | `JSON.createGenerator` | `supported` | Creates deterministic local JSONGenerator instances. |
| JSON | `JSON.createParser` | `supported` | Creates deterministic local JSONParser token streams and throws catchable JSONException for malformed JSON input. |
| JSON | `JSON.deserialize` | `supported` | Primitive, platform scalar, List, Set, nested Map<String,Object/value>, SObject, and registered or fixture source-backed Apex class shapes; catchable mapping errors for mismatched typed shapes and stable unsupported errors for unknown object targets. |
| JSON | `JSON.deserializeStrict` | `supported` | Rejects duplicate object fields and throws catchable JSONException for unknown fields on supported schema/class targets, including inherited and nested source-backed class fields. |
| JSON | `JSON.deserializeUntyped` | `supported` | Maps JSON into local primitive/list/map values with deterministic null and number handling, and throws catchable JSONException for malformed input. |
| JSON | `JSON.serialize` | `supported` | Compact output for supported primitive/list/set/map/object values, including suppressApexObjectNulls for object fields while preserving map/list nulls. |
| JSON | `JSON.serializePretty` | `supported` | Pretty output for supported primitive/list/set/map/object values with object-field null suppression and map/list null preservation. |
| JSON | `JSONGenerator` | `supported` | Object/array boundaries, field names, scalar string/number/Boolean/null, Date/Datetime/Time/Id/Blob, Object and validated raw value writers, getAsString, close, isClosed, and catchable JSONException for invalid nesting, pending fields, repeated roots, raw-value errors, and writes after close. |
| JSON | `JSONParser` | `supported` | Token navigation, current token/name/text, integer/long/decimal/double/Boolean/date/datetime/time/id/blob accessors, nextValue, skipChildren current-name state, clearCurrentToken, and catchable JSONException for wrong-token or malformed-input errors. |
| JSON | `JSONToken` | `supported` | Common parser token constants for object, array, field, string, number, Boolean, and null tokens. |
| Limits | `Limits modeled getters` | `supported` | SOQL, DML, heap, CPU, callout, future, queueable, batch, scheduled, aggregate async, and email counters plus limits are modeled for the local runtime. |
| Limits | `Limits unmodeled getters` | `unsupported` | Aggregate query, SOSL, query-locator, mobile push, find-similar, savepoint rollback, and publish-immediate getters return explicit UnsupportedFeature diagnostics. |
| List | `List.add` | `supported` | Adds typed local values, including indexed insertion. |
| List | `List.addAll` | `supported` | Appends typed values from local List or Set values. |
| List | `List.clear` | `supported` | Removes all local list elements. |
| List | `List.clone` | `supported` | Copies the local list container; elements keep local value identity. |
| List | `List.contains` | `supported` | Checks local values, including null, using local value equality. |
| List | `List.copyConstructor` | `supported` | Copies values from a local List or Set constructor argument. |
| List | `List.deepClone` | `supported` | No-argument local recursive clone for primitive, collection, and SObject graphs; SObject preserve-option overloads return explicit UnsupportedFeature diagnostics. |
| List | `List.get` | `supported` | Indexed lookup with stable bounds errors. |
| List | `List.indexOf` | `supported` | Local equality search, including null elements, with -1 for misses. |
| List | `List.isEmpty` | `supported` | Checks local list length. |
| List | `List.iterator` | `supported` | Returns a deterministic snapshot Iterator for local List values. |
| List | `List.remove` | `supported` | Indexed removal returns the removed value. |
| List | `List.set` | `supported` | Indexed replacement with typed value coercion. |
| List | `List.size` | `supported` | Returns local list length. |
| List | `List.sort` | `supported` | Deterministic sort for local primitive comparable values; object/Comparable sorting returns an explicit UnsupportedFeature diagnostic. |
| LoggingLevel | `LoggingLevel.name` | `supported` | Returns deterministic built-in enum member text. |
| LoggingLevel | `LoggingLevel.ordinal` | `supported` | Returns deterministic built-in enum order for the local logging level set. |
| LoggingLevel | `LoggingLevel.toString` | `supported` | Returns deterministic built-in enum member text. |
| LoggingLevel | `LoggingLevel.values` | `supported` | Returns NONE, ERROR, WARN, INFO, DEBUG, FINE, FINER, FINEST in deterministic order. |
| Long | `Long.MAX_VALUE` | `supported` | Exposes the public 64-bit Long maximum constant. |
| Long | `Long.MIN_VALUE` | `supported` | Exposes the public 64-bit Long minimum constant. |
| Long | `Long.format()` | `supported` | Formats local Long values with deterministic base-10 output and no locale grouping. |
| Long | `Long.valueOf` | `supported` | Parses integer strings and numeric values with overflow checks, including trimmed signed strings. |
| Map | `Map.clear` | `supported` | Removes all local map entries. |
| Map | `Map.clone` | `supported` | Copies the local map container; values keep local identity. |
| Map | `Map.containsKey` | `supported` | Checks local keys, including null keys, using deterministic key encoding. |
| Map | `Map.containsValue` | `supported` | Checks local values, including null values, using local value equality. |
| Map | `Map.copyConstructor` | `supported` | Copies entries from a local Map and supports Map<Id,SObject> construction from List<SObject> with non-null unique Ids. |
| Map | `Map.deepClone` | `supported` | No-argument local recursive clone for primitive, collection, and SObject graphs; SObject preserve-option overloads return explicit UnsupportedFeature diagnostics. |
| Map | `Map.get` | `supported` | Returns local value or null for missing keys. |
| Map | `Map.isEmpty` | `supported` | Checks local map size. |
| Map | `Map.keySet` | `supported` | Returns deterministic local key Set, preserving null keys. |
| Map | `Map.put` | `supported` | Stores typed local entries and returns the previous value. |
| Map | `Map.putAll` | `supported` | Copies local entries from another Map or SObject rows into Map<Id,SObject> by non-null unique Id. |
| Map | `Map.remove` | `supported` | Removes a key and returns its previous local value or null. |
| Map | `Map.size` | `supported` | Returns local map size. |
| Map | `Map.toString` | `supported` | Deterministic local entry string form for primitive, null, and nested collection values. |
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
| Messaging | `Messaging.MassEmailMessage` | `partial` | Common target, whatId, template, description, opt-out, and activity setters populate the local mass-message shape; no delivery transport. |
| Messaging | `Messaging.SendEmailResult` | `supported` | Local success result has deterministic constructor defaults and exposes isSuccess and getErrors getters. |
| Messaging | `Messaging.SingleEmailMessage` | `partial` | Common address, body, threading, template-reference, activity, signature, opt-out, priority, BCC sender, and attachment setters populate the local message shape; no delivery transport. |
| Messaging | `Messaging.sendEmail` | `partial` | Single and mass message list overloads, including Boolean allOrNothing, validate local message items, return one local SendEmailResult per message, and increment email limits; transport/template APIs and SendEmailOptions surfaces return explicit unsupported diagnostics. |
| Object | `Object.equals` | `supported` | Uses local value equality for primitives, collections, platform scalars, and object identity. |
| Object | `Object.hashCode` | `supported` | Deterministic within local value equality; object identity hashes are request-local. |
| Object | `Object.toString` | `supported` | Returns local string forms for primitives, collections, platform scalars, and objects. |
| PageReference | `PageReference` | `supported` | Constructor defaults, URL, redirect, typed local parameters and headers, currentPage/setCurrentPage storage, and URL-backed toString/String.valueOf are supported for the VM-local page model. |
| PageReference | `Visualforce rendering and PDF content` | `unsupported` | PageReference.getContent and getContentAsPDF return fixture-backed UnsupportedFeature diagnostics; no Visualforce rendering or PDF service is faked. |
| Pattern | `Matcher.appendReplacement/appendTail` | `unsupported` | Java StringBuffer append-position semantics return explicit unsupported errors. |
| Pattern | `Matcher.end` | `supported` | Local Go-regexp group end positions, including -1 for optional unmatched groups and stable invalid-group errors. |
| Pattern | `Matcher.find` | `supported` | Local Go-regexp matching with captured groups, region bounds, find(start) reset behavior, anchoring/transparent bound handling, and compiled flag state. |
| Pattern | `Matcher.group` | `supported` | Local Go-regexp group access with stale match state cleared after failed find/matches/lookingAt calls. |
| Pattern | `Matcher.groupCount` | `supported` | Capturing group count from the compiled local Go-regexp Pattern. |
| Pattern | `Matcher.hasAnchoringBounds` | `supported` | Returns the local anchoring-bounds flag, defaulting to true. |
| Pattern | `Matcher.hasTransparentBounds` | `supported` | Returns the local transparent-bounds flag, defaulting to false. |
| Pattern | `Matcher.lookingAt` | `supported` | Beginning-of-region local Go-regexp match with anchoring/transparent bounds. |
| Pattern | `Matcher.matches` | `supported` | Whole-region local Go-regexp match with anchoring/transparent bounds and supported inline i/m/s flags. |
| Pattern | `Matcher.region` | `supported` | Bounds matching to a local rune-indexed region, including anchored-region and transparent word-boundary cases. |
| Pattern | `Matcher.regionStart/regionEnd` | `supported` | Returns the Matcher local rune-indexed region bounds. |
| Pattern | `Matcher.replaceAll` | `supported` | Local Go-regexp replacement with region bounds, Java-style numeric capture parsing, escaped dollar/backslash handling, and named replacement references pinned unsupported. |
| Pattern | `Matcher.replaceFirst` | `supported` | Local Go-regexp first replacement with region bounds, Java-style numeric capture parsing, escaped dollar/backslash handling, and named replacement references pinned unsupported. |
| Pattern | `Matcher.reset` | `supported` | Clears local match state, resets the region to full input, and optionally replaces input. |
| Pattern | `Matcher.start` | `supported` | Local Go-regexp group start positions, including -1 for optional unmatched groups and stable invalid-group errors. |
| Pattern | `Matcher.useAnchoringBounds` | `supported` | Stores the local bounds flag and toggles whether ^/$ bind to region edges or full input edges. |
| Pattern | `Matcher.usePattern` | `supported` | Swaps the local Go-regexp Pattern, including compiled flag/literal state, and resets search state within the current region. |
| Pattern | `Matcher.useTransparentBounds` | `supported` | Stores the local bounds flag and toggles whether word-boundary checks use opaque region edges or full input context. |
| Pattern | `Pattern.compile Java regex dialect gaps` | `unsupported` | Java-only regex features, including lookaround, backreferences, named groups, possessive quantifiers, atomic groups, quote escapes, previous-match boundaries, Java-only inline flags, unsupported flag constants, Java Unicode character classes, class intersections, linebreak/grapheme escapes, and horizontal/vertical whitespace classes, return stable UnsupportedFeature diagnostics. |
| Pattern | `Pattern.compile local regexp dialect` | `supported` | Go regexp syntax with supported CASE_INSENSITIVE, MULTILINE, DOTALL, LITERAL, and UNICODE_CASE flag handling for the local Pattern/Matcher model. |
| Pattern | `Pattern.matcher` | `supported` | Creates a Matcher for the compiled local Go-regexp Pattern. |
| Pattern | `Pattern.matches` | `supported` | Whole-string local Go-regexp match with pinned UnsupportedFeature diagnostics for Java-only syntax. |
| Pattern | `Pattern.pattern` | `supported` | Returns original regex source, including for locally quoted Pattern.LITERAL compilation. |
| Pattern | `Pattern.quote` | `supported` | Returns a Go regexp-escaped literal pattern for local Pattern/Matcher use. |
| Pattern | `Pattern.split` | `supported` | Local Go-regexp split with limit semantics, compiled flag/literal state, Java-only regex fences, and nullable delimiter regexes fenced unsupported. |
| Pattern | `PatternSyntaxException` | `supported` | Invalid local regex syntax throws a catchable PatternSyntaxException with getDescription, getIndex, getPattern, getMessage, and exception hierarchy behavior. |
| QuickAction | `QuickAction namespace` | `unsupported` | Quick action UI execution and discovery calls return fixture-backed UnsupportedFeature diagnostics. |
| REST | `@RestResource local server dispatch` | `unsupported` | Custom Apex REST dispatch returns a stable unsupported error from the local server. |
| REST | `RestContext.request / RestContext.response` | `supported` | VM-local static request/response slots support RestRequest/RestResponse assignment, null clearing, nested field access, and lazy RestResponse creation after reset; platform dispatch lifecycle remains explicitly out of scope. |
| REST | `RestRequest / RestResponse object shapes` | `supported` | Local request/response objects expose URI/path/method/address, params, deterministic key helpers, case-insensitive headers/getHeader helpers, Blob body, status, null-map rebuilds, and compatibility-fixture coverage; broader Apex REST dispatch lifecycle remains unsupported. |
| RoundingMode | `RoundingMode.name` | `supported` | Returns deterministic built-in enum member text for Decimal rounding modes. |
| RoundingMode | `RoundingMode.ordinal` | `supported` | Returns deterministic built-in enum order for UP, DOWN, CEILING, FLOOR, HALF_UP, HALF_DOWN, HALF_EVEN, and UNNECESSARY. |
| RoundingMode | `RoundingMode.toString` | `supported` | Returns deterministic built-in enum member text for Decimal rounding modes. |
| RoundingMode | `RoundingMode.valueOf` | `supported` | Constructs built-in Decimal rounding-mode enum tokens by exact name with stable invalid-name errors. |
| RoundingMode | `RoundingMode.values` | `supported` | Returns UP, DOWN, CEILING, FLOOR, HALF_UP, HALF_DOWN, HALF_EVEN, and UNNECESSARY in deterministic order. |
| Schema | `DescribeFieldResult` | `supported` | Fixture-backed local field metadata covers names, labels, types, nillable/external-id/unique flags, reference targets, relationship names, picklist entries, and access booleans. |
| Schema | `DescribeFieldResult dependent picklist metadata` | `unsupported` | getController/getControllerValues return fixture-backed UnsupportedFeature diagnostics until dependent picklist controller metadata is modeled. |
| Schema | `DescribeSObjectResult` | `supported` | Fixture-backed local object metadata covers names, labels, key prefixes, fields, record types, child relationships, and common access booleans. |
| Schema | `DescribeSObjectResult field sets` | `unsupported` | fieldSets.getMap returns a fixture-backed UnsupportedFeature diagnostic until local field set metadata is modeled. |
| Schema | `Schema.describeSObjects` | `supported` | Fixture-backed local schema object-name and SObjectType-token lists return DescribeSObjectResult values. |
| Schema | `Schema.getGlobalDescribe` | `supported` | Fixture-backed local schema map returns SObjectType tokens keyed by object API name. |
| Search | `Search.* / SOSL FIND` | `unsupported` | Cloud search and SOSL execution are not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| Set | `Set.add` | `supported` | Adds typed local values and reports whether the Set changed. |
| Set | `Set.addAll` | `supported` | Adds typed values from local List or Set values. |
| Set | `Set.clear` | `supported` | Removes all local set elements. |
| Set | `Set.clone` | `supported` | Copies the local set container; elements keep local value identity. |
| Set | `Set.contains` | `supported` | Checks local values, including null, using local value equality. |
| Set | `Set.containsAll` | `supported` | Checks local List or Set membership. |
| Set | `Set.copyConstructor` | `supported` | Copies unique values from a local List or Set constructor argument. |
| Set | `Set.deepClone` | `supported` | No-argument local recursive clone for primitive, collection, and SObject graphs; SObject preserve-option overloads return explicit UnsupportedFeature diagnostics. |
| Set | `Set.isEmpty` | `supported` | Checks local set length. |
| Set | `Set.iterator` | `supported` | Returns a deterministic snapshot Iterator for local Set values. |
| Set | `Set.remove` | `supported` | Removes a local value, including null, and reports whether the Set changed. |
| Set | `Set.removeAll` | `supported` | Removes local List or Set members. |
| Set | `Set.retainAll` | `supported` | Retains only local List or Set members. |
| Set | `Set.size` | `supported` | Returns local set length. |
| String | `String.abbreviate` | `supported` | Apache-style abbreviation for one- and two-argument forms. |
| String | `String.center` | `supported` | Centers text to a requested rune width with space or supplied pad text. |
| String | `String.charAt` | `supported` | Rune-indexed one-character String. |
| String | `String.codePointAt` | `supported` | Rune-indexed Unicode code point. |
| String | `String.codePointBefore` | `supported` | Rune-indexed previous Unicode code point. |
| String | `String.codePointCount` | `supported` | Counts runes between validated indexes. |
| String | `String.commonPrefix` | `supported` | Returns the shared rune prefix for two strings. |
| String | `String.compareTo` | `supported` | Lexicographic comparison for local string values. |
| String | `String.contains` | `supported` | UTF-8 string contains. |
| String | `String.containsAny` | `supported` | Rune membership. |
| String | `String.containsIgnoreCase` | `supported` | Case-insensitive containment using local Unicode lowercasing. |
| String | `String.containsNone` | `supported` | Rune exclusion. |
| String | `String.containsOnly` | `supported` | Rune allow-list. |
| String | `String.containsWhitespace` | `supported` | Unicode whitespace. |
| String | `String.countMatches` | `supported` | Non-overlapping literal substring count. |
| String | `String.difference` | `supported` | Returns the differing suffix from the comparison string. |
| String | `String.endsWith` | `supported` | UTF-8 string suffix. |
| String | `String.endsWithIgnoreCase` | `supported` | Case-insensitive suffix check using local Unicode lowercasing. |
| String | `String.equals` | `supported` | Exact local string equality. |
| String | `String.equalsIgnoreCase` | `supported` | Unicode simple fold. |
| String | `String.escapeCsv` | `supported` | One-field RFC4180-style quoting for comma, quote, CR, and LF; doubles embedded quotes. |
| String | `String.escapeEcmaScript` | `supported` | JavaScript-style backslash, quote, slash, control, and UTF-16 Unicode escaping for local string values. |
| String | `String.escapeHtml3` | `supported` | Local core-markup escaping for &, <, >, double quote, and apostrophe; broad named-entity expansion is intentionally not modeled. |
| String | `String.escapeHtml4` | `supported` | Local core-markup escaping for &, <, >, double quote, and apostrophe; broad named-entity expansion is intentionally not modeled. |
| String | `String.escapeJava` | `supported` | Java-style backslash, double quote, control, and UTF-16 Unicode escaping for local string values. |
| String | `String.escapeSingleQuotes` | `supported` | Escapes single quotes with backslashes. |
| String | `String.escapeUnicode` | `supported` | Escapes non-ASCII and control runes as UTF-16 Unicode escape sequences. |
| String | `String.escapeXml` | `supported` | Escapes XML core entities without XML 1.0/1.1 invalid-code-point filtering; use escapeXml10/11 for versioned filtering. |
| String | `String.escapeXml10` | `supported` | Escapes XML core entities, drops XML 1.0-invalid code points, and numeric-escapes restricted control ranges. |
| String | `String.escapeXml11` | `supported` | Escapes XML core entities, drops XML 1.1-invalid nulls, and numeric-escapes restricted control ranges. |
| String | `String.format` | `supported` | Deterministic MessageFormat subset for numeric List placeholders, repeated and missing arguments, and apostrophe quoting; typed number/date/time/choice and locale-sensitive format elements return explicit UnsupportedFeature diagnostics. |
| String | `String.fromCharArray` | `supported` | Builds a string from valid Unicode code point integers. |
| String | `String.getChars` | `supported` | Returns Unicode code point integers for each rune. |
| String | `String.getCommonPrefix` | `supported` | Returns the shared rune prefix for a list of strings. |
| String | `String.getLevenshteinDistance` | `supported` | Rune-based edit distance, including threshold overloads that return -1 when exceeded. |
| String | `String.hashCode` | `supported` | Java-compatible UTF-16 code-unit string hash for local values. |
| String | `String.indexOf` | `supported` | Rune-indexed literal search with optional start position and empty-search edge handling. |
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
| String | `String.isEmpty` | `supported` | Null and empty string. |
| String | `String.isNotBlank` | `supported` | Null and whitespace. |
| String | `String.isNotEmpty` | `supported` | Null and empty string. |
| String | `String.isNumeric` | `supported` | Unicode decimal digits. |
| String | `String.isNumericSpace` | `supported` | Unicode decimal digits and space characters. |
| String | `String.isWhitespace` | `supported` | Unicode whitespace; empty string is true. |
| String | `String.join` | `supported` | List values and separator. |
| String | `String.lastIndexOf` | `supported` | Rune-indexed reverse literal search with optional start position and empty-search edge handling. |
| String | `String.lastIndexOfAny` | `supported` | Returns the last rune index whose character appears in the search set. |
| String | `String.lastOrdinalIndexOf` | `supported` | Finds the nth occurrence from the end using rune indexes. |
| String | `String.left` | `supported` | Returns the leftmost requested rune count, clamped to string length. |
| String | `String.leftPad` | `supported` | Pads on the left to a requested rune width with space or supplied pad text. |
| String | `String.length` | `supported` | Counts runes. |
| String | `String.mid` | `supported` | Returns a clamped rune-indexed middle slice by start and length. |
| String | `String.ordinalIndexOf` | `supported` | Finds the nth occurrence from the start using rune indexes. |
| String | `String.overlay` | `supported` | Overlays text between clamped rune indexes. |
| String | `String.remove` | `supported` | Literal non-overlapping removal; empty targets are a no-op. |
| String | `String.removeEnd` | `supported` | Removes a literal suffix when present. |
| String | `String.removeEndIgnoreCase` | `supported` | Removes a literal suffix with rune-wise case folding when present. |
| String | `String.removeIgnoreCase` | `supported` | Literal non-overlapping removal with rune-wise case folding. |
| String | `String.removeStart` | `supported` | Removes a literal prefix when present. |
| String | `String.removeStartIgnoreCase` | `supported` | Removes a literal prefix with rune-wise case folding when present. |
| String | `String.repeat` | `supported` | Repeats a string with optional separator and non-negative count. |
| String | `String.replace` | `supported` | Literal replacement; empty targets are a no-op. |
| String | `String.replaceAll` | `supported` | Local Go-regexp replacement with Java-style numeric capture parsing; Java-only regex features and named replacement references are explicitly unsupported. |
| String | `String.replaceFirst` | `supported` | Local Go-regexp first replacement with Java-style numeric capture parsing; Java-only regex features and named replacement references are explicitly unsupported. |
| String | `String.replaceIgnoreCase` | `supported` | Literal non-overlapping replacement with rune-wise case folding. |
| String | `String.replaceOnce` | `supported` | First literal replacement only. |
| String | `String.reverse` | `supported` | Reverses rune order. |
| String | `String.right` | `supported` | Returns the rightmost requested rune count, clamped to string length. |
| String | `String.rightPad` | `supported` | Pads on the right to a requested rune width with space or supplied pad text. |
| String | `String.rotate` | `supported` | Rune-based rotation; positive shifts rotate right. |
| String | `String.split` | `supported` | Local Go-regexp split with Apex limit shape; nullable regexes and Java-only regex features are explicitly unsupported. |
| String | `String.splitByCharacterType` | `supported` | Splits on coarse Unicode upper/lower/digit/space/other groups. |
| String | `String.splitByCharacterTypeCamelCase` | `supported` | Splits character types with camel-case upper-to-lower adjustment. |
| String | `String.startsWith` | `supported` | UTF-8 string prefix. |
| String | `String.startsWithIgnoreCase` | `supported` | Case-insensitive prefix check using local Unicode lowercasing. |
| String | `String.strip` | `supported` | Strips Unicode whitespace or a supplied rune set from both ends. |
| String | `String.stripAll` | `supported` | Strips each string in a list with optional strip-character set. |
| String | `String.stripEnd` | `supported` | Strips Unicode whitespace or a supplied rune set from the end. |
| String | `String.stripStart` | `supported` | Strips Unicode whitespace or a supplied rune set from the start. |
| String | `String.stripToEmpty` | `supported` | Strips whitespace and returns an empty string for all-blank values. |
| String | `String.stripToNull` | `supported` | Strips whitespace and returns null for all-blank values. |
| String | `String.substring` | `supported` | Rune-indexed substring. |
| String | `String.substringAfter` | `supported` | Returns text after the first literal separator, empty when absent. |
| String | `String.substringAfterLast` | `supported` | Returns text after the last literal separator, empty when absent. |
| String | `String.substringBefore` | `supported` | Returns text before the first literal separator, original text when absent. |
| String | `String.substringBeforeLast` | `supported` | Returns text before the last literal separator, original text when absent. |
| String | `String.substringBetween` | `supported` | Returns the first literal bracketed substring for one-tag and open/close forms, or null when absent. |
| String | `String.swapCase` | `supported` | Unicode upper/lower rune case swap. |
| String | `String.toLowerCase` | `supported` | Go Unicode lowercasing. |
| String | `String.toUpperCase` | `supported` | Go Unicode uppercasing. |
| String | `String.trim` | `supported` | Unicode whitespace trim. |
| String | `String.unescapeCsv` | `supported` | Unquotes one doubled-quote CSV field; plain strings are unchanged. |
| String | `String.unescapeEcmaScript` | `supported` | Unescapes JavaScript-style backslash, octal, slash, quote, and UTF-16 Unicode escape sequences. |
| String | `String.unescapeHtml3` | `supported` | Unescapes core HTML entities, numeric references, and a pinned high-use named-entity table; HTML apos and unlisted names stay unchanged. |
| String | `String.unescapeHtml4` | `supported` | Unescapes core HTML entities, numeric references, and a pinned high-use named-entity table; HTML apos and unlisted names stay unchanged. |
| String | `String.unescapeJava` | `supported` | Unescapes Java-style backslash, octal, quote, slash, and UTF-16 Unicode escape sequences. |
| String | `String.unescapeUnicode` | `supported` | Unescapes UTF-16 Unicode escape sequences, including surrogate-pair sequences. |
| String | `String.unescapeXml` | `supported` | Unescapes XML core entities and Unicode-valid numeric references while leaving malformed, null, surrogate, and out-of-range references unchanged. |
| String | `String.unescapeXml10` | `supported` | Unescapes XML core entities and XML 1.0-valid numeric references while leaving XML 1.0-invalid references unchanged. |
| String | `String.unescapeXml11` | `supported` | Unescapes XML core entities and XML 1.1-valid numeric references while leaving XML 1.1-invalid references unchanged. |
| String | `String.valueOf` | `supported` | Local value string conversion. |
| System | `System async lifecycle controls` | `unsupported` | Completed and unknown aborts plus scheduleBatch deferral return explicit unsupported diagnostics; broader async lifecycle control is not modeled. |
| System | `System.abortJob queued local async` | `supported` | Removes queued local Queueable and Schedulable jobs before Test.stopTest and marks local AsyncApexJob/CronTrigger rows consistently. |
| System | `System.assert` | `supported` | Assertion failure returns runtime error; Object and null message values use deterministic local string conversion. |
| System | `System.assertEquals` | `supported` | Assertion failure returns runtime error with deterministic local expected/actual text and Object/null message conversion. |
| System | `System.assertNotEquals` | `supported` | Assertion failure returns runtime error with deterministic local value text and Object/null message conversion. |
| System | `System.currentTimeMillis` | `supported` | Returns deterministic VM-clock epoch milliseconds. |
| System | `System.debug` | `supported` | One-argument and LoggingLevel overloads are collected in result debug output; null, LoggingLevel, and modeled Exception values use deterministic string forms; log framework text parity is not claimed. |
| System | `System.isBatch` | `supported` | Reflects the local batch drain context and returns false outside batch execution. |
| System | `System.isFuture` | `supported` | Reflects the local future drain context and returns false outside future execution. |
| System | `System.isQueueable` | `supported` | Reflects the local queueable drain context and returns false outside queueable execution. |
| System | `System.isScheduled` | `supported` | Reflects the local scheduled drain context and returns false outside scheduled execution. |
| System | `System.now` | `supported` | Returns deterministic VM-clock Datetime. |
| System | `System.today` | `supported` | Returns deterministic VM-clock Date. |
| Test | `Test.clearApexPageMessages` | `supported` | Clears VM-local ApexPages messages in test context. |
| Test | `Test.createSoqlStub` | `unsupported` | SOQL stub creation is not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| Test | `Test.createStub` | `unsupported` | Dynamic stub creation is not locally modeled; calls return explicit UnsupportedFeature diagnostics. |
| Test | `Test.getStandardPricebookId` | `supported` | Returns the deterministic local standard pricebook Id in test context and errors outside test context. |
| Test | `Test.isRunningTest` | `supported` | Reflects local test context. |
| Test | `Test.setCurrentPage` | `supported` | Sets the VM-local ApexPages current PageReference in test context. |
| Test | `Test.setFixedSearchResults` | `unsupported` | Fixed SOSL search results are deferred with the local search surface; calls return explicit UnsupportedFeature diagnostics. |
| Test | `Test.setMock HttpCalloutMock` | `supported` | String and Type-token HttpCalloutMock registrations route local test callouts to the supplied mock instance. |
| Test | `Test.setMock non-HTTP mocks` | `unsupported` | Other mock interfaces return explicit unsupported diagnostics instead of fake local service behavior. |
| Test | `Test.startTest` | `supported` | Resets the local inner governor window once per test method and preserves the outer counter window. |
| Test | `Test.stopTest` | `supported` | Drains supported local async work once per test method and restores the outer governor window. |
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
| TimeZone | `TimeZone.getDisplayName` | `supported` | Returns deterministic ID text for UTC, fixed GMT offsets, and the modeled named-zone table; the daylight Boolean overload returns modeled abbreviations, while locale/style overloads return UnsupportedFeature. |
| TimeZone | `TimeZone.getID` | `supported` | Returns canonical UTC, fixed GMT offset ID, or one of the modeled named-zone IDs in the deterministic local slice. |
| TimeZone | `TimeZone.getOffset` | `supported` | Returns fixed offset milliseconds for UTC/GMT offsets through ±14:00 and modeled DST decisions for the supported US, Europe, Tokyo, and Sydney named-zone slice; other named zones return UnsupportedFeature. |
| TimeZone | `TimeZone.getTimeZone` | `supported` | Supports UTC/GMT, fixed GMT/UTC offsets through ±14:00, and America/Los_Angeles, America/New_York, America/Chicago, America/Denver, Europe/London, Europe/Berlin, Asia/Tokyo, and Australia/Sydney; other named zones and trimmed/invalid IDs return UnsupportedFeature. |
| Type | `Type.equals` | `supported` | Compares local Type tokens by type name. |
| Type | `Type.forName` | `supported` | Local class/type token lookup, System namespace built-in tokens, common local SObjects, built-in and generic collection type strings, null for null/blank/unknown local names, and package tokens for explicit reflection fences. |
| Type | `Type.getName` | `supported` | Returns local type token name. |
| Type | `Type.hashCode` | `supported` | Matches the local String.hashCode of the type name. |
| Type | `Type.isAssignableFrom` | `supported` | Uses the local class/interface and built-in exception hierarchy. |
| Type | `Type.newInstance` | `supported` | Constructs local values and dispatches zero-arg constructors for registered classes; uninstantiable built-ins and unbacked namespace/package tokens return explicit UnsupportedFeature diagnostics. |
| Type | `Type.toString` | `supported` | Returns the local type token name. |
| URL | `URL` | `supported` | Constructors for deterministic absolute URL specs, context/spec resolution, and protocol/host/file forms with stable malformed input errors. |
| URL | `URL.getAuthority` | `supported` | Returns parsed authority for local URL values. |
| URL | `URL.getCurrentRequestUrl` | `unsupported` | Cloud request context is not modeled; returns an explicit unsupported current-request URL error. |
| URL | `URL.getDefaultPort` | `supported` | Returns HTTP/HTTPS/FTP defaults or -1. |
| URL | `URL.getFile` | `supported` | Returns path plus query for local URL values. |
| URL | `URL.getHost` | `supported` | Returns parsed hostname for local URL values. |
| URL | `URL.getOrgDomainUrl` | `supported` | Returns the deterministic local org URL; request context is not modeled. |
| URL | `URL.getPath` | `supported` | Returns parsed path for local URL values. |
| URL | `URL.getPort` | `supported` | Returns explicit port or -1. |
| URL | `URL.getProtocol` | `supported` | Returns parsed scheme for local URL values. |
| URL | `URL.getQuery` | `supported` | Returns parsed query for local URL values. |
| URL | `URL.getRef` | `supported` | Returns parsed fragment for local URL values. |
| URL | `URL.getSalesforceBaseUrl` | `supported` | Returns the deterministic local base URL; request context is not modeled. |
| URL | `URL.toExternalForm` | `supported` | Returns the stored local URL string. |
| Unsupported | `unimplemented platform/stdlib calls` | `supported` | Typed UnsupportedFeature errors with stable message text. |
| UserInfo | `UserInfo.getFirstName` | `supported` | Returns the current runAs, server, or default local user FirstName with deterministic fallback. |
| UserInfo | `UserInfo.getLanguage` | `supported` | Returns the current runAs/server user LanguageLocaleKey or deterministic en_US default; it does not enable locale-sensitive formatting. |
| UserInfo | `UserInfo.getLastName` | `supported` | Returns the current runAs, server, or default local user LastName with deterministic fallback. |
| UserInfo | `UserInfo.getLocale` | `supported` | Returns the current runAs/server user LocaleSidKey or deterministic en_US default; it does not enable locale-sensitive formatting. |
| UserInfo | `UserInfo.getName` | `supported` | Returns the current runAs, server, or default local user Name with deterministic fallback. |
| UserInfo | `UserInfo.getOrganizationId` | `supported` | Returns the deterministic local org ID. |
| UserInfo | `UserInfo.getProfileId` | `supported` | Returns the current runAs, server, or default local user ProfileId with deterministic fallback. |
| UserInfo | `UserInfo.getSessionId` | `supported` | Returns the deterministic empty local session ID. |
| UserInfo | `UserInfo.getTimeZone` | `supported` | Returns a TimeZone object for the current user TimeZoneSidKey in the modeled UTC/fixed-offset/named-zone slice; unsupported user zones return UnsupportedFeature and do not imply broader Salesforce zone or locale behavior. |
| UserInfo | `UserInfo.getUserEmail` | `supported` | Returns the current runAs, server, or default local user Email with deterministic fallback. |
| UserInfo | `UserInfo.getUserId` | `supported` | Returns the current runAs, server, or default local user ID with deterministic fallback. |
| UserInfo | `UserInfo.getUserName` | `supported` | Returns the current runAs, server, or default local user Username, falling back to the user ID. |
