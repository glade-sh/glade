package oaercli

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

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/apextest"
	"github.com/open-aer/oaer/internal/compat"
	"github.com/open-aer/oaer/internal/config"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/project"
	oaerschema "github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/trace"
	"github.com/open-aer/oaer/internal/typesys"
	"github.com/open-aer/oaer/internal/vm"
)

const Version = "0.0.0-dev"

// Run executes the oaer CLI and returns a process exit code.
func Run(ctx context.Context, args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	switch args[0] {
	case "help", "-h", "--help":
		printHelp(stdout)
		return 0
	case "version", "--version":
		fmt.Fprintf(stdout, "oaer %s\n", Version)
		return 0
	case "doctor":
		if err := runDoctor(ctx, stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "parse":
		result, err := runParse(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "inspect":
		index, err := runInspect(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		if index.HasErrors() {
			return 1
		}
		return 0
	case "schema":
		if err := runSchema(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "check":
		result, err := runCheck(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "exec":
		if err := runExec(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	case "test":
		result, err := runTest(ctx, args[1:], stdout)
		if err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		summary := result.Summary()
		if summary.Failed > 0 || summary.Errors > 0 {
			return 1
		}
		return 0
	case "compat":
		if err := runCompat(ctx, args[1:], stdout); err != nil {
			fmt.Fprintf(stderr, "oaer: %v\n", err)
			return 1
		}
		return 0
	default:
		report := diagnostic.Report{
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "OAERCLI001",
				Message:  fmt.Sprintf("unknown command %q", args[0]),
			}},
		}
		_ = report.WriteText(stderr)
		fmt.Fprintln(stderr)
		printHelp(stderr)
		return 2
	}
}

func printHelp(w io.Writer) {
	fmt.Fprint(w, strings.TrimSpace(`
oaer is a clean-room local Apex runtime.

Usage:
  oaer <command> [flags]

Commands:
  version   Print the oaer version.
  doctor    Print environment and project configuration status.
  parse     Parse Apex source files.
  inspect   Inspect indexed project symbols.
  schema    Load local Salesforce metadata schema.
  check     Run semantic checks over a project.
  exec      Execute anonymous Apex.
  test      Discover and run supported Apex tests.
  compat    Validate compatibility fixtures.
  help      Print this help text.
`)+"\n")
}

func runDoctor(ctx context.Context, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	cfg, cfgPath, err := config.LoadNearest(cwd)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return err
	}

	fmt.Fprintf(w, "oaer: %s\n", Version)
	fmt.Fprintf(w, "go: %s\n", runtime.Version())
	fmt.Fprintf(w, "os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Fprintf(w, "cwd: %s\n", cwd)

	if errors.Is(err, config.ErrNotFound) {
		fmt.Fprintln(w, "config: not found")
	} else {
		fmt.Fprintf(w, "config: %s\n", cfgPath)
		if cfg.Project.Root != "" {
			fmt.Fprintf(w, "project.root: %s\n", cfg.Project.Root)
		}
		if cfg.Project.DefaultNamespace != "" {
			fmt.Fprintf(w, "project.defaultNamespace: %s\n", cfg.Project.DefaultNamespace)
		}
	}

	fmt.Fprintln(w, "status: ok")
	return nil
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
		return apexast.Result{}, errors.New("usage: oaer parse <paths...> [--json]")
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
				Code:     "OAERPARSE000",
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
	if len(args) == 0 || args[0] != "symbols" {
		return typesys.Index{}, errors.New("usage: oaer inspect symbols [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return typesys.Index{}, err
	}
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := oaerschema.LoadProject(p)
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

func runSchema(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 || args[0] != "load" {
		return errors.New("usage: oaer schema load [--project <root>] [--json]")
	}

	root, jsonOut, err := parseProjectFlags(args[1:])
	if err != nil {
		return err
	}
	p, err := project.Load(root)
	if err != nil {
		return err
	}
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		return err
	}
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

func runCheck(ctx context.Context, args []string, w io.Writer) (sema.Result, error) {
	if err := ctx.Err(); err != nil {
		return sema.Result{}, err
	}

	root, jsonOut, err := parseProjectFlags(args)
	if err != nil {
		return sema.Result{}, err
	}
	index, err := loadIndex(root)
	if err != nil {
		return sema.Result{}, err
	}
	result := sema.Analyze(index)

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return result, enc.Encode(result)
	}

	fmt.Fprintf(w, "project: %s\n", result.Project.Root)
	fmt.Fprintf(w, "types: %d\n", result.Summary.Types)
	fmt.Fprintf(w, "triggers: %d\n", result.Summary.Triggers)
	fmt.Fprintf(w, "objects: %d\n", result.Summary.Objects)
	fmt.Fprintf(w, "diagnostics: %d\n", result.Summary.Diagnostics)
	if len(result.Diagnostics) > 0 {
		_ = diagnostic.Report{Diagnostics: result.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return result, nil
}

func loadIndex(root string) (typesys.Index, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, err
	}
	s, err := oaerschema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, err
	}
	return typesys.Build(p, s), nil
}

func runExec(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	jsonOut := false
	tracePath := ""
	sourceParts := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--json":
			jsonOut = true
		case "--trace":
			if i+1 >= len(args) {
				return errors.New("--trace requires a path")
			}
			tracePath = args[i+1]
			i++
		default:
			sourceParts = append(sourceParts, arg)
		}
	}
	if len(sourceParts) == 0 {
		return errors.New("usage: oaer exec [--json] [--trace <path>] '<anonymous apex>'")
	}

	program, err := vm.CompileAnonymous(strings.Join(sourceParts, " "))
	if err != nil {
		return err
	}

	stdout := w
	if jsonOut {
		stdout = nil
	}
	result, err := vm.Execute(program, stdout)
	if err != nil {
		return err
	}
	if tracePath != "" {
		if err := writeTraceFile(tracePath, result.Trace); err != nil {
			return err
		}
	}

	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}

	return nil
}

func runTest(ctx context.Context, args []string, w io.Writer) (testreport.Run, error) {
	if err := ctx.Err(); err != nil {
		return testreport.Run{}, err
	}

	root := "."
	filter := ""
	format := "console"
	junitPath := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--project":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--project requires a value")
			}
			root = args[i+1]
			i++
		case "--filter":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--filter requires a value")
			}
			filter = args[i+1]
			i++
		case "--json":
			format = "json"
		case "--junit":
			if i+1 >= len(args) {
				return testreport.Run{}, errors.New("--junit requires a path")
			}
			junitPath = args[i+1]
			i++
		default:
			return testreport.Run{}, fmt.Errorf("unknown flag %q", args[i])
		}
	}

	index, err := loadIndex(root)
	if err != nil {
		return testreport.Run{}, err
	}
	result := apextest.Run(index, apextest.Options{Filter: filter})
	if junitPath != "" {
		if err := writeJUnitFile(junitPath, result); err != nil {
			return result, err
		}
	}
	switch format {
	case "json":
		return result, testreport.WriteJSON(w, result)
	default:
		return result, testreport.WriteConsole(w, result)
	}
}

func writeJUnitFile(path string, result testreport.Run) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return testreport.WriteJUnitXML(file, result)
}

func writeTraceFile(path string, events []trace.Event) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	return trace.WriteJSON(file, trace.NewDocument(events))
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

func runCompat(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) < 2 || (args[0] != "validate" && args[0] != "run") {
		return errors.New("usage: oaer compat validate|run <fixture.json...>")
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
