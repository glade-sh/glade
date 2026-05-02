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
	{ID: "apex.parser.project-scale", Area: "Apex front end", Name: "Parse and index large SFDX projects", Status: StatusPartial, Required: true, Notes: "Parser and symbol baselines exist; method-body model and large-project compatibility fixtures are incomplete."},
	{ID: "apex.sema.body", Area: "Apex front end", Name: "Method-body semantic analysis", Status: StatusUnsupported, Required: true, Notes: "Current sema checks declarations and member type references only."},
	{ID: "vm.classes", Area: "Runtime", Name: "Classes, methods, constructors, statics, properties", Status: StatusPartial, Required: true, Notes: "The VM now registers class metadata from project tests, constructs objects with instance fields/properties, runs constructor bodies, stores static fields, dispatches overrides through inheritance, and supports super calls. Full Apex visibility enforcement, namespace resolution, initializer blocks, and generic overload selection remain incomplete."},
	{ID: "vm.control-flow", Area: "Runtime", Name: "Control flow and exceptions", Status: StatusPartial, Required: true, Notes: "Anonymous and test method execution now supports for/enhanced-for/do-while, break/continue, switch-on, throw, try/catch/finally, exception messages/getMessage, and basic exception stack reporting. Multi-catch, typed rethrow fidelity, and complete Apex exception hierarchy semantics remain incomplete."},
	{ID: "stdlib.core", Area: "Runtime", Name: "Core System/String/Date/Datetime/JSON/Math APIs", Status: StatusPartial, Required: true, Notes: "Assertions, debug, collections, selected String methods, Limits counters, Date/Datetime/Time basics, Math integer helpers, JSON serialize/deserializeUntyped, EncodingUtil, Crypto SHA-256, Schema global describe basics, UserInfo, FeatureManagement, Messaging, ApexPages, and HttpResponse-shaped callout mocks now exist for the supported VM subset."},
	{ID: "tests.runner", Area: "Tests", Name: "Run real Apex test classes", Status: StatusPartial, Required: true, Notes: "Discovery, method dispatch, @TestSetup execution, static reset, startTest/stopTest, runAs, Queueable stopTest draining, and assertion stack frames now work for the supported VM subset."},
	{ID: "tests.salesforce-semantics", Area: "Tests", Name: "@TestSetup, startTest/stopTest, runAs, isolation", Status: StatusPartial, Required: true, Notes: "@TestSetup methods execute before each test with statics reset before the test body; each test gets a fresh cloned org and VM for isolation; startTest/stopTest and runAs are modeled. Exact governor window restoration, profile/permission semantics, and platform auth details remain incomplete."},
	{ID: "async.core", Area: "Tests", Name: "Queueable/Future/Batch/Scheduled basics", Status: StatusPartial, Required: true, Notes: "System.enqueueJob queues object jobs in test context and Test.stopTest drains Queueable execute methods. Future, Batchable, Schedulable, chained job limits, and durable async job records remain incomplete."},
	{ID: "limits.core", Area: "Limits", Name: "Governor counters and strict/permissive enforcement", Status: StatusPartial, Required: true, Notes: "The VM now tracks SOQL queries/rows, DML statements/rows, approximate heap, statement-count CPU, callouts, and async jobs. Limits.* exposes current and max counters, permissive mode records violations, strict mode raises System.LimitException, and oaer exec/test accept --limit-mode. Exact Salesforce accounting and configurable per-test caps remain incomplete."},
	{ID: "sobject.apex", Area: "Data runtime", Name: "Apex-integrated SObject construction and field access", Status: StatusPartial, Required: true, Notes: "Apex now supports schema-backed new Account(Name='Acme'), typed field access, dotted assignment, get/put, Id propagation after DML, parent relationship projection access, and VM/storage record conversion. Typed describe APIs and broader SObject system fields remain incomplete."},
	{ID: "soql.apex", Area: "Data runtime", Name: "Static and dynamic SOQL from Apex", Status: StatusPartial, Required: true, Notes: "Static SOQL literals and Database.query now execute against the in-memory org with simple bind variables, projection, parent relationship fields, COUNT(), single-SObject assignment, equality/inequality filters, order, limit, and offset. Subqueries, broader aggregates, complex predicates, and SQLite planning remain incomplete."},
	{ID: "dml.apex", Area: "Data runtime", Name: "Apex DML statements and Database methods", Status: StatusPartial, Required: true, Notes: "Apex insert/update/delete/upsert/undelete syntax and Database.insert/update/delete allOrNone paths now call the DML engine, return SaveResult-like objects, set Ids, and roll back allOrNone failures. Merge, external-id upsert, undelete fidelity, and full error arrays remain incomplete."},
	{ID: "triggers.runtime", Area: "Data runtime", Name: "Trigger invocation and context", Status: StatusPartial, Required: true, Notes: "Project triggers are compiled and invoked from VM DML for before/after operations with Trigger.new/old/maps/flags/operationType/size basics and rollback on thrown errors. Full bulk ordering semantics, recursive limits, addError, undelete storage state, and relationship side effects remain incomplete."},
	{ID: "fixtures.persistence", Area: "Data runtime", Name: "Seed/export/reset local fixtures with persistence", Status: StatusPartial, Required: true, Notes: "SQLite-backed org storage now persists object definitions, records, ID sequences, fixture seed/export/reset/inspect, alias and relationship reference resolution, deterministic users/profiles/permissions, server persistence, and fixture/reset endpoints. Large-fixture performance tuning and richer permission semantics remain incomplete."},
	{ID: "dap.command", Area: "Developer experience", Name: "VS Code debug flow through oaer test/exec --debug", Status: StatusPartial, Required: true, Notes: "DAP content-length transport, setBreakpoints, continue/pause/next, stackTrace, scopes, variables, evaluate, and oaer exec/test --debug snapshot sessions are wired. True live VM suspension, step-in/out semantics, and breakpoint-driven execution control remain incomplete."},
	{ID: "lsp.command", Area: "Developer experience", Name: "oaer lsp core editor features", Status: StatusPartial, Required: true, Notes: "oaer lsp now runs a stdio LSP transport with initialize, diagnostics, document/workspace symbols, hover, and completion. Definition, references, semantic tokens, and incremental document sync remain incomplete."},
	{ID: "watch.command", Area: "Developer experience", Name: "oaer test --watch affected-test loop", Status: StatusPartial, Required: true, Notes: "oaer test --watch now runs a polling watch loop with debounce, JSON event stream, affected-test selection, reruns, and context cancellation. Native OS watcher backends and in-flight VM cancellation remain incomplete."},
	{ID: "profile.native", Area: "Developer experience", Name: "Native trace/profile reports", Status: StatusPartial, Required: true, Notes: "Trace/profile reports aggregate statements, methods, SOQL, DML, source offsets, event categories, and governor-like SOQL/DML deltas. pprof-compatible CPU output and per-statement wall-clock timing remain incomplete."},
	{ID: "server.local-api", Area: "Local API server", Name: "Salesforce-shaped local API with CRUD/query/executeAnonymous", Status: StatusPartial, Required: true, Notes: "CRUD/query/queryAll, describe/recent, limits, OAuth userinfo/id stubs, Tooling executeAnonymous, composite sObject insert, normal REST JSON payloads, Salesforce-shaped error arrays, SQLite persistence, and fixture/reset endpoints are wired. Full auth, Tooling object coverage, Composite Graph, Bulk API, and broader REST resources remain incomplete."},
	{ID: "compat.dashboard", Area: "Release", Name: "Generated compatibility dashboard and CI gate", Status: StatusPartial, Required: true, Notes: "The MVP gate, JSON matrix, generated Markdown dashboard, and CI drift check exist. Compatibility fixtures still need expansion before this can be supported."},
	{ID: "release.packaging", Area: "Release", Name: "Installable release binaries, checksums, docs", Status: StatusUnsupported, Required: true, Notes: "Release packaging is not implemented."},
}
