package probe

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Run orchestrates the full probe workflow: golden capture, local replay,
// diff, and report generation.
func Run(cfg Config) (*GapReport, error) {
	if len(cfg.ProbeIDs) == 0 {
		cfg.ProbeIDs = probeIDsForTier(cfg.Tier)
	}
	if cfg.Tier == "" {
		cfg.Tier = "full"
	}
	startedAt := time.Now().UTC()

	report := &GapReport{
		ProbesRun: len(cfg.ProbeIDs),
		Entries:   make([]GapEntry, 0),
		RunMeta: RunMeta{
			StartedAtUTC:    startedAt.Format(time.RFC3339),
			ProbeVersion:    ProbeVersion,
			ManifestVersion: ManifestVersion,
			SeedVersion:     SeedVersion,
			Tier:            cfg.Tier,
			Features:        append([]string(nil), cfg.Features...),
			GoldenSource:    "org",
		},
	}

	fmt.Println("=== Phase 1: Golden Capture (real org) ===")
	goldenStart := time.Now()
	var golden map[string]ProbeResult
	var goldenTimings []ProbeTiming
	var debugLogs []ProbeDebugLog
	var err error
	if cfg.UseGoldenCache {
		cachePath := goldenCachePath(cfg)
		cache, cacheErr := ReadGoldenCache(cachePath)
		if cacheErr != nil {
			return nil, fmt.Errorf("read golden cache %s: %w", cachePath, cacheErr)
		}
		if err := validateGoldenCache(cache, cfg.ProbeIDs); err != nil {
			return nil, fmt.Errorf("golden cache %s is not valid for this run: %w", cachePath, err)
		}
		golden = probeResultsByID(cache.Results)
		report.OrgShape = cache.OrgShape
		report.RunMeta.GoldenSource = "cache"
		fmt.Printf("Loaded %d golden responses from %s\n", len(golden), cachePath)
	} else {
		executor := strings.TrimSpace(cfg.GoldenExecutor)
		if executor == "" {
			executor = "rest"
		}
		var orgShape map[string]interface{}
		switch executor {
		case "rest":
			goldenExec := &RestExecutor{OrgAlias: cfg.OrgAlias, CaptureDebugLog: cfg.CaptureDebugLog}
			golden, goldenTimings, err = goldenExec.CaptureGolden(cfg.ProbeDir, cfg.ProbeIDs)
			orgShape = goldenExec.OrgShape
			debugLogs = append(debugLogs, goldenExec.DebugLogs...)
		case "sf":
			goldenExec := &SFDXExecutor{OrgAlias: cfg.OrgAlias, CaptureDebugLog: cfg.CaptureDebugLog}
			golden, goldenTimings, err = goldenExec.CaptureGolden(cfg.ProbeDir, cfg.ProbeIDs)
			orgShape = goldenExec.OrgShape
			debugLogs = append(debugLogs, goldenExec.DebugLogs...)
		default:
			return nil, fmt.Errorf("unknown golden executor %q (expected rest or sf)", executor)
		}
		if err != nil {
			return nil, fmt.Errorf("golden capture failed: %w", err)
		}
		report.OrgShape = orgShape
		if err := validateOrgShape(report.OrgShape, cfg.ProbeIDs, cfg.Features); err != nil {
			return nil, fmt.Errorf("org shape preflight failed: %w", err)
		}
	}
	report.Timings = append(report.Timings, Timing{Phase: "golden", DurationMS: time.Since(goldenStart).Milliseconds()})
	report.ProbeTimings = append(report.ProbeTimings, goldenTimings...)
	fmt.Printf("Captured %d golden responses\n", len(golden))

	fmt.Println("\n=== Phase 2: Local Replay (oaer VM) ===")
	localStart := time.Now()
	localExec := &LocalExecutor{ProbeDir: cfg.ProbeDir, Features: cfg.Features}
	var local map[string]ProbeResult
	var localTimings []ProbeTiming
	var localTraceSummaries []DebugLogSummary
	if cfg.CaptureDebugLog {
		local, localTimings, localTraceSummaries, err = localExec.CaptureLocalWithTrace(cfg.ProbeIDs)
	} else {
		local, localTimings, err = localExec.CaptureLocal(cfg.ProbeIDs)
	}
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
		manifest := selectedProbeSpecs(cfg.ProbeIDs)
		if err := WriteManifest(manifest, manifestPath); err != nil {
			return report, fmt.Errorf("write manifest: %w", err)
		}
		fmt.Printf("Wrote manifest to %s\n", manifestPath)
		if !cfg.UseGoldenCache {
			cachePath := goldenCachePath(cfg)
			if err := WriteGoldenCache(GoldenCache{
				RunMeta:  report.RunMeta,
				OrgShape: report.OrgShape,
				Manifest: manifest,
				Results:  orderedProbeResults(cfg.ProbeIDs, golden),
			}, cachePath); err != nil {
				return report, fmt.Errorf("write golden cache: %w", err)
			}
			fmt.Printf("Wrote golden cache to %s\n", cachePath)
		}
		if cfg.CaptureDebugLog && !cfg.UseGoldenCache {
			debugLogPath := filepath.Join(cfg.OutputDir, "debug-logs.json")
			if err := WriteDebugLogs(debugLogs, debugLogPath); err != nil {
				return report, fmt.Errorf("write debug logs: %w", err)
			}
			fmt.Printf("Wrote debug logs to %s\n", debugLogPath)
			summaryPath := filepath.Join(cfg.OutputDir, "debug-log-summaries.json")
			summaries := SummarizeDebugLogs(debugLogs)
			if err := WriteDebugLogSummaries(summaries, summaryPath); err != nil {
				return report, fmt.Errorf("write debug log summaries: %w", err)
			}
			fmt.Printf("Wrote debug log summaries to %s\n", summaryPath)
			localSummaryPath := filepath.Join(cfg.OutputDir, "local-trace-summaries.json")
			if err := WriteDebugLogSummaries(localTraceSummaries, localSummaryPath); err != nil {
				return report, fmt.Errorf("write local trace summaries: %w", err)
			}
			fmt.Printf("Wrote local trace summaries to %s\n", localSummaryPath)
			report.TraceDiffs = CompareTraceSummaries(cfg.ProbeIDs, summaries, localTraceSummaries)
			fmt.Printf("Trace diff summary: %s\n", formatTraceDiffSummary(report.TraceDiffs))
		}
		trendPath := filepath.Join(cfg.OutputDir, "probe-history.jsonl")
		if err := AppendTrend(trendPath, trendEntry(report)); err != nil {
			return report, fmt.Errorf("write trend history: %w", err)
		}
		fmt.Printf("Appended trend to %s\n", trendPath)

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

func goldenCachePath(cfg Config) string {
	if cfg.GoldenCache != "" {
		return cfg.GoldenCache
	}
	return filepath.Join(cfg.OutputDir, "golden-cache.json")
}

func validateGoldenCache(cache GoldenCache, probeIDs []string) error {
	if cache.RunMeta.ProbeVersion != ProbeVersion || cache.RunMeta.ManifestVersion != ManifestVersion || cache.RunMeta.SeedVersion != SeedVersion {
		return fmt.Errorf("cache versions probe=%q manifest=%q seed=%q, want probe=%q manifest=%q seed=%q", cache.RunMeta.ProbeVersion, cache.RunMeta.ManifestVersion, cache.RunMeta.SeedVersion, ProbeVersion, ManifestVersion, SeedVersion)
	}
	results := probeResultsByID(cache.Results)
	for _, id := range probeIDs {
		if _, ok := results[id]; !ok {
			return fmt.Errorf("missing probe %s", id)
		}
	}
	return nil
}

func validateOrgShape(shape map[string]interface{}, probeIDs []string, features []string) error {
	checkBool := func(key string) error {
		value, ok := shape[key].(bool)
		if !ok || !value {
			return fmt.Errorf("%s = %v, want true", key, shape[key])
		}
		return nil
	}
	for _, key := range []string{"hasProbeTestObject", "hasProbeTestEvent", "hasProbeTestMdt", "hasProbeTestSetting"} {
		if err := checkBool(key); err != nil {
			return err
		}
	}
	if hasFeature(features, "MultiCurrency") {
		if err := checkBool("isMultiCurrency"); err != nil {
			return err
		}
	}
	if rows, ok := numericShapeValue(shape["probeTestObjectRows"]); !ok || rows != 3 {
		return fmt.Errorf("probeTestObjectRows = %v, want 3", shape["probeTestObjectRows"])
	}
	expectedDeployedProbeCount := deployedProbeCount(probeIDs)
	if count, ok := numericShapeValue(shape["probeCount"]); !ok || count < float64(expectedDeployedProbeCount) {
		return fmt.Errorf("probeCount = %v, want at least %d", shape["probeCount"], expectedDeployedProbeCount)
	}
	return nil
}

func deployedProbeCount(probeIDs []string) int {
	count := 0
	for _, id := range probeIDs {
		if isStubContractProbeID(id) {
			continue
		}
		count++
	}
	return count
}

func numericShapeValue(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int:
		return float64(v), true
	default:
		return 0, false
	}
}

func hasFeature(features []string, feature string) bool {
	for _, value := range features {
		if strings.EqualFold(value, feature) {
			return true
		}
	}
	return false
}

func probeResultsByID(results []ProbeResult) map[string]ProbeResult {
	out := make(map[string]ProbeResult, len(results))
	for _, result := range results {
		out[result.ProbeID] = result
	}
	return out
}

func orderedProbeResults(ids []string, results map[string]ProbeResult) []ProbeResult {
	out := make([]ProbeResult, 0, len(ids))
	for _, id := range ids {
		if result, ok := results[id]; ok {
			out = append(out, result)
		}
	}
	return out
}

func trendEntry(report *GapReport) TrendEntry {
	entry := TrendEntry{
		StartedAtUTC:    report.RunMeta.StartedAtUTC,
		ProbeVersion:    report.RunMeta.ProbeVersion,
		ManifestVersion: report.RunMeta.ManifestVersion,
		SeedVersion:     report.RunMeta.SeedVersion,
		Tier:            report.RunMeta.Tier,
		ProbesRun:       report.ProbesRun,
		GapsFound:       report.GapsFound,
		Unsupported:     report.Unsupported,
		Behavioral:      report.Behavioral,
		Panics:          report.Panics,
		GoldenSource:    report.RunMeta.GoldenSource,
	}
	for _, timing := range report.Timings {
		switch timing.Phase {
		case "golden":
			entry.GoldenDurationMS = timing.DurationMS
		case "local":
			entry.LocalDurationMS = timing.DurationMS
		}
	}
	if orgID, ok := report.OrgShape["organizationId"].(string); ok {
		entry.OrganizationID = orgID
	}
	return entry
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
