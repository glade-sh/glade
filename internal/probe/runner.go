package probe

import (
	"fmt"
	"path/filepath"
	"sort"
	"time"
)

// Run orchestrates the full probe workflow: golden capture, local replay,
// diff, and report generation.
func Run(cfg Config) (*GapReport, error) {
	if len(cfg.ProbeIDs) == 0 {
		cfg.ProbeIDs = defaultProbeIDs()
	}

	report := &GapReport{
		ProbesRun: len(cfg.ProbeIDs),
		Entries:   make([]GapEntry, 0),
	}

	fmt.Println("=== Phase 1: Golden Capture (real org) ===")
	goldenExec := &SFDXExecutor{OrgAlias: cfg.OrgAlias}
	golden, err := goldenExec.CaptureGolden(cfg.ProbeDir, cfg.ProbeIDs)
	if err != nil {
		return nil, fmt.Errorf("golden capture failed: %w", err)
	}
	fmt.Printf("Captured %d golden responses\n", len(golden))

	fmt.Println("\n=== Phase 2: Local Replay (oaer VM) ===")
	localExec := &LocalExecutor{ProbeDir: cfg.ProbeDir, Features: cfg.Features}
	local, err := localExec.CaptureLocal(cfg.ProbeIDs)
	if err != nil {
		return nil, fmt.Errorf("local replay failed: %w", err)
	}
	fmt.Printf("Captured %d local responses\n", len(local))

	fmt.Println("\n=== Phase 3: Diff & Classify ===")
	for _, id := range cfg.ProbeIDs {
		g, ok1 := golden[id]
		l, ok2 := local[id]
		if !ok1 {
			fmt.Printf("  WARN: missing golden for %s\n", id)
			continue
		}
		if !ok2 {
			fmt.Printf("  WARN: missing local for %s\n", id)
			continue
		}

		gap := Compare(g, l)
		if gap != nil {
			report.Entries = append(report.Entries, *gap)
			switch gap.GapType {
			case GapTypePanic:
				report.Panics++
			case GapTypeUnsupported:
				report.Unsupported++
			case GapTypeBehavioral:
				report.Behavioral++
			}
		}
	}
	report.GapsFound = len(report.Entries)

	// Sort by severity: critical > high > medium > low
	severityOrder := map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3}
	sort.SliceStable(report.Entries, func(i, j int) bool {
		return severityOrder[report.Entries[i].Severity] < severityOrder[report.Entries[j].Severity]
	})

	fmt.Printf("\nFound %d gaps (%d unsupported, %d behavioral, %d panics)\n",
		report.GapsFound, report.Unsupported, report.Behavioral, report.Panics)

	if cfg.OutputDir != "" {
		reportPath := filepath.Join(cfg.OutputDir, "gap-report.json")
		if err := WriteReport(report, reportPath); err != nil {
			return report, fmt.Errorf("write report: %w", err)
		}
		fmt.Printf("Wrote report to %s\n", reportPath)

		fixtureDir := filepath.Join(cfg.OutputDir, "fixtures")
		for _, entry := range report.Entries {
			if entry.GapType == GapTypeUnsupported || entry.GapType == GapTypePanic {
				if err := WriteFixtureStub(entry, cfg.ProbeDir, fixtureDir); err != nil {
					fmt.Printf("  WARN: could not write fixture for %s: %v\n", entry.ProbeID, err)
				}
			}
		}
	}

	return report, nil
}

func defaultProbeIDs() []string {
	return []string{
		// Stdlib & System
		"stdlib.string.format-null",
		"stdlib.string.join-empty",
		"stdlib.string.containsIgnoreCase-null",
		"stdlib.string.valueOf-null",
		"stdlib.string.isBlank-whitespace",
		"stdlib.datetime.valueOf-null",
		"stdlib.datetime.leapYear",
		"stdlib.datetime.format-timezone",
		"stdlib.datetime.valueOf-invalid",
		"stdlib.datetime.yearZero",
		"stdlib.math.divide-scale",
		"stdlib.math.mod-negative",
		"stdlib.math.round-halfUp",
		"stdlib.math.decimalValueOf-null",
		"stdlib.math.log10",
		// Data Runtime
		"soql.select-all",
		"soql.aggregate-count",
		"soql.where-like",
		"soql.order-desc",
		"soql.dynamic",
		// DML & Triggers
		"dml.insert-trigger",
		"dml.update-return",
		"dml.delete-return",
		"dml.undelete",
		"dml.insert-fail-duplicate",
		// Limits & System
		"limits.soql-before-after",
		"limits.dml-rows",
		"limits.heap-size",
		"limits.limit-queries",
		"limits.cpu-time",
		// Collections & Language
		"collections.list-contains-null",
		"collections.map-null-key",
		"collections.set-contains-null",
		"collections.list-indexof-null",
		"collections.map-remove-null",
		// Async Apex
		"async.future-stub",
		"async.queueable-stub",
		"async.batchable-stub",
		"async.schedulable-stub",
		// Platform Events
		"platform-event.publish",
		"platform-event.describe",
		// Metadata
		"metadata.custom-metadata-query",
		"metadata.custom-setting-query",
		"metadata.custom-metadata-describe",
		// Email & Messaging
		"email.single-message",
		"email.limits",
		// Schema Describe
		"schema.global-describe-size",
		"schema.object-describe-fields",
		"schema.picklist-describe",
		"schema.record-type-describe",
		// Security & Sharing
		"security.user-info",
		"security.profile-name",
		"security.crud-check",
		"security.fls-check",
		// Integration
		"integration.http-request",
		"integration.json-serialize-complex",
		"integration.json-deserialize-untyped",
		"integration.encoding-util",
		"integration.json-serialize-sobject",
		"integration.url-encoding",
		// Bulk DML
		"bulkdml.partial-success",
		"bulkdml.errors-shape",
		"bulkdml.id-assignment",
		// SObject & Id
		"sobject.clone",
		"sobject.get-put",
		"sobject.getSObjectType",
		"id.validation",
		// More Collections
		"collections.list-sort",
		"collections.map-keyset",
		"collections.set-equality",
		// More Datetime
		"datetime.now",
		"datetime.today",
		"datetime.valueOfGmt",
		// More String
		"string.escapeSingleQuotes",
		"string.split",
		"string.trim",
		// More SOQL
		"soql.count-distinct",
		"soql.group-by",
		"soql.subquery",
		// System Assert
		"system.assert-pass",
		"system.assertEquals",
		// More Math
		"math.abs-negative",
		"math.max",
		"math.pow",
		"math.min",
	}
}

// Sleep between sfdx calls to avoid hitting API rate limits.
func delay() {
	time.Sleep(200 * time.Millisecond)
}
