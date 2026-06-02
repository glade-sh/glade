package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexdocs"
	"github.com/glade-sh/glade/internal/capability"
	"github.com/glade-sh/glade/internal/oracle"
	"github.com/glade-sh/glade/internal/surfaceledger"
	"github.com/glade-sh/glade/internal/typesys"
)

func runCompatOracle(ctx context.Context, args []string, w io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		printCompatOracleHelp(w)
		return nil
	}
	switch args[0] {
	case "doctor":
		return runCompatOracleDoctor(args[1:], w)
	case "inventory":
		return runCompatOracleInventory(args[1:], w)
	case "domains":
		return runCompatOracleDomains(args[1:], w)
	case "plan":
		return runCompatOraclePlan(args[1:], w)
	case "generate":
		return runCompatOracleGenerate(args[1:], w)
	case "scripts":
		return runCompatOracleScripts(args[1:], w)
	case "run-salesforce":
		return runCompatOracleRunSalesforce(ctx, args[1:], w)
	case "run-anon":
		return runCompatOracleRunAnon(ctx, args[1:], w)
	case "run-glade":
		return runCompatOracleRunGLADE(ctx, args[1:], w)
	case "diff":
		return runCompatOracleDiff(args[1:], w)
	case "promote":
		return runCompatOraclePromote(args[1:], w)
	case "report":
		return runCompatOracleReport(args[1:], w)
	case "next":
		return runCompatOracleNext(args[1:], w)
	case "resume":
		return runCompatOracleResume(args[1:], w)
	case "check":
		return runCompatOracleCheck(args[1:], w)
	default:
		return fmt.Errorf("unknown oracle subcommand %q", args[0])
	}
}

func printCompatOracleHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
Run deep Salesforce oracle workflows.

Usage:
  glade compat oracle <subcommand> [flags]

Subcommands:
  doctor          Check local prerequisites.
  inventory       Build inventory from stub contracts, or from documented gaps with --catalog/--inventory.
  domains         Emit reusable parameter domains.
  plan            Build probe manifest and work queue.
  generate        Generate Apex probe classes from queue.
  scripts         Generate resumable shard scripts.
  run-salesforce  Run Salesforce oracle test shard.
  run-anon        Run Salesforce oracle shard via synchronous anonymous Apex (no deploy).
  run-glade        Run GLADE oracle test shard.
  diff            Diff Salesforce and GLADE observations.
  promote         Promote run artifacts into docs/fixtures/oracle.
  report          Build coverage report from inventory.
  next            List next pending work items.
  resume          Re-run pending work queue items by shard script.
  check           Validate generated oracle artifacts.
`)+"\n")
}

func runCompatOracleDoctor(args []string, w io.Writer) error {
	jsonOut := hasFlag(args, "--json")
	doctor := map[string]any{
		"target": "apex oracle doctor",
		"ready":  true,
		"checks": []map[string]any{},
	}
	checks := make([]map[string]any, 0, 4)
	if _, err := exec.LookPath("sf"); err != nil {
		checks = append(checks, map[string]any{"name": "sf-cli", "ok": false, "message": err.Error()})
		doctor["ready"] = false
	} else {
		checks = append(checks, map[string]any{"name": "sf-cli", "ok": true})
	}
	if _, err := os.Stat("example-projects/stubs"); err != nil {
		checks = append(checks, map[string]any{"name": "stub-root", "ok": false, "message": err.Error()})
		doctor["ready"] = false
	} else {
		checks = append(checks, map[string]any{"name": "stub-root", "ok": true})
	}
	if _, err := os.Stat("probes/sfdx/sfdx-project.json"); err != nil {
		checks = append(checks, map[string]any{"name": "probe-sfdx", "ok": false, "message": err.Error()})
		doctor["ready"] = false
	} else {
		checks = append(checks, map[string]any{"name": "probe-sfdx", "ok": true})
	}
	doctor["checks"] = checks
	if jsonOut {
		return writeJSONOut(w, doctor)
	}
	fmt.Fprintf(w, "ready: %t\n", doctor["ready"])
	for _, c := range checks {
		fmt.Fprintf(w, "- %s: %t\n", c["name"], c["ok"])
	}
	if !doctor["ready"].(bool) {
		return errors.New("oracle doctor failed")
	}
	return nil
}

func runCompatOracleInventory(args []string, w io.Writer) error {
	stubRoot := "example-projects/stubs"
	catalogPath := ""
	docsInventoryPath := ""
	ledgerPath := ""
	gapClass := ""
	output := ""
	check := ""
	jsonOut := false
	worklistLimit := -1
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--stubs", "--source":
			i++
			if i >= len(args) {
				return errors.New("--stubs requires a value")
			}
			stubRoot = args[i]
		case "--catalog":
			i++
			if i >= len(args) {
				return errors.New("--catalog requires a value")
			}
			catalogPath = args[i]
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New("--inventory requires a value")
			}
			docsInventoryPath = args[i]
		case "--ledger":
			i++
			if i >= len(args) {
				return errors.New("--ledger requires a value")
			}
			ledgerPath = args[i]
		case "--gap-class":
			i++
			if i >= len(args) {
				return errors.New("--gap-class requires a value")
			}
			gapClass = args[i]
		case "--limit":
			i++
			if i >= len(args) {
				return errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n < 0 {
				return errors.New("--limit requires a non-negative integer (0 = unlimited)")
			}
			worklistLimit = n
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("--output requires a value")
			}
			output = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("--check requires a value")
			}
			check = args[i]
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	sourceCount := 0
	for _, set := range []bool{catalogPath != "", docsInventoryPath != "", ledgerPath != ""} {
		if set {
			sourceCount++
		}
	}
	if sourceCount > 1 {
		return errors.New("use only one of --catalog, --inventory, or --ledger")
	}
	if worklistLimit >= 0 && catalogPath == "" && docsInventoryPath == "" && ledgerPath == "" {
		return errors.New("--limit only applies with --catalog, --inventory, or --ledger")
	}

	var inv oracle.Inventory
	switch {
	case ledgerPath != "":
		ledger, err := surfaceledger.ReadLedgerJSON(ledgerPath)
		if err != nil {
			return err
		}
		inv = oracle.BuildInventoryFromLedger(ledger, gapClass, worklistLimit)
	case catalogPath != "" || docsInventoryPath != "":
		var catalog capability.Catalog
		if catalogPath != "" {
			read, err := capability.ReadCatalog(catalogPath)
			if err != nil {
				return err
			}
			catalog = read
		} else {
			docsInv, err := apexdocs.ReadInventory(docsInventoryPath)
			if err != nil {
				return err
			}
			catalog = capability.BuildCatalog(docsInv)
		}
		var rec capability.Reconciliation
		if worklistLimit >= 0 {
			rec = capability.BuildReconciliationLimited(catalog, typesys.StandardPlatformSymbols(), worklistLimit)
		} else {
			rec = capability.BuildReconciliation(catalog, typesys.StandardPlatformSymbols())
		}
		inv = oracle.BuildInventoryFromReconciliation(rec, catalog)
	default:
		built, err := oracle.BuildInventory(stubRoot)
		if err != nil {
			return err
		}
		inv = built
	}
	if check != "" {
		return checkJSONDrift(check, inv, "oracle inventory")
	}
	if output != "" {
		if err := oracle.WriteJSON(output, inv); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s: wrote\n", output)
		return nil
	}
	if jsonOut {
		return writeJSONOut(w, inv)
	}
	fmt.Fprintf(w, "surfaces: %d\n", len(inv.Surfaces))
	return nil
}

func runCompatOracleDomains(args []string, w io.Writer) error {
	output := ""
	check := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("--output requires a value")
			}
			output = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("--check requires a value")
			}
			check = args[i]
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	domains := oracle.BuildDomains()
	if check != "" {
		return checkJSONDrift(check, domains, "oracle domains")
	}
	if output != "" {
		if err := oracle.WriteJSON(output, domains); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s: wrote\n", output)
		return nil
	}
	if jsonOut {
		return writeJSONOut(w, domains)
	}
	fmt.Fprintf(w, "types: %d\n", len(domains.Values))
	return nil
}

func runCompatOraclePlan(args []string, w io.Writer) error {
	invPath := "docs/generated/apex-oracle/INVENTORY.json"
	domainsPath := ""
	area := ""
	limit := 0
	manifestPath := "docs/generated/apex-oracle/PROBE_MANIFEST.json"
	queuePath := "docs/generated/apex-oracle/WORK_QUEUE.json"
	shardCount := 32
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New("--inventory requires a value")
			}
			invPath = args[i]
		case "--area":
			i++
			if i >= len(args) {
				return errors.New("--area requires a value")
			}
			area = args[i]
		case "--domains":
			i++
			if i >= len(args) {
				return errors.New("--domains requires a value")
			}
			domainsPath = args[i]
		case "--limit":
			i++
			if i >= len(args) {
				return errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			limit = n
		case "--manifest":
			i++
			if i >= len(args) {
				return errors.New("--manifest requires a value")
			}
			manifestPath = args[i]
		case "--work-queue":
			i++
			if i >= len(args) {
				return errors.New("--work-queue requires a value")
			}
			queuePath = args[i]
		case "--shard-count":
			i++
			if i >= len(args) {
				return errors.New("--shard-count requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			shardCount = n
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	var inv oracle.Inventory
	if err := oracle.ReadJSON(invPath, &inv); err != nil {
		return err
	}
	manifest := oracle.BuildManifest(inv, area, limit)
	if domainsPath != "" {
		var domains oracle.Domains
		if err := oracle.ReadJSON(domainsPath, &domains); err != nil {
			return err
		}
	}
	queue := oracle.BuildWorkQueue(manifest, shardCount)
	if err := oracle.WriteJSON(manifestPath, manifest); err != nil {
		return err
	}
	if err := oracle.WriteJSON(queuePath, queue); err != nil {
		return err
	}
	if jsonOut {
		return writeJSONOut(w, map[string]any{"manifest": manifest, "workQueue": queue})
	}
	fmt.Fprintf(w, "manifest: %s\n", manifestPath)
	fmt.Fprintf(w, "workQueue: %s\n", queuePath)
	fmt.Fprintf(w, "probes: %d\n", len(manifest.Specs))
	return nil
}

func runCompatOracleGenerate(args []string, w io.Writer) error {
	queuePath := "docs/generated/apex-oracle/WORK_QUEUE.json"
	runsDir := ".glade/oracle/runs"
	runID := ""
	area := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--work-queue":
			i++
			if i >= len(args) {
				return errors.New("--work-queue requires a value")
			}
			queuePath = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				return errors.New("--runs-dir requires a value")
			}
			runsDir = args[i]
		case "--run-id":
			i++
			if i >= len(args) {
				return errors.New("--run-id requires a value")
			}
			runID = args[i]
		case "--area":
			i++
			if i >= len(args) {
				return errors.New("--area requires a value")
			}
			area = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if runID == "" {
		runID = oracle.DefaultRunID(time.Now())
	}
	if err := validateOracleRunID(runID); err != nil {
		return err
	}
	var queue oracle.WorkQueue
	if err := oracle.ReadJSON(queuePath, &queue); err != nil {
		return err
	}
	if area != "" {
		filtered := make([]oracle.WorkItem, 0, len(queue.Items))
		for _, item := range queue.Items {
			if item.Area == area {
				filtered = append(filtered, item)
			}
		}
		queue.Items = filtered
	}
	runDir := filepath.Join(runsDir, runID)
	if err := oracle.WriteJSON(filepath.Join(runDir, "work-queue.json"), queue); err != nil {
		return err
	}
	if err := oracle.GenerateApex(queue, runDir); err != nil {
		return err
	}
	if err := oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{RunID: runID, Step: "generate", Status: "ok", Message: fmt.Sprintf("classes=%d", len(queue.Items))}); err != nil {
		return err
	}
	fmt.Fprintf(w, "generated: %s\n", filepath.Join(runDir, "generated", "sfdx"))
	return nil
}

func runCompatOracleScripts(args []string, w io.Writer) error {
	queuePath := "docs/generated/apex-oracle/WORK_QUEUE.json"
	runsDir := ".glade/oracle/runs"
	runID := ""
	targetOrg := ""
	outDir := ""
	shardCount := 32
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--work-queue":
			i++
			if i >= len(args) {
				return errors.New("--work-queue requires a value")
			}
			queuePath = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				return errors.New("--runs-dir requires a value")
			}
			runsDir = args[i]
		case "--run-id":
			i++
			if i >= len(args) {
				return errors.New("--run-id requires a value")
			}
			runID = args[i]
		case "--target-org":
			i++
			if i >= len(args) {
				return errors.New("--target-org requires a value")
			}
			targetOrg = args[i]
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("--output requires a value")
			}
			outDir = args[i]
		case "--shard-count":
			i++
			if i >= len(args) {
				return errors.New("--shard-count requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			shardCount = n
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if runID == "" {
		runID = oracle.DefaultRunID(time.Now())
	}
	if err := validateOracleRunID(runID); err != nil {
		return err
	}
	if outDir == "" {
		outDir = filepath.Join(runsDir, runID, "generated", "scripts")
	}
	var queue oracle.WorkQueue
	if err := oracle.ReadJSON(queuePath, &queue); err != nil {
		return err
	}
	if err := oracle.GenerateScripts(queue, runID, runsDir, targetOrg, outDir, shardCount); err != nil {
		return err
	}
	fmt.Fprintf(w, "scripts: %s\n", outDir)
	return nil
}

// runCompatOracleRunAnon probes a shard synchronously via anonymous Apex. It
// deploys nothing and runs no async tests: it inlines the shard's probes into
// chunked anonymous Apex, runs each chunk with one `sf apex run`, and writes the
// same shard observation file the async path produces, so diff is unchanged.
func runCompatOracleRunAnon(ctx context.Context, args []string, w io.Writer) error {
	chunkSize := oracle.DefaultAnonChunkSize
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		if args[i] == "--chunk-size" {
			i++
			if i >= len(args) {
				return errors.New("--chunk-size requires a value")
			}
			n, convErr := strconv.Atoi(args[i])
			if convErr != nil || n <= 0 {
				return errors.New("--chunk-size requires a positive integer")
			}
			chunkSize = n
			continue
		}
		rest = append(rest, args[i])
	}

	runID, runsDir, project, targetOrg, filter, queuePath, area, shard, _, _, _, err := parseRunFlags(rest)
	if err != nil {
		return err
	}
	if strings.TrimSpace(targetOrg) == "" {
		return errors.New("--target-org is required")
	}

	var items []oracle.WorkItem
	if strings.TrimSpace(filter) == "" {
		queue, qErr := loadQueueForRun(queuePath, runsDir, runID)
		if qErr != nil {
			return errors.New("--filter is required when queue is unavailable")
		}
		items = oracle.QueueForShard(queue, shard, area).Items
		if len(items) == 0 {
			return fmt.Errorf("no probes found for shard=%d area=%q", shard, area)
		}
	} else {
		for _, name := range splitOracleFilterClasses(filter) {
			items = append(items, oracle.WorkItem{ProbeID: name, GeneratedClass: name, SurfaceID: name})
		}
	}

	runDir := filepath.Join(runsDir, runID)
	runs, runErr := (oracle.SalesforceRunner{}).RunAnonymousProbes(ctx, project, targetOrg, items, chunkSize)
	obsPath := filepath.Join(runDir, "salesforce", "observations", fmt.Sprintf("shard-%03d.json", shard))
	err = runErr
	if err == nil {
		err = oracle.WriteJSON(obsPath, runs)
	}
	status := "ok"
	msg := fmt.Sprintf("runs=%d", len(runs))
	if err != nil {
		status = "error"
		msg = err.Error()
	}
	_ = oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{RunID: runID, Step: "run-anon", Shard: shard, Status: status, Message: msg})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "salesforce: %s\n", obsPath)
	return nil
}

func runCompatOracleRunSalesforce(ctx context.Context, args []string, w io.Writer) error {
	runID, runsDir, project, targetOrg, filter, queuePath, area, shard, fetchLogs, logLimit, wait, err := parseRunFlags(args)
	if err != nil {
		return err
	}
	if strings.TrimSpace(targetOrg) == "" {
		return errors.New("--target-org is required")
	}
	shardClassNames := []string{}
	if strings.TrimSpace(filter) == "" {
		queue, qErr := loadQueueForRun(queuePath, runsDir, runID)
		if qErr != nil {
			return errors.New("--filter is required when queue is unavailable")
		}
		queue = oracle.QueueForShard(queue, shard, area)
		shardClassNames = oracle.ClassNames(queue)
		if len(shardClassNames) == 0 {
			return fmt.Errorf("no classes found for shard=%d area=%q", shard, area)
		}
		filter = strings.Join(shardClassNames, ",")
	} else {
		shardClassNames = splitOracleFilterClasses(filter)
	}
	runDir := filepath.Join(runsDir, runID)
	deployDir := filepath.Join(runDir, "generated", "sfdx")
	if _, statErr := os.Stat(filepath.Join(deployDir, "sfdx-project.json")); statErr == nil {
		deployProjectDir, prepErr := prepareShardDeployProject(runDir, deployDir, shard, shardClassNames)
		if prepErr != nil {
			return prepErr
		}
		if err := deployOracleGeneratedSource(ctx, deployProjectDir, targetOrg, wait); err != nil {
			_ = oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{
				RunID:   runID,
				Step:    "deploy-salesforce",
				Shard:   shard,
				Status:  "error",
				Message: err.Error(),
			})
			return err
		}
		_ = oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{
			RunID:   runID,
			Step:    "deploy-salesforce",
			Shard:   shard,
			Status:  "ok",
			Message: deployProjectDir,
		})
	}
	runs := make([]oracle.OracleRun, 0, len(shardClassNames))
	if len(shardClassNames) == 0 {
		shardClassNames = splitOracleFilterClasses(filter)
	}
	const batchSize = 100
	batches := chunkOracleClasses(shardClassNames, batchSize)
	for _, batch := range batches {
		batchFilter := strings.Join(batch, ",")
		batchRuns, runErr := (oracle.SalesforceRunner{}).RunTests(ctx, oracle.SalesforceRunOptions{
			Project:     project,
			OrgAlias:    targetOrg,
			Filter:      batchFilter,
			WaitMinute:  wait,
			CaptureLogs: fetchLogs,
			LogLimit:    logLimit,
		})
		if runErr != nil {
			err = runErr
			break
		}
		runs = append(runs, batchRuns...)
	}
	obsPath := filepath.Join(runDir, "salesforce", "observations", fmt.Sprintf("shard-%03d.json", shard))
	if err == nil {
		err = oracle.WriteJSON(obsPath, runs)
	}
	status := "ok"
	msg := fmt.Sprintf("runs=%d", len(runs))
	if err != nil {
		status = "error"
		msg = err.Error()
	}
	_ = oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{RunID: runID, Step: "run-salesforce", Shard: shard, Status: status, Message: msg})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "salesforce: %s\n", obsPath)
	return nil
}

func deployOracleGeneratedSource(ctx context.Context, projectDir, targetOrg string, wait int) error {
	if wait <= 0 {
		wait = 10
	}
	runner := oracle.ExecCommandRunner{}
	stdout, stderr, err := runner.Run(
		ctx,
		projectDir,
		"sf",
		"project", "deploy", "start",
		"--source-dir", "force-app",
		"--target-org", targetOrg,
		"--ignore-conflicts",
		"--wait", strconv.Itoa(wait),
		"--json",
	)
	if err != nil {
		msg := strings.TrimSpace(string(stderr))
		if msg == "" {
			msg = strings.TrimSpace(string(stdout))
		}
		return fmt.Errorf("sf project deploy start failed: %w (%s)", err, msg)
	}
	return nil
}

func prepareShardDeployProject(runDir, sourceProjectDir string, shard int, classNames []string) (string, error) {
	if len(classNames) == 0 {
		return "", errors.New("no classes provided for shard deploy")
	}
	shardProjectDir := filepath.Join(runDir, "deploy", fmt.Sprintf("shard-%03d", shard))
	classDir := filepath.Join(shardProjectDir, "force-app", "main", "default", "classes")
	if err := os.RemoveAll(shardProjectDir); err != nil {
		return "", err
	}
	if err := os.MkdirAll(classDir, 0o755); err != nil {
		return "", err
	}
	projectFile := filepath.Join(sourceProjectDir, "sfdx-project.json")
	projectData, err := os.ReadFile(projectFile)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(shardProjectDir, "sfdx-project.json"), projectData, 0o644); err != nil {
		return "", err
	}
	for _, className := range classNames {
		cls := filepath.Join(sourceProjectDir, "force-app", "main", "default", "classes", className+".cls")
		meta := cls + "-meta.xml"
		if err := copyPath(cls, filepath.Join(classDir, className+".cls")); err != nil {
			return "", err
		}
		if err := copyPath(meta, filepath.Join(classDir, className+".cls-meta.xml")); err != nil {
			return "", err
		}
	}
	return shardProjectDir, nil
}

func runCompatOracleRunGLADE(ctx context.Context, args []string, w io.Writer) error {
	runID, runsDir, project, _, filter, queuePath, area, shard, _, _, _, err := parseRunFlags(args)
	if err != nil {
		return err
	}
	localFilters := []string{}
	if strings.TrimSpace(filter) == "" {
		queue, qErr := loadQueueForRun(queuePath, runsDir, runID)
		if qErr != nil {
			return errors.New("--filter is required when queue is unavailable")
		}
		queue = oracle.QueueForShard(queue, shard, area)
		localFilters = oracleLocalFiltersForQueue(queue)
		if len(localFilters) == 0 {
			return fmt.Errorf("no classes found for shard=%d area=%q", shard, area)
		}
		project = filepath.Join(runsDir, runID, "generated", "sfdx")
	} else {
		localFilters = splitOracleFilterClasses(filter)
	}
	runs := make([]oracle.OracleRun, 0, len(localFilters))
	if len(localFilters) == 0 {
		localFilters = []string{filter}
	}
	for _, localFilter := range localFilters {
		classRuns, runErr := loadOrRunLocalOracle(ctx, project, localFilter, "")
		if runErr != nil {
			err = runErr
			break
		}
		if len(classRuns) == 0 {
			cls, meth := splitOracleLocalFilter(localFilter)
			classRuns = []oracle.OracleRun{{
				SchemaVersion: oracle.SchemaVersion,
				Source:        "glade",
				Project:       project,
				TestClass:     cls,
				TestMethod:    meth,
				Status:        oracle.OracleStatusCompileError,
				Exception:     &oracle.OracleException{Message: "glade produced no run for probe (compile gap or no test method)"},
			}}
		}
		for i := range classRuns {
			classRuns[i] = oracle.NormalizeProbeRun(classRuns[i])
		}
		runs = append(runs, classRuns...)
	}
	runDir := filepath.Join(runsDir, runID)
	obsPath := filepath.Join(runDir, "glade", "observations", fmt.Sprintf("shard-%03d.json", shard))
	if err == nil {
		err = oracle.WriteJSON(obsPath, runs)
	}
	status := "ok"
	msg := fmt.Sprintf("runs=%d", len(runs))
	if err != nil {
		status = "error"
		msg = err.Error()
	}
	_ = oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{RunID: runID, Step: "run-glade", Shard: shard, Status: status, Message: msg})
	if err != nil {
		return err
	}
	fmt.Fprintf(w, "glade: %s\n", obsPath)
	return nil
}

func oracleLocalFiltersForQueue(queue oracle.WorkQueue) []string {
	localFilters := make([]string, 0, len(queue.Items))
	for _, item := range queue.Items {
		if strings.TrimSpace(item.GeneratedClass) == "" {
			continue
		}
		localFilter := item.GeneratedClass
		if strings.TrimSpace(item.MethodName) != "" {
			localFilter += "." + item.MethodName
		}
		localFilters = append(localFilters, localFilter)
	}
	return localFilters
}

func splitOracleLocalFilter(localFilter string) (class, method string) {
	localFilter = strings.TrimSpace(localFilter)
	if i := strings.Index(localFilter, "."); i >= 0 {
		return localFilter[:i], localFilter[i+1:]
	}
	return localFilter, ""
}

func splitOracleFilterClasses(filter string) []string {
	raw := strings.FieldsFunc(filter, func(r rune) bool {
		return r == ',' || r == '|' || r == '\n' || r == '\r' || r == '\t' || r == ' '
	})
	out := make([]string, 0, len(raw))
	seen := map[string]struct{}{}
	for _, item := range raw {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

func chunkOracleClasses(items []string, size int) [][]string {
	if size <= 0 {
		size = 100
	}
	if len(items) == 0 {
		return nil
	}
	out := make([][]string, 0, (len(items)+size-1)/size)
	for start := 0; start < len(items); start += size {
		end := start + size
		if end > len(items) {
			end = len(items)
		}
		batch := append([]string(nil), items[start:end]...)
		out = append(out, batch)
	}
	return out
}

func runCompatOracleDiff(args []string, w io.Writer) error {
	runID, runsDir, shard, jsonOut, err := parseDiffFlags(args)
	if err != nil {
		return err
	}
	runDir := filepath.Join(runsDir, runID)
	sfPath := filepath.Join(runDir, "salesforce", "observations", fmt.Sprintf("shard-%03d.json", shard))
	gladePath := filepath.Join(runDir, "glade", "observations", fmt.Sprintf("shard-%03d.json", shard))
	sfRuns, err := oracle.ReadRuns(sfPath)
	if err != nil {
		return err
	}
	localRuns, err := oracle.ReadRuns(gladePath)
	if err != nil {
		return err
	}
	diffs := oracle.DiffRunSets(sfRuns, localRuns)
	diffPath := filepath.Join(runDir, "diffs", fmt.Sprintf("shard-%03d.json", shard))
	if err := oracle.WriteJSON(diffPath, diffs); err != nil {
		return err
	}
	report := oracle.NewReport("", runID, runDir, diffs, len(sfRuns), len(localRuns), false)
	_ = oracle.AppendLedger(filepath.Join(runDir, "ledger.jsonl"), oracle.LedgerRow{RunID: runID, Step: "diff", Shard: shard, Status: "ok", Message: fmt.Sprintf("pass=%d total=%d", report.Summary.Pass, report.Summary.Total)})
	if jsonOut {
		return writeJSONOut(w, report)
	}
	writeOracleReportText(w, report)
	if !report.Ready {
		return fmt.Errorf("oracle parity mismatch: %d non-pass comparisons", report.Summary.Total-report.Summary.Pass)
	}
	return nil
}

func runCompatOraclePromote(args []string, w io.Writer) error {
	runID := ""
	runsDir := ".glade/oracle/runs"
	outDir := "docs/fixtures/oracle"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run-id":
			i++
			if i >= len(args) {
				return errors.New("--run-id requires a value")
			}
			runID = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				return errors.New("--runs-dir requires a value")
			}
			runsDir = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return errors.New("--out requires a value")
			}
			outDir = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if runID == "" {
		return errors.New("--run-id is required")
	}
	runDir := filepath.Join(runsDir, runID)
	reportPath := filepath.Join(runDir, "report.json")
	report := oracle.OracleReport{}
	if err := oracle.ReadJSON(reportPath, &report); err != nil {
		matches, globErr := filepath.Glob(filepath.Join(runDir, "diffs", "*.json"))
		if globErr != nil || len(matches) == 0 {
			return err
		}
	}
	dst := filepath.Join(outDir, runID)
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return err
	}
	for _, src := range []string{"salesforce", "glade", "diffs", "ledger.jsonl"} {
		p := filepath.Join(runDir, src)
		if _, statErr := os.Stat(p); statErr != nil {
			continue
		}
		base := filepath.Base(p)
		if err := copyPath(p, filepath.Join(dst, base)); err != nil {
			return err
		}
	}
	fmt.Fprintf(w, "promoted: %s\n", dst)
	return nil
}

func runCompatOracleReport(args []string, w io.Writer) error {
	invPath := "docs/generated/apex-oracle/INVENTORY.json"
	output := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New("--inventory requires a value")
			}
			invPath = args[i]
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("--output requires a value")
			}
			output = args[i]
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	inv := oracle.Inventory{}
	if err := oracle.ReadJSON(invPath, &inv); err != nil {
		return err
	}
	coverage := oracle.BuildCoverage(inv)
	if output != "" {
		if err := oracle.WriteJSON(output, coverage); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s: wrote\n", output)
		return nil
	}
	if jsonOut {
		return writeJSONOut(w, coverage)
	}
	fmt.Fprintf(w, "surfaces: %d\n", coverage.TotalSurfaces)
	for k, v := range coverage.ByStatus {
		fmt.Fprintf(w, "- %s: %d\n", k, v)
	}
	return nil
}

func runCompatOracleNext(args []string, w io.Writer) error {
	queuePath := "docs/generated/apex-oracle/WORK_QUEUE.json"
	runID := ""
	runsDir := ".glade/oracle/runs"
	limit := 25
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run-id":
			i++
			if i >= len(args) {
				return errors.New("--run-id requires a value")
			}
			runID = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				return errors.New("--runs-dir requires a value")
			}
			runsDir = args[i]
		case "--work-queue":
			i++
			if i >= len(args) {
				return errors.New("--work-queue requires a value")
			}
			queuePath = args[i]
		case "--limit":
			i++
			if i >= len(args) {
				return errors.New("--limit requires a value")
			}
			n, err := strconv.Atoi(args[i])
			if err != nil {
				return err
			}
			limit = n
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	queue := oracle.WorkQueue{}
	if runID != "" {
		loaded, err := loadQueueForRun(queuePath, runsDir, runID)
		if err != nil {
			return err
		}
		queue = loaded
	} else if err := oracle.ReadJSON(queuePath, &queue); err != nil {
		return err
	}
	next := oracle.NextItems(queue, limit)
	if jsonOut {
		return writeJSONOut(w, map[string]any{"items": next, "count": len(next)})
	}
	fmt.Fprintf(w, "next: %d\n", len(next))
	for _, item := range next {
		fmt.Fprintf(w, "- %s shard=%d %s\n", item.ID, item.Shard, item.ProbeID)
	}
	return nil
}

func runCompatOracleResume(args []string, w io.Writer) error {
	queuePath := "docs/generated/apex-oracle/WORK_QUEUE.json"
	runID := ""
	runsDir := ".glade/oracle/runs"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run-id":
			i++
			if i >= len(args) {
				return errors.New("--run-id requires a value")
			}
			runID = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				return errors.New("--runs-dir requires a value")
			}
			runsDir = args[i]
		case "--work-queue":
			i++
			if i >= len(args) {
				return errors.New("--work-queue requires a value")
			}
			queuePath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	queue := oracle.WorkQueue{}
	if runID != "" {
		loaded, err := loadQueueForRun(queuePath, runsDir, runID)
		if err != nil {
			return err
		}
		queue = loaded
	} else if err := oracle.ReadJSON(queuePath, &queue); err != nil {
		return err
	}
	pending := oracle.NextItems(queue, 0)
	if runID != "" {
		ledgerPath := filepath.Join(runsDir, runID, "ledger.jsonl")
		if rows, err := oracle.ReadLedger(ledgerPath); err == nil {
			fmt.Fprintf(w, "ledgerRows: %d\n", len(rows))
		}
	}
	fmt.Fprintf(w, "pending: %d\n", len(pending))
	for _, item := range pending {
		fmt.Fprintf(w, "- %s shard=%d status=%s\n", item.ID, item.Shard, item.Status)
	}
	return nil
}

func runCompatOracleCheck(args []string, w io.Writer) error {
	inventoryPath := "docs/generated/apex-oracle/INVENTORY.json"
	manifestPath := "docs/generated/apex-oracle/PROBE_MANIFEST.json"
	queuePath := "docs/generated/apex-oracle/WORK_QUEUE.json"
	fixturesDir := "docs/fixtures/oracle"
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New("--inventory requires a value")
			}
			inventoryPath = args[i]
		case "--manifest":
			i++
			if i >= len(args) {
				return errors.New("--manifest requires a value")
			}
			manifestPath = args[i]
		case "--work-queue":
			i++
			if i >= len(args) {
				return errors.New("--work-queue requires a value")
			}
			queuePath = args[i]
		case "--fixtures":
			i++
			if i >= len(args) {
				return errors.New("--fixtures requires a value")
			}
			fixturesDir = args[i]
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	result := map[string]any{"target": "oracle artifact check", "ready": true, "checks": []map[string]any{}}
	checks := []map[string]any{}
	for _, path := range []string{inventoryPath, manifestPath, queuePath} {
		_, err := os.Stat(path)
		ok := err == nil
		checks = append(checks, map[string]any{"path": path, "ok": ok, "error": errString(err)})
		if !ok {
			result["ready"] = false
		}
	}
	if entries, err := os.ReadDir(fixturesDir); err != nil || len(entries) == 0 {
		checks = append(checks, map[string]any{"path": fixturesDir, "ok": false, "error": errString(err)})
		result["ready"] = false
	} else {
		checks = append(checks, map[string]any{"path": fixturesDir, "ok": true, "count": len(entries)})
	}
	var inv oracle.Inventory
	if err := oracle.ReadJSON(inventoryPath, &inv); err == nil {
		unknown := 0
		for _, s := range inv.Surfaces {
			if s.Status == oracle.SurfaceUnknown {
				unknown++
			}
		}
		ok := unknown == 0
		checks = append(checks, map[string]any{"path": "inventory:unknown-surfaces", "ok": ok, "count": unknown})
		if !ok {
			result["ready"] = false
		}
	}
	result["checks"] = checks
	if jsonOut {
		if err := writeJSONOut(w, result); err != nil {
			return err
		}
	} else {
		fmt.Fprintf(w, "ready: %t\n", result["ready"])
		for _, c := range checks {
			fmt.Fprintf(w, "- %s: %t\n", c["path"], c["ok"])
		}
	}
	if ready, _ := result["ready"].(bool); !ready {
		return errors.New("oracle artifact check failed")
	}
	return nil
}

func parseRunFlags(args []string) (runID, runsDir, project, targetOrg, filter, queuePath, area string, shard int, fetchLogs bool, logLimit int, wait int, err error) {
	runID = oracle.DefaultRunID(time.Now())
	runsDir = ".glade/oracle/runs"
	project = "."
	queuePath = "docs/generated/apex-oracle/WORK_QUEUE.json"
	wait = 10
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run-id":
			i++
			if i >= len(args) {
				err = errors.New("--run-id requires a value")
				return
			}
			runID = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				err = errors.New("--runs-dir requires a value")
				return
			}
			runsDir = args[i]
		case "--project":
			i++
			if i >= len(args) {
				err = errors.New("--project requires a value")
				return
			}
			project = args[i]
		case "--target-org":
			i++
			if i >= len(args) {
				err = errors.New("--target-org requires a value")
				return
			}
			targetOrg = args[i]
		case "--filter":
			i++
			if i >= len(args) {
				err = errors.New("--filter requires a value")
				return
			}
			filter = args[i]
		case "--work-queue":
			i++
			if i >= len(args) {
				err = errors.New("--work-queue requires a value")
				return
			}
			queuePath = args[i]
		case "--area":
			i++
			if i >= len(args) {
				err = errors.New("--area requires a value")
				return
			}
			area = args[i]
		case "--shard":
			i++
			if i >= len(args) {
				err = errors.New("--shard requires a value")
				return
			}
			shard, err = strconv.Atoi(args[i])
			if err != nil {
				return
			}
		case "--fetch-logs":
			fetchLogs = true
		case "--log-limit":
			i++
			if i >= len(args) {
				err = errors.New("--log-limit requires a value")
				return
			}
			logLimit, err = strconv.Atoi(args[i])
			if err != nil {
				return
			}
		case "--wait":
			i++
			if i >= len(args) {
				err = errors.New("--wait requires a value")
				return
			}
			wait, err = strconv.Atoi(args[i])
			if err != nil {
				return
			}
		default:
			err = fmt.Errorf("unknown flag %q", args[i])
			return
		}
	}
	err = validateOracleRunID(runID)
	return
}

func parseDiffFlags(args []string) (runID, runsDir string, shard int, jsonOut bool, err error) {
	runID = ""
	runsDir = ".glade/oracle/runs"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--run-id":
			i++
			if i >= len(args) {
				err = errors.New("--run-id requires a value")
				return
			}
			runID = args[i]
		case "--runs-dir":
			i++
			if i >= len(args) {
				err = errors.New("--runs-dir requires a value")
				return
			}
			runsDir = args[i]
		case "--shard":
			i++
			if i >= len(args) {
				err = errors.New("--shard requires a value")
				return
			}
			shard, err = strconv.Atoi(args[i])
			if err != nil {
				return
			}
		case "--json":
			jsonOut = true
		default:
			err = fmt.Errorf("unknown flag %q", args[i])
			return
		}
	}
	if runID == "" {
		err = errors.New("--run-id is required")
	}
	return
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func writeJSONOut(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func checkJSONDrift(path string, v any, label string) error {
	var existing any
	if err := oracle.ReadJSON(path, &existing); err != nil {
		return err
	}
	current, err := json.Marshal(v)
	if err != nil {
		return err
	}
	stored, err := json.Marshal(existing)
	if err != nil {
		return err
	}
	if string(current) != string(stored) {
		return fmt.Errorf("%s drift: run with --output %s", label, path)
	}
	return nil
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func copyPath(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if err := os.MkdirAll(dst, 0o755); err != nil {
			return err
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return err
		}
		for _, e := range entries {
			if err := copyPath(filepath.Join(src, e.Name()), filepath.Join(dst, e.Name())); err != nil {
				return err
			}
		}
		return nil
	}
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o644)
}

func loadQueueForRun(queuePath, runsDir, runID string) (oracle.WorkQueue, error) {
	candidates := []string{
		filepath.Join(runsDir, runID, "work-queue.json"),
		queuePath,
	}
	var queue oracle.WorkQueue
	var lastErr error
	for _, candidate := range candidates {
		if err := oracle.ReadJSON(candidate, &queue); err == nil {
			return queue, nil
		} else {
			lastErr = err
		}
	}
	return oracle.WorkQueue{}, lastErr
}
