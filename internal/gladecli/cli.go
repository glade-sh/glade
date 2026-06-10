package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/perfscan"
	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var Version = "0.0.0-dev"

// Run executes the glade CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		_ = cliui.WriteHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		if len(args) > 1 {
			printHelpTopic(stdout, args[1:])
			return 0
		}
		_ = cliui.WriteHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "glade %s\n", Version)
		return 0
	case "doctor":
		if err := runDoctor(ctx, args[1:], stdout); err != nil {
			_ = cliui.WriteCLIError(stderr, err)
			return 1
		}
		return 0
	case "parse":
		result, err := runParse(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "inspect":
		index, err := runInspect(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if index.HasErrors() {
			return 1
		}
		return 0
	case "schema":
		if err := runSchema(ctx, args[1:], stdout, stderr); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "check":
		result, err := runCheck(ctx, args[1:], stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "exec":
		if err := runExec(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "test":
		result, err := runTest(ctx, args[1:], stdout, stderr)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		summary := result.Summary()
		if summary.Failed > 0 || summary.Errors > 0 {
			return 1
		}
		return 0
	case "dev":
		result, ranTests, err := runDev(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		if ranTests {
			summary := result.Summary()
			if summary.Failed > 0 || summary.Errors > 0 {
				return 1
			}
		}
		return 0
	case "report":
		if err := runReport(args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "lsp":
		if err := runLSP(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "profile":
		if err := runProfile(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "debug":
		if err := runDebug(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "editor":
		if err := runEditor(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "dap":
		if err := runDAP(ctx, args[1:], os.Stdin, stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "package":
		if err := runPackage(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "server":
		if err := runServer(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "playground":
		if err := runPlayground(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	case "db":
		if err := runDB(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "glade: %v\n", err)
			return 1
		}
		return 0
	default:
		report := diagnostic.Report{
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADECLI001",
				Message:  fmt.Sprintf("unknown command %q", args[0]),
			}},
		}
		_ = report.WriteText(stderr)
		fmt.Fprintln(stderr)
		_ = cliui.WriteHelp(stderr)
		return 2
	}
}

func printHelpTopic(w io.Writer, args []string) {
	if len(args) == 0 {
		_ = cliui.WriteHelp(w)
		return
	}
	switch args[0] {
	case "test":
		_ = cliui.WriteTestHelp(w)
	default:
		_ = cliui.WriteHelp(w)
	}
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func runPackage(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "build" {
		return errors.New("usage: glade package build --project <root> --namespace <namespace> --output <artifact> [--version <version>] [--json]")
	}

	root := "."
	namespace := ""
	version := ""
	output := ""
	jsonOut := false
	for i := 1; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--namespace":
			if i+1 >= len(args) {
				return errors.New("--namespace requires a value")
			}
			namespace = strings.TrimSpace(args[i+1])
			i++
		case "--version":
			if i+1 >= len(args) {
				return errors.New("--version requires a value")
			}
			version = strings.TrimSpace(args[i+1])
			i++
		case "--output":
			if i+1 >= len(args) {
				return errors.New("--output requires a value")
			}
			output = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if namespace == "" {
		return errors.New("--namespace is required")
	}
	if output == "" {
		return errors.New("--output is required")
	}

	p, err := project.Load(root)
	if err != nil {
		return err
	}
	p.Namespace = namespace
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return err
	}
	idx := typesys.Build(p, s)
	artifact, err := packageartifact.Build(namespace, version, p, s, packageArtifactTypes(idx.Types))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		return err
	}
	if err := packageartifact.WriteJSON(output, artifact); err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(artifact)
	}
	fmt.Fprintf(w, "package artifact: %s\n", output)
	fmt.Fprintf(w, "namespace: %s\n", artifact.Namespace)
	if artifact.Version != "" {
		fmt.Fprintf(w, "version: %s\n", artifact.Version)
	}
	fmt.Fprintf(w, "apexTypes: %d\n", len(artifact.ApexTypes))
	fmt.Fprintf(w, "objects: %d\n", len(artifact.Objects))
	fmt.Fprintf(w, "sourceHash: %s\n", artifact.SourceHash)
	return nil
}

func packageArtifactTypes(types []typesys.TypeSymbol) []packageartifact.ApexType {
	out := make([]packageartifact.ApexType, 0, len(types))
	for _, typ := range types {
		out = append(out, packageartifact.ApexType{
			Kind:       typ.Kind,
			Name:       typ.Name,
			File:       typ.File,
			Namespace:  typ.Namespace,
			SourceRoot: typ.SourceRoot,
			Version:    typ.Version,
			Dependency: typ.Dependency,
			Modifiers:  append([]string(nil), typ.Modifiers...),
			IsTest:     typ.IsTest,
			SuperClass: typ.SuperClass,
			Interfaces: append([]string(nil), typ.Interfaces...),
			Range:      typ.Range,
			Members:    packageArtifactMembers(typ.Members),
		})
	}
	return out
}

func packageArtifactMembers(members []typesys.MemberSymbol) []packageartifact.ApexMember {
	out := make([]packageartifact.ApexMember, 0, len(members))
	for _, member := range members {
		out = append(out, packageartifact.ApexMember{
			Kind:       member.Kind,
			Name:       member.Name,
			Type:       member.Type,
			Modifiers:  append([]string(nil), member.Modifiers...),
			Parameters: append([]apexast.Parameter(nil), member.Parameters...),
			Accessors:  append([]apexast.Accessor(nil), member.Accessors...),
			IsTest:     member.IsTest,
			Range:      member.Range,
		})
	}
	return out
}

func runDoctor(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	root := "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
	if info, err := os.Stat(root); err != nil {
		return fmt.Errorf("project root %q: %w", root, err)
	} else if !info.IsDir() {
		return fmt.Errorf("project root %q is not a directory", root)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, cfgPath, err := config.LoadNearest(root)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}

	parserStatus := parserSelfCheck()
	info := cliui.DoctorInfo{
		Version:      Version,
		GoVersion:    runtime.Version(),
		OSArch:       runtime.GOOS + "/" + runtime.GOARCH,
		CWD:          cwd,
		ParserStatus: parserStatus,
		ParserOK:     cliui.ParserStatusOK(parserStatus),
	}
	if errors.Is(err, config.ErrNotFound) {
		info.ConfigMissing = true
	} else {
		info.ConfigPath = cfgPath
		info.ProjectRoot = cfg.Project.Root
		info.DefaultNamespace = cfg.Project.DefaultNamespace
	}
	return cliui.WriteDoctor(w, info)
}

// parserSelfCheck parses a trivial Apex class and reports whether the bundled
// tree-sitter parser is available. Binaries built without CGO ship a stub that
// emits APEXPARSECGO and cannot parse project sources; surfacing this in doctor
// makes a broken distribution obvious right away.
func parserSelfCheck() string {
	file := apexast.NewParser().ParseSource("doctor.cls", "public class GladeDoctor {}")
	for _, diag := range file.Diagnostics {
		if diag.Code == "APEXPARSECGO" {
			return "UNAVAILABLE (binary built without CGO; check/test/parse on project sources will fail)"
		}
	}
	if file.Kind == apexast.FileKindClass || len(file.Declarations) > 0 {
		return "ok (tree-sitter)"
	}
	return "UNAVAILABLE (could not parse a trivial class)"
}

func runParse(ctx context.Context, args []string, w io.Writer) (apexast.Result, error) {
	if err := ctx.Err(); err != nil {
		return apexast.Result{}, err
	}

	jsonOut := false
	paths := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		return apexast.Result{}, errors.New("usage: glade parse <paths...> [--json]")
	}

	files, err := expandApexPaths(paths)
	if err != nil {
		return apexast.Result{}, err
	}

	parser := apexast.NewParser()
	result := apexast.Result{Files: make([]apexast.File, 0, len(files))}
	for _, path := range files {
		file, err := parser.ParseFile(path)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "GLADEPARSE000",
				Message:  err.Error(),
				File:     path,
			})
			continue
		}
		result.Files = append(result.Files, file)
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return result, enc.Encode(result)
	}

	for _, file := range result.Files {
		if len(file.Diagnostics) > 0 {
			_ = diagnostic.Report{Diagnostics: file.Diagnostics}.WriteText(w)
			fmt.Fprintln(w)
			continue
		}
		fmt.Fprintf(w, "%s: %s", file.Path, file.Kind)
		if len(file.Declarations) > 0 {
			fmt.Fprintf(w, " %s", file.Declarations[0].Name)
		}
		fmt.Fprintln(w)
	}
	if len(result.Diagnostics) > 0 {
		_ = diagnostic.Report{Diagnostics: result.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return result, nil
}

func runInspect(ctx context.Context, args []string, w io.Writer) (typesys.Index, error) {
	if err := ctx.Err(); err != nil {
		return typesys.Index{}, err
	}
	if len(args) == 0 {
		return typesys.Index{}, errors.New("usage: glade inspect symbols|performance [--project <root>] [--json]")
	}
	if args[0] == "performance" {
		if err := runInspectPerformance(ctx, args[1:], w); err != nil {
			return typesys.Index{}, err
		}
		return typesys.Index{}, nil
	}
	if args[0] != "symbols" {
		return typesys.Index{}, errors.New("usage: glade inspect symbols|performance [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return typesys.Index{}, err
	}
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, err
	}
	index := typesys.Build(p, s)

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return index, enc.Encode(index)
	}

	fmt.Fprintf(w, "project: %s\n", index.Project.Root)
	if index.Project.Namespace != "" {
		fmt.Fprintf(w, "namespace: %s\n", index.Project.Namespace)
	}
	fmt.Fprintf(w, "types: %d\n", len(index.Types))
	fmt.Fprintf(w, "triggers: %d\n", len(index.Triggers))
	fmt.Fprintf(w, "objects: %d\n", len(index.Objects))
	for _, typ := range index.Types {
		fmt.Fprintf(w, "%s %s %s\n", typ.Kind, typ.Name, typ.File)
		for _, member := range typ.Members {
			fmt.Fprintf(w, "  %s %s", member.Kind, member.Name)
			if member.Type != "" {
				fmt.Fprintf(w, " %s", member.Type)
			}
			if member.IsTest {
				fmt.Fprint(w, " @isTest")
			}
			fmt.Fprintln(w)
		}
	}
	for _, trigger := range index.Triggers {
		fmt.Fprintf(w, "trigger %s on %s %s\n", trigger.Name, trigger.ObjectName, trigger.File)
	}
	for _, object := range index.Objects {
		fmt.Fprintf(w, "sobject %s fields=%d\n", object.Name, len(object.Fields))
		for _, field := range object.Fields {
			fmt.Fprintf(w, "  field %s %s\n", field.Name, field.Type)
		}
	}
	if len(index.Diagnostics) > 0 {
		_ = diagnostic.Report{Diagnostics: index.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return index, nil
}

func runInspectPerformance(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	jsonOut := false
	tracePath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--json":
			jsonOut = true
		case "--trace":
			if i+1 >= len(args) {
				return errors.New("--trace requires a value")
			}
			tracePath = args[i+1]
			i++
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}

	report, err := perfscan.AnalyzeProject(perfscan.Options{ProjectRoot: root, TracePath: tracePath})
	if err != nil {
		return err
	}
	if jsonOut {
		return perfscan.WriteJSON(w, report)
	}
	return perfscan.WriteMarkdown(w, report)
}

func runSchema(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "load" {
		return errors.New("usage: glade schema load [--project <root>] [--json]")
	}

	root, jsonOut, progressMode, err := parseProjectProgressFlags(args[1:])
	if err != nil {
		return err
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	})
	renderer.Render(cliui.Event{
		Kind:  cliui.EventPhaseStart,
		Phase: "schema",
		Label: "Loading project",
	})
	p, err := project.Load(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "schema failed"})
		return err
	}

	renderer.Render(cliui.Event{
		Kind:    cliui.EventPhaseTick,
		Phase:   "schema",
		Label:   "Loading metadata",
		Current: 1,
		Total:   2,
	})
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "schema failed"})
		return err
	}
	renderer.Render(cliui.Event{
		Kind:    cliui.EventPhaseEnd,
		Phase:   "schema",
		Label:   "Metadata loaded",
		Current: 2,
		Total:   2,
	})
	renderer.Finish(cliui.Result{OK: true, Label: "schema loaded"})
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(s)
	}

	fmt.Fprintf(w, "objects: %d\n", len(s.Objects))
	for _, object := range s.Objects {
		fmt.Fprintf(w, "%s fields=%d\n", object.Name, len(object.Fields))
	}
	return nil
}

func runCheck(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (sema.Result, error) {
	if err := ctx.Err(); err != nil {
		return sema.Result{}, err
	}

	root, jsonOut, progressMode, err := parseProjectProgressFlags(args)
	if err != nil {
		return sema.Result{}, err
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	})
	renderer.Render(cliui.Event{
		Kind:  cliui.EventPhaseStart,
		Phase: "check",
		Label: "Loading project",
	})
	p, err := project.Load(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "check failed"})
		return sema.Result{}, err
	}
	renderer.Render(cliui.Event{
		Kind:    cliui.EventPhaseTick,
		Phase:   "check",
		Label:   "Loading metadata",
		Current: 1,
		Total:   4,
	})
	s, err := gladeschema.LoadProject(p)
	var index typesys.Index
	if err != nil {
		index = typesys.Build(p, gladeschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		})
	} else {
		renderer.Render(cliui.Event{
			Kind:    cliui.EventPhaseTick,
			Phase:   "check",
			Label:   "Indexing Apex symbols",
			Current: 2,
			Total:   4,
		})
		index = typesys.Build(p, s)
	}
	renderer.Render(cliui.Event{
		Kind:    cliui.EventPhaseTick,
		Phase:   "check",
		Label:   "Running semantic checks",
		Current: 3,
		Total:   4,
	})
	result := sema.Analyze(index)
	renderer.Render(cliui.Event{
		Kind:    cliui.EventPhaseEnd,
		Phase:   "check",
		Label:   "Semantic checks complete",
		Current: 4,
		Total:   4,
	})
	renderer.Finish(cliui.Result{
		OK:    !result.HasErrors(),
		Label: "check complete",
	})

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return result, enc.Encode(result)
	}

	return result, cliui.WriteCheckResult(w, cliui.CheckResultInfo{
		ProjectRoot: result.Project.Root,
		Types:       result.Summary.Types,
		Triggers:    result.Summary.Triggers,
		Objects:     result.Summary.Objects,
		Diagnostics: result.Diagnostics,
	})
}

func loadIndex(root string) (typesys.Index, error) {
	_, index, err := loadProjectIndex(root)
	if err != nil {
		return typesys.Index{}, err
	}
	return index, nil
}

func loadProjectIndex(root string) (project.Project, typesys.Index, error) {
	p, err := project.Load(root)
	if err != nil {
		return project.Project{}, typesys.Index{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		index := typesys.Build(p, gladeschema.Schema{})
		index.Diagnostics = append(index.Diagnostics, diagnostic.Diagnostic{
			Severity: diagnostic.Error,
			Code:     "GLADESCHEMA001",
			Message:  fmt.Sprintf("metadata schema load failed: %v", err),
		})
		return p, index, nil
	}
	return p, typesys.Build(p, s), nil
}

func runExec(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	jsonOut := false
	debug := false
	tracePath := ""
	debugLogPath := ""
	limitMode := vm.LimitMode("")
	sourceParts := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			jsonOut = true
		case "--debug":
			debug = true
		case "--trace":
			if i+1 >= len(args) {
				return errors.New("--trace requires a path")
			}
			tracePath = args[i+1]
			i++
		case "--debug-log":
			if i+1 >= len(args) {
				return errors.New("--debug-log requires a path (use - for stdout)")
			}
			debugLogPath = args[i+1]
			i++
		case "--limit-mode":
			if i+1 >= len(args) {
				return errors.New("--limit-mode requires a value")
			}
			mode, err := parseLimitMode(args[i+1])
			if err != nil {
				return err
			}
			limitMode = mode
			i++
		default:
			sourceParts = append(sourceParts, arg)
		}
	}
	if len(sourceParts) == 0 {
		return errors.New("usage: glade exec [--json] [--trace <path>] [--debug-log <path>] '<anonymous apex>'")
	}

	program, err := vm.CompileAnonymous(strings.Join(sourceParts, " "))
	if err != nil {
		return err
	}

	stdout := w
	if jsonOut || debugLogPath != "" {
		stdout = nil
	}
	machine := vm.New(stdout)
	machine.SetTraceEnabled(tracePath != "" || debug || jsonOut || debugLogPath != "")
	if limitMode != "" {
		machine.SetLimitMode(limitMode)
	}
	result, execErr := machine.Execute(program)
	if debugLogPath != "" {
		log := apexlog.Format(&result, execErr, apexlog.Options{})
		if err := writeDebugLog(debugLogPath, log, w); err != nil {
			return err
		}
	}
	if execErr != nil {
		return execErr
	}
	if tracePath != "" {
		if err := writeTraceFile(tracePath, result.Trace); err != nil {
			return err
		}
	}
	if debug {
		return serveDAPSnapshot(dap.NewSnapshot(result.Trace, result.Vars), w)
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return nil
}

func serveDAPSnapshot(snapshot dap.Snapshot, w io.Writer) error {
	if file, ok := w.(*os.File); ok && file.Fd() == os.Stdout.Fd() {
		return dap.Serve(os.Stdin, w, dap.NewHandler(snapshot))
	}
	return dap.Write(w, dap.NewHandler(snapshot).Handle(dap.Request{Seq: 1, Type: dap.MessageTypeRequest, Command: dap.CommandInitialize})[0])
}

func parseLimitMode(raw string) (vm.LimitMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "permissive":
		return vm.LimitModePermissive, nil
	case "strict":
		return vm.LimitModeStrict, nil
	default:
		return "", fmt.Errorf("unsupported limit mode %q", raw)
	}
}

func runLSP(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	diagnosticsOnce := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--diagnostics-once":
			diagnosticsOnce = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	index, err := loadIndex(root)
	if err != nil {
		return err
	}
	handler := lsp.NewHandler(index)
	if diagnosticsOnce {
		for _, notification := range handler.PublishDiagnostics(sema.Analyze(index).Diagnostics) {
			if err := lsp.WriteMessage(w, notification); err != nil {
				return err
			}
		}
		return nil
	}
	return lsp.Serve(os.Stdin, w, handler)
}

func runProfile(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "analyze" {
		return errors.New("usage: glade profile analyze <trace.json> [--json]")
	}
	jsonOut := false
	tracePath := ""
	for _, arg := range args[1:] {
		switch arg {
		case "--json":
			jsonOut = true
		default:
			if tracePath != "" {
				return fmt.Errorf("unexpected argument %q", arg)
			}
			tracePath = arg
		}
	}
	if tracePath == "" {
		return errors.New("usage: glade profile analyze <trace.json> [--json]")
	}
	file, err := os.Open(tracePath)
	if err != nil {
		return err
	}
	defer file.Close()
	doc, err := profile.ReadTrace(file)
	if err != nil {
		return err
	}
	report := profile.Analyze(doc)
	if jsonOut {
		return profile.WriteJSON(w, report)
	}
	return profile.WriteMarkdown(w, report)
}

func storageBaseline() storage.OrgState {
	org := storage.NewOrgState()
	org.APIVersion = "61.0"
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Type: storage.FieldString},
			},
		},
		Records: make(map[storage.ID]storage.Record),
	}
	storage.EnsureDeterministicPlatformData(&org)
	return org
}

func parseProjectFlags(args []string) (root string, jsonOut bool, err error) {
	root = "."
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--project":
			if i+1 >= len(args) {
				return "", false, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		default:
			return "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return root, jsonOut, nil
}

func parseProjectProgressFlags(args []string) (root string, jsonOut bool, progressMode cliui.ProgressMode, err error) {
	root = "."
	progressMode = cliui.ProgressAuto
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOut = true
		case "--progress":
			progressMode = cliui.ProgressLine
		case "--progress-json":
			progressMode = cliui.ProgressJSON
		case "--no-progress", "--quiet":
			progressMode = cliui.ProgressOff
		case "--project":
			if i+1 >= len(args) {
				return "", false, cliui.ProgressOff, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		default:
			return "", false, cliui.ProgressOff, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return root, jsonOut, progressMode, nil
}

func expandApexPaths(paths []string) ([]string, error) {
	var files []string
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			files = append(files, path)
			continue
		}
		err = filepath.WalkDir(path, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(path)) {
			case ".cls", ".trigger":
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return files, nil
}
