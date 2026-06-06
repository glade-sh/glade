# Standard Library Coverage

Generated from `internal/capability`.

Status values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`.

| Area | API | Status | Notes |
| --- | --- | --- | --- |
| AccessLevel | `AccessLevel.withPermissionSetId(String)` | `unsupported` | Permission-set-scoped user mode requires permission-set semantics and returns a stable UnsupportedFeature diagnostic locally. |
| Answers | `Answers.findSimilar(Question)` | `unsupported` | Zone similar-question search requires Answers service data and returns a stable UnsupportedFeature diagnostic locally. |
| ApexPages | `ApexPages.Message` | `partial` | Constructor and getters; no Visualforce rendering lifecycle. |
| ApexPages | `ApexPages.addMessage` | `supported` | Stores page messages on the VM instance. |
| ApexPages | `ApexPages.addMessage(ApexPages.Message)` | `supported` | Stores page messages on the VM instance. |
| ApexPages | `ApexPages.addMessages(Exception)` | `supported` | Converts supported exception values into VM-local page messages. |
| ApexPages | `ApexPages.addMessages(Object)` | `supported` | Converts supported exception and message values into VM-local page messages. |
| ApexPages | `ApexPages.currentPage` | `supported` | Returns a deterministic local PageReference. |
| ApexPages | `ApexPages.currentPage()` | `supported` | Returns the VM-local PageReference. |
| ApexPages | `ApexPages.getMessages` | `supported` | Returns VM-local page messages. |
| ApexPages | `ApexPages.getMessages()` | `supported` | Returns VM-local page messages. |
| ApexPages | `ApexPages.hasMessages` | `supported` | Checks VM-local page messages. |
| ApexPages | `ApexPages.hasMessages()` | `supported` | Checks VM-local page messages. |
| ApexPages | `ApexPages.hasMessages(ApexPages.Severity)` | `supported` | Checks VM-local page messages by severity. |
| Approval | `Approval.process(Approval.ProcessRequest)` | `unsupported` | Approval execution requires org approval-process metadata and returns a stable UnsupportedFeature diagnostic locally. |
| Approval | `Approval.process(Approval.ProcessRequest, Boolean)` | `unsupported` | Approval execution with allOrNone requires org approval-process metadata and returns a stable UnsupportedFeature diagnostic locally. |
| Assert | `Assert.areEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.areNotEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.fail` | `supported` | Raises local System.AssertException with optional message text. |
| Assert | `Assert.isFalse` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNotNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isTrue` | `supported` | Routes through local assertion failures with optional message text. |
| Boolean | `Boolean.valueOf(Object)` | `supported` | Converts supported local field/object values into Boolean values. |
| Boolean | `Boolean.valueOf(String)` | `supported` | Converts strings to Boolean using Apex-shaped true/false parsing. |
| BusinessHours | `BusinessHours.add(String, Datetime, Long)` | `unsupported` | Business-hours math requires BusinessHours and holiday metadata; no 24x7 default calendar is assumed. |
| BusinessHours | `BusinessHours.addGmt(String, Datetime, Long)` | `unsupported` | Business-hours GMT math requires BusinessHours and holiday metadata; no 24x7 default calendar is assumed. |
| BusinessHours | `BusinessHours.diff(String, Datetime, Datetime)` | `unsupported` | Business-hours interval calculation requires BusinessHours and holiday metadata; no 24x7 default calendar is assumed. |
| BusinessHours | `BusinessHours.isWithin(String, Datetime)` | `unsupported` | Business-hours membership calculation requires BusinessHours and holiday metadata; no 24x7 default calendar is assumed. |
| BusinessHours | `BusinessHours.nextStartDate(String, Datetime)` | `unsupported` | Business-hours reopening calculation requires BusinessHours and holiday metadata; no 24x7 default calendar is assumed. |
| Crypto | `Crypto.generateDigest` | `partial` | MD5, SHA1, and SHA-256. |
| Database | `Database.UnitOfWork` | `supported` | Queues local DML operations and applies them on commitWork; discardWork drops pending local work. |
| Database | `Database.convertLead` | `supported` | Local lead conversion creates Account, Contact, and optional Opportunity records and updates Lead conversion fields. |
| Database | `Database.countQuery` | `supported` | Dynamic SOQL count execution against the local org with local AccessLevel parsing. |
| Database | `Database.countQueryWithBinds` | `supported` | Bind-map dynamic SOQL count execution with local AccessLevel parsing. |
| Database | `Database.delete` | `supported` | DML pipeline with result/error shapes and local AccessLevel parsing for supported SObjects. |
| Database | `Database.deleteAsync` | `supported` | Local async delete alias runs through the DML pipeline and returns DeleteResult shape. |
| Database | `Database.deleteImmediate` | `supported` | Local immediate delete alias runs through the DML pipeline and returns DeleteResult shape. |
| Database | `Database.emptyRecycleBin` | `supported` | Local hard-delete result shape with allOrNone rollback for supported SObjects. |
| Database | `Database.executeBatch` | `supported` | Queues local Batchable jobs and drains start/execute chunks/finish during Test.stopTest. |
| Database | `Database.getAsyncDeleteResult` | `stub` | Returns materialized local DeleteResult values when the locator is local; unknown locators return a deterministic pending stub result. |
| Database | `Database.getAsyncLocator` | `supported` | Returns deterministic VM-local locator strings for local result and locator objects; no external async service lookup. |
| Database | `Database.getAsyncSaveResult` | `stub` | Returns materialized local SaveResult values when the locator is local; unknown locators return a deterministic pending stub result. |
| Database | `Database.getCursor` | `supported` | Local cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getCursorWithBinds` | `supported` | Bind-map local cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getDeleted` | `stub` | Returns a deterministic empty deleted-record sync window stub; full org sync tracking is not modeled. |
| Database | `Database.getPaginationCursor` | `supported` | Local pagination cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getPaginationCursorWithBinds` | `supported` | Bind-map local pagination cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getQueryLocator` | `supported` | Supported SOQL executes eagerly for local batch scopes with local AccessLevel parsing. |
| Database | `Database.getQueryLocatorWithBinds` | `supported` | Bind-map dynamic SOQL locator execution with iterable local query locators. |
| Database | `Database.getUpdated` | `stub` | Returns a deterministic empty updated-record sync window stub; full org sync tracking is not modeled. |
| Database | `Database.insert` | `supported` | DML pipeline with result/error shapes and local AccessLevel parsing for supported SObjects. |
| Database | `Database.insertAsync` | `supported` | Local async insert alias runs through the DML pipeline and returns SaveResult shape. |
| Database | `Database.insertImmediate` | `supported` | Local immediate insert alias runs through the DML pipeline and returns SaveResult shape. |
| Database | `Database.lock` | `supported` | Local row-lock result shape with allOrNone rollback for supported SObjects. |
| Database | `Database.merge` | `supported` | Local merge behavior with result shape and AccessLevel parsing for supported schema-backed data. |
| Database | `Database.query` | `supported` | Dynamic SOQL execution against the local org with catchable QueryException parse errors. |
| Database | `Database.queryWithBinds` | `supported` | Bind-map dynamic SOQL execution with scalar and collection binds. |
| Database | `Database.releaseSavepoint` | `supported` | Releases the selected local savepoint and later savepoints without rolling back org state. |
| Database | `Database.rollback` | `supported` | Local org-state savepoint rollback with no external side effects. |
| Database | `Database.setSavepoint` | `supported` | Local org-state snapshots with later-savepoint invalidation. |
| Database | `Database.treeSave` | `supported` | Local parent insert/update plus first-level child insert/update with NestedSaveResult relationship result shape. |
| Database | `Database.undelete` | `supported` | Soft-delete restoration with local AccessLevel parsing for supported local records. |
| Database | `Database.unlock` | `supported` | Local row-unlock result shape with allOrNone rollback for supported SObjects. |
| Database | `Database.update` | `supported` | DML pipeline with result/error shapes and local AccessLevel parsing for supported SObjects. |
| Database | `Database.updateAsync` | `supported` | Local async update alias runs through the DML pipeline and returns SaveResult shape. |
| Database | `Database.updateImmediate` | `supported` | Local immediate update alias runs through the DML pipeline and returns SaveResult shape. |
| Database | `Database.upsert` | `supported` | Schema-backed external-ID matching with local AccessLevel parsing for supported local records. |
| Date | `Date.addDays` | `supported` | Local Gregorian day arithmetic. |
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
| Decimal | `Decimal.divide(Decimal,Integer,RoundingMode)` | `supported` | Divides local Decimal values with explicit scale and RoundingMode. |
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
| Exception | `InvalidParameterValueException constructors` | `supported` | Supports zero-arg, message, cause, and existing platform-specific constructor shapes. |
| Exception | `NoAccessException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| Exception | `NoDataFoundException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| Exception | `NullPointerException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
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
| Label | `Label.get(String,String)` | `supported` | Resolves local custom label metadata with existing platform and managed-namespace fallbacks. |
| Label | `Label.get(String,String,String)` | `supported` | Resolves local custom label metadata for an explicit language, then falls back to the local label resolver. |
| Label | `Label.translationExists(String,String,String)` | `supported` | Returns true when local label metadata has a matching explicit language translation. |
| Limits | `Limits.get*` | `partial` | SOQL, DML, heap, CPU, async, callout, and email counters. |
| Limits | `Limits.getAsyncCalls` | `supported` | Returns the local async-call counter. |
| Limits | `Limits.getLimitAsyncCalls` | `supported` | Returns the local async-call limit. |
| Math | `Math.abs` | `supported` | Integer and Decimal values. |
| Math | `Math.ceil` | `supported` | Numeric values. |
| Math | `Math.floor` | `supported` | Numeric values. |
| Math | `Math.max` | `supported` | Integer and Decimal values. |
| Math | `Math.min` | `supported` | Integer and Decimal values. |
| Math | `Math.pow` | `supported` | Numeric values. |
| Math | `Math.round` | `supported` | Numeric values. |
| Math | `Math.sqrt` | `supported` | Numeric values. |
| Messaging | `Messaging.SingleEmailMessage` | `partial` | Common setters only; no delivery transport. |
| Messaging | `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption)` | `partial` | Uses local stored-template rendering and accepts the attachment option shape; attachment retrieval is not modeled. |
| Messaging | `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption,Boolean)` | `partial` | Uses local stored-template rendering; updateEmailTemplateUsage is accepted for shape and ignored locally. |
| Messaging | `Messaging.sendEmail` | `partial` | Returns local SendEmailResult and increments email limits. |
| Messaging | `Messaging.sendEmail(Messaging.Email[],Boolean)` | `supported` | Returns ordered local SendEmailResult values for supported email message DTOs; no delivery transport. |
| PageReference | `PageReference` | `partial` | Constructor, URL, redirect, parameters, headers, and string conversion basics. |
| PageReference | `PageReference(partialURL)` | `supported` | Builds a VM-local PageReference from a partial URL with mutable parameters and headers. |
| PageReference | `PageReference(record)` | `unknown` | Symbol shape is present, but the record constructor runtime path requires constructor handling outside this packet's write scope. |
| Pattern | `Matcher.find` | `partial` | Go regexp-backed matching. |
| Pattern | `Matcher.group` | `partial` | Latest matched group only. |
| Pattern | `Matcher.matches` | `partial` | Go regexp-backed matching. |
| Pattern | `Pattern.compile` | `partial` | Go regexp syntax. |
| Pattern | `Pattern.matches` | `partial` | Whole-string Go regexp match. |
| SObject | `SObject.setOptions(Database.DMLOptions)` | `supported` | Stores a cloned DMLOptions value on the local SObject for later DML option use. |
| Schema | `DescribeFieldResult` | `partial` | Common field metadata and access booleans. |
| Schema | `DescribeSObjectResult` | `partial` | Common object metadata, fields, record types, and child relationships. |
| Schema | `Schema.describeDataCategoryGroupStructures(List<Schema.DataCategoryGroupSobjectTypePair>,Boolean)` | `partial` | Deterministic local data category structures from org metadata; no external category service lookup. |
| Schema | `Schema.describeDataCategoryGroups(List<String>)` | `partial` | Deterministic local data category group describes from org metadata; empty when no metadata is loaded. |
| Schema | `Schema.describeSObjects(List<String>)` | `partial` | Object names and SObjectType tokens for local schema. |
| Schema | `Schema.getGlobalDescribe()` | `partial` | Local schema-backed describe map. |
| Search | `Search.find` | `partial` | Returns deterministic SearchResult DTOs from Test.setFixedSearchResults; no external search ranking/snippets. |
| Search | `Search.find(String,AccessLevel)` | `partial` | Uses deterministic fixed search results and accepts AccessLevel for shape; external ranking/snippets are not modeled. |
| Search | `Search.query / SOSL FIND` | `partial` | Parses RETURNING clauses and returns deterministic rows from Test.setFixedSearchResults, or empty result groups without external search. |
| Search | `Search.query(String,AccessLevel)` | `partial` | Uses the deterministic local SOSL model and accepts AccessLevel for shape; external search security is not modeled. |
| Search | `Search.suggest` | `partial` | Returns an empty deterministic SuggestionResults DTO; no external suggestion service. |
| Search | `Search.suggest(String,String,Search.SuggestionOption)` | `partial` | Returns an empty deterministic SuggestionResults DTO without external suggestion service calls. |
| Search | `Search.suggest(String,String,Search.SuggestionOption,AccessLevel)` | `partial` | Accepts AccessLevel for shape and returns the deterministic local SuggestionResults DTO. |
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
| Test | `Test.createSoqlStub(Schema.SObjectType,SoqlStubProvider)` | `supported` | Registers test-local SOQL stubs per SObject type. |
| Test | `Test.createStub(Type,StubProvider)` | `supported` | Creates dynamic test stubs backed by StubProvider. |
| Test | `Test.createStubQueryRow` | `partial` | Builds local SObject rows from field maps for SOQL stub providers. |
| Test | `Test.createStubQueryRow(Schema.SObjectType,Map<String,Object>)` | `supported` | Builds one local SObject row from a field map for SOQL stub providers. |
| Test | `Test.createStubQueryRows` | `partial` | Builds local SObject row lists from field maps for SOQL stub providers. |
| Test | `Test.createStubQueryRows(Schema.SObjectType,List<Map<String,Object>>)` | `supported` | Builds local SObject row lists from field maps for SOQL stub providers. |
| Test | `Test.getStandardPricebookId` | `partial` | Deterministic test-context-only ID. |
| Test | `Test.isRunningTest` | `supported` | Reflects local test context. |
| Test | `Test.loadData` | `partial` | Loads CSV static-resource content into local org storage through DML. |
| Test | `Test.setCurrentPage(PageReference)` | `supported` | Sets the VM-local current PageReference in test context. |
| Test | `Test.setCurrentPageReference(PageReference)` | `supported` | Sets the VM-local current PageReference in test context. |
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
| UserInfo | `UserInfo.getFirstName` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getLanguage` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getLastName` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getLocale` | `partial` | Deterministic local value. |
| UserInfo | `UserInfo.getName` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getOrganizationId` | `partial` | Local org identity. |
| UserInfo | `UserInfo.getProfileId` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getSessionId` | `partial` | Empty local session value. |
| UserInfo | `UserInfo.getTimeZone` | `supported` | Returns the deterministic local user timezone. |
| UserInfo | `UserInfo.getUserEmail` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getUserId` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getUserName` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.getUserType` | `partial` | Current runAs/default user. |
| UserInfo | `UserInfo.isMultiCurrencyOrganization` | `partial` | Local org metadata flag. |
