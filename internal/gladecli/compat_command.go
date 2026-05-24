package gladecli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexdocs"
	"github.com/glade-sh/glade/internal/capability"
	"github.com/glade-sh/glade/internal/compat"
	"github.com/glade-sh/glade/internal/examplescan"
	"github.com/glade-sh/glade/internal/probe"
	"github.com/glade-sh/glade/internal/projectscan"
)

func runCompat(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New(compatUsage())
	}
	if isHelpArg(args[0]) {
		printCompatHelp(w)
		return nil
	}
	switch args[0] {
	case "matrix", "mvp":
		return runCompatCapabilities(args[1:], w)
	case "post-parity":
		return runCompatPostParity(args[1:], w)
	case "local-tests":
		return runCompatLocalTests(args[1:], w)
	case "oracle":
		return runCompatOracle(ctx, args[1:], w)
	case "oracle-tests":
		return runCompatOracleTests(ctx, args[1:], w)
	case "replay":
		return runCompatReplay(args[1:], w)
	case "ui-controllers":
		return runCompatUIControllers(args[1:], w)
	case "examples":
		return runCompatExamples(args[1:], w)
	case "server-examples":
		return runCompatServerExamples(args[1:], w)
	case "dashboard":
		return runCompatDashboard(args[1:], w)
	case "gaps":
		return runCompatGaps(args[1:], w)
	case "stdlib":
		return runCompatStdlib(args[1:], w)
	case "docs-inventory":
		return runCompatDocsInventory(args[1:], w)
	case "catalog":
		return runCompatCatalog(args[1:], w)
	case "salesforce-coverage":
		return runCompatSalesforceCoverage(args[1:], w)
	case "standard-objects":
		return runCompatStandardObjects(args[1:], w)
	case "stub-behavior":
		return runCompatStubBehavior(args[1:], w)
	case "stub-contracts":
		return runCompatStubContracts(args[1:], w)
	case "stub-discovery":
		return runCompatStubDiscovery(args[1:], w)
	case "stub-inventory":
		return runCompatStubInventory(args[1:], w)
	case "product-namespaces":
		return runCompatProductNamespaces(args[1:], w)
	case "tooling-fixtures":
		return runCompatToolingFixtures(args[1:], w)
	case "evidence":
		return runCompatEvidence(args[1:], w)
	case "validate", "run":
		if len(args) < 2 {
			return errors.New("usage: glade compat validate|run <fixture.json...>")
		}
	default:
		return errors.New(compatUsage())
	}

	for _, path := range args[1:] {
		fixture, err := compat.LoadFile(path)
		if err != nil {
			return err
		}
		if err := compat.Validate(fixture); err != nil {
			return err
		}
		if args[0] == "run" {
			result, err := compat.Run(fixture)
			if err != nil {
				return err
			}
			fmt.Fprintf(w, "%s: %s ok=%t\n", path, result.Kind, result.OK)
			continue
		}
		fmt.Fprintf(w, "%s: ok\n", path)
	}
	return nil
}

func compatUsage() string {
	return "usage: glade compat validate|run <fixture.json...> | matrix|mvp [--json] [--require-ready] | local-tests [--project <root>] [--class <name>] [--class-list <a,b>] [--class-file <path>] [--start-class <name>] [--method <name>] [--changed-since <ref>] [--blockers-only] [--top-failures <n>] [--max-failure-groups <n>] [--timeout <ms-per-test>] [--parallel <n|auto>] [--parallel-methods] [--shard-count <n|auto>] [--shard-index <i|auto>] [--write-class-shards <dir>] [--duration-history <path>] [--progress] [--analyze] [--profile-on-timeout] [--cpu-profile <path>] [--mem-profile <path>] [--perf-json <path>] [--json] [--check <path>] | oracle <subcommand> [flags] | oracle-tests [--project <root>] [--target-org <alias>] [--filter <class[.method]>] [--salesforce-run <path>] [--local-run <path>] [--golden-only] [--anonymous <apex>] [--fetch-logs] [--log-limit <n>] [--runs-dir <path>] [--run-id <id>] [--json] [--check <path>] | replay [--json] [--continue-on-error] [--artifacts <dir>] <bundle-dir...> | ui-controllers [--project <root>] [--json|--check <path>] | post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready] | examples [--project <root>] [--json|--output <path>|--check <path>] | server-examples [--project <root>] [--project-filter <substring>] [--route <substring>] [--probe <substring>] [--outcome <pass|fail|unsupported|missing>] [--blockers-only] [--json] | dashboard|gaps|stdlib [--output <path>|--check <path>] | stdlib --json | docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>] | catalog --inventory <path> [--json|--output <path>|--check <path>] | salesforce-coverage [--source <dir>|--inventory <path>|--catalog <path>] [--tooling-completions <path>] [--tooling-symbols <path>] [--json|--output <path>|--check <path>] | standard-objects [--json|--output <path>|--check <path>] | stub-contracts [--source <dir>] [--json|--output <path>|--check <path>] | stub-discovery [--source <dir>] [--project <probe-sfdx-dir>] [--tier smoke|core|full|local] [--limit <n>] [--no-exec] [--json|--output <path>] | stub-behavior [--json|--output <path>|--check <path>] | stub-inventory [--source <dir>] [--json|--output <path>|--check <path>] | product-namespaces [--source <dir>|--inventory <path>|--catalog <path>] [--tooling-completions <path>] [--symbols-go] [--json|--output <path>|--check <path>] | tooling-fixtures <report.json...> [--json] | evidence --catalog <path> <fixture.json...> [--json]"
}

type postParityReadiness struct {
	Target       string                   `json:"target"`
	Ready        bool                     `json:"ready"`
	Project      string                   `json:"project"`
	Summary      projectscan.Summary      `json:"summary"`
	StageCounts  []postParityCount        `json:"stageCounts"`
	StatusCounts []postParityCount        `json:"statusCounts"`
	Areas        []postParityArea         `json:"areas"`
	Surfaces     []projectscan.Surface    `json:"surfaces"`
	TopBlockers  []projectscan.TopBlocker `json:"topBlockers"`
}

func runCompatLocalTests(args []string, w io.Writer) error {
	if len(args) > 0 && isHelpArg(args[0]) {
		printCompatLocalTestsHelp(w)
		return nil
	}
	options := compat.LocalTestOptions{Project: "."}
	jsonOut := false
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--check":
			if i+1 >= len(args) {
				return errors.New("--check requires a value")
			}
			checkPath = args[i+1]
			i++
		case "--blockers-only":
			options.BlockersOnly = true
		case "--trace-blockers":
			options.TraceBlocked = true
		case "--slow-test-ms":
			if i+1 >= len(args) {
				return errors.New("--slow-test-ms requires a value")
			}
			parsed, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || parsed < 0 {
				return fmt.Errorf("--slow-test-ms must be a non-negative integer")
			}
			options.SlowTestThresholdMS = parsed
			i++
		case "--top-failures":
			if i+1 >= len(args) {
				return errors.New("--top-failures requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return fmt.Errorf("--top-failures must be a non-negative integer")
			}
			options.TopFailures = parsed
			i++
		case "--max-failure-groups":
			if i+1 >= len(args) {
				return errors.New("--max-failure-groups requires a value")
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return fmt.Errorf("--max-failure-groups must be a non-negative integer")
			}
			options.MaxFailureGroups = parsed
			i++
		case "--timeout":
			if i+1 >= len(args) {
				return errors.New("--timeout requires a value")
			}
			parsed, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil || parsed < 0 {
				return fmt.Errorf("--timeout must be a non-negative integer")
			}
			options.TimeoutMS = parsed
			i++
		case "--parallel":
			if i+1 >= len(args) {
				return errors.New("--parallel requires a value")
			}
			if strings.EqualFold(strings.TrimSpace(args[i+1]), "auto") {
				options.AutoTune = true
				i++
				continue
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return fmt.Errorf("--parallel must be a non-negative integer")
			}
			options.Parallelism = parsed
			i++
		case "--progress":
			options.ProgressWriter = os.Stderr
		case "--analyze":
			options.ForceAnalysis = true
		case "--profile-on-timeout":
			options.ProfileOnTimeout = true
		case "--parallel-methods":
			options.ParallelMethods = true
		case "--shard-count":
			if i+1 >= len(args) {
				return errors.New("--shard-count requires a value")
			}
			if strings.EqualFold(strings.TrimSpace(args[i+1]), "auto") {
				options.AutoTune = true
				options.AutoShardCount = true
				i++
				continue
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return fmt.Errorf("--shard-count must be a non-negative integer")
			}
			options.ShardCount = parsed
			i++
		case "--shard-index":
			if i+1 >= len(args) {
				return errors.New("--shard-index requires a value")
			}
			if strings.EqualFold(strings.TrimSpace(args[i+1]), "auto") {
				options.AutoTune = true
				options.AutoShardIndex = true
				i++
				continue
			}
			parsed, err := strconv.Atoi(args[i+1])
			if err != nil || parsed < 0 {
				return fmt.Errorf("--shard-index must be a non-negative integer")
			}
			options.ShardIndex = parsed
			i++
		case "--write-class-shards":
			if i+1 >= len(args) {
				return errors.New("--write-class-shards requires a path")
			}
			options.WriteClassShards = args[i+1]
			i++
		case "--duration-history":
			if i+1 >= len(args) {
				return errors.New("--duration-history requires a path")
			}
			options.DurationHistoryPath = args[i+1]
			i++
		case "--cpu-profile":
			if i+1 >= len(args) {
				return errors.New("--cpu-profile requires a path")
			}
			options.CPUProfilePath = args[i+1]
			i++
		case "--mem-profile":
			if i+1 >= len(args) {
				return errors.New("--mem-profile requires a path")
			}
			options.MemProfilePath = args[i+1]
			i++
		case "--perf-json":
			if i+1 >= len(args) {
				return errors.New("--perf-json requires a path")
			}
			options.PerfJSONPath = args[i+1]
			i++
		case "--changed-since":
			if i+1 >= len(args) {
				return errors.New("--changed-since requires a value")
			}
			options.ChangedSince = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			options.Project = args[i+1]
			i++
		case "--class":
			if i+1 >= len(args) {
				return errors.New("--class requires a value")
			}
			options.Class = args[i+1]
			i++
		case "--class-list":
			if i+1 >= len(args) {
				return errors.New("--class-list requires a value")
			}
			for _, className := range strings.Split(args[i+1], ",") {
				if className = strings.TrimSpace(className); className != "" {
					options.ClassList = append(options.ClassList, className)
				}
			}
			i++
		case "--class-file":
			if i+1 >= len(args) {
				return errors.New("--class-file requires a value")
			}
			options.ClassFile = args[i+1]
			i++
		case "--start-class":
			if i+1 >= len(args) {
				return errors.New("--start-class requires a value")
			}
			options.StartClass = args[i+1]
			i++
		case "--method":
			if i+1 >= len(args) {
				return errors.New("--method requires a value")
			}
			options.Method = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if options.Project == "." &&
		options.Class == "" &&
		len(options.ClassList) == 0 &&
		options.ClassFile == "" &&
		options.StartClass == "" &&
		options.Method == "" &&
		options.ChangedSince == "" &&
		options.Parallelism == 0 &&
		options.ShardCount == 0 &&
		options.ShardIndex == 0 &&
		!options.ParallelMethods &&
		!options.ForceAnalysis {
		options.AutoTune = true
		options.AutoShardCount = true
		options.AutoShardIndex = true
	}
	if checkPath != "" {
		if options.Project != "." || options.Class != "" || len(options.ClassList) != 0 || options.ClassFile != "" || options.StartClass != "" || options.Method != "" || options.BlockersOnly || options.TraceBlocked || options.SlowTestThresholdMS != 0 || options.TopFailures != 0 || options.MaxFailureGroups != 0 || options.TimeoutMS != 0 || options.Parallelism != 0 || options.ProgressWriter != nil || options.ForceAnalysis || options.ProfileOnTimeout || options.ChangedSince != "" || options.ParallelMethods || options.ShardCount != 0 || options.ShardIndex != 0 || options.WriteClassShards != "" || options.DurationHistoryPath != "" || options.CPUProfilePath != "" || options.MemProfilePath != "" || options.PerfJSONPath != "" {
			return errors.New("--check cannot be combined with --project, --class, --class-list, --class-file, --start-class, --method, --changed-since, --parallel-methods, --shard-count, --shard-index, --write-class-shards, --duration-history, --blockers-only, --trace-blockers, --slow-test-ms, --top-failures, --max-failure-groups, --timeout, --parallel, --progress, --analyze, --profile-on-timeout, --cpu-profile, --mem-profile, or --perf-json")
		}
		report, err := compat.CheckLocalTestCorpus(checkPath)
		if jsonOut {
			if writeErr := compat.WriteLocalTestCorpusJSON(w, report); writeErr != nil {
				return writeErr
			}
		} else {
			compat.WriteLocalTestCorpusText(w, report)
		}
		return err
	}
	report, err := compat.RunLocalTests(options)
	if err != nil {
		return err
	}
	if jsonOut {
		return compat.WriteLocalTestJSON(w, report)
	}
	compat.WriteLocalTestText(w, report)
	return nil
}

func runCompatReplay(args []string, w io.Writer) error {
	jsonOut := false
	continueOnError := false
	artifactsDir := ""
	paths := []string{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--continue-on-error":
			continueOnError = true
		case "--artifacts":
			if i+1 >= len(args) {
				return errors.New("--artifacts requires a path")
			}
			artifactsDir = args[i+1]
			i++
		default:
			if strings.HasPrefix(args[i], "-") {
				return fmt.Errorf("unknown flag %q", args[i])
			}
			paths = append(paths, args[i])
		}
	}
	if len(paths) == 0 {
		return errors.New("usage: glade compat replay [--json] [--continue-on-error] [--artifacts <dir>] <bundle-dir...>")
	}
	report, err := compat.RunReplayBundles(paths, compat.ReplayOptions{
		ContinueOnError: continueOnError,
		ArtifactsDir:    artifactsDir,
		CommandArgs:     append([]string{"compat", "replay"}, args...),
	})
	if err != nil {
		return err
	}
	if jsonOut {
		if err := compat.WriteReplayJSON(w, report); err != nil {
			return err
		}
	} else {
		compat.WriteReplayText(w, report)
	}
	if !report.OK {
		return errors.New("compat replay failed")
	}
	return nil
}

func runCompatUIControllers(args []string, w io.Writer) error {
	projectRoot := "."
	jsonOut := false
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			projectRoot = args[i+1]
			i++
		case "--check":
			if i+1 >= len(args) {
				return errors.New("--check requires a value")
			}
			checkPath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if checkPath != "" {
		if projectRoot != "." {
			return errors.New("--check cannot be combined with --project")
		}
		report, err := compat.CheckUIControllerDiscovery(checkPath)
		if jsonOut {
			if writeErr := compat.WriteUIControllerJSON(w, report); writeErr != nil {
				return writeErr
			}
		} else {
			compat.WriteUIControllerText(w, report)
		}
		return err
	}
	report, err := compat.RunUIControllerDiscovery(projectRoot, false)
	if err != nil {
		return err
	}
	if jsonOut {
		return compat.WriteUIControllerJSON(w, report)
	}
	compat.WriteUIControllerText(w, report)
	return nil
}

type postParityCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type postParityArea struct {
	Area     string                `json:"area"`
	Surfaces []projectscan.Surface `json:"surfaces"`
}

func runCompatExamples(args []string, w io.Writer) error {
	roots := []string{"."}
	jsonOut := false
	outputPath := ""
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--output":
			if i+1 >= len(args) {
				return errors.New("usage: glade compat examples [--project <root>] [--json|--output <path>|--check <path>]")
			}
			outputPath = args[i+1]
			i++
		case "--check":
			if i+1 >= len(args) {
				return errors.New("usage: glade compat examples [--project <root>] [--json|--output <path>|--check <path>]")
			}
			checkPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			if len(roots) == 1 && roots[0] == "." {
				roots = []string{args[i+1]}
			} else {
				roots = append(roots, args[i+1])
			}
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}

	type examplesReport struct {
		Projects []examplescan.Report `json:"projects"`
	}
	var report examplesReport
	for _, root := range roots {
		r, err := examplescan.Scan(root, examplescan.Options{
			Name:           filepath.Base(root),
			RunSema:        true,
			RunSurfaceScan: true,
		})
		if err != nil {
			return fmt.Errorf("scan %s: %w", root, err)
		}
		report.Projects = append(report.Projects, r)
	}

	switch {
	case jsonOut:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	case outputPath != "":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
		return os.WriteFile(outputPath, buf.Bytes(), 0o644)
	case checkPath != "":
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("example project report drift: %s is out of sync (run with --output to regenerate)", checkPath)
		}
		fmt.Fprintf(w, "%s: ok\n", checkPath)
		return nil
	default:
		for _, p := range report.Projects {
			fmt.Fprintf(w, "project: %s\n", p.Name)
			fmt.Fprintf(w, "  root: %s\n", p.Root)
			fmt.Fprintf(w, "  layout: %s\n", p.SourceLayout)
			fmt.Fprintf(w, "  classes: %d\n", p.Counts.ApexClasses)
			fmt.Fprintf(w, "  triggers: %d\n", p.Counts.ApexTriggers)
			fmt.Fprintf(w, "  test classes: %d\n", p.Counts.TestClasses)
			fmt.Fprintf(w, "  objects: %d\n", p.Counts.Objects)
			fmt.Fprintf(w, "  fields: %d\n", p.Counts.Fields)
			fmt.Fprintf(w, "  field sets: %d\n", p.Counts.FieldSets)
			fmt.Fprintf(w, "  vf pages: %d\n", p.Counts.VisualforcePages)
			fmt.Fprintf(w, "  vf components: %d\n", p.Counts.VisualforceComponents)
			fmt.Fprintf(w, "  aura: %d\n", p.Counts.AuraComponents)
			fmt.Fprintf(w, "  lwc: %d\n", p.Counts.LWCComponents)
			fmt.Fprintf(w, "  workflows: %d\n", p.Counts.Workflows)
			fmt.Fprintf(w, "  flows: %d\n", p.Counts.Flows)
			fmt.Fprintf(w, "  profiles: %d\n", p.Counts.Profiles)
			fmt.Fprintf(w, "  permission sets: %d\n", p.Counts.PermissionSets)
			fmt.Fprintf(w, "  static resources: %d\n", p.Counts.StaticResources)
			fmt.Fprintf(w, "  custom metadata: %d\n", p.Counts.CustomMetadata)
			fmt.Fprintf(w, "  named credentials: %d\n", p.Counts.NamedCredentials)
			fmt.Fprintf(w, "  remote sites: %d\n", p.Counts.RemoteSites)
			fmt.Fprintf(w, "  labels: %d\n", p.Counts.Labels)
			fmt.Fprintf(w, "  annotations: %v\n", p.Constructs.Annotations)
			fmt.Fprintf(w, "  async interfaces: %v\n", p.Constructs.AsyncInterfaces)
			fmt.Fprintf(w, "  soql features: %v\n", p.RuntimeUsage.SOQLFeatures)
			fmt.Fprintf(w, "  dml features: %v\n", p.RuntimeUsage.DMLFeatures)
			fmt.Fprintf(w, "  namespace refs: %v\n", p.RuntimeUsage.NamespaceRefs)
			fmt.Fprintf(w, "  blockers: %d\n", len(p.TopBlockers))
			for _, b := range p.TopBlockers {
				fmt.Fprintf(w, "    - %s (%s): count=%d files=%d\n", b.CapabilityID, b.Title, b.Count, b.AffectedFiles)
			}
			fmt.Fprintf(w, "  observed blockers: %d\n", len(p.Diagnostics.ObservedBlockers))
			for _, d := range p.Diagnostics.ObservedBlockers {
				fmt.Fprintf(w, "    - %s: %s (%d)\n", d.Code, d.Message, d.Count)
			}
			fmt.Fprintln(w)
		}
		return nil
	}
}

func runCompatPostParity(args []string, w io.Writer) error {
	root := "."
	jsonOut := false
	requireReady := false
	outputPath := ""
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--require-ready":
			requireReady = true
		case "--output":
			if i+1 >= len(args) {
				return errors.New("usage: glade compat post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready]")
			}
			outputPath = args[i+1]
			i++
		case "--check":
			if i+1 >= len(args) {
				return errors.New("usage: glade compat post-parity [--project <root>] [--json|--output <path>|--check <path>] [--require-ready]")
			}
			checkPath = args[i+1]
			i++
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}

	report, err := projectscan.Scan(root)
	if err != nil {
		return err
	}
	readiness := postParityReadiness{
		Target:       "legacy-project local test readiness",
		Ready:        report.Summary.TestBlockingFindings == 0,
		Project:      report.Project,
		Summary:      report.Summary,
		StageCounts:  countPostParitySurfaceField(report.Surfaces, func(surface projectscan.Surface) string { return surface.Stage }, nil),
		StatusCounts: countPostParitySurfaceField(report.Surfaces, func(surface projectscan.Surface) string { return surface.Status }, []string{"supported", "partial", "stub", "unsupported", "unknown"}),
		Areas:        groupPostParitySurfacesByArea(report.Surfaces),
		Surfaces:     report.Surfaces,
		TopBlockers:  report.TopBlockers,
	}
	switch {
	case jsonOut:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(readiness); err != nil {
			return err
		}
	case outputPath != "":
		var buf strings.Builder
		if err := writePostParityReadinessMarkdown(&buf, readiness); err != nil {
			return err
		}
		if err := os.WriteFile(outputPath, []byte(buf.String()), 0o644); err != nil {
			return err
		}
	case checkPath != "":
		var buf strings.Builder
		if err := writePostParityReadinessMarkdown(&buf, readiness); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("post-parity readiness drift: run `glade compat post-parity --project %s --output %s`", root, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
	default:
		writePostParityReadinessText(w, readiness)
	}
	if requireReady && !readiness.Ready {
		return fmt.Errorf("post-parity readiness gate failed: %d test-blocking findings", readiness.Summary.TestBlockingFindings)
	}
	return nil
}

func runCompatServerExamples(args []string, w io.Writer) error {
	root := "."
	jsonOut := false
	options := compat.ServerExampleHarnessOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--blockers-only":
			options.BlockersOnly = true
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--project-filter":
			if i+1 >= len(args) {
				return errors.New("--project-filter requires a value")
			}
			options.ProjectFilter = args[i+1]
			i++
		case "--route":
			if i+1 >= len(args) {
				return errors.New("--route requires a value")
			}
			options.RouteFilter = args[i+1]
			i++
		case "--probe":
			if i+1 >= len(args) {
				return errors.New("--probe requires a value")
			}
			options.ProbeFilter = args[i+1]
			i++
		case "--outcome":
			if i+1 >= len(args) {
				return errors.New("--outcome requires a value")
			}
			options.OutcomeFilter = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	report, err := compat.RunServerExampleHarnessWithOptions(root, options)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	writeServerExampleHarnessText(w, report)
	return nil
}

func writeServerExampleHarnessText(w io.Writer, report compat.ServerExampleHarnessReport) {
	status := "pass"
	if !report.OK {
		status = "blocked"
	}
	fmt.Fprintf(w, "Server example harness: %s\n", status)
	fmt.Fprintf(w, "Root: %s\n", report.Root)
	fmt.Fprintf(w, "Probe counts: pass=%d fail=%d unsupported=%d missing=%d\n", report.Counts.Pass, report.Counts.Fail, report.Counts.Unsupported, report.Counts.Missing)
	for _, project := range report.Projects {
		fmt.Fprintf(w, "%s: %s dataFiles=%d seededObjects=%d seededRecords=%d restRoutes=%d\n", project.Path, project.Status, project.DataFiles, project.SeededObjects, project.SeededRecords, len(project.RestResources))
		if project.Message != "" {
			fmt.Fprintf(w, "  %s\n", project.Message)
		}
	}
	for _, lane := range report.OwnerLanes {
		if lane.Counts.Fail == 0 && lane.Counts.Unsupported == 0 && lane.Counts.Missing == 0 {
			continue
		}
		fmt.Fprintf(w, "%s: pass=%d fail=%d unsupported=%d missing=%d\n", lane.OwnerLane, lane.Counts.Pass, lane.Counts.Fail, lane.Counts.Unsupported, lane.Counts.Missing)
		for _, blocker := range lane.FirstBlockers {
			fmt.Fprintf(w, "  %s %s %s -> %s %d %s\n", blocker.Family, blocker.Method, blocker.Path, blocker.Outcome, blocker.StatusCode, blocker.ErrorCode)
		}
	}
}

func countPostParitySurfaceField(surfaces []projectscan.Surface, value func(projectscan.Surface) string, seed []string) []postParityCount {
	counts := map[string]int{}
	for _, name := range seed {
		counts[name] = 0
	}
	for _, surface := range surfaces {
		name := value(surface)
		if name == "" {
			name = "unknown"
		}
		counts[name]++
	}
	seen := map[string]struct{}{}
	names := make([]string, 0, len(counts))
	for _, name := range seed {
		if _, ok := seen[name]; ok {
			continue
		}
		if _, ok := counts[name]; ok {
			names = append(names, name)
			seen[name] = struct{}{}
		}
	}
	extras := make([]string, 0, len(counts))
	for name := range counts {
		if _, ok := seen[name]; ok {
			continue
		}
		extras = append(extras, name)
	}
	sort.Strings(extras)
	names = append(names, extras...)
	out := make([]postParityCount, 0, len(names))
	for _, name := range names {
		out = append(out, postParityCount{Name: name, Count: counts[name]})
	}
	return out
}

func groupPostParitySurfacesByArea(surfaces []projectscan.Surface) []postParityArea {
	grouped := map[string][]projectscan.Surface{}
	for _, surface := range surfaces {
		area := surface.Area
		if area == "" {
			area = "unknown"
		}
		grouped[area] = append(grouped[area], surface)
	}
	areas := make([]string, 0, len(grouped))
	for area := range grouped {
		areas = append(areas, area)
	}
	sort.Strings(areas)
	out := make([]postParityArea, 0, len(areas))
	for _, area := range areas {
		surfaces := grouped[area]
		sort.Slice(surfaces, func(i, j int) bool {
			return surfaces[i].Capability < surfaces[j].Capability
		})
		out = append(out, postParityArea{Area: area, Surfaces: surfaces})
	}
	return out
}

func writePostParityReadinessText(w io.Writer, readiness postParityReadiness) {
	status := "ready"
	if !readiness.Ready {
		status = "not ready"
	}
	fmt.Fprintf(w, "Post-parity readiness: %s\n", status)
	fmt.Fprintf(w, "Target: %s\n", readiness.Target)
	fmt.Fprintf(w, "Project: %s\n", readiness.Project)
	fmt.Fprintf(w, "Files scanned: %d\n", readiness.Summary.FilesScanned)
	fmt.Fprintf(w, "Reports: %d\n", readiness.Summary.Reports)
	fmt.Fprintf(w, "Dashboards: %d\n", readiness.Summary.Dashboards)
	fmt.Fprintf(w, "Surfaces: %d\n", readiness.Summary.Surfaces)
	fmt.Fprintf(w, "Findings: %d\n", readiness.Summary.Findings)
	fmt.Fprintf(w, "Test-blocking findings: %d\n", readiness.Summary.TestBlockingFindings)
	writePostParityCountsText(w, "Status counts", readiness.StatusCounts)
	writePostParityCountsText(w, "Stage counts", readiness.StageCounts)
	if len(readiness.TopBlockers) > 0 {
		fmt.Fprintln(w, "Top blockers:")
		for _, blocker := range readiness.TopBlockers {
			fmt.Fprintf(w, "- %s: %d findings across %d files\n", blocker.Capability, blocker.Count, blocker.AffectedFiles)
		}
	}
	if len(readiness.Areas) > 0 {
		fmt.Fprintln(w, "Surfaces by area:")
		for _, area := range readiness.Areas {
			fmt.Fprintf(w, "- %s:\n", area.Area)
			for _, surface := range area.Surfaces {
				fmt.Fprintf(w, "  - %s [%s/%s]: %d findings across %d files; next %s\n", surface.Capability, surface.Stage, surface.Status, surface.Count, surface.AffectedFiles, surface.SuggestedCapability)
			}
		}
	}
}

func writePostParityCountsText(w io.Writer, title string, counts []postParityCount) {
	if len(counts) == 0 {
		return
	}
	fmt.Fprintf(w, "%s:\n", title)
	for _, count := range counts {
		fmt.Fprintf(w, "- %s: %d\n", count.Name, count.Count)
	}
}

func writePostParityReadinessMarkdown(w io.Writer, readiness postParityReadiness) error {
	status := "ready"
	if !readiness.Ready {
		status = "not ready"
	}
	if _, err := fmt.Fprintf(w, "# Post-Parity Readiness\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Post-parity readiness is **%s** for `%s`.\n\n", status, readiness.Project); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "This dashboard is separate from the MVP readiness gate. Scanner discovery does not promote a surface to supported without explicit status plumbing and tests.\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "## Summary\n\n"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Metric | Count |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: |"); err != nil {
		return err
	}
	rows := []struct {
		label string
		count int
	}{
		{"Files scanned", readiness.Summary.FilesScanned},
		{"Reports", readiness.Summary.Reports},
		{"Dashboards", readiness.Summary.Dashboards},
		{"Detected surfaces", readiness.Summary.Surfaces},
		{"Findings", readiness.Summary.Findings},
		{"Test-blocking findings", readiness.Summary.TestBlockingFindings},
	}
	for _, row := range rows {
		if _, err := fmt.Fprintf(w, "| %s | %d |\n", row.label, row.count); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}
	if err := writePostParityCountsMarkdown(w, "Status Counts", readiness.StatusCounts); err != nil {
		return err
	}
	if err := writePostParityCountsMarkdown(w, "Stage Counts", readiness.StageCounts); err != nil {
		return err
	}
	if _, err := fmt.Fprint(w, "## Top Blockers\n\n"); err != nil {
		return err
	}
	if len(readiness.TopBlockers) == 0 {
		if _, err := fmt.Fprint(w, "No test-blocking post-parity findings were detected.\n\n"); err != nil {
			return err
		}
	} else {
		if _, err := fmt.Fprintln(w, "| Capability | Title | Findings | Files |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | --- | ---: | ---: |"); err != nil {
			return err
		}
		for _, blocker := range readiness.TopBlockers {
			if _, err := fmt.Fprintf(w, "| `%s` | %s | %d | %d |\n", blocker.Capability, blocker.Title, blocker.Count, blocker.AffectedFiles); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprint(w, "## Surfaces By Area\n\n"); err != nil {
		return err
	}
	if len(readiness.Areas) == 0 {
		_, err := fmt.Fprint(w, "No post-parity surfaces were detected.\n\n")
		return err
	}
	for _, area := range readiness.Areas {
		if _, err := fmt.Fprintf(w, "### %s\n\n", area.Area); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| Capability | Stage | Status | Findings | Files | Suggested next capability |"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(w, "| --- | --- | --- | ---: | ---: | --- |"); err != nil {
			return err
		}
		for _, surface := range area.Surfaces {
			if _, err := fmt.Fprintf(w, "| `%s` | %s | %s | %d | %d | `%s` |\n", surface.Capability, surface.Stage, surface.Status, surface.Count, surface.AffectedFiles, surface.SuggestedCapability); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintln(w); err != nil {
			return err
		}
	}
	return nil
}

func writePostParityCountsMarkdown(w io.Writer, title string, counts []postParityCount) error {
	if _, err := fmt.Fprintf(w, "## %s\n\n", title); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Name | Count |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | ---: |"); err != nil {
		return err
	}
	for _, count := range counts {
		if _, err := fmt.Fprintf(w, "| %s | %d |\n", count.Name, count.Count); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w)
	return err
}

func runCompatCapabilities(args []string, w io.Writer) error {
	jsonOut := false
	requireReady := false
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		case "--require-ready":
			requireReady = true
		default:
			return fmt.Errorf("unknown flag %q", arg)
		}
	}
	report := capability.MVPReport()
	if jsonOut {
		if err := capability.WriteJSON(w, report); err != nil {
			return err
		}
	} else if err := capability.WriteText(w, report); err != nil {
		return err
	}
	if requireReady && !report.Ready {
		return fmt.Errorf("MVP readiness gate failed: %d required capabilities incomplete", report.Incomplete)
	}
	return nil
}

func runCompatDashboard(args []string, w io.Writer) error {
	return runCompatGeneratedMarkdown(args, w, "dashboard", "compatibility dashboard", capability.WriteMarkdown)
}

func runCompatGaps(args []string, w io.Writer) error {
	return runCompatGeneratedMarkdown(args, w, "gaps", "known gaps", capability.WriteKnownGapsMarkdown)
}

func runCompatStdlib(args []string, w io.Writer) error {
	jsonOut := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--json" {
			jsonOut = true
			continue
		}
		filtered = append(filtered, arg)
	}
	if jsonOut {
		if len(filtered) != 0 {
			return errors.New("usage: glade compat stdlib [--json|--output <path>|--check <path>]")
		}
		return capability.WriteStdlibJSON(w)
	}
	return runCompatStaticMarkdown(filtered, w, "stdlib", "standard library coverage", capability.WriteStdlibMarkdown)
}

func runCompatDocsInventory(args []string, w io.Writer) error {
	source := ""
	outputPath := ""
	checkPath := ""
	diffPath := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			source = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			checkPath = args[i]
		case "--diff":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
			}
			diffPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if source == "" {
		return errors.New("usage: glade compat docs-inventory --source <dir> [--json|--output <path>|--check <path>|--diff <path>]")
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != "", diffPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, --check, or --diff")
	}

	inv, err := apexdocs.BuildInventory(source)
	if err != nil {
		return err
	}

	switch {
	case jsonOut:
		return apexdocs.WriteJSON(w, inv)
	case outputPath != "":
		var buf strings.Builder
		if err := apexdocs.WriteJSON(&buf, inv); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := apexdocs.WriteJSON(&buf, inv); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("docs inventory drift: run `glade compat docs-inventory --source %s --output %s`", source, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	case diffPath != "":
		oldInv, err := apexdocs.ReadInventory(diffPath)
		if err != nil {
			return err
		}
		diff := apexdocs.DiffInventories(oldInv, inv)
		return apexdocs.WriteDiffJSON(w, diff)
	default:
		writeDocsInventorySummary(w, inv)
		return nil
	}
}

func writeDocsInventorySummary(w io.Writer, inv apexdocs.Inventory) {
	fmt.Fprintf(w, "schemaVersion: %d\n", inv.SchemaVersion)
	fmt.Fprintf(w, "documents: %d\n", inv.TotalFiles)
	fmt.Fprintf(w, "members: %d\n", inv.TotalMembers)
	fmt.Fprintf(w, "namespaces: %d\n", len(inv.Namespaces))
	if len(inv.Namespaces) == 0 {
		return
	}
	fmt.Fprintln(w, "namespace summary:")
	for _, summary := range inv.Namespaces {
		fmt.Fprintf(w, "  %s: documents=%d members=%d", summary.Namespace, summary.Documents, summary.Members)
		if summary.Classes > 0 {
			fmt.Fprintf(w, " classes=%d", summary.Classes)
		}
		if summary.Interfaces > 0 {
			fmt.Fprintf(w, " interfaces=%d", summary.Interfaces)
		}
		if summary.Enums > 0 {
			fmt.Fprintf(w, " enums=%d", summary.Enums)
		}
		if summary.Inputs > 0 {
			fmt.Fprintf(w, " inputs=%d", summary.Inputs)
		}
		if summary.Outputs > 0 {
			fmt.Fprintf(w, " outputs=%d", summary.Outputs)
		}
		fmt.Fprintln(w)
	}
}

func runCompatCatalog(args []string, w io.Writer) error {
	inventoryPath := ""
	outputPath := ""
	checkPath := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
			}
			inventoryPath = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if inventoryPath == "" {
		return errors.New("usage: glade compat catalog --inventory <path> [--json|--output <path>|--check <path>]")
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	inv, err := apexdocs.ReadInventory(inventoryPath)
	if err != nil {
		return err
	}
	catalog := capability.BuildCatalog(inv)
	switch {
	case jsonOut:
		return capability.WriteCatalogJSON(w, catalog)
	case outputPath != "":
		var buf strings.Builder
		if err := capability.WriteCatalogJSON(&buf, catalog); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := capability.WriteCatalogJSON(&buf, catalog); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("capability catalog drift: run `glade compat catalog --inventory %s --output %s`", inventoryPath, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		writeCatalogSummary(w, catalog)
		return nil
	}
}

func runCompatSalesforceCoverage(args []string, w io.Writer) error {
	source := ""
	inventoryPath := ""
	catalogPath := ""
	toolingCompletionsPath := ""
	toolingSymbolsPath := ""
	outputPath := ""
	checkPath := ""
	jsonOut := false
	usage := "usage: glade compat salesforce-coverage [--source <dir>|--inventory <path>|--catalog <path>] [--tooling-completions <path>] [--tooling-symbols <path>] [--json|--output <path>|--check <path>]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			source = args[i]
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			inventoryPath = args[i]
		case "--catalog":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			catalogPath = args[i]
		case "--tooling-completions":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			toolingCompletionsPath = args[i]
		case "--tooling-symbols":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			toolingSymbolsPath = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	sources := 0
	for _, set := range []bool{source != "", inventoryPath != "", catalogPath != ""} {
		if set {
			sources++
		}
	}
	if sources > 1 {
		return errors.New("use only one of --source, --inventory, or --catalog")
	}
	if sources == 0 {
		source = defaultSalesforceDocsSource()
	}
	if toolingCompletionsPath == "" {
		if defaultPath := defaultSalesforceToolingCompletionsSource(); fileExists(defaultPath) {
			toolingCompletionsPath = defaultPath
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}

	catalog, err := loadSalesforceCoverageCatalog(source, inventoryPath, catalogPath)
	if err != nil {
		return err
	}
	var tooling *capability.ToolingCompletions
	var apexClassSymbols *capability.ToolingApexClassSymbols
	if toolingCompletionsPath != "" {
		completions, err := capability.ReadToolingCompletions(toolingCompletionsPath)
		if err != nil {
			return err
		}
		tooling = &completions
	}
	if toolingSymbolsPath != "" {
		symbols, err := capability.ReadToolingApexClassSymbols(toolingSymbolsPath)
		if err != nil {
			return err
		}
		apexClassSymbols = &symbols
	}
	toolingSource := toolingCompletionsPath
	if toolingSymbolsPath != "" {
		if toolingSource != "" {
			toolingSource += ", "
		}
		toolingSource += toolingSymbolsPath
	}
	toolingSource = displayToolingSource(toolingSource)
	report := capability.BuildSalesforceCoverageReportWithTooling(catalog, tooling, apexClassSymbols, toolingSource)
	switch {
	case jsonOut:
		return capability.WriteSalesforceCoverageJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := writeSalesforceCoverageOutput(&buf, report, outputPath); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := writeSalesforceCoverageOutput(&buf, report, checkPath); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("Salesforce coverage drift: run `glade compat salesforce-coverage --output %s`", checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		return capability.WriteSalesforceCoverageText(w, report)
	}
}

func writeSalesforceCoverageOutput(w io.Writer, report capability.SalesforceCoverageReport, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return capability.WriteSalesforceCoverageJSON(w, report)
	}
	return capability.WriteSalesforceCoverageMarkdown(w, report)
}

func displayToolingSource(source string) string {
	if source == "" {
		return ""
	}
	parts := strings.Split(source, ", ")
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = filepath.Base(part)
	}
	return strings.Join(parts, ", ")
}

func runCompatStandardObjects(args []string, w io.Writer) error {
	outputPath := ""
	checkPath := ""
	jsonOut := false
	usage := "usage: glade compat standard-objects [--json|--output <path>|--check <path>]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	report := capability.BuildStandardObjectCoverageReport()
	switch {
	case jsonOut:
		return capability.WriteStandardObjectCoverageJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := writeStandardObjectCoverageOutput(&buf, report, outputPath); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := writeStandardObjectCoverageOutput(&buf, report, checkPath); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("standard object coverage drift: run `glade compat standard-objects --output %s`", checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		fmt.Fprintf(w, "objects: %d\n", report.Totals.Objects)
		fmt.Fprintf(w, "fields: %d\n", report.Totals.Fields)
		fmt.Fprintf(w, "relationships: %d\n", report.Totals.Relationships)
		fmt.Fprintf(w, "recordTypes: %d\n", report.Totals.RecordTypes)
		return nil
	}
}

func writeStandardObjectCoverageOutput(w io.Writer, report capability.StandardObjectCoverageReport, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return capability.WriteStandardObjectCoverageJSON(w, report)
	}
	return capability.WriteStandardObjectCoverageMarkdown(w, report)
}

func runCompatStubBehavior(args []string, w io.Writer) error {
	outputPath := ""
	checkPath := ""
	jsonOut := false
	usage := "usage: glade compat stub-behavior [--json|--output <path>|--check <path>]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	report := capability.BuildStubBehaviorReport()
	switch {
	case jsonOut:
		return capability.WriteStubBehaviorJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := writeStubBehaviorOutput(&buf, report, outputPath); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := writeStubBehaviorOutput(&buf, report, checkPath); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("stub behavior drift: run `glade compat stub-behavior --output %s`", checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		fmt.Fprintf(w, "entries: %d\n", report.Totals.Entries)
		fmt.Fprintf(w, "types: %d\n", report.Totals.Types)
		fmt.Fprintf(w, "members: %d\n", report.Totals.Members)
		fmt.Fprintf(w, "implemented: %d\n", report.Totals.Implemented)
		fmt.Fprintf(w, "passive-default: %d\n", report.Totals.PassiveDefault)
		fmt.Fprintf(w, "unsupported: %d\n", report.Totals.Unsupported)
		fmt.Fprintf(w, "unknown: %d\n", report.Totals.Unknown)
		return nil
	}
}

func runCompatStubContracts(args []string, w io.Writer) error {
	sourceRoot := filepath.Join("example-projects", "stubs")
	outputPath := ""
	checkPath := ""
	probeManifestPath := ""
	probeTier := "full"
	jsonOut := false
	usage := "usage: glade compat stub-contracts [--source <dir>] [--json|--output <path>|--check <path>] [--probe-manifest <path>] [--probe-tier smoke|core|full|local]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			sourceRoot = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			checkPath = args[i]
		case "--probe-manifest":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			probeManifestPath = args[i]
		case "--probe-tier":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			probeTier = strings.ToLower(strings.TrimSpace(args[i]))
			switch probeTier {
			case "smoke", "core", "full", "local":
			default:
				return errors.New(usage)
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	report, err := capability.BuildStubContractReport(sourceRoot)
	if err != nil {
		return err
	}
	if probeManifestPath != "" {
		specs := capability.BuildStubContractProbeManifest(report, probeTier)
		var manifestBuf strings.Builder
		if err := capability.WriteStubContractProbeManifestJSON(&manifestBuf, specs); err != nil {
			return err
		}
		if err := os.WriteFile(probeManifestPath, []byte(manifestBuf.String()), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(w, "%s: wrote %d probe specs (tier=%s)\n", probeManifestPath, len(specs), probeTier)
	}
	switch {
	case jsonOut:
		return capability.WriteStubContractsJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := writeStubContractsOutput(&buf, report, outputPath); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := writeStubContractsOutput(&buf, report, checkPath); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("stub contracts drift: run `glade compat stub-contracts --output %s`", checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		fmt.Fprintf(w, "entries: %d\n", report.Totals.Entries)
		fmt.Fprintf(w, "types: %d\n", report.Totals.Types)
		fmt.Fprintf(w, "members: %d\n", report.Totals.Members)
		fmt.Fprintf(w, "withProbe: %d\n", report.Totals.WithProbe)
		fmt.Fprintf(w, "org-diff: %d\n", report.Totals.ByMode[string(capability.StubContractOrgDiff)])
		fmt.Fprintf(w, "local-contract: %d\n", report.Totals.ByMode[string(capability.StubContractLocalOnly)])
		fmt.Fprintf(w, "passive-dto: %d\n", report.Totals.ByMode[string(capability.StubContractPassiveDTO)])
		fmt.Fprintf(w, "compile-shape: %d\n", report.Totals.ByMode[string(capability.StubContractCompileShape)])
		return nil
	}
}

func writeStubContractsOutput(w io.Writer, report capability.StubContractReport, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return capability.WriteStubContractsJSON(w, report)
	}
	return capability.WriteStubContractsMarkdown(w, report)
}

type stubDiscoveryReport struct {
	Target            string                        `json:"target"`
	Source            string                        `json:"source"`
	Tier              string                        `json:"tier"`
	ProbeProject      string                        `json:"probeProject"`
	Requested         int                           `json:"requested"`
	Executed          int                           `json:"executed"`
	GeneratedAtUTC    string                        `json:"generatedAtUtc"`
	OutcomeCounts     map[string]int                `json:"outcomeCounts"`
	OddityRiskCounts  map[string]int                `json:"oddityRiskCounts"`
	Candidates        []stubDiscoveryCandidate      `json:"candidates"`
	TopImplementation []stubDiscoveryCandidateBrief `json:"topImplementation"`
}

type stubDiscoveryCandidate struct {
	ProbeID       string   `json:"probeId"`
	ContractID    string   `json:"contractId"`
	Type          string   `json:"type"`
	Member        string   `json:"member,omitempty"`
	Kind          string   `json:"kind"`
	Mode          string   `json:"mode"`
	Owner         string   `json:"owner"`
	Outcome       string   `json:"outcome"`
	ExceptionType string   `json:"exceptionType,omitempty"`
	ExceptionMsg  string   `json:"exceptionMessage,omitempty"`
	OddityRisk    string   `json:"oddityRisk,omitempty"`
	EdgeTags      []string `json:"edgeTags,omitempty"`
	Normalization string   `json:"normalization,omitempty"`
	FailureShape  string   `json:"failureShape,omitempty"`
	NextAction    string   `json:"nextAction"`
}

type stubDiscoveryCandidateBrief struct {
	ContractID string `json:"contractId"`
	ProbeID    string `json:"probeId"`
	Owner      string `json:"owner"`
	Outcome    string `json:"outcome"`
	OddityRisk string `json:"oddityRisk,omitempty"`
	NextAction string `json:"nextAction"`
}

func runCompatStubDiscovery(args []string, w io.Writer) error {
	sourceRoot := filepath.Join("example-projects", "stubs")
	probeProject := filepath.Join("probes", "sfdx")
	tier := "smoke"
	limit := 200
	noExec := false
	jsonOut := false
	outputPath := ""
	usage := "usage: glade compat stub-discovery [--source <dir>] [--project <probe-sfdx-dir>] [--tier smoke|core|full|local] [--limit <n>] [--no-exec] [--json|--output <path>]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			sourceRoot = args[i]
		case "--project":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			probeProject = args[i]
		case "--tier":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			tier = strings.ToLower(strings.TrimSpace(args[i]))
			switch tier {
			case "smoke", "core", "full", "local":
			default:
				return errors.New(usage)
			}
		case "--limit":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			n, err := strconv.Atoi(args[i])
			if err != nil || n <= 0 {
				return errors.New(usage)
			}
			limit = n
		case "--no-exec":
			noExec = true
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if jsonOut && outputPath != "" {
		return errors.New("use only one of --json or --output")
	}
	contracts, err := capability.BuildStubContractReport(sourceRoot)
	if err != nil {
		return err
	}
	manifest := capability.BuildStubContractProbeManifest(contracts, tier)
	if len(manifest) == 0 {
		return errors.New("stub discovery manifest is empty for requested tier")
	}
	if limit > len(manifest) {
		limit = len(manifest)
	}
	contractByID := map[string]capability.StubContractEntry{}
	for _, entry := range contracts.Entries {
		contractByID[entry.ID] = entry
	}
	selected := manifest[:limit]
	probeIDs := make([]string, 0, len(selected))
	for _, spec := range selected {
		probeIDs = append(probeIDs, spec.ID)
	}
	results := map[string]probe.ProbeResult{}
	if !noExec {
		localExec := &probe.LocalExecutor{ProbeDir: probeProject}
		var captureErr error
		results, _, captureErr = localExec.CaptureLocal(probeIDs)
		if captureErr != nil {
			return captureErr
		}
	}
	report := stubDiscoveryReport{
		Target:           "stub contract implementation discovery",
		Source:           sourceRoot,
		Tier:             tier,
		ProbeProject:     probeProject,
		Requested:        len(selected),
		Executed:         len(results),
		GeneratedAtUTC:   time.Now().UTC().Format(time.RFC3339),
		OutcomeCounts:    map[string]int{},
		OddityRiskCounts: map[string]int{},
		Candidates:       make([]stubDiscoveryCandidate, 0, len(selected)),
	}
	for _, spec := range selected {
		contract := contractByID[spec.ContractID]
		candidate := stubDiscoveryCandidate{}
		if noExec {
			candidate = buildStubDiscoveryCandidateNoExec(spec, contract)
		} else {
			result, ok := results[spec.ID]
			if !ok {
				continue
			}
			candidate = buildStubDiscoveryCandidate(spec, contract, result)
		}
		report.Candidates = append(report.Candidates, candidate)
		report.OutcomeCounts[candidate.Outcome]++
		if candidate.OddityRisk != "" {
			report.OddityRiskCounts[candidate.OddityRisk]++
		}
	}
	sort.SliceStable(report.Candidates, func(i, j int) bool {
		ri := stubDiscoveryPriority(report.Candidates[i])
		rj := stubDiscoveryPriority(report.Candidates[j])
		if ri != rj {
			return ri > rj
		}
		return report.Candidates[i].ContractID < report.Candidates[j].ContractID
	})
	top := 50
	if top > len(report.Candidates) {
		top = len(report.Candidates)
	}
	report.TopImplementation = make([]stubDiscoveryCandidateBrief, 0, top)
	for _, c := range report.Candidates[:top] {
		report.TopImplementation = append(report.TopImplementation, stubDiscoveryCandidateBrief{
			ContractID: c.ContractID,
			ProbeID:    c.ProbeID,
			Owner:      c.Owner,
			Outcome:    c.Outcome,
			OddityRisk: c.OddityRisk,
			NextAction: c.NextAction,
		})
	}
	switch {
	case jsonOut:
		return writeStubDiscoveryJSON(w, report)
	case outputPath != "":
		var b strings.Builder
		if strings.EqualFold(filepath.Ext(outputPath), ".json") {
			if err := writeStubDiscoveryJSON(&b, report); err != nil {
				return err
			}
		} else {
			if err := writeStubDiscoveryMarkdown(&b, report); err != nil {
				return err
			}
		}
		return os.WriteFile(outputPath, []byte(b.String()), 0o644)
	default:
		fmt.Fprintf(w, "executed: %d/%d (tier=%s, no-exec=%t)\n", report.Executed, report.Requested, report.Tier, noExec)
		fmt.Fprintf(w, "outcomes: returns=%d unsupported=%d exception=%d compile_error=%d missing=%d\n",
			report.OutcomeCounts["returns"], report.OutcomeCounts["unsupported"], report.OutcomeCounts["exception"], report.OutcomeCounts["compile_error"], report.OutcomeCounts["missing"])
		fmt.Fprintf(w, "discovery: needs_org_probe=%d needs_local_probe=%d unverified=%d\n",
			report.OutcomeCounts["needs_org_probe"], report.OutcomeCounts["needs_local_probe"], report.OutcomeCounts["unverified"])
		fmt.Fprintf(w, "oddity-risk: high=%d medium=%d low=%d\n",
			report.OddityRiskCounts["high"], report.OddityRiskCounts["medium"], report.OddityRiskCounts["low"])
		return nil
	}
}

func buildStubDiscoveryCandidateNoExec(spec capability.StubContractProbeSpec, contract capability.StubContractEntry) stubDiscoveryCandidate {
	outcome := "unverified"
	if contract.Mode == capability.StubContractOrgDiff {
		outcome = "needs_org_probe"
	}
	if contract.Mode == capability.StubContractLocalOnly {
		outcome = "needs_local_probe"
	}
	return stubDiscoveryCandidate{
		ProbeID:       spec.ID,
		ContractID:    spec.ContractID,
		Type:          contract.Type,
		Member:        contract.Member,
		Kind:          contract.Kind,
		Mode:          string(contract.Mode),
		Owner:         contract.Owner,
		Outcome:       outcome,
		OddityRisk:    contract.OddityRisk,
		EdgeTags:      append([]string(nil), contract.EdgeTags...),
		Normalization: contract.Normalization,
		FailureShape:  contract.FailureShape,
		NextAction:    stubDiscoveryNextAction(contract, outcome),
	}
}

func buildStubDiscoveryCandidate(spec capability.StubContractProbeSpec, contract capability.StubContractEntry, result probe.ProbeResult) stubDiscoveryCandidate {
	outcome := "returns"
	excType := ""
	excMsg := ""
	if result.ExceptionType != nil {
		excType = *result.ExceptionType
		if result.ExceptionMessage != nil {
			excMsg = *result.ExceptionMessage
		}
		switch {
		case strings.Contains(strings.ToLower(excType), "unsupported") || strings.Contains(strings.ToLower(excMsg), "unsupported"):
			outcome = "unsupported"
		case strings.Contains(strings.ToLower(excType), "unknownprobeexception"):
			outcome = "missing"
		case strings.Contains(strings.ToLower(excType), "compile"):
			outcome = "compile_error"
		default:
			outcome = "exception"
		}
	}
	return stubDiscoveryCandidate{
		ProbeID:       spec.ID,
		ContractID:    spec.ContractID,
		Type:          contract.Type,
		Member:        contract.Member,
		Kind:          contract.Kind,
		Mode:          string(contract.Mode),
		Owner:         contract.Owner,
		Outcome:       outcome,
		ExceptionType: excType,
		ExceptionMsg:  excMsg,
		OddityRisk:    contract.OddityRisk,
		EdgeTags:      append([]string(nil), contract.EdgeTags...),
		Normalization: contract.Normalization,
		FailureShape:  contract.FailureShape,
		NextAction:    stubDiscoveryNextAction(contract, outcome),
	}
}

func stubDiscoveryNextAction(contract capability.StubContractEntry, outcome string) string {
	switch outcome {
	case "needs_org_probe":
		return "execute org probe and record golden behavior for parity implementation"
	case "needs_local_probe":
		return "execute local probe and confirm deterministic unsupported or passive-local behavior"
	case "unverified":
		return "run probe execution for this contract and classify observed behavior"
	case "returns":
		if contract.Mode == capability.StubContractOrgDiff {
			return "run org-diff probe and compare payload parity"
		}
		return "retain behavior and add regression fixture"
	case "unsupported":
		return "decide keep-unsupported or implement deterministic local model"
	case "compile_error":
		return "fix parser/sema/runtime call shape for this signature"
	case "missing":
		return "repair generated probe invocation template for this signature"
	default:
		return "capture exception shape and implement Apex-compatible behavior"
	}
}

func stubDiscoveryPriority(c stubDiscoveryCandidate) int {
	score := 0
	switch c.Outcome {
	case "compile_error":
		score += 400
	case "missing":
		score += 350
	case "exception":
		score += 300
	case "unsupported":
		score += 200
	default:
		score += 100
	}
	switch c.OddityRisk {
	case "high":
		score += 30
	case "medium":
		score += 20
	case "low":
		score += 10
	}
	if c.Mode == string(capability.StubContractOrgDiff) {
		score += 15
	}
	return score
}

func writeStubDiscoveryJSON(w io.Writer, report stubDiscoveryReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(report)
}

func writeStubDiscoveryMarkdown(w io.Writer, report stubDiscoveryReport) error {
	if _, err := fmt.Fprintln(w, "# Stub Discovery"); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "\nTier: `%s`\n", report.Tier); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Executed: %d/%d\n", report.Executed, report.Requested); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "- Outcomes: returns=%d unsupported=%d exception=%d compile_error=%d missing=%d\n",
		report.OutcomeCounts["returns"], report.OutcomeCounts["unsupported"], report.OutcomeCounts["exception"], report.OutcomeCounts["compile_error"], report.OutcomeCounts["missing"]); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "\n## Top Implementation Candidates"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| Contract | Probe | Owner | Outcome | Risk | Next |"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, "| --- | --- | --- | --- | --- | --- |"); err != nil {
		return err
	}
	for _, c := range report.TopImplementation {
		if _, err := fmt.Fprintf(w, "| `%s` | `%s` | %s | `%s` | `%s` | %s |\n", c.ContractID, c.ProbeID, c.Owner, c.Outcome, c.OddityRisk, c.NextAction); err != nil {
			return err
		}
	}
	return nil
}

func writeStubBehaviorOutput(w io.Writer, report capability.StubBehaviorReport, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return capability.WriteStubBehaviorJSON(w, report)
	}
	return capability.WriteStubBehaviorMarkdown(w, report)
}

func runCompatStubInventory(args []string, w io.Writer) error {
	sourceRoot := filepath.Join("example-projects", "stubs")
	outputPath := ""
	checkPath := ""
	jsonOut := false
	usage := "usage: glade compat stub-inventory [--source <dir>] [--json|--output <path>|--check <path>]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			sourceRoot = args[i]
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	report, err := capability.BuildStubInventoryReport(sourceRoot)
	if err != nil {
		return err
	}
	switch {
	case jsonOut:
		return capability.WriteStubInventoryJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := writeStubInventoryOutput(&buf, report, outputPath); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := writeStubInventoryOutput(&buf, report, checkPath); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("stub inventory drift: run `glade compat stub-inventory --output %s`", checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		fmt.Fprintf(w, "systemStubClasses: %d\n", report.Source.SystemStubClasses)
		fmt.Fprintf(w, "generatedPlatformTypes: %d\n", report.Generated.PlatformTypes)
		fmt.Fprintf(w, "sobjectStubClasses: %d\n", report.Source.SObjectStubClasses)
		fmt.Fprintf(w, "activeStandardObjects: %d\n", report.Active.StandardObjects)
		fmt.Fprintf(w, "systemSourceMissingGeneratedTypeCount: %d\n", report.Gaps.SystemSourceMissingGeneratedTypeCount)
		fmt.Fprintf(w, "sobjectSourceMissingActiveCount: %d\n", report.Gaps.SObjectSourceMissingActiveCount)
		return nil
	}
}

func writeStubInventoryOutput(w io.Writer, report capability.StubInventoryReport, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return capability.WriteStubInventoryJSON(w, report)
	}
	return capability.WriteStubInventoryMarkdown(w, report)
}

func loadSalesforceCoverageCatalog(source, inventoryPath, catalogPath string) (capability.Catalog, error) {
	switch {
	case catalogPath != "":
		return capability.ReadCatalog(catalogPath)
	case inventoryPath != "":
		inv, err := apexdocs.ReadInventory(inventoryPath)
		if err != nil {
			return capability.Catalog{}, err
		}
		return capability.BuildCatalog(inv), nil
	default:
		inv, err := apexdocs.BuildInventory(source)
		if err != nil {
			return capability.Catalog{}, err
		}
		return capability.BuildCatalog(inv), nil
	}
}

func defaultSalesforceDocsSource() string {
	return "/Users/matt/Downloads/Kimi_Agent_Salesforce Docs Scraper (1)/salesforce-docs"
}

func defaultSalesforceToolingCompletionsSource() string {
	return filepath.Join("testdata", "generated", "tooling_system_symbols.json.gz")
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func writeCatalogSummary(w io.Writer, catalog capability.Catalog) {
	fmt.Fprintf(w, "schemaVersion: %d\n", catalog.SchemaVersion)
	fmt.Fprintf(w, "sourceDocuments: %d\n", catalog.SourceDocuments)
	fmt.Fprintf(w, "sourceMembers: %d\n", catalog.SourceMembers)
	fmt.Fprintf(w, "entries: %d\n", len(catalog.Entries))
	if len(catalog.Summary) == 0 {
		return
	}
	fmt.Fprintln(w, "summary:")
	for _, summary := range catalog.Summary {
		fmt.Fprintf(w, "  %s [%s/%s]: entries=%d documents=%d members=%d\n", summary.Area, summary.Target, summary.Status, summary.Entries, summary.Documents, summary.Members)
	}
}

func runCompatProductNamespaces(args []string, w io.Writer) error {
	source := ""
	inventoryPath := ""
	catalogPath := ""
	toolingCompletionsPath := ""
	outputPath := ""
	checkPath := ""
	jsonOut := false
	symbolsGo := false
	usage := "usage: glade compat product-namespaces [--source <dir>|--inventory <path>|--catalog <path>] [--tooling-completions <path>] [--symbols-go] [--json|--output <path>|--check <path>]"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			source = args[i]
		case "--inventory":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			inventoryPath = args[i]
		case "--catalog":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			catalogPath = args[i]
		case "--tooling-completions":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			toolingCompletionsPath = args[i]
		case "--symbols-go":
			symbolsGo = true
		case "--json":
			jsonOut = true
		case "--output":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return errors.New(usage)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	sources := 0
	for _, set := range []bool{source != "", inventoryPath != "", catalogPath != ""} {
		if set {
			sources++
		}
	}
	if sources > 1 {
		return errors.New("use only one of --source, --inventory, or --catalog")
	}
	if sources == 0 {
		source = defaultSalesforceDocsSource()
	}
	requested := 0
	for _, set := range []bool{jsonOut, outputPath != "", checkPath != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	if jsonOut && symbolsGo {
		return errors.New("use only one of --json or --symbols-go")
	}
	if symbolsGo && toolingCompletionsPath == "" {
		if defaultPath := defaultSalesforceToolingCompletionsSource(); fileExists(defaultPath) {
			toolingCompletionsPath = defaultPath
		}
	}
	catalog, err := loadSalesforceCoverageCatalog(source, inventoryPath, catalogPath)
	if err != nil {
		return err
	}
	var tooling *capability.ToolingCompletions
	if toolingCompletionsPath != "" {
		completions, err := capability.ReadToolingCompletions(toolingCompletionsPath)
		if err != nil {
			return err
		}
		tooling = &completions
	}
	if symbolsGo {
		switch {
		case outputPath != "":
			var buf strings.Builder
			if err := capability.WriteProductNamespaceSymbolsGo(&buf, catalog, tooling); err != nil {
				return err
			}
			return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
		case checkPath != "":
			var buf strings.Builder
			if err := capability.WriteProductNamespaceSymbolsGo(&buf, catalog, tooling); err != nil {
				return err
			}
			existing, err := os.ReadFile(checkPath)
			if err != nil {
				return err
			}
			if string(existing) != buf.String() {
				return fmt.Errorf("product namespace symbols drift: run `glade compat product-namespaces --symbols-go --output %s`", checkPath)
			}
			fmt.Fprintf(w, "%s: up to date\n", checkPath)
			return nil
		default:
			return capability.WriteProductNamespaceSymbolsGo(w, catalog, tooling)
		}
	}
	report := capability.BuildProductNamespaceReport(catalog)
	switch {
	case jsonOut:
		return capability.WriteProductNamespaceJSON(w, report)
	case outputPath != "":
		var buf strings.Builder
		if err := writeProductNamespaceOutput(&buf, report, outputPath); err != nil {
			return err
		}
		return os.WriteFile(outputPath, []byte(buf.String()), 0o644)
	case checkPath != "":
		var buf strings.Builder
		if err := writeProductNamespaceOutput(&buf, report, checkPath); err != nil {
			return err
		}
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != buf.String() {
			return fmt.Errorf("product namespace report drift: run `glade compat product-namespaces --catalog %s --output %s`", catalogPath, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		return capability.WriteProductNamespaceText(w, report)
	}
}

func writeProductNamespaceOutput(w io.Writer, report capability.ProductNamespaceReport, path string) error {
	if strings.EqualFold(filepath.Ext(path), ".json") {
		return capability.WriteProductNamespaceJSON(w, report)
	}
	return capability.WriteProductNamespaceMarkdown(w, report)
}

func runCompatToolingFixtures(args []string, w io.Writer) error {
	jsonOut := false
	var paths []string
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if strings.HasPrefix(arg, "-") {
				return fmt.Errorf("unknown flag %q", arg)
			}
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		return errors.New("usage: glade compat tooling-fixtures <report.json...> [--json]")
	}
	type checkedReport struct {
		Path     string `json:"path"`
		Snippets int    `json:"snippets"`
	}
	checked := make([]checkedReport, 0, len(paths))
	for _, path := range paths {
		report, err := probe.ReadToolingSnippetReport(path)
		if err != nil {
			return err
		}
		if err := probe.ValidateToolingSnippetReport(report); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		checked = append(checked, checkedReport{Path: path, Snippets: len(report.Snippets)})
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			Reports []checkedReport `json:"reports"`
		}{Reports: checked})
	}
	for _, report := range checked {
		fmt.Fprintf(w, "%s: ok (%d snippets)\n", report.Path, report.Snippets)
	}
	return nil
}

func runCompatEvidence(args []string, w io.Writer) error {
	catalogPath := ""
	jsonOut := false
	fixturePaths := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--catalog":
			i++
			if i >= len(args) {
				return errors.New("usage: glade compat evidence --catalog <path> <fixture.json...> [--json]")
			}
			catalogPath = args[i]
		case "--json":
			jsonOut = true
		default:
			fixturePaths = append(fixturePaths, args[i])
		}
	}
	if catalogPath == "" || len(fixturePaths) == 0 {
		return errors.New("usage: glade compat evidence --catalog <path> <fixture.json...> [--json]")
	}
	catalog, err := capability.ReadCatalog(catalogPath)
	if err != nil {
		return err
	}
	fixtures := make([]compat.Fixture, 0, len(fixturePaths))
	for _, path := range fixturePaths {
		fixture, err := compat.LoadFile(path)
		if err != nil {
			return err
		}
		if err := compat.Validate(fixture); err != nil {
			return err
		}
		fixtures = append(fixtures, fixture)
	}
	report := compat.BuildEvidenceReport(catalog, fixtures)
	if jsonOut {
		return compat.WriteEvidenceJSON(w, report)
	}
	writeEvidenceSummary(w, report)
	return nil
}

func writeEvidenceSummary(w io.Writer, report compat.EvidenceReport) {
	fmt.Fprintf(w, "catalogEntries: %d\n", report.CatalogEntries)
	fmt.Fprintf(w, "fixtures: %d\n", report.Fixtures)
	fmt.Fprintf(w, "evidence: %d\n", report.Evidence)
	fmt.Fprintf(w, "covered: %d\n", len(report.Covered))
	fmt.Fprintf(w, "unmatchedEvidence: %d\n", len(report.UnmatchedEvidence))
	fmt.Fprintf(w, "ungatedPromoted: %d\n", len(report.UngatedPromoted))
	if len(report.Summary) == 0 {
		return
	}
	fmt.Fprintln(w, "summary:")
	for _, summary := range report.Summary {
		fmt.Fprintf(w, "  %s [%s/%s]: covered=%d entries=%d", summary.Area, summary.Target, summary.Status, summary.Covered, summary.Entries)
		if summary.Ungated > 0 {
			fmt.Fprintf(w, " ungated=%d", summary.Ungated)
		}
		fmt.Fprintln(w)
	}
}

func runCompatGeneratedMarkdown(args []string, w io.Writer, command, label string, write func(io.Writer, capability.Report) error) error {
	return runCompatStaticMarkdown(args, w, command, label, func(w io.Writer) error {
		return write(w, capability.MVPReport())
	})
}

func runCompatStaticMarkdown(args []string, w io.Writer, command, label string, write func(io.Writer) error) error {
	outputPath := ""
	checkPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: glade compat %s [--output <path>|--check <path>]", command)
			}
			outputPath = args[i]
		case "--check":
			i++
			if i >= len(args) {
				return fmt.Errorf("usage: glade compat %s [--output <path>|--check <path>]", command)
			}
			checkPath = args[i]
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if outputPath != "" && checkPath != "" {
		return errors.New("use only one of --output or --check")
	}

	var buf strings.Builder
	if err := write(&buf); err != nil {
		return err
	}
	content := buf.String()

	switch {
	case outputPath != "":
		return os.WriteFile(outputPath, []byte(content), 0o644)
	case checkPath != "":
		existing, err := os.ReadFile(checkPath)
		if err != nil {
			return err
		}
		if string(existing) != content {
			return fmt.Errorf("%s drift: run `glade compat %s --output %s`", label, command, checkPath)
		}
		fmt.Fprintf(w, "%s: up to date\n", checkPath)
		return nil
	default:
		_, err := io.WriteString(w, content)
		return err
	}
}
