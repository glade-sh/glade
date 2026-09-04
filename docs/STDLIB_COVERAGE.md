# Standard Library Coverage

Generated from the first-party compat plugin capability catalog.

Status values match the compatibility dashboard: `supported`, `partial`, `stub`, `unsupported`, and `unknown`.

| Area | API | Status | Notes |
| --- | --- | --- | --- |
| AccessLevel | `AccessLevel.withPermissionSetId(String)` | `supported` | Creates a local permission-set-scoped user-mode token used by supported SOQL and DML permission checks. |
| Answers | `Answers.findSimilar(Question)` | `supported` | Local execution returns a deterministic empty List<Id>; this harness does not implement the hosted similarity-search service. |
| ApexPages | `ApexPages.Message` | `supported` | Constructor, getters, add/get/has message state, and Visualforce action reset behavior are modeled. |
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
| ApexPages | `Visualforce full rendering lifecycle` | `unsupported` | Full Visualforce rendering lifecycle requires the hosted renderer and is fenced by PageReference rendering diagnostics locally. |
| Approval | `Approval.process hosted approval engine routing` | `unsupported` | Hosted criteria evaluation, queue/group approver routing, and live approval service routing require Salesforce approval services. |
| Approval | `Approval.process(Approval.ProcessRequest)` | `supported` | Runs a seeded local ProcessDefinition, ProcessNode, ProcessInstance, and ProcessInstanceWorkitem engine for submit, approve, and reject request shapes. |
| Approval | `Approval.process(Approval.ProcessRequest, Boolean)` | `supported` | Runs the seeded local approval engine with allOrNone error shaping for submit, approve, and reject request shapes. |
| Approval | `Approval.process(List<Approval.ProcessRequest>)` | `supported` | Runs ordered local submit and workitem request lists and rolls back earlier successful local approval records when the default allOrNone path fails later in the list. |
| Approval | `Approval.process(List<Approval.ProcessRequest>, Boolean)` | `supported` | Runs ordered local submit and workitem request lists, returning per-request errors when allOrNone is false and rolling back the local list transaction when allOrNone is true. |
| Assert | `Assert.areEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.areNotEqual` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.fail` | `supported` | Raises local System.AssertException with optional message text. |
| Assert | `Assert.isFalse` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNotNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isNull` | `supported` | Routes through local assertion failures with optional message text. |
| Assert | `Assert.isTrue` | `supported` | Routes through local assertion failures with optional message text. |
| Boolean | `Boolean.valueOf(Object)` | `supported` | Converts supported local field/object values into Boolean values. |
| Boolean | `Boolean.valueOf(String)` | `supported` | Converts strings to Boolean using Apex-shaped true/false parsing. |
| BusinessHours | `BusinessHours malformed local holiday metadata` | `unsupported` | Malformed seeded holiday metadata raises a stable UnsupportedFeature diagnostic naming the unsupported local field shape. |
| BusinessHours | `BusinessHours.add(Id, Datetime, Long)` | `supported` | Runs deterministic local week-schedule math from seeded BusinessHours, Holiday, OperatingHours, and OperatingHoursHoliday records with timezone handling, all-day closures, partial-day closures, recurring holidays, and linked holidays. |
| BusinessHours | `BusinessHours.addGmt(Id, Datetime, Long)` | `supported` | Runs deterministic local calendar math from seeded BusinessHours, Holiday, OperatingHours, and OperatingHoursHoliday records with GMT Datetime output. |
| BusinessHours | `BusinessHours.diff(String, Datetime, Datetime)` | `supported` | Counts deterministic local business milliseconds across seeded week schedules, timezones, all-day closures, partial-day closures, recurring holidays, and linked holidays. |
| BusinessHours | `BusinessHours.isWithin(String, Datetime)` | `supported` | Checks seeded local week schedules, timezones, Holiday closures, OperatingHoursHoliday links, partial-day closures, and recurring holidays. |
| BusinessHours | `BusinessHours.nextStartDate(Id, Datetime)` | `supported` | Finds the next deterministic local start from seeded week schedules, timezones, Holiday closures, OperatingHoursHoliday links, partial-day closures, and recurring holidays. |
| Crypto | `Crypto.decryptWithManagedIV` | `supported` | Managed-IV AES-GCM decryption, including additional authenticated data, is executable in the local runtime; ciphertext values remain unstable. |
| Crypto | `Crypto.encryptWithManagedIV` | `supported` | Managed-IV AES-GCM encryption, including additional authenticated data, is executable in the local runtime; ciphertext values remain unstable. |
| Crypto | `Crypto.generateDigest` | `supported` | MD5, SHA-1/SHA1, SHA-256/SHA256, SHA-384/SHA384, SHA-512/SHA512, and SHA3-256/384/512 digests are fixture-pinned, with Salesforce-shaped SecurityException diagnostics for unknown names. |
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
| Database | `Database.getAsyncLocator` | `supported` | Returns deterministic VM-local locator strings for local result and locator objects. |
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
| Database | `Database.rollback` | `supported` | Restores the local org-state snapshot for the selected savepoint. |
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
| Decimal | `Decimal.divide(Decimal,Integer,RoundingMode)` | `unsupported` | Text-backed Decimal division is tested locally, but untexted Decimal division remains explicitly unsupported. |
| Decimal | `Decimal.doubleValue` | `supported` | Returns an explicitly Double-backed local value. |
| Decimal | `Decimal.intValue` | `supported` | Truncates to integer. |
| Decimal | `Decimal.round` | `supported` | Oracle-pinned finite Decimal rounding uses Salesforce default HALF_EVEN ties and explicit RoundingMode behavior. |
| Decimal | `Decimal.setScale` | `supported` | Oracle-pinned finite Decimal scaling covers positive and negative scales, RoundingMode ties, UNNECESSARY MathException, and Salesforce scale bounds. |
| EncodingUtil | `EncodingUtil.base64Decode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.base64Encode` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertFromHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.convertToHex` | `supported` | Blob-shaped local value. |
| EncodingUtil | `EncodingUtil.urlDecode` | `supported` | Oracle-pinned URL form decoding covers UTF-8, US-ASCII replacement, ISO-8859-1 aliases, and UTF-16 mixed escaped/plain input. |
| EncodingUtil | `EncodingUtil.urlEncode` | `supported` | Oracle-pinned URL form encoding covers UTF-8, US-ASCII replacement, ISO-8859-1 aliases, and UTF-16 escaped output. |
| Exception | `InvalidParameterValueException constructors` | `supported` | Supports zero-arg, message, cause, and existing platform-specific constructor shapes. |
| Exception | `NoAccessException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| Exception | `NoDataFoundException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| Exception | `NullPointerException constructors` | `supported` | Supports zero-arg, message, cause, and message-plus-cause constructor shapes. |
| FeatureManagement | `FeatureManagement.checkPermission` | `supported` | Checks local runAs user permissions, permission sets, and custom-permission assignments. |
| HTTP | `Http.send(HttpRequest)` | `supported` | Routes local callouts through registered HttpCalloutMock implementations and records local callout limits. |
| HTTP | `HttpRequest` | `supported` | Endpoint, method, headers, timeout, body, blob body, and local DTO accessors are modeled. |
| HTTP | `HttpRequest client certificate local mock metadata` | `supported` | Client certificate name and PEM accessors read deterministic local certificate metadata for mock-backed callouts. |
| HTTP | `HttpRequest client certificate real TLS handshake` | `unsupported` | Real mutual-TLS handshakes and Salesforce certificate-store delivery require hosted callout infrastructure. |
| HTTP | `HttpResponse` | `supported` | Status, status code, headers, body, blob body, and local DTO accessors are modeled. |
| JSON | `JSON.deserialize` | `supported` | Oracle-pinned DTO, SObject, primitive, collection, Object, string-key, and scalar-key map paths preserve duplicate-key last-value behavior and field order. |
| JSON | `JSON.deserializeStrict` | `supported` | Oracle-pinned strict DTO and SObject paths accept known fields, reject unknown fields with Salesforce-shaped diagnostics, and keep the last duplicate SObject field value. |
| JSON | `JSON.deserializeUntyped` | `supported` | Oracle-pinned untyped primitive, list, and map containers preserve order, collapse duplicate keys to the last value, and throw Salesforce-shaped oversized integer errors. |
| JSON | `JSON.serialize` | `supported` | Oracle-pinned serialization covers DTO, SObject, collection, map, primitive, enum, Object, and suppressApexObjectNulls paths with deterministic field order. |
| JSON | `JSON.serializePretty` | `supported` | Oracle-pinned pretty serialization covers DTO, SObject, collection, map, primitive, and Object paths with Salesforce-style spacing. |
| Label | `Label.get(String,String)` | `supported` | Resolves local custom label metadata with existing platform and managed-namespace fallbacks. |
| Label | `Label.get(String,String,String)` | `supported` | Resolves local custom label metadata for an explicit language, then falls back to the local label resolver. |
| Label | `Label.translationExists(String,String,String)` | `supported` | Returns true when local label metadata has a matching explicit language translation. |
| Limits | `Exact Salesforce governor accounting profiles` | `unsupported` | Exact hosted governor deltas require Salesforce runtime accounting and remain outside local execution. |
| Limits | `Limits.get*` | `supported` | Returns deterministic local counters and configurable caps for all tracked getter families, including zero-valued local service counters. |
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
| Messaging | `Messaging.SendEmailOptions` | `supported` | Stores triggerUserEmail, triggerOtherEmail, and triggerAutoResponseEmail flags for local captured email records. |
| Messaging | `Messaging.SingleEmailMessage` | `supported` | Common DTO setters/getters, recipient fields, template fields, body fields, flags, and local file attachment values are modeled. |
| Messaging | `Messaging.renderStoredEmailTemplate hosted usage mutation` | `unsupported` | Hosted EmailTemplate usage counters and send-side usage mutation require Salesforce email services. |
| Messaging | `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption)` | `supported` | Renders local stored EmailTemplate subject, HTML, text bodies, merge fields, static-resource attachments, Attachment rows, and ContentDocumentLink attachments for NONE, METADATA_ONLY, BODY, and METADATA_WITH_BODY. |
| Messaging | `Messaging.renderStoredEmailTemplate(String,String,String,Messaging.AttachmentRetrievalOption,Boolean)` | `supported` | Renders local stored templates like the four-argument overload and accepts updateEmailTemplateUsage without remote usage mutation. |
| Messaging | `Messaging.sendEmail` | `supported` | Returns ordered local SendEmailResult values, validates supported DTOs, captures sent messages with SendEmailOptions metadata, and increments local email limits. |
| Messaging | `Messaging.sendEmail delivery transport` | `unsupported` | Outbound delivery depends on Salesforce email services and remains fenced behind stable UnsupportedFeature diagnostics locally. |
| Messaging | `Messaging.sendEmail(Messaging.Email[],Boolean)` | `supported` | Returns ordered local SendEmailResult values for supported email message DTOs; outbound delivery remains hosted. |
| PageReference | `PageReference` | `supported` | Constructor, URL, redirect, mutable parameters, headers, cookies, and current-page state are modeled. |
| PageReference | `PageReference(partialURL)` | `supported` | Builds a VM-local PageReference from a partial URL with mutable parameters and headers. |
| PageReference | `PageReference(record)` | `supported` | Builds a Visualforce PageReference from a local ApexPage SObject record. |
| PageReference | `PageReference.getContent and getContentAsPDF` | `unsupported` | Visualforce rendering and PDF generation require the hosted rendering service and raise stable UnsupportedFeature diagnostics locally. |
| Pattern | `Matcher.find` | `supported` | Oracle- and fixture-pinned regexp2-backed Java regex matching covers lookaround, named groups/backrefs, atomic groups, possessive quantifiers, \G, \R, \h/\v, (?U), Java Unicode aliases, UAX #29 \X and \b{g}, nested class algebra, and Apex UTF-16 code-unit match indexes. |
| Pattern | `Matcher.group` | `supported` | Returns fixture-pinned whole-match and capture groups, including unmatched optional groups, over the supported Java regex path with Apex UTF-16 code-unit group bounds. |
| Pattern | `Matcher.matches` | `supported` | Whole-region matching over the supported regexp2-backed Java regex path. |
| Pattern | `Pattern.compile` | `supported` | Compiles the supported regexp2-backed Java regex path: common flags, quote escapes, lookaround, named groups/backrefs, atomic groups, possessive quantifiers, \G, \R, \h/\v, (?U), Java Unicode aliases, UAX #29 \X and \b{g}, and nested class algebra. |
| Pattern | `Pattern.matches` | `supported` | Whole-string matching over the supported regexp2-backed Java regex path, including UAX #29 grapheme patterns. |
| QuickAction | `QuickAction.describeAvailableActions` | `supported` | Local QuickAction metadata, predefined default values, and deterministic DTO results are modeled. |
| QuickAction | `QuickAction.describeAvailableQuickActions(String)` | `supported` | Local QuickAction metadata, predefined default values, and deterministic DTO results are modeled. |
| QuickAction | `QuickAction.describeQuickActions(List<String>)` | `supported` | Local QuickAction metadata, predefined default values, and deterministic DTO results are modeled. |
| QuickAction | `QuickAction.performQuickAction` | `supported` | Returns deterministic local QuickActionResult DTOs for supported request shapes. |
| QuickAction | `QuickAction.performQuickAction(QuickAction.QuickActionRequest)` | `supported` | Returns deterministic local QuickActionResult DTOs for supported request shapes. |
| QuickAction | `QuickAction.performQuickAction(QuickAction.QuickActionRequest,Boolean)` | `supported` | Returns deterministic local QuickActionResult DTOs for supported request shapes. |
| QuickAction | `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>)` | `supported` | Returns deterministic local QuickActionResult DTOs for supported request shapes. |
| QuickAction | `QuickAction.performQuickActions(List<QuickAction.QuickActionRequest>,Boolean)` | `supported` | Returns deterministic local QuickActionResult DTOs for supported request shapes. |
| QuickAction | `QuickAction.retrieveQuickActionTemplate(String,Id)` | `supported` | Local QuickAction metadata, predefined default values, and deterministic template DTOs are modeled. |
| QuickAction | `QuickAction.retrieveQuickActionTemplates(List<String>,Id)` | `supported` | Local QuickAction metadata, predefined default values, and deterministic template DTOs are modeled. |
| Request | `Request.getCurrent()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| Request | `Request.getQuiddity()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| Request | `Request.getRequestId()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| Request | `RequestImpl.getCurrent()` | `supported` | Returns deterministic local request context values for local Apex execution. |
| ResetPasswordResult | `ResetPasswordResult.getPassword()` | `unsupported` | Password reset output is produced by Salesforce identity services and is not generated locally. |
| SObject | `SObject.setOptions(Database.DMLOptions)` | `supported` | Stores a cloned DMLOptions value on the local SObject for later DML option use. |
| Sandbox | `SandboxContext.organizationId()` | `supported` | Invokes the local test harness implementation. |
| Sandbox | `SandboxContext.sandboxId()` | `supported` | Invokes the local test harness implementation. |
| Sandbox | `SandboxContext.sandboxName()` | `supported` | Invokes the local test harness implementation. |
| Sandbox | `SandboxPostCopy.runApexClass(SandboxContext)` | `supported` | Invokes the local test harness implementation. |
| Schedulable | `Schedulable.execute(SchedulableContext)` | `supported` | Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs. |
| Schedulable | `SchedulableContext.getTriggerId()` | `supported` | Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs. |
| Schema | `DescribeFieldResult` | `supported` | Returns local metadata-backed field labels, picklists, reference targets, and access, create, and update booleans. |
| Schema | `DescribeSObjectResult` | `supported` | Returns local object labels, key prefixes, query and search flags, fields, record types, and child relationships. |
| Schema | `Hosted org describe and data category services` | `unsupported` | Live full-org describe breadth, admin-computed permissions, and hosted data category service lookup require Salesforce org services. |
| Schema | `Schema.describeDataCategoryGroupStructures(List<Schema.DataCategoryGroupSobjectTypePair>,Boolean)` | `supported` | Returns deterministic local data category structures from loaded org metadata. |
| Schema | `Schema.describeDataCategoryGroups(List<String>)` | `supported` | Returns deterministic local data category group describes from loaded org metadata and an empty typed list when absent. |
| Schema | `Schema.describeSObjects(List<String>)` | `supported` | Returns local metadata object names, labels, key prefixes, access booleans, fields, record types, and SObjectType tokens. |
| Schema | `Schema.getGlobalDescribe()` | `supported` | Returns a local describe map for loaded standard, custom, namespaced, and project objects. |
| Search | `Hosted search ranking and analyzers` | `unsupported` | Ranking, stemming, synonyms, language analyzers, external indexes, and hosted suggestion services require Salesforce search infrastructure. |
| Search | `Search.find` | `supported` | Returns deterministic SearchResult DTOs from fixed search results or local org records. |
| Search | `Search.find(String,AccessLevel)` | `supported` | Uses deterministic local org search with AccessLevel permission checks. |
| Search | `Search.find(String,Object)` | `supported` | Accepts Object-cast AccessLevel values against deterministic local org search. |
| Search | `Search.query / SOSL FIND` | `supported` | Parses supported RETURNING clauses and returns deterministic rows from fixed search results or local org records. |
| Search | `Search.query(String,AccessLevel)` | `supported` | Uses deterministic local org SOSL with AccessLevel permission checks. |
| Search | `Search.query(String,Object)` | `supported` | Accepts Object-cast AccessLevel values against deterministic local org SOSL. |
| Search | `Search.suggest` | `supported` | Returns deterministic local org suggestion DTOs from name-like fields. |
| Search | `Search.suggest(String,String,Object)` | `supported` | Accepts Object-cast SuggestionOption values and returns deterministic local org suggestions. |
| Search | `Search.suggest(String,String,Object,Object)` | `supported` | Accepts Object-cast SuggestionOption and AccessLevel values for deterministic local org suggestions. |
| Search | `Search.suggest(String,String,Search.SuggestionOption)` | `supported` | Returns deterministic local org suggestions and respects local limit options. |
| Search | `Search.suggest(String,String,Search.SuggestionOption,AccessLevel)` | `supported` | Returns deterministic local org suggestions with AccessLevel permission checks. |
| String | `String.contains` | `supported` | UTF-8 string contains. |
| String | `String.endsWith` | `supported` | UTF-8 string suffix. |
| String | `String.equalsIgnoreCase` | `supported` | Unicode simple fold. |
| String | `String.indexOf` | `supported` | Rune-indexed Unicode code-point search; unlike substring, indexes are not UTF-16 code units. |
| String | `String.isBlank` | `supported` | Null and whitespace. |
| String | `String.isNotBlank` | `supported` | Null and whitespace. |
| String | `String.join` | `supported` | List values and separator. |
| String | `String.lastIndexOf` | `supported` | Rune-indexed Unicode code-point search; unlike substring, indexes are not UTF-16 code units. |
| String | `String.length` | `supported` | Counts Apex UTF-16 code units. |
| String | `String.replace` | `supported` | Literal replacement. |
| String | `String.split` | `supported` | regexp2-backed Java Pattern split covers limits, empty-pattern splits, zero-width and nullable delimiters, numeric backreference delimiters, \Q...\E quote escapes, UAX #29 graphemes, nested class algebra, and Apex UTF-16 code-unit boundaries. |
| String | `String.startsWith` | `supported` | UTF-8 string prefix. |
| String | `String.substring` | `supported` | Indexes and slices by Apex UTF-16 code units. |
| String | `String.toLowerCase` | `supported` | Go Unicode lowercasing. |
| String | `String.toUpperCase` | `supported` | Go Unicode uppercasing. |
| String | `String.trim` | `supported` | Unicode whitespace trim. |
| String | `String.valueOf` | `supported` | Local value string conversion. |
| System | `System.assert` | `supported` | Assertion failure returns runtime error. |
| System | `System.assertEquals` | `supported` | Assertion failure returns runtime error. |
| System | `System.debug` | `supported` | Collected in result debug output. |
| System | `System.enqueueJob hosted wall-clock queue scheduling` | `unsupported` | Wall-clock queue scheduling and cross-transaction async locks require Salesforce async infrastructure. |
| System | `System.enqueueJob(Object,Object)` | `supported` | Accepts Integer and AsyncOptions queueable delay values, duplicate signatures, and local AsyncOptions maximumQueueableStackDepth for Test.stopTest drain behavior. |
| System | `System.runAs package install and license validation` | `unsupported` | Package installation and license validation require hosted subscriber org state. |
| System | `System.runAs(Object,Object)` | `supported` | Tracks local package-version test context during runAs execution. |
| System | `System.runAs(Package.Version)` | `supported` | Tracks local package-version test context during runAs execution. |
| System | `System.schedule(String,String,Object)` | `supported` | Queues and drains local Scheduled Apex jobs during Test.stopTest with deterministic CronTrigger IDs. |
| Test | `Test.createSoqlStub(Schema.SObjectType,SoqlStubProvider)` | `supported` | Registers test-local SOQL stubs per SObject type. |
| Test | `Test.createStub(Type,StubProvider)` | `supported` | Creates dynamic test stubs backed by StubProvider. |
| Test | `Test.createStubQueryRow` | `supported` | Builds one local SObject row from a field map for SOQL stub providers. |
| Test | `Test.createStubQueryRow(Schema.SObjectType,Map<String,Object>)` | `supported` | Builds one local SObject row from a field map for SOQL stub providers. |
| Test | `Test.createStubQueryRows` | `supported` | Builds local SObject row lists from field maps for SOQL stub providers. |
| Test | `Test.createStubQueryRows(Schema.SObjectType,List<Map<String,Object>>)` | `supported` | Builds local SObject row lists from field maps for SOQL stub providers. |
| Test | `Test.enableChangeDataCapture()` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.getEventBus()` | `supported` | Returns a local event-bus test broker that delivers queued platform-event triggers. |
| Test | `Test.getExternalService live service execution` | `unsupported` | External Service execution requires hosted named credential and generated service infrastructure. |
| Test | `Test.getExternalService()` | `supported` | Returns a deterministic local external-service harness. |
| Test | `Test.getStandardPricebookId` | `supported` | Returns the deterministic local standard pricebook ID in test context. |
| Test | `Test.invokeContinuationMethod(Object,Continuation)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.invokePage(PageReference)` | `supported` | Returns a typed Component.apex.page handle in test context without rendering Visualforce. |
| Test | `Test.isRunningTest` | `supported` | Reflects local test context. |
| Test | `Test.loadData` | `supported` | Loads CSV static-resource content with typed field coercion, DML routing, missing-resource errors, and bad-header diagnostics. |
| Test | `Test.loadData packaged resource and relationship external-ID expansion` | `unsupported` | Packaged static-resource namespace lookup and relationship external-ID fixture expansion require hosted packaging and schema behavior beyond local CSV loading. |
| Test | `Test.newSendEmailQuickActionDefaults(Id,Id)` | `supported` | Builds deterministic local send-email QuickAction defaults for test execution. |
| Test | `Test.setContinuationResponse(String,HttpResponse)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.setCurrentPage(PageReference)` | `supported` | Sets the VM-local current PageReference in test context. |
| Test | `Test.setCurrentPageReference(Object)` | `supported` | Accepts PageReference object values and stores the VM-local current page. |
| Test | `Test.setCurrentPageReference(PageReference)` | `supported` | Sets the VM-local current PageReference in test context. |
| Test | `Test.setMock` | `supported` | Registers HttpCalloutMock and WebServiceMock routes for local tests. |
| Test | `Test.startTest` | `supported` | Resets the active governor window for deterministic local counters. |
| Test | `Test.startTest and stopTest hosted service accounting` | `unsupported` | Hosted async service execution and exact Salesforce governor accounting require Salesforce runtime services. |
| Test | `Test.stopTest` | `supported` | Restores outer governor counters and drains supported local async work. |
| Test | `Test.testInstall(InstallHandler,Version)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.testInstall(InstallHandler,Version,Boolean)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.testNotificationActionHandler(Messaging.NotificationActionHandler,Messaging.ActionableNotification)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.testSandboxPostCopyScript(SandboxPostCopy,Id,Id,String,Boolean)` | `supported` | Invokes the local test harness implementation. |
| Test | `Test.testUninstall(UninstallHandler)` | `supported` | Invokes the local test harness implementation. |
| Time | `Time.hour` | `supported` | Local time component. |
| Time | `Time.minute` | `supported` | Local time component. |
| Time | `Time.newInstance` | `supported` | Validates time parts. |
| Time | `Time.second` | `supported` | Local time component. |
| Time | `Time.valueOf` | `supported` | Parses supported time strings. |
| TimeZone | `TimeZone.getDisplayName` | `supported` | Returns deterministic display names for local timezone values. |
| TimeZone | `TimeZone.getID` | `supported` | Returns local timezone IDs. |
| TimeZone | `TimeZone.getOffset` | `supported` | Returns offsets from the deterministic local timezone model. |
| TimeZone | `TimeZone.getTimeZone` | `supported` | Resolves timezone IDs into local timezone values. |
| TrailblazerIdentity | `TrailblazerIdentity.generateUserEmailVerificationToken(String,String,String)` | `supported` | Returns a deterministic local verification token for test execution. |
| TrailblazerIdentity | `TrailblazerIdentity.getUserOrgInfo(List<String>)` | `supported` | Returns an empty local UserOrgInfo list for deterministic test execution. |
| TrailblazerIdentity | `TrailblazerIdentity.splunkLog(String,String)` | `supported` | Accepts local log calls and returns null for deterministic test execution. |
| Type | `Type.forName` | `supported` | Resolves local classes, SObjects, built-ins, generic type names, generated platform types, and common platform namespace aliases. |
| Type | `Type.forName hosted package namespace reflection` | `unsupported` | Live package namespace reflection beyond local source and generated platform aliases requires hosted org metadata. |
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
| WebServiceCallout | `WebServiceCallout outbound SOAP transport` | `unsupported` | Real outbound SOAP transport requires external network service execution and is fenced locally. |
| WebServiceCallout | `WebServiceCallout.invoke(Object,Object,Map,List)` | `supported` | Validates SOAP options, routes generated SOAP callouts through registered WebServiceMock implementations, and materializes a deterministic local response shell when no mock is registered. |
| WebServiceCallout | `WebServiceCallout.invoke(Object,Object,Map<String,Object>,List<String>)` | `supported` | Validates SOAP options, routes generated SOAP callouts through registered WebServiceMock implementations, and materializes a deterministic local response shell when no mock is registered. |
