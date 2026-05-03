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
	Target     string    `json:"target"`
	Ready      bool      `json:"ready"`
	Total      int       `json:"total"`
	Required   int       `json:"required"`
	Complete   int       `json:"complete"`
	Incomplete int       `json:"incomplete"`
	Features   []Feature `json:"features"`
}

func MVPFeatures() []Feature {
	return cloneFeatures(mvpFeatures)
}

func MVPReport() Report {
	features := MVPFeatures()
	report := Report{
		Target:   "full-featured aer-parity MVP",
		Ready:    true,
		Total:    len(features),
		Features: features,
	}
	for _, feature := range features {
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
	counts := map[Status]int{}
	for _, feature := range report.Features {
		counts[feature.Status]++
	}

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
		if _, err := fmt.Fprintf(w, "| `%s` | %d |\n", status, counts[status]); err != nil {
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
	{ID: "apex.parser.project-scale", Area: "Apex front end", Name: "Parse and index large SFDX projects", Status: StatusPartial, Required: true, Notes: "Parser and symbol baselines exist, including qualified nested type symbols, stable malformed-parse diagnostics, type-index/sema panic recovery diagnostics, and an enterprise-style multi-class check fixture. Broader real-repository scale and method-body model fidelity are incomplete."},
	{ID: "apex.sema.body", Area: "Apex front end", Name: "Method-body semantic analysis", Status: StatusPartial, Required: true, Notes: "Current sema checks declarations, member and parameter type references, project namespace-qualified and namespace-token schema references, basic visibility conflicts, interface member visibility, override markers, missing concrete interface/abstract method implementations, and a conservative method-body baseline for local declarations, duplicate locals, local initializer and simple assignment type mismatches, simple return type mismatches and missing non-void returns, constructor references, constructor chaining, non-instantiable interface/enum/abstract constructor calls, unknown variable reads in call arguments, project method calls, inherited/interface/super calls, this/super field and return type inference, inherited instance field scope, private/protected method and field visibility through inheritance chains, @TestVisible method access from test classes, known-receiver overload arity/argument matching with exact and narrowest numeric specificity, nearest class/interface specificity, null specificity, and ambiguous overload diagnostics, integer-to-Long/Decimal/Double widening, decimal-literal argument typing, simple binary expression typing, class/interface object assignability, generic collection constructor assignability, known method-call return typing for receiver and chained constructor calls, an IR-backed sema pass for scoped local reads, Boolean conditions, declaration/assignment/return type checks, all-path non-void returns, known user-object field reads/writes, known receiver/same-class method calls, and constructor-call validation across statements/control-flow bodies, and token-level ranges for body diagnostics. Full expression typing and full flow analysis remain incomplete."},
	{ID: "vm.classes", Area: "Runtime", Name: "Classes, methods, constructors, statics, properties", Status: StatusSupported, Required: true, Notes: "The VM registers class metadata from project tests, constructs objects with instance fields/properties, invokes property getter/setter bodies, runs constructor bodies, supports this(...) and super(...) constructor chaining, rejects interface/enum/abstract instantiation, blocks abstract method invocation, rejects non-void method fallthrough, matches overloaded methods/constructors by argument types with numeric, class/interface, null, and ambiguity specificity baselines, executes static and instance field initializer expressions and initializer blocks in source order, resets statics through field initializers and static blocks, stores static fields, dispatches overrides through inheritance including superclass-typed and interface-typed references, resolves super calls from the declaring class, prefers inherited concrete methods before interface fallback methods, resolves inherited static fields and static methods through subclass names, supports interface fallback method lookup, resolves namespace-qualified class names and namespace-token SObject/field aliases through construction, access, DML, and SOQL, supports nested classes with constructors, fields, methods, static members, relative owner-local type names, nested interfaces, nested enum values/methods, explicit object toString() dispatch for calls/debug/assert messages, user object identity equality, and typed coercion for locals, params, returns, fields, collection members, null/object/enum values, numeric widening, and schema-backed DML storage, and enforces private/protected visibility through inheritance chains, @TestVisible method access from test classes, and namespace-global class/member access."},
	{ID: "vm.control-flow", Area: "Runtime", Name: "Control flow and exceptions", Status: StatusPartial, Required: true, Notes: "Anonymous and test method execution now supports for/enhanced-for/do-while, break/continue, switch-on with switch-local break, throw, ordered catch blocks, pipe-style multi-catch, try/catch/finally including finally-on-return, return override, throw unwinding, finally-preserved and finally-overridden loop signals, bare rethrow with original stack preservation, exception messages/getMessage, getTypeName, getLineNumber, getStackTraceString, System.*Exception name normalization, catchable null dereference, interface-based catch matching, and exception hierarchy matching for common Apex exception types. Remaining gaps are outside control-flow and exception semantics."},
	{ID: "stdlib.core", Area: "Runtime", Name: "Core System/String/Date/Datetime/JSON/Math APIs", Status: StatusPartial, Required: true, Notes: "Assertions, debug, collections, common String helpers including trim/indexOf/lastIndexOf/replace/split/join/blank checks/equality, Pattern/Matcher basics, Limits counters including future, queueable, and email getters, Date/Datetime/Time factories, parsing, arithmetic, and component access basics, Decimal literals/arithmetic/storage conversion, Math integer helpers, Type.forName and Type.newInstance basics, JSON serialize overloads including suppressApexObjectNulls, serializePretty, deserialize, deserializeStrict, and deserializeUntyped, Database.getQueryLocator for supported SOQL and batch starts, Database.setSavepoint/rollback for local org snapshots, EncodingUtil, Crypto SHA-256, Schema global describe and describeSObjects basics, SObjectType getDescribe, SObject fields.getMap, SObject/field describe access booleans, field describe basics with picklist entries, SObject record type describe maps/lists with common RecordTypeInfo methods, child relationship describe basics, Test.isRunningTest, Test.getStandardPricebookId, UserInfo user/profile identity in test context, FeatureManagement user permission-list checks, Messaging, ApexPages, and HttpResponse-shaped callout mocks now exist for the supported VM subset."},
	{ID: "tests.runner", Area: "Tests", Name: "Run real Apex test classes", Status: StatusSupported, Required: true, Notes: "Discovery, method dispatch, project helper classes/triggers, @TestSetup data snapshots, per-method org/VM isolation, static reset, startTest/stopTest governor windows, runAs user/profile/permission identity, Queueable/Future/Batch/Scheduled stopTest draining, durable AsyncApexJob/CronTrigger records, and statement-level assertion/runtime stack frames are covered for the supported VM subset."},
	{ID: "tests.salesforce-semantics", Area: "Tests", Name: "@TestSetup, startTest/stopTest, runAs, isolation", Status: StatusSupported, Required: true, Notes: "@TestSetup runs once per test class into an org snapshot, each test method gets a fresh clone with statics reset, test mutations and async jobs stay isolated, startTest/stopTest preserve and restore outer governor counters, runAs scopes UserInfo identity and FeatureManagement permission-list checks, mixed DML is guarded with runAs escape for supported local test modes, and local tests run in pinned system-sharing mode."},
	{ID: "async.core", Area: "Tests", Name: "Queueable/Future/Batch/Scheduled basics", Status: StatusSupported, Required: true, Notes: "System.enqueueJob, @future calls, Database.executeBatch, and System.schedule queue deterministic jobs in test context; Test.stopTest drains Queueable execute, Future methods, Batchable start/execute chunks/finish, and Schedulable execute; Queueable chaining is capped to one child per job by the async limit window; AsyncApexJob and CronTrigger records expose durable local job state."},
	{ID: "limits.core", Area: "Limits", Name: "Governor counters and strict/permissive enforcement", Status: StatusPartial, Required: true, Notes: "The VM now tracks SOQL queries/rows including projected child relationship rows, DML statements/rows including cascade-delete child rows, deterministic live heap approximation across locals and mutated collections, deterministic CPU cost from statements plus SOQL/DML row work, callouts, aggregate async jobs, future calls, queueable jobs, batch jobs, scheduled jobs, and email invocations. Limits.* exposes current and max counters for supported SOQL, DML, heap, CPU, callout, aggregate async, future, queueable, batch, scheduled, and email counters, Test.startTest/Test.stopTest reset and restore test windows, permissive mode records violations, strict mode raises System.LimitException, and oaer exec/test/server plus compatibility exec/test fixtures accept limit-mode selection. Exact Salesforce accounting and configurable per-test caps remain incomplete."},
	{ID: "sobject.apex", Area: "Data runtime", Name: "Apex-integrated SObject construction and field access", Status: StatusPartial, Required: true, Notes: "Apex now supports schema-backed new Account(Name='Acme'), typed field access, dotted assignment, get/put with previous-value return, isSet, clear, getPopulatedFieldsAsMap including explicit nulls, common system fields after DML and SOQL projection, SObjectType getDescribe, fields.getMap, field describe basics with picklist values, record type describe maps/lists with deterministic local IDs and common RecordTypeInfo methods, child relationship describe basics, object-level and field-level addError with optional escapeHtml arguments, multi-error hasErrors/getErrors and DML result shaping, Id propagation after DML, parent relationship projection access, and VM/storage record conversion. Permissions, complete typed describe APIs, and broader system field parity remain incomplete."},
	{ID: "soql.apex", Area: "Data runtime", Name: "Static and dynamic SOQL from Apex", Status: StatusPartial, Required: true, Notes: "Static SOQL literals, Database.query, and Database.queryWithBinds now execute against the in-memory org with bind variables beside operators, dotted bind paths for frame variables, bind maps, collection binds, projection, FIELDS(ALL/STANDARD/CUSTOM), TYPEOF relationship projection, multi-hop parent relationship fields/filters, polymorphic relationship target resolution from multi-reference metadata, child relationship subqueries, semi-joins, anti-joins, COUNT(), COUNT(field), COUNT_DISTINCT, SUM, MIN, MAX, AVG, GROUP BY, ROLLUP, CUBE, HAVING on aggregate expressions, aggregate aliases, GROUPING(field), common date literals, AggregateResult exprN fields, single-SObject assignment, serialized row attributes type/url shape, soft-deleted row visibility through ALL ROWS, equality/inequality/comparison filters with single-field equality index candidates, AND/OR boolean combinations, IN/NOT IN, LIKE, NOT, parentheses, comma-separated ORDER BY ASC/DESC with NULLS FIRST/LAST, FOR UPDATE parsing with local lock markers and contention errors for already locked local rows, WITH SECURITY_ENFORCED/USER_MODE/SYSTEM_MODE parsing with local projection validation, limit, offset, and QueryException parse errors. Full permission enforcement and broader polymorphic relationship edge cases remain incomplete."},
	{ID: "dml.apex", Area: "Data runtime", Name: "Apex DML statements and Database methods", Status: StatusPartial, Required: true, Notes: "Apex insert/update/delete/upsert/undelete/merge syntax and Database.insert/update/delete/upsert/undelete/merge allOrNone paths now call the DML engine, return SaveResult/UpsertResult/MergeResult objects with single and multi-entry Database.Error lists carrying statusCode, message, and fields arrays, isCreated, merged record IDs, set Ids, stamp common system fields, roll back allOrNone failures, soft-delete and undelete records, match implicit and explicit external-ID upserts, reject ID/object mismatches, enforce unique fields, validate lookup references, enforce simple Metadata API validation rules, reparent lookups on merge, fire supported upsert insert/update and merge update/delete trigger hooks only for successful rows, and cascade soft-delete children from relationship metadata. Trigger context includes operation flags, size, new/old lists, nullable unavailable contexts, and newMap/oldMap for supported operations. Complex validation-rule formulas, full merge loser relationship result details, and full Salesforce status-code parity remain incomplete."},
	{ID: "triggers.runtime", Area: "Data runtime", Name: "Trigger invocation and context", Status: StatusPartial, Required: true, Notes: "Project triggers are compiled and invoked from VM DML for before/after operations with Trigger.new/old/maps/flags/operationType/size basics, upsert split into insert/update trigger contexts, after-undelete context, no before-undelete invocation, bulk partial-success row alignment before and after engine validation, deterministic recursion guard rollback, merge master update hooks, merge duplicate delete hooks, rollback on thrown errors, and object-level/field-level addError shaping single and multiple row SaveResult errors with field lists. Full bulk ordering semantics and relationship side effects remain incomplete."},
	{ID: "fixtures.persistence", Area: "Data runtime", Name: "Seed/export/reset local fixtures with persistence", Status: StatusPartial, Required: true, Notes: "SQLite-backed org storage now persists object definitions, records, ID sequences, schema migrations/versioning, fixture seed/export/reset/inspect, alias and relationship reference resolution, deterministic users/profiles/permissions, DB lifecycle compatibility coverage, server persistence, and fixture/reset endpoints. Large-fixture performance tuning and richer permission semantics remain incomplete."},
	{ID: "dap.command", Area: "Developer experience", Name: "VS Code debug flow through oaer test/exec --debug", Status: StatusPartial, Required: true, Notes: "DAP content-length transport, setBreakpoints, continue/pause/next, stackTrace with trace-provided line/column positions, scopes, variables, evaluate, and oaer exec/test --debug snapshot sessions are wired. True live VM suspension, step-in/out semantics, and breakpoint-driven execution control remain incomplete."},
	{ID: "lsp.command", Area: "Developer experience", Name: "oaer lsp core editor features", Status: StatusPartial, Required: true, Notes: "oaer lsp now runs a stdio LSP transport with initialize, diagnostics, document/workspace symbols, hover, and completion. Definition, references, semantic tokens, and incremental document sync remain incomplete."},
	{ID: "watch.command", Area: "Developer experience", Name: "oaer test --watch affected-test loop", Status: StatusPartial, Required: true, Notes: "oaer test --watch now runs a polling watch loop with debounce, JSON event stream, affected-test selection, reruns, and context cancellation. Native OS watcher backends and in-flight VM cancellation remain incomplete."},
	{ID: "profile.native", Area: "Developer experience", Name: "Native trace/profile reports", Status: StatusPartial, Required: true, Notes: "Trace/profile reports aggregate statements, methods, SOQL, DML, source offsets, statement line/column ranges, event categories, and governor-like SOQL/DML deltas. pprof-compatible CPU output and per-statement wall-clock timing remain incomplete."},
	{ID: "server.local-api", Area: "Local API server", Name: "Salesforce-shaped local API with CRUD/query/executeAnonymous", Status: StatusPartial, Required: true, Notes: "CRUD/query/queryAll, describe/recent, limits, OAuth userinfo/id stubs, Tooling executeAnonymous, composite sObject insert, normal REST JSON payloads, Salesforce-shaped error arrays, SQLite persistence, and fixture/reset endpoints are wired. Full auth, Tooling object coverage, Composite Graph, Bulk API, and broader REST resources remain incomplete."},
	{ID: "compat.dashboard", Area: "Release", Name: "Generated compatibility dashboard and CI gate", Status: StatusPartial, Required: true, Notes: "The MVP gate, JSON matrix, generated Markdown dashboard, CI drift check, and parse/check/exec/test/DB lifecycle fixture runner exist. Compatibility fixtures still need expansion before this can be supported."},
	{ID: "release.packaging", Area: "Release", Name: "Installable release binaries, checksums, docs", Status: StatusPartial, Required: true, Notes: "Release archives, checksums, GitHub Release upload workflow, install docs, release policy, and smoke coverage exist. Published package-manager distribution, stronger artifact signing, and release promotion automation remain incomplete."},
}
