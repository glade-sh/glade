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
	goldenStart := time.Now()
	goldenExec := &SFDXExecutor{OrgAlias: cfg.OrgAlias}
	golden, goldenTimings, err := goldenExec.CaptureGolden(cfg.ProbeDir, cfg.ProbeIDs)
	if err != nil {
		return nil, fmt.Errorf("golden capture failed: %w", err)
	}
	report.OrgShape = goldenExec.OrgShape
	report.Timings = append(report.Timings, Timing{Phase: "golden", DurationMS: time.Since(goldenStart).Milliseconds()})
	report.ProbeTimings = append(report.ProbeTimings, goldenTimings...)
	fmt.Printf("Captured %d golden responses\n", len(golden))

	fmt.Println("\n=== Phase 2: Local Replay (oaer VM) ===")
	localStart := time.Now()
	localExec := &LocalExecutor{ProbeDir: cfg.ProbeDir, Features: cfg.Features}
	local, localTimings, err := localExec.CaptureLocal(cfg.ProbeIDs)
	if err != nil {
		return nil, fmt.Errorf("local replay failed: %w", err)
	}
	report.Timings = append(report.Timings, Timing{Phase: "local", DurationMS: time.Since(localStart).Milliseconds()})
	report.ProbeTimings = append(report.ProbeTimings, localTimings...)
	fmt.Printf("Captured %d local responses\n", len(local))

	fmt.Println("\n=== Phase 3: Diff & Classify ===")
	diffStart := time.Now()
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
	report.Timings = append(report.Timings, Timing{Phase: "diff", DurationMS: time.Since(diffStart).Milliseconds()})

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
		manifestPath := filepath.Join(cfg.OutputDir, "probe-manifest.json")
		if err := WriteManifest(selectedProbeSpecs(cfg.ProbeIDs), manifestPath); err != nil {
			return report, fmt.Errorf("write manifest: %w", err)
		}
		fmt.Printf("Wrote manifest to %s\n", manifestPath)

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

func selectedProbeSpecs(ids []string) []ProbeSpec {
	specs := make([]ProbeSpec, 0, len(ids))
	for _, id := range ids {
		specs = append(specs, probeSpecByID(id))
	}
	return specs
}

// Sleep between sfdx calls to avoid hitting API rate limits.
func delay() {
	time.Sleep(200 * time.Millisecond)
}
