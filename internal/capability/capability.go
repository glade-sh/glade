package capability

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

type Status string

const (
	StatusSupported   Status = "supported"
	StatusPartial     Status = "partial"
	StatusStub        Status = "stub"
	StatusUnsupported Status = "unsupported"
	StatusUnknown     Status = "unknown"
)

type Feature struct {
	ID       string   `json:"id"`
	Area     string   `json:"area"`
	Name     string   `json:"name"`
	Status   Status   `json:"status"`
	Required bool     `json:"requiredForMVP"`
	Notes    string   `json:"notes,omitempty"`
	Blocks   []string `json:"blocks,omitempty"`
}

type Report struct {
	Target       string         `json:"target"`
	Ready        bool           `json:"ready"`
	Total        int            `json:"total"`
	Required     int            `json:"required"`
	Complete     int            `json:"complete"`
	Incomplete   int            `json:"incomplete"`
	StatusCounts map[Status]int `json:"statusCounts"`
	Features     []Feature      `json:"features"`
}

func MVPFeatures() []Feature {
	return cloneFeatures(mvpFeatures)
}

func MVPReport() Report {
	features := MVPFeatures()
	report := Report{
		Target: "full-featured glade-parity MVP",
		Ready:  true,
		Total:  len(features),
		StatusCounts: map[Status]int{
			StatusSupported:   0,
			StatusPartial:     0,
			StatusStub:        0,
			StatusUnsupported: 0,
			StatusUnknown:     0,
		},
		Features: features,
	}
	for _, feature := range features {
		report.StatusCounts[feature.Status]++
		if !feature.Required {
			continue
		}
		report.Required++
		if feature.Status == StatusSupported {
			report.Complete++
			continue
		}
		report.Incomplete++
		report.Ready = false
	}
	return report
}

func WriteJSON(w io.Writer, report Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func WriteText(w io.Writer, report Report) error {
	if report.Ready {
		_, _ = io.WriteString(w, "MVP readiness: ready\n")
	} else {
		_, _ = io.WriteString(w, "MVP readiness: not ready\n")
	}
	_, _ = io.WriteString(w, "Target: "+report.Target+"\n")
	_, _ = io.WriteString(w, "Required complete: ")
	_, _ = io.WriteString(w, itoa(report.Complete)+"/"+itoa(report.Required)+"\n")
	_, _ = io.WriteString(w, "Status counts: ")
	for i, status := range []Status{StatusSupported, StatusPartial, StatusStub, StatusUnsupported, StatusUnknown} {
		if i > 0 {
			_, _ = io.WriteString(w, " ")
		}
		_, _ = io.WriteString(w, string(status)+"="+itoa(report.StatusCounts[status]))
	}
	_, _ = io.WriteString(w, "\n")
	for _, feature := range report.Features {
		if !feature.Required || feature.Status == StatusSupported {
			continue
		}
		_, _ = io.WriteString(w, "- ["+string(feature.Status)+"] "+feature.Area+": "+feature.Name+"\n")
		if feature.Notes != "" {
			_, _ = io.WriteString(w, "  "+feature.Notes+"\n")
		}
	}
	return nil
}

func WriteMarkdown(w io.Writer, report Report) error {
	ready := "not ready"
	if report.Ready {
		ready = "ready"
	}

	if _, err := fmt.Fprintf(w, "# Compatibility Dashboard\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Generated from `internal/capability`.\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "## MVP Gate\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Target: %s\n", report.Target); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Readiness: %s\n", ready); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Required complete: %d/%d\n", report.Complete, report.Required); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Required incomplete: %d\n\n", report.Incomplete); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "## Status Counts\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| Status | Features |\n| --- | ---: |\n"); err != nil {
		return err
	}
	for _, status := range []Status{StatusSupported, StatusPartial, StatusStub, StatusUnsupported, StatusUnknown} {
		if _, err := fmt.Fprintf(w, "| `%s` | %d |\n", status, report.StatusCounts[status]); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "\n## Required MVP Capabilities\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| Area | ID | Status | Capability | Notes |\n| --- | --- | --- | --- | --- |\n"); err != nil {
		return err
	}
	for _, feature := range report.Features {
		if !feature.Required {
			continue
		}
		if _, err := fmt.Fprintf(
			w,
			"| %s | `%s` | `%s` | %s | %s |\n",
			escapeMarkdownTable(feature.Area),
			escapeMarkdownTable(feature.ID),
			feature.Status,
			escapeMarkdownTable(feature.Name),
			escapeMarkdownTable(feature.Notes),
		); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "\n## Tracked Post-MVP Capabilities\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "| Area | ID | Status | Capability | Notes |\n| --- | --- | --- | --- | --- |\n"); err != nil {
		return err
	}
	for _, feature := range report.Features {
		if feature.Required {
			continue
		}
		if _, err := fmt.Fprintf(
			w,
			"| %s | `%s` | `%s` | %s | %s |\n",
			escapeMarkdownTable(feature.Area),
			escapeMarkdownTable(feature.ID),
			feature.Status,
			escapeMarkdownTable(feature.Name),
			escapeMarkdownTable(feature.Notes),
		); err != nil {
			return err
		}
	}
	return nil
}

func WriteKnownGapsMarkdown(w io.Writer, report Report) error {
	if _, err := fmt.Fprintf(w, "# Known Gaps\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Generated from `internal/capability`.\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "The MVP target is `%s`. This document lists required capabilities that are not yet `supported`.\n\n", report.Target); err != nil {
		return err
	}
	if report.Ready {
		_, err := fmt.Fprintf(w, "No required MVP capability gaps are currently tracked.\n")
		return err
	}

	if _, err := fmt.Fprintf(w, "## Summary\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Required complete: %d/%d\n", report.Complete, report.Required); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Required incomplete: %d\n\n", report.Incomplete); err != nil {
		return err
	}

	currentArea := ""
	for _, feature := range report.Features {
		if !feature.Required || feature.Status == StatusSupported {
			continue
		}
		if feature.Area != currentArea {
			currentArea = feature.Area
			if _, err := fmt.Fprintf(w, "## %s\n\n", currentArea); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "### `%s`: %s\n\n", feature.ID, feature.Name); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "- Status: `%s`\n", feature.Status); err != nil {
			return err
		}
		if feature.Notes != "" {
			if _, err := fmt.Fprintf(w, "- Gap: %s\n", feature.Notes); err != nil {
				return err
			}
		}
		if len(feature.Blocks) > 0 {
			if _, err := fmt.Fprintf(w, "- Blocks: %s\n", strings.Join(feature.Blocks, ", ")); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(w, "\n"); err != nil {
			return err
		}
	}
	return nil
}

func cloneFeatures(features []Feature) []Feature {
	out := make([]Feature, len(features))
	copy(out, features)
	for i := range out {
		out[i].Blocks = append([]string(nil), out[i].Blocks...)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Area != out[j].Area {
			return out[i].Area < out[j].Area
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}

func escapeMarkdownTable(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	if value == "" {
		return " "
	}
	return value
}

var mvpFeatures = []Feature{
	{ID: "apex.parser.project-scale", Area: "Apex front end", Name: "Parse and index large SFDX projects", Status: StatusSupported, Required: true, Notes: "Parser and symbol baselines cover qualified nested type symbols, stable malformed-parse diagnostics, type-index/sema panic recovery diagnostics, enterprise-style multi-class check fixtures, selector/service/domain fixtures with object metadata schemas, namespace/package-directory fixtures, and bounded large-index stress coverage for the local Apex execution corpus."},
	{ID: "apex.sema.body", Area: "Apex front end", Name: "Method-body semantic analysis", Status: StatusSupported, Required: true, Notes: "Method-body sema covers declarations, member and parameter type references, project namespace-qualified and namespace-token schema references, visibility and override checks, interface and abstract implementation checks, local declarations, duplicate locals, initializer and assignment type mismatches, return type and non-void path checks, constructor references and chaining, non-instantiable constructor calls, unknown variable reads, project method calls, inherited/interface/super calls, this/super field and return type inference, inherited instance field scope, @TestVisible access, overload arity and specificity, numeric widening, decimal literal typing, simple binary expression typing, object assignability, generic collection constructor assignability, receiver and constructor-call return typing, IR-backed scoped local reads, Boolean conditions, user-object field reads and writes, constructor-call validation across statements/control-flow bodies, and token-level diagnostic ranges for the supported VM subset."},
	{ID: "vm.classes", Area: "Runtime", Name: "Classes, methods, constructors, statics, properties", Status: StatusSupported, Required: true, Notes: "The VM registers class metadata from project tests, constructs objects with instance fields/properties, invokes property getter/setter bodies, runs constructor bodies, supports this(...) and super(...) constructor chaining, rejects interface/enum/abstract instantiation, blocks abstract method invocation, rejects non-void method fallthrough, matches overloaded methods/constructors by argument types with numeric, class/interface, null, and ambiguity specificity baselines, executes static and instance field initializer expressions and initializer blocks in source order, resets statics through field initializers and static blocks, stores static fields, dispatches overrides through inheritance including superclass-typed and interface-typed references, resolves super calls from the declaring class, prefers inherited concrete methods before interface fallback methods, resolves inherited static fields and static methods through subclass names, supports interface fallback method lookup, resolves namespace-qualified class names and namespace-token SObject/field aliases through construction, access, DML, and SOQL, supports nested classes with constructors, fields, methods, static members, relative owner-local type names, nested interfaces, nested enum values/methods, explicit object toString() dispatch for calls/debug/assert messages, user object identity equality, and typed coercion for locals, params, returns, fields, collection members, null/object/enum values, numeric widening, and schema-backed DML storage, and enforces private/protected visibility through inheritance chains, @TestVisible method access from test classes, and namespace-global class/member access."},
	{ID: "vm.control-flow", Area: "Runtime", Name: "Control flow and exceptions", Status: StatusSupported, Required: true, Notes: "Anonymous and test method execution supports for/enhanced-for/do-while, break/continue, switch-on with switch-local break, throw, ordered catch blocks, pipe-style multi-catch, try/catch/finally including finally-on-return, return override, throw unwinding, finally-preserved and finally-overridden loop signals, bare rethrow with original stack preservation, exception messages/getMessage, getTypeName, getLineNumber, getStackTraceString, System.*Exception name normalization, catchable null dereference, interface-based catch matching, and exception hierarchy matching for common Apex exception types, with VM tests and a focused compatibility fixture covering the row."},
	{ID: "stdlib.core", Area: "Runtime", Name: "Core System/String/Date/Datetime/JSON/Math APIs", Status: StatusSupported, Required: true, Notes: "Local MVP stdlib support includes assertions, debug, collections, common String helpers including trim/indexOf/lastIndexOf/replace/split/join/blank checks/equality, Pattern/Matcher basics, Limits counters including future, queueable, email, and runAs getters, Date/Datetime/Time factories, parsing, arithmetic, and component access basics, Decimal literals/arithmetic/storage conversion plus setScale/round/intValue, Math numeric helpers including pow/sqrt/floor/ceil/round, Type.forName and Type.newInstance basics, JSON serialize overloads including suppressApexObjectNulls, serializePretty, deserialize, deserializeStrict, and deserializeUntyped, Database.getQueryLocator for supported SOQL and batch starts, Database.setSavepoint/rollback for local org snapshots, EncodingUtil base64/hex/url helpers, Crypto MD5/SHA1/SHA-256 digests, Schema global describe and describeSObjects basics, SObjectType getDescribe, SObject fields.getMap, SObject/field describe access booleans, field describe basics with picklist entries, SObject record type describe maps/lists with common RecordTypeInfo methods, child relationship describe basics, Test.isRunningTest, Test.getStandardPricebookId, Test.createStubQueryRow(s), Test.loadData from local static resources, UserInfo user/profile/org/locale/timezone basics, FeatureManagement user permission-list checks, Messaging send email result/message basics, ApexPages message/current page basics, URL/PageReference basics, request/response-shaped callout mocks, and explicit UnsupportedFeature diagnostics for calls outside the local stdlib contract."},
	{ID: "tests.runner", Area: "Tests", Name: "Run real Apex test classes", Status: StatusSupported, Required: true, Notes: "Discovery, method dispatch, project helper classes/triggers, @TestSetup data snapshots, per-method org/VM isolation, static reset, startTest/stopTest governor windows, runAs user/profile/permission identity, Queueable/Future/Batch/Scheduled stopTest draining, durable AsyncApexJob/CronTrigger records, and statement-level assertion/runtime stack frames are covered for the supported VM subset."},
	{ID: "tests.salesforce-semantics", Area: "Tests", Name: "@TestSetup, startTest/stopTest, runAs, isolation", Status: StatusSupported, Required: true, Notes: "@TestSetup runs once per test class into an org snapshot, each test method gets a fresh clone with statics reset, test mutations and async jobs stay isolated, startTest/stopTest preserve and restore outer governor counters, runAs scopes UserInfo identity and FeatureManagement permission-list checks, mixed DML is guarded with runAs escape for supported local test modes, and local tests run in pinned system-sharing mode."},
	{ID: "async.core", Area: "Tests", Name: "Queueable/Future/Batch/Scheduled basics", Status: StatusSupported, Required: true, Notes: "System.enqueueJob, @future calls, Database.executeBatch, and System.schedule queue deterministic jobs in test context; Test.stopTest drains Queueable execute, Future methods, Batchable start/execute chunks/finish, and Schedulable execute; Queueable chaining is capped to one child per job by the async limit window; AsyncApexJob and CronTrigger records expose durable local job state."},
	{ID: "limits.core", Area: "Limits", Name: "Governor counters and strict/permissive enforcement", Status: StatusSupported, Required: true, Notes: "Local MVP governor support tracks SOQL queries/rows including projected child relationship rows, DML statements/rows including cascade-delete child rows, deterministic live heap approximation across locals and mutated collections, deterministic CPU cost from statements plus SOQL/DML row work, callouts, aggregate async jobs, future calls, queueable jobs, batch jobs, scheduled jobs, email invocations, and runAs calls. Limits.* exposes current and max counters for supported SOQL, DML, heap, CPU, callout, aggregate async, future, queueable, batch, scheduled, email, and runAs counters; Test.startTest/Test.stopTest reset and restore test windows; permissive mode records violations; strict mode raises System.LimitException; and glade exec/test/server plus compatibility exec/test fixtures accept limit-mode selection."},
	{ID: "sobject.apex", Area: "Data runtime", Name: "Apex-integrated SObject construction and field access", Status: StatusSupported, Required: true, Notes: "Apex supports schema-backed SObject construction, typed field access, dotted assignment, get/put with previous-value return, isSet, clear, getPopulatedFieldsAsMap including explicit nulls, common system fields after DML and SOQL projection, SObjectType getDescribe, fields.getMap, field describe basics with picklist values, record type describe maps/lists with deterministic local IDs and common RecordTypeInfo methods, child relationship describe basics, object-level and field-level addError with optional escapeHtml arguments, multi-error hasErrors/getErrors and DML result shaping, Id propagation after DML, parent relationship projection access, VM/storage record conversion, and bounded describe-heavy stress coverage."},
	{ID: "soql.apex", Area: "Data runtime", Name: "Static and dynamic SOQL from Apex", Status: StatusSupported, Required: true, Notes: "Static SOQL literals, Database.query, and Database.queryWithBinds execute against the in-memory org with bind variables beside operators, dotted bind paths for frame variables, bind maps, collection binds, projection, FIELDS(ALL/STANDARD/CUSTOM), TYPEOF relationship projection, multi-hop parent relationship fields/filters, polymorphic relationship target resolution from multi-reference metadata, child relationship subqueries, semi-joins, anti-joins, COUNT(), COUNT(field), COUNT_DISTINCT, SUM, MIN, MAX, AVG, GROUP BY, ROLLUP, CUBE, HAVING on aggregate expressions, aggregate aliases, GROUPING(field), common date literals, AggregateResult exprN fields, single-SObject assignment, serialized row attributes type/url shape, soft-deleted row visibility through ALL ROWS, equality/inequality/comparison filters with single-field equality index candidates, AND/OR boolean combinations, IN/NOT IN, LIKE, NOT, parentheses, comma-separated ORDER BY ASC/DESC with NULLS FIRST/LAST, FOR UPDATE parsing with local lock markers and contention errors for already locked local rows, WITH SECURITY_ENFORCED/USER_MODE/SYSTEM_MODE parsing with local projection validation, limit, offset, and QueryException parse errors for the local Apex execution corpus."},
	{ID: "dml.apex", Area: "Data runtime", Name: "Apex DML statements and Database methods", Status: StatusSupported, Required: true, Notes: "Apex insert/update/delete/upsert/undelete/merge syntax and Database.insert/update/delete/upsert/undelete/merge allOrNone paths call the DML engine, return SaveResult/UpsertResult/MergeResult objects with single and multi-entry Database.Error lists carrying statusCode, message, and fields arrays, isCreated, merged record IDs, set Ids, stamp common system fields, roll back allOrNone failures, soft-delete and undelete records, match implicit and explicit external-ID upserts, reject ID/object mismatches, enforce unique fields, validate lookup references, enforce simple Metadata API validation rules, reparent lookups on merge, fire supported upsert insert/update and merge update/delete trigger hooks only for successful rows, cascade soft-delete children from relationship metadata, and include bounded bulk-DML stress coverage. Trigger context includes operation flags, size, new/old lists, nullable unavailable contexts, and newMap/oldMap for supported operations."},
	{ID: "triggers.runtime", Area: "Data runtime", Name: "Trigger invocation and context", Status: StatusSupported, Required: true, Notes: "Project triggers compile and invoke from VM DML for before/after operations with Trigger.new/old/maps/flags/operationType/size basics, upsert split into insert/update trigger contexts, after-undelete context, no before-undelete invocation, bulk partial-success row alignment before and after engine validation, deterministic recursion guard rollback, merge master update hooks, merge duplicate delete hooks, rollback on thrown errors, and object-level/field-level addError shaping single and multiple row SaveResult errors with field lists."},
	{ID: "fixtures.persistence", Area: "Data runtime", Name: "Seed/export/reset local fixtures with persistence", Status: StatusSupported, Required: true, Notes: "SQLite-backed org storage persists object definitions, records, ID sequences, schema migrations/versioning, fixture seed/export/reset/inspect, object-aware alias and relationship reference resolution, qualified Object.alias refs, ambiguity checks, reference-target validation, deterministic Organization/Profile/UserRole/User/PermissionSet/PermissionSetAssignment/RecordType platform data, local org settings, DB lifecycle and export re-import compatibility coverage, server persistence, scoped fixture reset endpoints for data/users/platform/limits/async, transaction-scoped prepared inserts, storage performance pragmas, large-fixture save/load and stress coverage, cloned-org commit boundaries for mutating server requests and Tooling executeAnonymous, and serialized server request handling to avoid concurrent lost updates."},
	{ID: "dap.command", Area: "Developer experience", Name: "VS Code debug flow through glade test/exec --debug", Status: StatusSupported, Required: true, Notes: "Local MVP DAP support includes content-length transport, setBreakpoints, continue/pause/next/stepIn/stepOut/disconnect, stackTrace with trace-provided line/column positions, scopes, variables, evaluate, VM debug pause hooks, live statement breakpoint stops with stack/locals/static snapshots, stack-depth live stepping, Locals/Statics/Trigger scopes with object, SObject, exception, and collection children, paused-context watch expressions for locals, fields, statics, Trigger values, list/set indexes, and map keys, plus glade exec/test --debug snapshot sessions."},
	{ID: "lsp.command", Area: "Developer experience", Name: "glade lsp core editor features", Status: StatusSupported, Required: true, Notes: "Local MVP LSP support includes stdio transport with initialize, diagnostics matching the shared glade check diagnostic model, open-buffer parse overlays, test-result diagnostics from stack frames, incremental text document sync through didOpen/didChange/didClose overlays, document/workspace symbols, semantic tokens, definition, references, prepare-rename/rename workspace edits, hover, and completion for Apex types, members, SObjects, fields, and keywords."},
	{ID: "watch.command", Area: "Developer experience", Name: "glade test --watch affected-test loop", Status: StatusSupported, Required: true, Notes: "Local MVP watch support includes glade test --watch and --watch-once with fsnotify native watching, recursive directory registration, automatic polling fallback, explicit --watch-backend auto|native|poll selection, debounce, versioned newline-delimited JSON events with stable run IDs and test class arrays, incremental Apex-only type-index updates, dependency-graph affected-test selection, cancellable in-flight VM/test reruns, and stale run-result suppression."},
	{ID: "profile.native", Area: "Developer experience", Name: "Native trace/profile reports", Status: StatusSupported, Required: true, Notes: "Local MVP profile support reads native Chrome Trace Event output and emits JSON/Markdown reports aggregating statements, methods, SOQL, DML, describe, callout, email, async enqueue/run, trigger, and limit-summary events with hot-event, category, runtime-section, SOQL/DML row-delta, and platform/resource counter attribution."},
	{ID: "server.local-api", Area: "Local API server", Name: "Salesforce-shaped local API with CRUD/query/executeAnonymous", Status: StatusSupported, Required: true, Notes: "Local MVP server support includes version/root discovery, SObject CRUD with normal REST JSON payloads and explicit nulls, describe/recent, query/queryAll with REST-shaped attributes and soft-deleted row visibility, limits, OAuth userinfo/id stubs with local user selection, Tooling executeAnonymous GET/POST success and failure shapes, supported local-object Tooling queries, Composite sObject insert with referenceId/partial success/allOrNone rollback, Salesforce-shaped error arrays, SQLite persistence, and Glade fixture/scoped reset endpoints."},
	{ID: "compat.dashboard", Area: "Release", Name: "Generated compatibility dashboard and CI gate", Status: StatusSupported, Required: true, Notes: "Local MVP support includes the MVP gate, JSON matrix, generated Markdown dashboard, CI drift check, and parse/check/exec/test/DB/server lifecycle fixture runner, including schema-aware enterprise check fixtures, trigger-heavy, async-heavy, describe-heavy, namespace/package-style, and server black-box fixtures."},
	{ID: "release.packaging", Area: "Release", Name: "Installable release binaries, checksums, docs", Status: StatusSupported, Required: true, Notes: "Local MVP support includes parser-capable release archives built with CGO enabled, checksum files, GitHub Release upload workflow, install docs, release policy, and smoke coverage for the shippable host binary path."},
	{ID: "dap.live-ide-orchestration", Area: "Developer experience", Name: "Richer live IDE debug orchestration", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers richer IDE launch/run orchestration beyond the local glade exec/test --debug DAP sessions."},
	{ID: "lsp.context-completion", Area: "Developer experience", Name: "Context-aware Apex completion", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers deeper context-aware completion ranking and insertion beyond local MVP type, member, SObject, field, and keyword completion."},
	{ID: "profile.pprof-and-timing", Area: "Developer experience", Name: "pprof output and wall-clock statement timing", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers pprof-compatible CPU output and per-statement wall-clock attribution beyond native trace/profile JSON and Markdown reports."},
	{ID: "watch.profile-trace-reports", Area: "Developer experience", Name: "Profile-driven watch reports", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers profile and trace driven watch summaries beyond local affected-test selection and rerun events."},
	{ID: "limits.exact-accounting", Area: "Limits", Name: "Exact Salesforce governor accounting and configurable caps", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers exact Salesforce accounting deltas and configurable per-test cap profiles beyond the deterministic local MVP governor model."},
	{ID: "server.rest-breadth", Area: "Local API server", Name: "Broader REST, Tooling, Composite, Bulk, auth, and layout coverage", Status: StatusPartial, Required: false, Notes: "Post-MVP REST breadth is split from server.local-api: full auth, unmodeled Tooling objects, Composite batch/Graph, Bulk API, layout metadata, and broader REST namespaces stay explicit unsupported until fixture-backed local integration tests need them."},
	{ID: "compat.fixture-expansion", Area: "Release", Name: "Compatibility fixture expansion", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers additional compatibility fixture breadth beyond the local MVP fixture gate."},
	{ID: "release.distribution-automation", Area: "Release", Name: "Package-manager distribution, signing, and promotion automation", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers published package-manager distribution, stronger artifact signing, and release promotion automation beyond parser-capable release archives and checksums."},
	{ID: "stdlib.platform-breadth", Area: "Runtime", Name: "Broader System stdlib and platform service parity", Status: StatusPartial, Required: false, Notes: "Post-MVP work covers broader platform stdlib service parity and exact edge semantics beyond the local MVP supported calls and explicit UnsupportedFeature fences."},
}
