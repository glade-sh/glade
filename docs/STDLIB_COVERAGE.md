# Standard Library Coverage

Generated from the first-party compat plugin capability catalog.

Status values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`.

| Area | API | Status | Notes |
| --- | --- | --- | --- |
| AccessLevel | `AccessLevel.withPermissionSetId(String)` | `supported` | Creates a local permission-set-scoped user-mode token used by supported SOQL and DML permission checks. |
| Answers | `Answers.findSimilar(Question)` | `unsupported` | Zone similar-question search requires Answers service data and returns a stable UnsupportedFeature diagnostic locally. |
| ApexPages | `ApexPages.Message` | `partial` | Constructor, getters, add/get/has message state, and Visualforce action reset behavior; no full rendering lifecycle. |
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
| Approval | `Approval.process(Approval.ProcessRequest)` | `partial` | Deterministic local approval result DTOs for submit/workitem request shapes; no live approval engine routing. |
| Approval | `Approval.process(Approval.ProcessRequest, Boolean)` | `partial` | Deterministic local approval result DTOs for submit/workitem request shapes; no live approval engine routing. |
| Assert | `Assert.areEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.areNotEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.fail` | `supported` | Raises local System.AssertException with optional message text. |
| Assert | `Assert.isFalse` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNotNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isTrue` | `supported` | Routes through local assertion failures with optional message text. |
| Boolean | `Boolean.valueOf(Object)` | `supported` | Converts supported local field/object values into Boolean values. |
| Boolean | `Boolean.valueOf(String)` | `supported` | Converts strings to Boolean using Apex-shaped true/false parsing. |
| BusinessHours | `BusinessHours.add(String, Datetime, Long)` | `partial` | Local BusinessHours record-backed week schedule math; holiday and service-calendar edge behavior is not modeled. |
| BusinessHours | `BusinessHours.addGmt(String, Datetime, Long)` | `partial` | Local BusinessHours record-backed week schedule math; holiday and service-calendar edge behavior is not modeled. |
| BusinessHours | `BusinessHours.diff(String, Datetime, Datetime)` | `partial` | Local BusinessHours record-backed week schedule math; holiday and service-calendar edge behavior is not modeled. |
| BusinessHours | `BusinessHours.isWithin(String, Datetime)` | `partial` | Local BusinessHours record-backed week schedule math; holiday and service-calendar edge behavior is not modeled. |
| BusinessHours | `BusinessHours.nextStartDate(String, Datetime)` | `partial` | Local BusinessHours record-backed week schedule math; holiday and service-calendar edge behavior is not modeled. |
| Crypto | `Crypto.generateDigest` | `partial` | MD5, SHA-1/SHA1, SHA-256/SHA256, SHA-384/SHA384, SHA-512/SHA512, and SHA3-256/384/512 are modeled; unsupported digest names raise local SecurityException. |
| Database | `Database.UnitOfWork` | `supported` | Queues local DML operations and applies them on commitWork; discardWork drops pending local work. |
| Database | `Database.convertLead` | `supported` | Local lead conversion creates Account, Contact, and optional Opportunity records and updates Lead conversion fields. |
| Database | `Database.countQuery` | `supported` | Dynamic SOQL count execution against the local org with local AccessLevel parsing. |
| Database | `Database.countQueryWithBinds` | `supported` | Bind-map dynamic SOQL count execution with local AccessLevel parsing. |
| Database | `Database.delete` | `supported` | DML pipeline with result/error shapes and local AccessLevel parsing for supported SObjects. |
| Database | `Database.deleteAsync` | `supported` | Local async delete alias runs through the DML pipeline and returns DeleteResult shape. |
| Database | `Database.deleteImmediate` | `supported` | Local immediate delete alias runs through the DML pipeline and returns DeleteResult shape. |
| Database | `Database.emptyRecycleBin` | `supported` | Local hard-delete result shape with allOrNone rollback for supported SObjects. |
| Database | `Database.executeBatch` | `supported` | Queues local Batchable jobs and drains start/execute chunks/finish during Test.stopTest. |
| Database | `Database.getAsyncDeleteResult` | `supported` | Returns materialized local DeleteResult values and rejects unknown async locator strings instead of fabricating pending results. |
| Database | `Database.getAsyncLocator` | `supported` | Returns deterministic VM-local locator strings for local result and locator objects; no external async service lookup. |
| Database | `Database.getAsyncSaveResult` | `supported` | Returns materialized local SaveResult values and rejects unknown async locator strings instead of fabricating pending results. |
| Database | `Database.getCursor` | `supported` | Local cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getCursorWithBinds` | `supported` | Bind-map local cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getDeleted` | `supported` | Returns local soft-deleted record tombstones whose deletion timestamp falls within the requested sync window. |
| Database | `Database.getPaginationCursor` | `supported` | Local pagination cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getPaginationCursorWithBinds` | `supported` | Bind-map local pagination cursor over supported SOQL results with deterministic fetch windows. |
| Database | `Database.getQueryLocator` | `supported` | Supported SOQL executes eagerly for local batch scopes with local AccessLevel parsing. |
| Database | `Database.getQueryLocatorWithBinds` | `supported` | Bind-map dynamic SOQL locator execution with iterable local query locators. |
| Database | `Database.getUpdated` | `supported` | Returns local record IDs whose system-modified timestamp falls within the requested sync window. |
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
| Datetime | `Datetime.addHours` | `supported` | UTC-local arithmetic. |
| Datetime | `Datetime.addMinutes` | `supported` | UTC-local arithmetic. |
| Datetime | `Datetime.addMonths` | `supported` | UTC-local month arithmetic with end-of-month clamping. |
| Datetime | `Datetime.addSeconds` | `supported` | UTC-local arithmetic. |
| Datetime | `Datetime.addYears` | `supported` | UTC-local year arithmetic with leap-day clamping. |
| Datetime | `Datetime.format` | `supported` | Formats using the deterministic local timezone model. |
| Datetime | `Datetime.formatGmt` | `supported` | Formats using GMT. |
| Datetime | `Datetime.newInstance` | `supported` | Validates date and time parts. |
| Datetime | `Datetime.now` | `supported` | Returns the deterministic local runtime datetime. |
| Datetime | `Datetime.valueOf` | `supported` | Parses supported datetime strings. |
| Decimal | `Decimal.divide(Decimal,Integer,RoundingMode)` | `supported` | Divides local Decimal values with explicit scale and RoundingMode. |
| Decimal | `Decimal.doubleValue` | `supported` | Returns local Decimal value. |
| Decimal | `Decimal.intValue` | `supported` | Truncates to integer. |
| Decimal | `Decimal.round` | `partial` | Uses deterministic local HALF_UP tie behavior for finite local Decimal values; full Salesforce Decimal precision is not modeled. |
| Decimal | `Decimal.setScale` | `partial` | Models finite local Decimal values, negative scale, and common RoundingMode ties up to the local scale fence; full Salesforce Decimal precision is not modeled. |
| EncodingUtil | `EncodingUtil.base64Decode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.base64Encode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertFromHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertToHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.urlDecode` | `partial` | Models URL form decoding for UTF-8, US-ASCII, and ISO-8859-1 aliases; broader Salesforce charset replacement behavior is not modeled. |
| EncodingUtil | `EncodingUtil.urlEncode` | `partial` | Models URL form encoding for UTF-8, US-ASCII, and ISO-8859-1 aliases with strict local charset checks; broader Salesforce charset replacement behavior is not modeled. |
| Exception | `InvalidParameterValueException constructors` | `supported` | Supports zero-arg, message, cause, and existing platform-specific constructor shapes. |
| Exception | `NoAccessException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| Exception | `NoDataFoundException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| Exception | `NullPointerException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| FeatureManagement | `FeatureManagement.checkPermission` | `partial` | Checks local runAs permission-list state. |
| HTTP | `Http.send(HttpRequest)` | `supported` | Routes local callouts through registered HttpCalloutMock implementations; real outbound network transport is intentionally not executed by the local runtime. |
| HTTP | `HttpRequest` | `partial` | Endpoint, method, headers, timeout, body, and blob body accessors. |
| HTTP | `HttpResponse` | `partial` | Status, status code, headers, body, and blob body accessors. |
| JSON | `JSON.deserialize` | `partial` | SObject, class, inner DTO, enum, collection, primitive, and string-key map shapes for supported targets; unsupported map key targets remain fenced. |
| JSON | `JSON.deserializeStrict` | `partial` | Rejects unknown fields for supported schema/class targets, including nested local DTO fields; unsupported target coercions remain outside the local model. |
| JSON | `JSON.deserializeUntyped` | `partial` | Maps JSON into local primitive/list/map values with deterministic Apex-shaped containers; full Salesforce parser edge parity is not modeled. |
| JSON | `JSON.serialize` | `partial` | Serializes supported Apex DTO, enum, SObject, collection, primitive, and suppressApexObjectNulls values; full Salesforce generator edge parity is not modeled. |
| JSON | `JSON.serializePretty` | `partial` | Pretty output for supported Apex DTO, SObject, collection, and primitive values; full Salesforce generator edge parity is not modeled. |
| Label | `Label.get(String,String)` | `supported` | Resolves local custom label metadata with existing platform and managed-namespace fallbacks. |
| Label | `Label.get(String,String,String)` | `supported` | Resolves local custom label metadata for an explicit language, then falls back to the local label resolver. |
| Label | `Label.translationExists(String,String,String)` | `supported` | Returns true when local label metadata has a matching explicit language translation. |
| Limits | `Limits.get*` | `partial` | SOQL, SOSL, DML, heap, CPU, async, callout, email, runAs, and savepoint counters are modeled; unmodeled getter families remain local defaults. |
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
| Messaging | `Messaging.SingleEmailMessage` | `partial` | Common DTO setters/getters and local file attachments are modeled; no delivery transport or Salesforce content attachment service. |
| Messaging | `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption)` | `partial` | Uses local stored-template rendering and static-resource attachment retrieval for METADATA_ONLY/METADATA_WITH_BODY; Salesforce content attachment relationships are not modeled. |
| Messaging | `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption,Boolean)` | `partial` | Uses local stored-template rendering and static-resource attachment retrieval; updateEmailTemplateUsage is accepted for shape and ignored locally. |
| Messaging | `Messaging.sendEmail` | `partial` | Returns local SendEmailResult values, validates supported DTOs, and increments email limits; no delivery transport or send-options overloads. |
| Messaging | `Messaging.sendEmail(Messaging.Email[],Boolean)` | `supported` | Returns ordered local SendEmailResult values for supported email message DTOs; no delivery transport. |
| PageReference | `PageReference` | `partial` | Constructor, URL, redirect, mutable parameters, headers, and current-page state; getContent, PDF, and full rendering remain outside the local model. |
| PageReference | `PageReference(partialURL)` | `supported` | Builds a VM-local PageReference from a partial URL with mutable parameters and headers. |
| PageReference | `PageReference(record)` | `supported` | Builds a Visualforce PageReference from a local ApexPage SObject record. |
| Pattern | `Matcher.find` | `partial` | Go regexp-backed Java subset with local match state; Java-only matcher and regex features remain fenced. |
| Pattern | `Matcher.group` | `partial` | Returns local whole-match and capture groups after find/matches over the supported regex subset; Java-only matcher features remain fenced. |
| Pattern | `Matcher.matches` | `partial` | Whole-string matching over the local Go regexp-backed Java subset; Java-only regex features remain fenced. |
| Pattern | `Pattern.compile` | `partial` | Compiles the local Go regexp-backed Java subset with common flags and quote escapes; Java-only regex classes and flags remain fenced. |
| Pattern | `Pattern.matches` | `partial` | Whole-string matching over the local Go regexp-backed Java subset; Java-only regex features remain fenced. |
| QuickAction | `QuickAction.describeAvailableActions` | `partial` | Local QuickAction metadata and deterministic DTO results; no live UI action service execution. |
| QuickAction | `QuickAction.describeAvailableQuickActions(String)` | `partial` | Local QuickAction metadata and deterministic DTO results; no live UI action service execution. |
| QuickAction | `QuickAction.describeQuickActions(List<String>)` | `partial` | Local QuickAction metadata and deterministic DTO results; no live UI action service execution. |
| QuickAction | `QuickAction.performQuickAction` | `partial` | Returns deterministic local QuickActionResult DTOs for supported request shapes; no live UI action service execution. |
| QuickAction | `QuickAction.performQuickAction(QuickAction.QuickActionRequest)` | `partial` | Returns deterministic local QuickActionResult DTOs for supported request shapes; no live UI action service execution. |
| QuickAction | `QuickAction.performQuickAction(QuickAction.QuickActionRequest,Boolean)` | `partial` | Returns deterministic local QuickActionResult DTOs for supported request shapes; no live UI action service execution. |
| QuickAction | `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>)` | `partial` | Returns deterministic local QuickActionResult DTOs for supported request shapes; no live UI action service execution. |
| QuickAction | `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>,Boolean)` | `partial` | Returns deterministic local QuickActionResult DTOs for supported request shapes; no live UI action service execution. |
| QuickAction | `QuickAction.retrieveQuickActionTemplate(String,Id)` | `partial` | Local QuickAction metadata and deterministic DTO results; no live UI action service execution. |
| QuickAction | `QuickAction.retrieveQuickActionTemplates(List<String>,Id)` | `partial` | Local QuickAction metadata and deterministic DTO results; no live UI action service execution. |
| Request | `Request.getCurrent()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| Request | `Request.getQuiddity()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| Request | `Request.getRequestId()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| Request | `RequestImpl.getCurrent()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| ResetPasswordResult | `ResetPasswordResult.getPassword()` | `unsupported` | Password reset output is produced by Salesforce identity services and is not generated locally. |
| SObject | `SObject.setOptions(Database.DMLOptions)` | `supported` | Stores a cloned DMLOptions value on the local SObject for later DML option use. |
| Sandbox | `SandboxContext.organizationId()` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Sandbox | `SandboxContext.sandboxId()` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Sandbox | `SandboxContext.sandboxName()` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Sandbox | `SandboxPostCopy.runApexClass(SandboxContext)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Schedulable | `Schedulable.execute(SchedulableContext)` | `supported` | Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs. |
| Schedulable | `SchedulableContext.getTriggerId()` | `supported` | Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs. |
| Schema | `DescribeFieldResult` | `partial` | Local metadata-backed field labels, picklists, reference targets, and access/create/update booleans; full org metadata parity is not modeled. |
| Schema | `DescribeSObjectResult` | `partial` | Local metadata-backed object labels, key prefixes, query/search flags, fields, record types, and child relationships; full org metadata parity is not modeled. |
| Schema | `Schema.describeDataCategoryGroupStructures(List<Schema.DataCategoryGroupSobjectTypePair>,Boolean)` | `partial` | Deterministic local data category structures from org metadata; no external category service lookup. |
| Schema | `Schema.describeDataCategoryGroups(List<String>)` | `partial` | Deterministic local data category group describes from org metadata; empty when no metadata is loaded. |
| Schema | `Schema.describeSObjects(List<String>)` | `partial` | Local metadata-backed object names, labels, key prefixes, access booleans, fields, record types, and SObjectType tokens; full org metadata parity is not modeled. |
| Schema | `Schema.getGlobalDescribe()` | `partial` | Local metadata-backed describe map for loaded standard and project objects; full org metadata parity is not modeled. |
| Search | `Search.find` | `partial` | Returns deterministic SearchResult DTOs from fixed search results or local org records; no external ranking, stemming, synonyms, language, or rich snippets. |
| Search | `Search.find(String,AccessLevel)` | `partial` | Uses deterministic local org search with AccessLevel permission checks; no external ranking, stemming, synonyms, language, or rich snippets. |
| Search | `Search.find(String,Object)` | `partial` | Accepts recognized local AccessLevel or SuggestionOption object values against deterministic local org search; no external ranking, stemming, synonyms, or language model. |
| Search | `Search.query / SOSL FIND` | `partial` | Parses RETURNING clauses and returns deterministic rows from fixed search results or local org records; no ranking, stemming, synonyms, language, or external search service. |
| Search | `Search.query(String,AccessLevel)` | `partial` | Uses deterministic local org SOSL with AccessLevel permission checks; no ranking, stemming, synonyms, language, or external search service. |
| Search | `Search.query(String,Object)` | `partial` | Accepts recognized local AccessLevel or SuggestionOption object values against deterministic local org SOSL; no external ranking, stemming, synonyms, or language model. |
| Search | `Search.suggest` | `partial` | Returns deterministic local org suggestion DTOs from name-like fields; no external suggestion service. |
| Search | `Search.suggest(String,String,Object)` | `partial` | Accepts recognized SuggestionOption values and returns deterministic local org suggestions; no external suggestion service. |
| Search | `Search.suggest(String,String,Object,Object)` | `partial` | Accepts recognized SuggestionOption and AccessLevel values for deterministic local org suggestions; no external suggestion service. |
| Search | `Search.suggest(String,String,Search.SuggestionOption)` | `partial` | Returns deterministic local org suggestions and respects local limit options; no external suggestion service. |
| Search | `Search.suggest(String,String,Search.SuggestionOption,AccessLevel)` | `partial` | Returns deterministic local org suggestions with AccessLevel permission checks; no external suggestion service. |
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
| String | `String.split` | `partial` | Uses local Java-regex-like split for the supported Go regexp subset, including limits and \Q...\E quote escapes; empty-match regexes and Java-only features remain fenced. |
| String | `String.startsWith` | `supported` | UTF-8 string prefix. |
| String | `String.substring` | `supported` | Rune-indexed substring. |
| String | `String.toLowerCase` | `supported` | Go Unicode lowercasing. |
| String | `String.toUpperCase` | `supported` | Go Unicode uppercasing. |
| String | `String.trim` | `supported` | Unicode whitespace trim. |
| String | `String.valueOf` | `supported` | Local value string conversion. |
| System | `System.assert` | `supported` | Assertion failure returns runtime error. |
| System | `System.assertEquals` | `supported` | Assertion failure returns runtime error. |
| System | `System.debug` | `supported` | Collected in result debug output. |
| System | `System.enqueueJob(Object,Object)` | `partial` | Accepts local AsyncOptions maximumQueueableStackDepth for queueable chaining and Test.stopTest drain behavior; delay semantics are not modeled. |
| System | `System.runAs(Object,Object)` | `partial` | Tracks local package-version test context without installing or licensing packages. |
| System | `System.runAs(Package.Version)` | `partial` | Tracks local package-version test context without installing or licensing packages. |
| System | `System.schedule(String,String,Object)` | `supported` | Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs. |
| Test | `Test.createSoqlStub(Schema.SObjectType,SoqlStubProvider)` | `supported` | Registers test-local SOQL stubs per SObject type. |
| Test | `Test.createStub(Type,StubProvider)` | `supported` | Creates dynamic test stubs backed by StubProvider. |
| Test | `Test.createStubQueryRow` | `partial` | Builds local SObject rows from field maps for SOQL stub providers. |
| Test | `Test.createStubQueryRow(Schema.SObjectType,Map<String,Object>)` | `supported` | Builds one local SObject row from a field map for SOQL stub providers. |
| Test | `Test.createStubQueryRows` | `partial` | Builds local SObject row lists from field maps for SOQL stub providers. |
| Test | `Test.createStubQueryRows(Schema.SObjectType,List<Map<String,Object>>)` | `supported` | Builds local SObject row lists from field maps for SOQL stub providers. |
| Test | `Test.enableChangeDataCapture()` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.getEventBus()` | `partial` | Returns a local event-bus test broker that delivers queued platform-event triggers; no live Salesforce event-bus service is contacted. |
| Test | `Test.getExternalService()` | `partial` | Returns a deterministic local external-service harness and fences live service execution. |
| Test | `Test.getStandardPricebookId` | `partial` | Deterministic test-context-only ID. |
| Test | `Test.invokeContinuationMethod(Object,Continuation)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.invokePage(PageReference)` | `supported` | Returns a typed Component.apex.page handle in test context without rendering Visualforce. |
| Test | `Test.isRunningTest` | `supported` | Reflects local test context. |
| Test | `Test.loadData` | `partial` | Loads CSV static-resource content with Go CSV parsing, typed field coercion, DML routing, missing-resource errors, and bad-header diagnostics; full Salesforce fixture semantics are not modeled. |
| Test | `Test.newSendEmailQuickActionDefaults(Id,Id)` | `supported` | Builds deterministic local send-email QuickAction defaults for test execution. |
| Test | `Test.setContinuationResponse(String,HttpResponse)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.setCurrentPage(PageReference)` | `supported` | Sets the VM-local current PageReference in test context. |
| Test | `Test.setCurrentPageReference(Object)` | `partial` | Accepts local PageReference object values; arbitrary Object values remain rejected. |
| Test | `Test.setCurrentPageReference(PageReference)` | `supported` | Sets the VM-local current PageReference in test context. |
| Test | `Test.setMock` | `partial` | HttpCalloutMock and WebServiceMock routing for local tests; live transport is not executed. |
| Test | `Test.startTest` | `partial` | Resets the active governor window for supported local counters; unmodeled service counters remain local boundaries. |
| Test | `Test.stopTest` | `partial` | Restores outer governor counters and drains supported async work; unmodeled async/service work remains outside the local model. |
| Test | `Test.testInstall(InstallHandler,Version)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.testInstall(InstallHandler,Version,Boolean)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.testNotificationActionHandler(Messaging.NotificationActionHandler,Messaging.ActionableNotification)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String,Boolean)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Test | `Test.testUninstall(UninstallHandler)` | `supported` | Invokes the local test harness implementation; no live Salesforce service is contacted. |
| Time | `Time.hour` | `supported` | Local time component. |
| Time | `Time.minute` | `supported` | Local time component. |
| Time | `Time.newInstance` | `supported` | Validates time parts. |
| Time | `Time.second` | `supported` | Local time component. |
| Time | `Time.valueOf` | `supported` | Parses supported time strings. |
| TimeZone | `TimeZone.getDisplayName` | `supported` | Returns deterministic display names for local timezone values. |
| TimeZone | `TimeZone.getID` | `supported` | Returns local timezone IDs. |
| TimeZone | `TimeZone.getOffset` | `supported` | Returns offsets from the deterministic local timezone model. |
| TimeZone | `TimeZone.getTimeZone` | `supported` | Resolves timezone IDs into local timezone values. |
| TrailblazerIdentity | `TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)` | `unsupported` | Trailblazer identity service calls are not executed by the local Apex runtime. |
| TrailblazerIdentity | `TrailblazerIdentity.getUserOrgInfo(List<String>)` | `unsupported` | Trailblazer identity service calls are not executed by the local Apex runtime. |
| TrailblazerIdentity | `TrailblazerIdentity.splunkLog(String,String)` | `unsupported` | Trailblazer identity service calls are not executed by the local Apex runtime. |
| Type | `Type.forName` | `partial` | Local class/type token lookup. |
| Type | `Type.getName` | `supported` | Returns local type token name. |
| Type | `Type.newInstance` | `supported` | Constructs local classes through zero-argument constructor dispatch and rejects uninstantiable built-ins. |
| UIRequest | `UIRequest.getCurrent()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| UIRequest | `UIRequest.getRequestHeader(String)` | `supported` | Returns deterministic local request context values for local Apex execution. |
| URL | `URL.getOrgDomainUrl` | `supported` | Deterministic local org URL. |
| URL | `URL.getSalesforceBaseUrl` | `supported` | Deterministic local base URL. |
| Unsupported | `unimplemented platform/stdlib calls` | `supported` | Typed UnsupportedFeature errors with stable message text. |
| UserInfo | `UserInfo.getFirstName` | `supported` | Returns the current runAs/default user's local first name. |
| UserInfo | `UserInfo.getLanguage` | `supported` | Returns the current runAs/default user's local language value. |
| UserInfo | `UserInfo.getLastName` | `supported` | Returns the current runAs/default user's local last name. |
| UserInfo | `UserInfo.getLocale` | `supported` | Returns the current runAs/default user's local locale value. |
| UserInfo | `UserInfo.getName` | `supported` | Returns the current runAs/default user's local display name. |
| UserInfo | `UserInfo.getOrganizationId` | `supported` | Returns the deterministic local org identity. |
| UserInfo | `UserInfo.getProfileId` | `supported` | Returns the current runAs/default user's local profile id. |
| UserInfo | `UserInfo.getSessionId` | `supported` | Returns the deterministic empty local session value. |
| UserInfo | `UserInfo.getTimeZone` | `supported` | Returns the deterministic local user timezone. |
| UserInfo | `UserInfo.getUserEmail` | `supported` | Returns the current runAs/default user's local email value. |
| UserInfo | `UserInfo.getUserId` | `supported` | Returns the current runAs/default user's local user id. |
| UserInfo | `UserInfo.getUserName` | `supported` | Returns the current runAs/default user's local username. |
| UserInfo | `UserInfo.getUserType` | `supported` | Returns the current runAs/default user's local user type. |
| UserInfo | `UserInfo.hasPackageLicense(Id)` | `supported` | Checks local PackageLicense and UserPackageLicense records for the current runAs/default user. |
| UserInfo | `UserInfo.isCurrentUserLicensedForPackage(Id)` | `supported` | Checks local PackageLicense and UserPackageLicense records for the current runAs/default user. |
| UserInfo | `UserInfo.isMultiCurrencyOrganization` | `supported` | Returns the local org multi-currency metadata flag. |
| WebServiceCallout | `WebServiceCallout.invoke(Object,Object,Map,List)` | `partial` | Routes generated SOAP callouts through registered WebServiceMock implementations and materializes a deterministic local response shell when no mock is registered; real outbound SOAP transport is not executed. |
| WebServiceCallout | `WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)` | `partial` | Routes generated SOAP callouts through registered WebServiceMock implementations and materializes a deterministic local response shell when no mock is registered; real outbound SOAP transport is not executed. |
