package oaercli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/open-aer/oaer/internal/oracle"
)

func runCompatOracleTests(ctx context.Context, args []string, w io.Writer) error {
	project := "."
	targetOrg := ""
	filter := ""
	salesforceRunPath := ""
	localRunPath := ""
	runsDir := ".oaer/runs"
	runID := ""
	anonymous := ""
	checkPath := ""
	waitMinutes := 10
	jsonOut := false
	goldenOnly := false

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			project = args[i+1]
			i++
		case "--target-org":
			if i+1 >= len(args) {
				return errors.New("--target-org requires a value")
			}
			targetOrg = args[i+1]
			i++
		case "--filter":
			if i+1 >= len(args) {
				return errors.New("--filter requires a value")
			}
			filter = args[i+1]
			i++
		case "--salesforce-run":
			if i+1 >= len(args) {
				return errors.New("--salesforce-run requires a path")
			}
			salesforceRunPath = args[i+1]
			i++
		case "--local-run":
			if i+1 >= len(args) {
				return errors.New("--local-run requires a path")
			}
			localRunPath = args[i+1]
			i++
		case "--runs-dir":
			if i+1 >= len(args) {
				return errors.New("--runs-dir requires a path")
			}
			runsDir = args[i+1]
			i++
		case "--run-id":
			if i+1 >= len(args) {
				return errors.New("--run-id requires a value")
			}
			runID = args[i+1]
			i++
		case "--anonymous":
			if i+1 >= len(args) {
				return errors.New("--anonymous requires Apex source")
			}
			anonymous = args[i+1]
			i++
		case "--check":
			if i+1 >= len(args) {
				return errors.New("--check requires a path")
			}
			checkPath = args[i+1]
			i++
		case "--wait":
			if i+1 >= len(args) {
				return errors.New("--wait requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed <= 0 {
				return errors.New("--wait must be a positive integer")
			}
			waitMinutes = parsed
			i++
		case "--golden-only":
			goldenOnly = true
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	if strings.TrimSpace(checkPath) != "" {
		if project != "." || targetOrg != "" || filter != "" || salesforceRunPath != "" || localRunPath != "" || runsDir != ".oaer/runs" || runID != "" || anonymous != "" || goldenOnly || waitMinutes != 10 {
			return errors.New("--check cannot be combined with --project, --target-org, --filter, --salesforce-run, --local-run, --golden-only, --anonymous, --runs-dir, --run-id, or --wait")
		}
		report, err := oracle.CheckCorpus(checkPath)
		if jsonOut {
			if writeErr := oracle.WriteCorpusJSON(w, report); writeErr != nil {
				return writeErr
			}
		} else {
			oracle.WriteCorpusText(w, report)
		}
		return err
	}

	if strings.TrimSpace(runID) == "" {
		runID = oracle.DefaultRunID(time.Now())
	}
	if err := validateOracleRunID(runID); err != nil {
		return err
	}
	if strings.TrimSpace(anonymous) != "" && !goldenOnly && strings.TrimSpace(localRunPath) == "" {
		return errors.New("--anonymous currently requires --golden-only or --local-run until local anonymous oracle execution is wired")
	}
	artifactDir := filepath.Join(runsDir, runID, "oracle")

	salesforceRuns, err := loadOrRunSalesforceOracle(ctx, project, targetOrg, filter, salesforceRunPath, anonymous, waitMinutes)
	if err != nil {
		report := oracle.InfrastructureReport(project, runID, artifactDir, err.Error())
		if persistErr := oracle.PersistArtifacts(artifactDir, nil, nil, report.Comparisons, report); persistErr != nil {
			return persistErr
		}
		if jsonOut {
			_ = oracle.WriteReportJSON(w, report)
		} else {
			writeOracleReportText(w, report)
		}
		return err
	}

	if goldenOnly {
		report := oracle.NewReport(project, runID, artifactDir, nil, len(salesforceRuns), 0, true)
		if err := oracle.PersistArtifacts(artifactDir, salesforceRuns, nil, nil, report); err != nil {
			return err
		}
		if jsonOut {
			return oracle.WriteReportJSON(w, report)
		}
		writeOracleReportText(w, report)
		return nil
	}

	localRuns, err := loadOrRunLocalOracle(ctx, project, filter, localRunPath)
	if err != nil {
		report := oracle.InfrastructureReport(project, runID, artifactDir, err.Error())
		if persistErr := oracle.PersistArtifacts(artifactDir, salesforceRuns, nil, report.Comparisons, report); persistErr != nil {
			return persistErr
		}
		if jsonOut {
			_ = oracle.WriteReportJSON(w, report)
		} else {
			writeOracleReportText(w, report)
		}
		return err
	}

	diffs := oracle.DiffRunSets(salesforceRuns, localRuns)
	report := oracle.NewReport(project, runID, artifactDir, diffs, len(salesforceRuns), len(localRuns), false)
	if err := oracle.PersistArtifacts(artifactDir, salesforceRuns, localRuns, diffs, report); err != nil {
		return err
	}
	if jsonOut {
		if err := oracle.WriteReportJSON(w, report); err != nil {
			return err
		}
	} else {
		writeOracleReportText(w, report)
	}
	if !report.Ready {
		return fmt.Errorf("oracle parity mismatch: %d non-pass comparisons", report.Summary.Total-report.Summary.Pass)
	}
	return nil
}

func validateOracleRunID(runID string) error {
	trimmed := strings.TrimSpace(runID)
	if trimmed == "" {
		return errors.New("--run-id cannot be empty")
	}
	if filepath.IsAbs(trimmed) || strings.ContainsAny(trimmed, `/\`) || filepath.Clean(trimmed) != trimmed || trimmed == "." || trimmed == ".." {
		return fmt.Errorf("invalid --run-id %q: use a single path-safe name", runID)
	}
	return nil
}

func loadOrRunSalesforceOracle(ctx context.Context, project, targetOrg, filter, path, anonymous string, waitMinutes int) ([]oracle.OracleRun, error) {
	if strings.TrimSpace(path) != "" {
		return oracle.ReadRuns(path)
	}
	if strings.TrimSpace(anonymous) != "" {
		if strings.TrimSpace(targetOrg) == "" {
			return nil, errors.New("--target-org is required with --anonymous")
		}
		run, err := (oracle.SalesforceRunner{}).RunAnonymous(project, targetOrg, "anonymous", "oracle", anonymous)
		if err != nil {
			return nil, err
		}
		return []oracle.OracleRun{run}, nil
	}
	return (oracle.SalesforceRunner{}).RunTests(ctx, oracle.SalesforceRunOptions{
		Project:    project,
		OrgAlias:   targetOrg,
		Filter:     filter,
		WaitMinute: waitMinutes,
	})
}

func loadOrRunLocalOracle(ctx context.Context, project, filter, path string) ([]oracle.OracleRun, error) {
	if strings.TrimSpace(path) != "" {
		return oracle.ReadRuns(path)
	}
	args := []string{"--project", project}
	if strings.TrimSpace(filter) != "" {
		args = append(args, "--filter", filter)
	}
	run, err := runTest(ctx, args, io.Discard)
	if err != nil {
		return nil, err
	}
	return oracle.LocalRunsFromTestReport(project, run), nil
}

func writeOracleReportText(w io.Writer, report oracle.OracleReport) {
	status := "not ready"
	if report.Ready {
		status = "ready"
	}
	fmt.Fprintf(w, "Oracle parity: %s\n", status)
	fmt.Fprintf(w, "Project: %s\n", report.Project)
	fmt.Fprintf(w, "Artifacts: %s\n", report.ArtifactDir)
	fmt.Fprintf(w, "Summary: pass=%d total=%d trace_mismatch=%d state_mismatch=%d exception_mismatch=%d unsupported=%d compile_gap=%d infrastructure_error=%d\n",
		report.Summary.Pass,
		report.Summary.Total,
		report.Summary.TraceMismatch,
		report.Summary.StateMismatch,
		report.Summary.ExceptionMismatch,
		report.Summary.Unsupported,
		report.Summary.CompileGap,
		report.Summary.InfrastructureError,
	)
	for _, comparison := range report.Comparisons {
		if comparison.Outcome == oracle.OracleOutcomePass {
			continue
		}
		fmt.Fprintf(w, "- %s.%s: %s", comparison.TestClass, comparison.TestMethod, comparison.Outcome)
		if len(comparison.Details) > 0 {
			fmt.Fprintf(w, " (%s)", comparison.Details[0])
		}
		fmt.Fprintln(w)
	}
}
