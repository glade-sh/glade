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
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/orgdescribe"
	"github.com/glade-sh/glade/internal/packageartifact"
	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

var Version = "0.0.0-dev"

type versionInfo struct {
	Version string `json:"version"`
	Go      string `json:"go"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

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
	}

	if topic, ok := helpRequestTopic(args); ok {
		printHelpTopic(stdout, topic)
		return 0
	}

	switch args[0] {
	case "version", "--version":
		if err := runVersion(args[1:], stdout); err != nil {
			_ = cliui.WriteCLIError(stderr, err)
			return 1
		}
		return 0
	case "completion":
		if err := runCompletion(args[1:], stdout); err != nil {
			_ = cliui.WriteCLIError(stderr, err)
			return 1
		}
		return 0
	case "doctor":
		if err := runDoctor(ctx, args[1:], stdout); err != nil {
			_ = cliui.WriteCLIError(stderr, err)
			return 1
		}
		return 0
	case "config":
		if err := runConfig(".", args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "init":
		if err := runConfigInit(".", args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "parse":
		result, err := runParse(ctx, args[1:], stdout, stderr)
		if err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "inspect":
		index, err := runInspect(ctx, args[1:], stdout)
		if err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		if index.HasErrors() {
			return 1
		}
		return 0
	case "schema":
		if err := runSchema(ctx, args[1:], stdout, stderr); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "check":
		result, err := runCheck(ctx, args[1:], stdout, stderr)
		if err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		if result.HasErrors() {
			return 1
		}
		return 0
	case "exec":
		if err := runExec(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "test":
		result, err := runTest(ctx, args[1:], stdout, stderr)
		if err != nil {
			writeCommandError(stderr, args[0], err)
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
			writeCommandError(stderr, args[0], err)
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
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "lsp":
		if err := runLSP(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "profile":
		if err := runProfile(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "debug":
		if err := runDebug(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "editor":
		if err := runEditor(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "dap":
		if err := runDAP(ctx, args[1:], os.Stdin, stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "package":
		if err := runPackage(ctx, args[1:], stdout, stderr); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "server":
		if err := runServer(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "playground":
		if err := runPlayground(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "db":
		if err := runDB(ctx, args[1:], stdout, stderr); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "plugins":
		if err := runPlugins(ctx, args[1:], stdout, stderr); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	default:
		if code, ok := runInstalledPluginCommand(ctx, args, stdout, stderr); ok {
			return code
		}
		message := fmt.Sprintf("unknown command %q", args[0])
		if suggestion := flagparse.Suggest(args[0], cliui.CommandNames()); suggestion != "" {
			message += fmt.Sprintf("; did you mean %q?", suggestion)
		}
		report := diagnostic.Report{
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADECLI001",
				Message:  message,
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
	case "exit-codes":
		_ = cliui.WriteExitCodesHelp(w)
	case "test":
		_ = writeTestHelp(w)
	case "schema":
		if len(args) >= 3 && args[1] == "import" && args[2] == "describe" {
			_ = writeSchemaImportDescribeHelp(w)
			return
		}
		_ = cliui.WriteCommandHelp(w, args)
	default:
		_ = cliui.WriteCommandHelp(w, args)
	}
}

func isHelpArg(arg string) bool {
	return arg == "help" || arg == "-h" || arg == "--help"
}

func writeCommandError(w io.Writer, command string, err error) {
	message := err.Error()
	if !strings.Contains(message, "did you mean") {
		if suggestion := suggestFlagForCommand(command, message); suggestion != "" {
			message += fmt.Sprintf("; did you mean %q?", suggestion)
		}
	}
	fmt.Fprintf(w, "glade: %s\n", message)
}

func suggestFlagForCommand(command, message string) string {
	if !strings.HasPrefix(message, "unknown flag ") {
		return ""
	}
	flag := quotedValue(message)
	if flag == "" {
		return ""
	}
	ref, ok := cliui.FindCommandHelp(command)
	if !ok {
		return ""
	}
	candidates := make([]string, 0, len(ref.Flags))
	for _, flagHelp := range ref.Flags {
		candidates = append(candidates, flagHelp.Name)
	}
	return flagparse.Suggest(flag, candidates)
}

func quotedValue(message string) string {
	start := strings.IndexByte(message, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexByte(message[start+1:], '"')
	if end < 0 {
		return ""
	}
	return message[start+1 : start+1+end]
}

func shellCommand(args ...string) string {
	quoted := make([]string, 0, len(args))
	for _, arg := range args {
		quoted = append(quoted, shellQuoteArg(arg))
	}
	return strings.Join(quoted, " ")
}

func shellQuoteArg(arg string) string {
	if arg == "" {
		return "''"
	}
	if !strings.ContainsAny(arg, " \t\n'\"\\$`!*?[]{}()&;<>|") {
		return arg
	}
	return "'" + strings.ReplaceAll(arg, "'", "'\\''") + "'"
}

func takeFlagValue(args []string, index *int, message string) (string, error) {
	if *index+1 >= len(args) || flagTokenMissingValue(args[*index+1]) {
		return "", errors.New(message)
	}
	*index++
	return args[*index], nil
}

func flagTokenMissingValue(value string) bool {
	return strings.HasPrefix(value, "-") && value != "-"
}

func helpRequestTopic(args []string) ([]string, bool) {
	if len(args) < 2 {
		return nil, false
	}
	if _, ok := cliui.FindCommandHelp(args[0]); !ok {
		return nil, false
	}
	for i := 1; i < len(args); i++ {
		if !isHelpArg(args[i]) {
			continue
		}
		topic := []string{args[0]}
		for _, arg := range args[1:i] {
			if strings.HasPrefix(arg, "-") {
				continue
			}
			topic = append(topic, arg)
		}
		return topic, true
	}
	return nil, false
}

func runVersion(args []string, w io.Writer) error {
	parsed, err := flagparse.New("glade version").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return err
	}
	if parsed.Bool("json") {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(versionInfo{
			Version: Version,
			Go:      runtime.Version(),
			OS:      runtime.GOOS,
			Arch:    runtime.GOARCH,
		})
	}
	fmt.Fprintf(w, "glade %s\n", Version)
	return nil
}

func runPackage(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: glade package build|info|validate|diff ...")
	}

	switch args[0] {
	case "build":
		return runPackageBuild(ctx, args[1:], w, progressW)
	case "info":
		return runPackageInfo(args[1:], w)
	case "validate":
		return runPackageValidate(args[1:], w)
	case "diff":
		return runPackageDiff(args[1:], w)
	default:
		return errors.New("usage: glade package build|info|validate|diff ...")
	}
}

func runPackageBuild(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root := "."
	namespace := ""
	version := ""
	output := ""
	parsed, err := flagparse.New("glade package build").
		String("project", "p").
		String("namespace", "n").
		String("version", "v").
		String("output", "o").
		Bool("json", "j").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		Parse(args)
	if err != nil {
		return err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	namespace = strings.TrimSpace(parsed.String("namespace"))
	version = strings.TrimSpace(parsed.String("version"))
	output = parsed.String("output")
	jsonOut := parsed.Bool("json")
	if namespace == "" {
		return errors.New("--namespace is required")
	}
	if output == "" {
		return errors.New("--output is required")
	}
	progressMode := cliui.ProgressAuto
	switch {
	case parsed.Bool("progress-json"):
		progressMode = cliui.ProgressJSON
	case parsed.Bool("progress"):
		progressMode = cliui.ProgressLine
	case parsed.Bool("no-progress") || parsed.Bool("quiet"):
		progressMode = cliui.ProgressOff
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{
		Stderr: progressW,
		Mode:   progressMode,
	})

	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "package", Label: "Loading project"})
	p, err := project.Load(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "package failed"})
		return err
	}
	p.Namespace = namespace
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "package", Label: "Loading metadata", Current: 1, Total: 4})
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "package failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "package", Label: "Indexing package symbols", Current: 2, Total: 4})
	idx := typesys.Build(p, s)
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "package", Label: "Building artifact", Current: 3, Total: 4})
	artifact, err := packageartifact.Build(namespace, version, p, s, packageArtifactTypes(idx.Types))
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "package failed"})
		return err
	}
	if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "package failed"})
		return err
	}
	if err := packageartifact.WriteJSON(output, artifact); err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "package failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "package", Label: "Artifact written", Detail: output, Current: 4, Total: 4})
	renderer.Finish(cliui.Result{OK: true, Label: "package built"})
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

func runPackageInfo(args []string, w io.Writer) error {
	parsed, err := flagparse.New("glade package info").
		Bool("json", "j").
		AllowPositionals(true).
		Parse(args)
	if err != nil {
		return err
	}
	if len(parsed.Positionals) != 1 {
		return errors.New("usage: glade package info <artifact.json> [--json]")
	}
	artifact, err := packageartifact.ReadJSON(parsed.Positionals[0])
	if err != nil {
		return err
	}
	info := packageartifact.Inspect(artifact)
	if parsed.Bool("json") {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}
	fmt.Fprintf(w, "namespace: %s\n", info.Namespace)
	if info.Version != "" {
		fmt.Fprintf(w, "version: %s\n", info.Version)
	}
	fmt.Fprintf(w, "sourceRoot: %s\n", info.SourceRoot)
	fmt.Fprintf(w, "sourceApiVersion: %s\n", info.SourceAPIVersion)
	fmt.Fprintf(w, "apexTypes: %d\n", info.ApexTypes)
	fmt.Fprintf(w, "objects: %d\n", info.Objects)
	fmt.Fprintf(w, "customMetadataRecords: %d\n", info.CustomMetadataRecords)
	fmt.Fprintf(w, "labels: %d\n", info.Labels)
	fmt.Fprintf(w, "staticResources: %d\n", info.StaticResources)
	fmt.Fprintf(w, "sourceHash: %s\n", info.SourceHash)
	return nil
}

func runPackageValidate(args []string, w io.Writer) error {
	parsed, err := flagparse.New("glade package validate").
		Bool("json", "j").
		AllowPositionals(true).
		Parse(args)
	if err != nil {
		return err
	}
	if len(parsed.Positionals) != 1 {
		return errors.New("usage: glade package validate <artifact.json> [--json]")
	}
	artifact, err := packageartifact.ReadJSON(parsed.Positionals[0])
	if err != nil {
		return err
	}
	issues := packageartifact.Validate(artifact)
	if parsed.Bool("json") {
		out := struct {
			OK     bool     `json:"ok"`
			Issues []string `json:"issues,omitempty"`
		}{OK: len(issues) == 0, Issues: issues}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if len(issues) == 0 {
		fmt.Fprintln(w, "package artifact ok")
		return nil
	}
	for _, issue := range issues {
		fmt.Fprintf(w, "package artifact issue: %s\n", issue)
	}
	return fmt.Errorf("package artifact has %d issue(s)", len(issues))
}

func runPackageDiff(args []string, w io.Writer) error {
	parsed, err := flagparse.New("glade package diff").
		Bool("json", "j").
		AllowPositionals(true).
		Parse(args)
	if err != nil {
		return err
	}
	if len(parsed.Positionals) != 2 {
		return errors.New("usage: glade package diff <from.json> <to.json> [--json]")
	}
	from, err := packageartifact.ReadJSON(parsed.Positionals[0])
	if err != nil {
		return err
	}
	to, err := packageartifact.ReadJSON(parsed.Positionals[1])
	if err != nil {
		return err
	}
	diff := packageartifact.Compare(from, to)
	if parsed.Bool("json") {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(diff)
	}
	fmt.Fprintf(w, "changed: %t\n", diff.Changed)
	fmt.Fprintf(w, "types: +%d -%d changed=%d\n", diff.AddedTypes, diff.RemovedTypes, diff.ChangedTypes)
	fmt.Fprintf(w, "objects: +%d -%d changed=%d\n", diff.AddedObjects, diff.RemovedObjects, diff.ChangedObjects)
	if diff.SourceHashChanged {
		fmt.Fprintln(w, "sourceHash: changed")
	}
	for _, name := range diff.AddedTypeNames {
		fmt.Fprintf(w, "+ type %s\n", name)
	}
	for _, name := range diff.RemovedTypeNames {
		fmt.Fprintf(w, "- type %s\n", name)
	}
	for _, name := range diff.ChangedTypeNames {
		fmt.Fprintf(w, "~ type %s\n", name)
	}
	for _, name := range diff.AddedObjectNames {
		fmt.Fprintf(w, "+ object %s\n", name)
	}
	for _, name := range diff.RemovedObjectNames {
		fmt.Fprintf(w, "- object %s\n", name)
	}
	for _, name := range diff.ChangedObjectNames {
		fmt.Fprintf(w, "~ object %s\n", name)
	}
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
	parsed, err := flagparse.New("glade doctor").
		String("project", "p").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
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
	if parsed.Bool("json") {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
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

func runParse(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (apexast.Result, error) {
	if err := ctx.Err(); err != nil {
		return apexast.Result{}, err
	}

	parsed, err := flagparse.New("glade parse").
		Bool("json", "j").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		AllowPositionals(true).
		Parse(args)
	if err != nil {
		return apexast.Result{}, err
	}
	jsonOut := parsed.Bool("json")
	paths := parsed.Positionals
	if len(paths) == 0 {
		return apexast.Result{}, errors.New("usage: glade parse <paths...> [--json]")
	}
	progressMode := cliui.ProgressAuto
	switch {
	case parsed.Bool("progress-json"):
		progressMode = cliui.ProgressJSON
	case parsed.Bool("progress"):
		progressMode = cliui.ProgressLine
	case parsed.Bool("no-progress") || parsed.Bool("quiet"):
		progressMode = cliui.ProgressOff
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "parse", Label: "Finding Apex files"})

	files, err := expandApexPaths(paths)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "parse failed"})
		return apexast.Result{}, err
	}

	parser := apexast.NewParser()
	result := apexast.Result{Files: make([]apexast.File, 0, len(files))}
	for i, path := range files {
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "parse", Label: filepath.Base(path), Current: i + 1, Total: len(files)})
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
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseEnd, Phase: "parse", Label: "Apex parsed", Current: len(files), Total: len(files)})
	renderer.Finish(cliui.Result{OK: !result.HasErrors(), Label: "parse complete"})

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
		return typesys.Index{}, errors.New("usage: glade inspect symbols [--project <root>] [--json]")
	}
	if args[0] != "symbols" {
		if args[0] == "performance" {
			return typesys.Index{}, errors.New("performance scans are provided by the performance plugin; " +
				"run `glade plugins install @glade/performance`, then `glade performance scan --project .`")
		}
		return typesys.Index{}, errors.New("usage: glade inspect symbols [--project <root>] [--json]")
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

func runSchema(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) >= 2 && args[0] == "import" && args[1] == "describe" {
		return runSchemaImportDescribe(args[2:], w)
	}
	if len(args) == 0 || args[0] != "load" {
		return errors.New("usage: glade schema load [--project <root>] [--json] | glade schema import describe --input <describe.json> [--output <schema.json>]")
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

func runSchemaImportDescribe(args []string, w io.Writer) error {
	if len(args) > 0 && isHelpArg(args[0]) {
		return writeSchemaImportDescribeHelp(w)
	}
	input := ""
	output := ""
	parsed, err := flagparse.New("glade schema import describe").
		String("input", "").
		String("output", "").
		Parse(args)
	if err != nil {
		return err
	}
	input = parsed.String("input")
	output = parsed.String("output")
	if strings.TrimSpace(input) == "" {
		return errors.New("usage: glade schema import describe --input <describe.json> [--output <schema.json>]")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var catalog orgdescribe.Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return err
	}
	schemaData, err := json.MarshalIndent(catalog.ToSchema(), "", "  ")
	if err != nil {
		return err
	}
	schemaData = append(schemaData, '\n')
	if strings.TrimSpace(output) == "" {
		_, err := w.Write(schemaData)
		return err
	}
	return os.WriteFile(output, schemaData, 0o644)
}

func writeSchemaImportDescribeHelp(w io.Writer) error {
	_, err := fmt.Fprintln(w, strings.TrimSpace(`
Import captured Salesforce describe JSON into a local Glade schema.

Usage:
  glade schema import describe --input <describe.json> [--output <schema.json>]

Flags:
  --input <describe.json>   Captured org describe catalog JSON.
  --output <schema.json>    Write schema JSON to a file. Defaults to stdout.

Live org capture belongs in a plugin.
`))
	return err
}

func runCheck(ctx context.Context, args []string, w io.Writer, progressW io.Writer) (sema.Result, error) {
	if err := ctx.Err(); err != nil {
		return sema.Result{}, err
	}

	root, outputFormat, outputPath, progressMode, err := parseCheckFlags(args)
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

	out := w
	var file *os.File
	if outputPath != "" {
		if err := os.MkdirAll(filepath.Dir(outputPath), 0o755); err != nil {
			return result, err
		}
		file, err = os.Create(outputPath)
		if err != nil {
			return result, err
		}
		defer file.Close()
		out = file
	}

	switch outputFormat {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if out != w {
			enc = json.NewEncoder(out)
			enc.SetIndent("", "  ")
		}
		return result, enc.Encode(result)
	case "sarif":
		return result, diagnostic.Report{Diagnostics: result.Diagnostics}.WriteSARIF(out)
	case "github":
		return result, diagnostic.Report{Diagnostics: result.Diagnostics}.WriteGitHubAnnotations(out)
	}

	return result, cliui.WriteCheckResult(out, cliui.CheckResultInfo{
		ProjectRoot: result.Project.Root,
		Types:       result.Summary.Types,
		Triggers:    result.Summary.Triggers,
		Objects:     result.Summary.Objects,
		Diagnostics: result.Diagnostics,
	})
}

func parseCheckFlags(args []string) (root string, outputFormat string, outputPath string, progressMode cliui.ProgressMode, err error) {
	root = "."
	outputFormat = "text"
	progressMode = cliui.ProgressAuto
	parsed, err := flagparse.New("glade check").
		String("project", "p").
		Bool("json", "j").
		String("format", "").
		String("output", "o").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		Parse(args)
	if err != nil {
		return "", "", "", cliui.ProgressOff, err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	if parsed.Bool("json") {
		outputFormat = "json"
	}
	if parsed.String("format") != "" {
		outputFormat = strings.ToLower(strings.TrimSpace(parsed.String("format")))
	}
	switch outputFormat {
	case "text", "json", "sarif", "github":
	default:
		return "", "", "", cliui.ProgressOff, fmt.Errorf("--format must be text, json, sarif, or github")
	}
	outputPath = parsed.String("output")
	switch {
	case parsed.Bool("progress-json"):
		progressMode = cliui.ProgressJSON
	case parsed.Bool("progress"):
		progressMode = cliui.ProgressLine
	case parsed.Bool("no-progress") || parsed.Bool("quiet"):
		progressMode = cliui.ProgressOff
	}
	return root, outputFormat, outputPath, progressMode, nil
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

	debug := false
	tracePath := ""
	debugLogPath := ""
	limitMode := vm.LimitMode("")
	parsed, err := flagparse.New("glade exec").
		Bool("json", "j").
		Bool("debug", "").
		String("trace", "t").
		String("debug-log", "").
		String("limit-mode", "").
		AllowPositionals(true).
		Parse(args)
	if err != nil {
		return err
	}
	jsonOut := parsed.Bool("json")
	debug = parsed.Bool("debug")
	tracePath = parsed.String("trace")
	debugLogPath = parsed.String("debug-log")
	if parsed.String("limit-mode") != "" {
		mode, err := parseLimitMode(parsed.String("limit-mode"))
		if err != nil {
			return err
		}
		limitMode = mode
	}
	sourceParts := parsed.Positionals
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
	parsed, err := flagparse.New("glade lsp").
		String("project", "p").
		Bool("diagnostics-once", "").
		Parse(args)
	if err != nil {
		return err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	index, err := loadIndex(root)
	if err != nil {
		return err
	}
	handler := lsp.NewHandler(index)
	if parsed.Bool("diagnostics-once") {
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
	tracePath := ""
	parsed, err := flagparse.New("glade profile analyze").
		Bool("json", "j").
		AllowPositionals(true).
		Parse(args[1:])
	if err != nil {
		return err
	}
	if len(parsed.Positionals) > 1 {
		return fmt.Errorf("unexpected argument %q", parsed.Positionals[1])
	}
	if len(parsed.Positionals) == 1 {
		tracePath = parsed.Positionals[0]
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
	if parsed.Bool("json") {
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
	parsed, err := flagparse.New("glade project").
		String("project", "p").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return "", false, err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	return root, parsed.Bool("json"), nil
}

func parseProjectProgressFlags(args []string) (root string, jsonOut bool, progressMode cliui.ProgressMode, err error) {
	root = "."
	progressMode = cliui.ProgressAuto
	parsed, err := flagparse.New("glade project progress").
		String("project", "p").
		Bool("json", "j").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		Parse(args)
	if err != nil {
		return "", false, cliui.ProgressOff, err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	switch {
	case parsed.Bool("progress"):
		progressMode = cliui.ProgressLine
	case parsed.Bool("progress-json"):
		progressMode = cliui.ProgressJSON
	case parsed.Bool("no-progress") || parsed.Bool("quiet"):
		progressMode = cliui.ProgressOff
	}
	return root, parsed.Bool("json"), progressMode, nil
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
