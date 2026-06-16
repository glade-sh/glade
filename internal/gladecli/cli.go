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
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/config"
	"github.com/glade-sh/glade/internal/dap"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/gladehome"
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

type cliJSONEnvelope struct {
	SchemaVersion string   `json:"schemaVersion"`
	Command       string   `json:"command"`
	Status        string   `json:"status"`
	ExitCode      int      `json:"exitCode"`
	Project       any      `json:"project,omitempty"`
	Summary       any      `json:"summary,omitempty"`
	Diagnostics   any      `json:"diagnostics,omitempty"`
	Artifacts     []any    `json:"artifacts,omitempty"`
	Timings       any      `json:"timings,omitempty"`
	Suggestions   []string `json:"suggestions,omitempty"`
	Tests         any      `json:"tests,omitempty"`
	Data          any      `json:"data,omitempty"`
}

func writeCLIJSONEnvelope(w io.Writer, env cliJSONEnvelope) error {
	env.SchemaVersion = "1.0"
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(env)
}

func statusForOK(ok bool) string {
	if ok {
		return "passed"
	}
	return "failed"
}

func exitCodeForOK(ok bool) int {
	if ok {
		return 0
	}
	return 1
}

func progressModeForFlags(jsonOut, progress, progressJSON, noProgress bool) cliui.ProgressMode {
	switch {
	case progressJSON:
		return cliui.ProgressJSON
	case progress:
		return cliui.ProgressLine
	case noProgress || jsonOut:
		return cliui.ProgressOff
	default:
		return cliui.ProgressAuto
	}
}

func diagnosticsJSON(projectRoot string, diags []diagnostic.Diagnostic) []map[string]any {
	out := []map[string]any{}
	for _, diag := range diags {
		row := map[string]any{
			"severity": diag.Severity,
			"message":  diag.Message,
		}
		if diag.Code != "" {
			row["code"] = diag.Code
			row["docs"] = "https://glade.sh/guide/errors#" + strings.ToLower(diag.Code)
		}
		if diag.File != "" {
			row["file"] = cliui.ProjectRelativePath(projectRoot, diag.File)
		}
		if diag.Range != nil {
			row["line"] = diag.Range.Start.Line
			row["column"] = diag.Range.Start.Column
			row["range"] = diag.Range
		}
		if why := diagnosticWhy(diag.Code); why != "" {
			row["why"] = why
		}
		if try := diagnosticTry(diag.Code); len(try) > 0 {
			row["try"] = try
		}
		out = append(out, row)
	}
	return out
}

func diagnosticWhy(code string) string {
	switch code {
	case "GLADESEMA002":
		return "The Apex type reference is not present in local Apex, schema, or platform symbols."
	case "GLADESCHEMA001":
		return "Glade could not load local Salesforce metadata for this project."
	default:
		return ""
	}
}

func diagnosticTry(code string) []string {
	switch code {
	case "GLADESEMA002", "GLADESCHEMA001":
		return []string{"glade schema load --project .", "glade check --project ."}
	default:
		return nil
	}
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
	case "examples":
		if err := runExamples(args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "explain":
		if len(args) != 2 {
			writeCommandError(stderr, args[0], errors.New("usage: glade explain <error-code>"))
			return 1
		}
		if err := cliui.WriteErrorCodeHelp(stdout, args[1]); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "support":
		if len(args) > 1 {
			writeCommandError(stderr, args[0], errors.New("usage: glade support"))
			return 1
		}
		_ = cliui.WriteSupportHelp(stdout)
		return 0
	case "doctor":
		code, err := runDoctor(ctx, args[1:], stdout)
		if err != nil {
			_ = cliui.WriteCLIError(stderr, err)
			return 1
		}
		return code
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
	case "toolchain":
		if err := runToolchain(ctx, args[1:], stdout); err != nil {
			_ = cliui.WriteCLIError(stderr, err)
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
		if err := runReport(ctx, args[1:], stdout); err != nil {
			writeCommandError(stderr, args[0], err)
			return 1
		}
		return 0
	case "refactor":
		if err := runRefactor(args[1:], stdout); err != nil {
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
	case "org":
		if err := runOrg(ctx, args[1:], stdout); err != nil {
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
	case "commands":
		_ = cliui.WriteCommandsHelp(w)
	case "workflows":
		_ = cliui.WriteWorkflowsHelp(w)
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
	case "dev":
		if len(args) >= 2 && args[1] == "vf" {
			printDevVFHelp(w)
			return
		}
		if len(args) >= 2 && args[1] == "lwc" {
			printDevLWCHelp(w)
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
	progressMode := progressModeForFlags(jsonOut, parsed.Bool("progress"), parsed.Bool("progress-json"), parsed.Bool("no-progress") || parsed.Bool("quiet"))
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

func runDoctor(ctx context.Context, args []string, w io.Writer) (int, error) {
	if err := ctx.Err(); err != nil {
		return 1, err
	}

	root := "."
	parsed, err := flagparse.New("glade doctor").
		String("project", "p").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return 1, err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	if info, err := os.Stat(root); err != nil {
		return 1, fmt.Errorf("project root %q: %w", root, err)
	} else if !info.IsDir() {
		return 1, fmt.Errorf("project root %q is not a directory", root)
	}

	cwd, err := os.Getwd()
	if err != nil {
		return 1, err
	}

	cfg, cfgPath, err := config.LoadNearest(root)
	if err != nil && !errors.Is(err, config.ErrNotFound) {
		return 1, err
	}

	parserStatus := parserSelfCheck()
	toolchainPath, toolchainOK, toolchainDetail := gladehome.ToolchainStatus()
	info := cliui.DoctorInfo{
		SchemaVersion:   "1.0",
		Command:         "doctor",
		Version:         Version,
		GoVersion:       runtime.Version(),
		OSArch:          runtime.GOOS + "/" + runtime.GOARCH,
		CWD:             cwd,
		ParserStatus:    parserStatus,
		ParserOK:        cliui.ParserStatusOK(parserStatus),
		ToolchainPath:   toolchainPath,
		ToolchainStatus: toolchainDetail,
		ToolchainOK:     toolchainOK,
	}
	if errors.Is(err, config.ErrNotFound) {
		info.ConfigMissing = true
	} else {
		info.ConfigPath = cfgPath
		info.ProjectRoot = cfg.Project.Root
		info.DefaultNamespace = cfg.Project.DefaultNamespace
	}
	ok := info.ParserOK && info.ToolchainOK && !info.ConfigMissing
	info.Status = statusForOK(ok)
	info.ExitCode = exitCodeForOK(ok)
	info.Suggestions = []string{"glade check", "glade test changed --since origin/main", "glade playground --examples --open"}
	if parsed.Bool("json") {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return info.ExitCode, enc.Encode(info)
	}
	return info.ExitCode, cliui.WriteDoctor(w, info)
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
	progressMode := progressModeForFlags(jsonOut, parsed.Bool("progress"), parsed.Bool("progress-json"), parsed.Bool("no-progress") || parsed.Bool("quiet"))
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
		return typesys.Index{}, errors.New(inspectUsage())
	}
	if args[0] == "graph" {
		return typesys.Index{}, runInspectGraph(ctx, args[1:], w)
	}
	if args[0] == "definition" {
		return typesys.Index{}, runInspectDefinition(ctx, args[1:], w)
	}
	if args[0] == "references" {
		return typesys.Index{}, runInspectReferences(ctx, args[1:], w)
	}
	if args[0] != "symbols" {
		if args[0] == "performance" {
			return typesys.Index{}, errors.New("performance scans are provided by the performance plugin; " +
				"run `glade plugins install @glade/performance`, then `glade performance scan --project .`")
		}
		return typesys.Index{}, errors.New(inspectUsage())
	}

	root, jsonOut, fullPaths, kindFilter, err := parseInspectSymbolsFlags(args[1:])
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
		return index, writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:     "inspect symbols",
			Status:      statusForOK(!index.HasErrors()),
			ExitCode:    exitCodeForOK(!index.HasErrors()),
			Project:     index.Project,
			Summary:     inspectSymbolsSummary(index),
			Diagnostics: diagnosticsJSON(index.Project.Root, index.Diagnostics),
			Data:        index,
			Suggestions: []string{"glade check --project .", "glade inspect graph --project ."},
		})
	}

	fmt.Fprintln(w, "Project symbols")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Summary:")
	fmt.Fprintf(w, "  Apex types   %d\n", len(index.Types))
	fmt.Fprintf(w, "  Triggers     %d\n", len(index.Triggers))
	fmt.Fprintf(w, "  Objects      %d\n", len(index.Objects))
	fmt.Fprintf(w, "  Fields       %d\n", countIndexFields(index))
	if index.Project.Namespace != "" {
		fmt.Fprintf(w, "  Namespace    %s\n", index.Project.Namespace)
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Symbols:")
	fmt.Fprintln(w, "  Kind     Name             File")
	for _, typ := range index.Types {
		if kindFilter != "" && kindFilter != string(typ.Kind) {
			continue
		}
		file := typ.File
		if !fullPaths {
			file = cliui.ProjectRelativePath(index.Project.Root, file)
		}
		fmt.Fprintf(w, "  %-8s %-16s %s\n", typ.Kind, typ.Name, file)
	}
	for _, trigger := range index.Triggers {
		if kindFilter != "" && kindFilter != "trigger" {
			continue
		}
		file := trigger.File
		if !fullPaths {
			file = cliui.ProjectRelativePath(index.Project.Root, file)
		}
		fmt.Fprintf(w, "  %-8s %-16s %s\n", "trigger", trigger.Name, file)
	}
	for _, object := range index.Objects {
		if kindFilter != "" && kindFilter != "object" && kindFilter != "sobject" {
			continue
		}
		fmt.Fprintf(w, "  %-8s %-16s %s\n", "object", object.Name, "local schema")
	}
	if len(index.Diagnostics) > 0 {
		fmt.Fprintln(w)
		_ = diagnostic.Report{Diagnostics: index.Diagnostics}.WriteText(w)
		fmt.Fprintln(w)
	}
	return index, nil
}

func inspectUsage() string {
	return "usage: glade inspect symbols|graph|definition|references [--project <root>] [--json]"
}

type inspectDefinitionFlags struct {
	root    string
	symbol  string
	file    string
	line    int
	column  int
	jsonOut bool
}

type inspectReferencesFlags struct {
	root               string
	symbol             string
	jsonOut            bool
	includeDeclaration bool
}

type inspectDefinitionData struct {
	Symbol     string                    `json:"symbol"`
	Definition inspectIntelligenceSymbol `json:"definition"`
}

type inspectReferencesData struct {
	Symbol     string                    `json:"symbol"`
	Count      int                       `json:"count"`
	References []inspectIntelligenceUse  `json:"references"`
	Definition inspectIntelligenceSymbol `json:"definition,omitempty"`
}

type inspectIntelligenceSymbol struct {
	ID        codeintel.SymbolID   `json:"id"`
	Name      string               `json:"name"`
	Kind      codeintel.SymbolKind `json:"kind"`
	File      string               `json:"file,omitempty"`
	Range     diagnostic.Range     `json:"range"`
	Signature string               `json:"signature,omitempty"`
	Container codeintel.SymbolID   `json:"container,omitempty"`
}

type inspectIntelligenceUse struct {
	File   string            `json:"file"`
	Line   int               `json:"line"`
	Column int               `json:"column"`
	Kind   codeintel.UseKind `json:"kind"`
	Name   string            `json:"name"`
	Range  diagnostic.Range  `json:"range"`
}

func runInspectDefinition(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags, err := parseInspectDefinitionFlags(args)
	if err != nil {
		return err
	}
	index, graph, _, err := buildInspectCodeIntel(flags.root)
	if err != nil {
		return err
	}
	symbol, query, err := resolveInspectDefinition(graph, flags)
	if err != nil {
		return err
	}
	data := inspectDefinitionData{
		Symbol:     query,
		Definition: inspectSymbolJSON(index.Project.Root, symbol),
	}
	if flags.jsonOut {
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:     "inspect definition",
			Status:      "passed",
			ExitCode:    0,
			Project:     index.Project,
			Diagnostics: diagnosticsJSON(index.Project.Root, index.Diagnostics),
			Data:        data,
		})
	}
	writeInspectDefinitionText(w, data.Definition)
	return nil
}

func runInspectReferences(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	flags, err := parseInspectReferencesFlags(args)
	if err != nil {
		return err
	}
	index, graph, loadedProject, err := buildInspectCodeIntel(flags.root)
	if err != nil {
		return err
	}
	symbol, err := findInspectSymbol(graph, flags.symbol)
	if err != nil {
		return err
	}
	refs := graph.References(symbol.ID, flags.includeDeclaration)
	refs = fillInspectDeclarationLocations(loadedProject, symbol, refs)
	data := inspectReferencesData{
		Symbol:     flags.symbol,
		Count:      len(refs),
		Definition: inspectSymbolJSON(index.Project.Root, symbol),
		References: inspectUsesJSON(index.Project.Root, refs),
	}
	if flags.jsonOut {
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:     "inspect references",
			Status:      "passed",
			ExitCode:    0,
			Project:     index.Project,
			Diagnostics: diagnosticsJSON(index.Project.Root, index.Diagnostics),
			Data:        data,
		})
	}
	writeInspectReferencesText(w, data)
	return nil
}

func parseInspectDefinitionFlags(args []string) (inspectDefinitionFlags, error) {
	parsed, err := flagparse.New("glade inspect definition").
		String("project", "p").
		String("symbol", "").
		String("file", "").
		String("line", "").
		String("column", "").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return inspectDefinitionFlags{}, err
	}
	line, err := parseInspectPositiveInt(parsed.String("line"), "line")
	if err != nil {
		return inspectDefinitionFlags{}, err
	}
	column, err := parseInspectPositiveInt(parsed.String("column"), "column")
	if err != nil {
		return inspectDefinitionFlags{}, err
	}
	flags := inspectDefinitionFlags{
		root:    ".",
		symbol:  strings.TrimSpace(parsed.String("symbol")),
		file:    strings.TrimSpace(parsed.String("file")),
		line:    line,
		column:  column,
		jsonOut: parsed.Bool("json"),
	}
	if parsed.String("project") != "" {
		flags.root = parsed.String("project")
	}
	hasSymbol := flags.symbol != ""
	hasLocation := flags.file != "" || flags.line != 0 || flags.column != 0
	if hasSymbol == hasLocation {
		return inspectDefinitionFlags{}, errors.New("usage: glade inspect definition --project <root> --symbol <name> [--json] | --file <path> --line <n> --column <n> [--json]")
	}
	if hasLocation && (flags.file == "" || flags.line <= 0 || flags.column <= 0) {
		return inspectDefinitionFlags{}, errors.New("usage: glade inspect definition --file <path> --line <n> --column <n> [--project <root>] [--json]")
	}
	return flags, nil
}

func parseInspectPositiveInt(value, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("--%s must be a positive integer", name)
	}
	return n, nil
}

func parseInspectReferencesFlags(args []string) (inspectReferencesFlags, error) {
	parsed, err := flagparse.New("glade inspect references").
		String("project", "p").
		String("symbol", "").
		Bool("include-declaration", "").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return inspectReferencesFlags{}, err
	}
	flags := inspectReferencesFlags{
		root:               ".",
		symbol:             strings.TrimSpace(parsed.String("symbol")),
		jsonOut:            parsed.Bool("json"),
		includeDeclaration: parsed.Bool("include-declaration"),
	}
	if parsed.String("project") != "" {
		flags.root = parsed.String("project")
	}
	if flags.symbol == "" {
		return inspectReferencesFlags{}, errors.New("usage: glade inspect references --project <root> --symbol <name> [--include-declaration] [--json]")
	}
	return flags, nil
}

func buildInspectCodeIntel(root string) (typesys.Index, codeintel.Graph, project.Project, error) {
	p, err := project.Load(root)
	if err != nil {
		return typesys.Index{}, codeintel.Graph{}, project.Project{}, err
	}
	s, err := gladeschema.LoadProject(p)
	if err != nil {
		return typesys.Index{}, codeintel.Graph{}, project.Project{}, err
	}
	index := typesys.Build(p, s)
	graph := codeintel.Build(index, codeintel.Options{UseCache: true})
	return index, graph, p, nil
}

func resolveInspectDefinition(graph codeintel.Graph, flags inspectDefinitionFlags) (codeintel.Symbol, string, error) {
	if flags.symbol != "" {
		symbol, err := findInspectSymbol(graph, flags.symbol)
		return symbol, flags.symbol, err
	}
	id, ok := findInspectSymbolAtLocation(graph, flags.file, flags.line, flags.column)
	if !ok {
		return codeintel.Symbol{}, "", fmt.Errorf("no symbol found at %s:%d:%d", flags.file, flags.line, flags.column)
	}
	symbol, ok := graph.Definition(id)
	if !ok {
		return codeintel.Symbol{}, "", fmt.Errorf("no definition found for symbol at %s:%d:%d", flags.file, flags.line, flags.column)
	}
	return symbol, symbol.Name, nil
}

func findInspectSymbol(graph codeintel.Graph, query string) (codeintel.Symbol, error) {
	if symbol, ok := graph.Definition(codeintel.SymbolID(query)); ok {
		return symbol, nil
	}
	var matches []codeintel.Symbol
	for _, symbol := range graph.SortedSymbols() {
		if inspectSymbolMatchesQuery(graph, symbol, query) {
			matches = append(matches, symbol)
		}
	}
	if len(matches) == 0 {
		return codeintel.Symbol{}, fmt.Errorf("symbol %q not found", query)
	}
	if len(matches) > 1 {
		return codeintel.Symbol{}, fmt.Errorf("symbol %q is ambiguous; use a fully qualified symbol id", query)
	}
	return matches[0], nil
}

func inspectSymbolMatchesQuery(graph codeintel.Graph, symbol codeintel.Symbol, query string) bool {
	if symbol.Name == query {
		return true
	}
	if symbol.Kind == codeintel.SymbolSObjectField {
		parts := codeintel.ParseID(symbol.ID)
		return len(parts) == 4 && query == parts[2]+"."+parts[3]
	}
	if symbol.Kind != codeintel.SymbolApexMember || !strings.Contains(query, ".") {
		return false
	}
	parts := strings.Split(query, ".")
	if len(parts) != 2 || parts[1] != symbol.Name {
		return false
	}
	container, ok := graph.Definition(symbol.Container)
	return ok && container.Name == parts[0]
}

func findInspectSymbolAtLocation(graph codeintel.Graph, file string, line, column int) (codeintel.SymbolID, bool) {
	normalized := normalizeInspectPath(file)
	var bestID codeintel.SymbolID
	bestWidth := int(^uint(0) >> 1)
	for _, use := range graph.Uses {
		if inspectSameProjectPath(graph.ProjectRoot, use.File, normalized) && inspectRangeContains(use.Range, line, column) && use.SymbolID != "" {
			if width := inspectRangeWidth(use.Range); width < bestWidth {
				bestID = use.SymbolID
				bestWidth = width
			}
		}
	}
	for _, symbol := range graph.SortedSymbols() {
		if inspectSameProjectPath(graph.ProjectRoot, symbol.File, normalized) && inspectRangeContains(symbol.Range, line, column) {
			if width := inspectRangeWidth(symbol.Range); width < bestWidth {
				bestID = symbol.ID
				bestWidth = width
			}
		}
	}
	return bestID, bestID != ""
}

func inspectSameProjectPath(projectRoot, candidate, normalizedQuery string) bool {
	normalizedCandidate := normalizeInspectPath(candidate)
	if normalizedCandidate == normalizedQuery {
		return true
	}
	if projectRoot == "" {
		return false
	}
	return normalizeInspectPath(cliui.ProjectRelativePath(projectRoot, candidate)) == normalizedQuery
}

func inspectRangeContains(r diagnostic.Range, line, column int) bool {
	if r.Start.Line == 0 {
		return false
	}
	if line < r.Start.Line || line > r.End.Line {
		return false
	}
	if line == r.Start.Line && column < r.Start.Column {
		return false
	}
	if line == r.End.Line && r.End.Column > 0 && column > r.End.Column {
		return false
	}
	return true
}

func inspectRangeWidth(r diagnostic.Range) int {
	if r.Start.Offset != 0 || r.End.Offset != 0 {
		return r.End.Offset - r.Start.Offset
	}
	return (r.End.Line-r.Start.Line)*10000 + (r.End.Column - r.Start.Column)
}

func normalizeInspectPath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func inspectSymbolJSON(projectRoot string, symbol codeintel.Symbol) inspectIntelligenceSymbol {
	file := symbol.File
	if file != "" {
		file = cliui.ProjectRelativePath(projectRoot, file)
	}
	return inspectIntelligenceSymbol{
		ID:        symbol.ID,
		Name:      symbol.Name,
		Kind:      symbol.Kind,
		File:      file,
		Range:     symbol.Range,
		Signature: symbol.Signature,
		Container: symbol.Container,
	}
}

func inspectUsesJSON(projectRoot string, refs []codeintel.Use) []inspectIntelligenceUse {
	out := make([]inspectIntelligenceUse, 0, len(refs))
	for _, ref := range refs {
		file := ref.File
		if file != "" {
			file = cliui.ProjectRelativePath(projectRoot, file)
		}
		out = append(out, inspectIntelligenceUse{
			File:   file,
			Line:   ref.Range.Start.Line,
			Column: ref.Range.Start.Column,
			Kind:   ref.Kind,
			Name:   ref.Name,
			Range:  ref.Range,
		})
	}
	return out
}

func fillInspectDeclarationLocations(p project.Project, symbol codeintel.Symbol, refs []codeintel.Use) []codeintel.Use {
	file := symbol.File
	if file == "" {
		file = inspectMetadataDeclarationFile(p, symbol)
	}
	if file == "" {
		return refs
	}
	out := make([]codeintel.Use, len(refs))
	copy(out, refs)
	for i := range out {
		if out[i].Kind != codeintel.UseDeclaration || out[i].File != "" {
			continue
		}
		out[i].File = file
		if out[i].Range.Start.Line == 0 {
			out[i].Range = diagnostic.Range{
				Start: diagnostic.Position{Line: 1, Column: 1},
				End:   diagnostic.Position{Line: 1, Column: 1},
			}
		}
	}
	return out
}

func inspectMetadataDeclarationFile(p project.Project, symbol codeintel.Symbol) string {
	parts := codeintel.ParseID(symbol.ID)
	switch symbol.Kind {
	case codeintel.SymbolSObject:
		if len(parts) != 3 {
			return ""
		}
		suffix := normalizeInspectPath(filepath.Join("objects", parts[2], parts[2]+".object-meta.xml"))
		for _, file := range p.ObjectFiles {
			if strings.HasSuffix(normalizeInspectPath(file), suffix) {
				return file
			}
		}
	case codeintel.SymbolSObjectField:
		if len(parts) != 4 {
			return ""
		}
		suffix := normalizeInspectPath(filepath.Join("objects", parts[2], "fields", parts[3]+".field-meta.xml"))
		for _, file := range p.FieldFiles {
			if strings.HasSuffix(normalizeInspectPath(file), suffix) {
				return file
			}
		}
	}
	return ""
}

func writeInspectDefinitionText(w io.Writer, symbol inspectIntelligenceSymbol) {
	fmt.Fprintln(w, "Definition")
	fmt.Fprintf(w, "  symbol: %s\n", symbol.Name)
	fmt.Fprintf(w, "  kind: %s\n", symbol.Kind)
	if symbol.File != "" {
		fmt.Fprintf(w, "  file: %s\n", symbol.File)
	}
	fmt.Fprintf(w, "  range: %s\n", inspectRangeString(symbol.Range))
}

func writeInspectReferencesText(w io.Writer, data inspectReferencesData) {
	fmt.Fprintln(w, "References")
	fmt.Fprintf(w, "  symbol: %s\n", data.Symbol)
	fmt.Fprintf(w, "  count: %d\n", data.Count)
	for _, ref := range data.References {
		fmt.Fprintf(w, "  %s:%d:%d %s\n", ref.File, ref.Line, ref.Column, ref.Kind)
	}
}

func inspectRangeString(r diagnostic.Range) string {
	return fmt.Sprintf("%d:%d-%d:%d", r.Start.Line, r.Start.Column, r.End.Line, r.End.Column)
}

func parseInspectSymbolsFlags(args []string) (root string, jsonOut bool, fullPaths bool, kind string, err error) {
	root = "."
	parsed, err := flagparse.New("glade inspect symbols").
		String("project", "p").
		Bool("json", "j").
		Bool("full-paths", "").
		String("kind", "").
		Parse(args)
	if err != nil {
		return "", false, false, "", err
	}
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	kind = strings.ToLower(strings.TrimSpace(parsed.String("kind")))
	return root, parsed.Bool("json"), parsed.Bool("full-paths"), kind, nil
}

func inspectSymbolsSummary(index typesys.Index) map[string]int {
	return map[string]int{
		"apexTypes": len(index.Types),
		"triggers":  len(index.Triggers),
		"objects":   len(index.Objects),
		"fields":    countIndexFields(index),
	}
}

func countIndexFields(index typesys.Index) int {
	fields := 0
	for _, object := range index.Objects {
		fields += len(object.Fields)
	}
	return fields
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
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:  "schema load",
			Status:   "passed",
			ExitCode: 0,
			Project:  p,
			Summary: map[string]any{
				"objects": len(s.Objects),
				"fields":  countSchemaFields(s),
				"sources": packageDirectoryPaths(p),
			},
			Data:        s,
			Suggestions: []string{"glade check"},
		})
	}

	fmt.Fprintln(w, "Glade schema load")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Loaded local metadata")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "Objects   %d\n", len(s.Objects))
	fmt.Fprintf(w, "Fields    %d\n", countSchemaFields(s))
	if sources := packageDirectoryPaths(p); len(sources) > 0 {
		fmt.Fprintf(w, "Sources   %s\n", strings.Join(sources, ", "))
	}
	if len(s.Objects) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Objects:")
		for _, object := range s.Objects {
			fmt.Fprintf(w, "  %s\n", object.Name)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Next:")
	fmt.Fprintln(w, "  glade check")
	return nil
}

func countSchemaFields(s gladeschema.Schema) int {
	total := 0
	for _, object := range s.Objects {
		total += len(object.Fields)
	}
	return total
}

func packageDirectoryPaths(p project.Project) []string {
	out := make([]string, 0, len(p.PackageDirectories))
	for _, dir := range p.PackageDirectories {
		if strings.TrimSpace(dir.Path) != "" {
			out = append(out, filepath.ToSlash(dir.Path))
		}
	}
	return out
}

func runSchemaImportDescribe(args []string, w io.Writer) error {
	if len(args) > 0 && isHelpArg(args[0]) {
		return writeSchemaImportDescribeHelp(w)
	}
	input := ""
	output := ""
	projectCache := ""
	parsed, err := flagparse.New("glade schema import describe").
		String("input", "").
		String("output", "").
		String("project-cache", "").
		Parse(args)
	if err != nil {
		return err
	}
	input = parsed.String("input")
	output = parsed.String("output")
	projectCache = parsed.String("project-cache")
	if strings.TrimSpace(input) == "" {
		return errors.New("usage: glade schema import describe --input <describe.json> [--output <schema.json>] [--project-cache <root>]")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		return err
	}
	var catalog orgdescribe.Catalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return err
	}
	importedSchema := catalog.ToSchema()
	schemaData, err := json.MarshalIndent(importedSchema, "", "  ")
	if err != nil {
		return err
	}
	schemaData = append(schemaData, '\n')
	if strings.TrimSpace(output) == "" {
		if _, err := w.Write(schemaData); err != nil {
			return err
		}
	} else if err := os.WriteFile(output, schemaData, 0o644); err != nil {
		return err
	}
	if strings.TrimSpace(projectCache) == "" {
		return nil
	}
	if err := requireGladeProjectRoot(projectCache); err != nil {
		return err
	}
	return codeintel.WriteSchemaCache(projectCache, importedSchema)
}

func writeSchemaImportDescribeHelp(w io.Writer) error {
	_, err := fmt.Fprintln(w, strings.TrimSpace(`
Import captured Salesforce describe JSON into a local Glade schema.

Usage:
  glade schema import describe --input <describe.json> [--output <schema.json>] [--project-cache <root>]

Flags:
  --input <describe.json>   Captured org describe catalog JSON.
  --output <schema.json>    Write schema JSON to a file. Defaults to stdout.
  --project-cache <root>    Write schema symbols into the project codeintel cache.

Live org capture belongs in a plugin.
`))
	return err
}

func requireGladeProjectRoot(root string) error {
	cleanRoot, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	cleanRoot = filepath.Clean(cleanRoot)
	for _, marker := range []string{"sfdx-project.json", "glade.yml"} {
		_, err := os.Stat(filepath.Join(cleanRoot, marker))
		if err == nil {
			return nil
		}
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return fmt.Errorf("%s is not a Glade project root (missing sfdx-project.json or glade.yml)", cleanRoot)
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
		OK:       !result.HasErrors(),
		Label:    "check complete",
		ExitCode: exitCodeForOK(!result.HasErrors()),
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
		return result, writeCLIJSONEnvelope(out, cliJSONEnvelope{
			Command:     "check",
			Status:      statusForOK(!result.HasErrors()),
			ExitCode:    exitCodeForOK(!result.HasErrors()),
			Project:     result.Project,
			Summary:     result.Summary,
			Diagnostics: diagnosticsJSON(result.Project.Root, result.Diagnostics),
			Artifacts:   []any{},
			Suggestions: []string{"glade schema load --project .", "glade check --project ."},
			Data:        result,
		})
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
	progressMode = progressModeForFlags(outputFormat == "json", parsed.Bool("progress"), parsed.Bool("progress-json"), parsed.Bool("no-progress") || parsed.Bool("quiet"))
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
	debugLogMode := "summary"
	limitMode := vm.LimitMode("")
	limitProfile := ""
	var limitCaps vm.LimitCaps
	limitCapsSet := false
	execParser := flagparse.New("glade exec").
		Bool("json", "j").
		Bool("debug", "").
		String("project", "p").
		String("db", "").
		Bool("dry-run", "").
		String("trace", "t").
		String("debug-log", "").
		String("log-out", "").
		String("limit-mode", "").
		String("limit-profile", "").
		AllowPositionals(true).
		String("limit-queries", "").
		String("limit-query-rows", "").
		String("limit-dml-statements", "").
		String("limit-dml-rows", "").
		String("limit-heap-size", "").
		String("limit-cpu-ms", "").
		String("limit-callouts", "").
		String("limit-email-invocations", "").
		String("limit-async-jobs", "").
		String("limit-future-calls", "").
		String("limit-queueable-jobs", "").
		String("limit-batch-jobs", "").
		String("limit-scheduled-jobs", "").
		String("limit-sosl-queries", "").
		String("limit-query-locator-rows", "").
		String("limit-run-as", "").
		String("limit-savepoints", "").
		String("limit-savepoint-rollbacks", "").
		String("limit-publish-immediate-dml", "")
	parsed, err := execParser.Parse(args)
	if err != nil {
		return err
	}
	jsonOut := parsed.Bool("json")
	debug = parsed.Bool("debug")
	projectRoot := strings.TrimSpace(parsed.String("project"))
	runtimeProjectRoot := projectRoot
	dbPath := strings.TrimSpace(parsed.String("db"))
	dryRun := parsed.Bool("dry-run")
	if dbPath != "" && projectRoot == "" {
		projectRoot = "."
	}
	tracePath = parsed.String("trace")
	debugLogArg := strings.TrimSpace(parsed.String("debug-log"))
	logOutPath := strings.TrimSpace(parsed.String("log-out"))
	switch debugLogArg {
	case "", "summary":
		debugLogMode = "summary"
	case "raw":
		debugLogMode = "raw"
	case "-":
		debugLogMode = "raw"
		debugLogPath = "-"
	default:
		debugLogPath = debugLogArg
	}
	if logOutPath != "" {
		debugLogPath = logOutPath
	}
	if parsed.String("limit-mode") != "" {
		mode, err := parseLimitMode(parsed.String("limit-mode"))
		if err != nil {
			return err
		}
		limitMode = mode
	}
	limitProfile = strings.TrimSpace(parsed.String("limit-profile"))
	limitCaps, limitCapsSet, err = parseLimitCapsFromFlags(limitProfile, parsed.String)
	if err != nil {
		return err
	}
	sourceParts := parsed.Positionals
	if len(sourceParts) == 0 {
		return errors.New("usage: glade exec [--project <root>] [--db <path>] [--dry-run] [--json] [--trace <path>] [--debug-log <path>] '<anonymous apex>'")
	}

	program, err := vm.CompileAnonymous(strings.Join(sourceParts, " "))
	if err != nil {
		return err
	}

	stdout := w
	if jsonOut || debug || debugLogMode == "summary" || debugLogMode == "raw" || debugLogPath != "" {
		stdout = nil
	}
	machine := vm.New(stdout)
	machine.SetTraceEnabled(tracePath != "" || debug || jsonOut || debugLogMode != "" || debugLogPath != "")
	if limitMode != "" {
		machine.SetLimitMode(limitMode)
	}
	if limitCapsSet {
		machine.SetLimitCaps(limitCaps)
	}
	var store *storage.SQLiteStore
	var projectIndex typesys.Index
	hasProjectRuntime := false
	if runtimeProjectRoot != "" {
		p, index, err := loadProjectIndex(runtimeProjectRoot)
		if err != nil {
			return err
		}
		projectIndex = index
		hasProjectRuntime = true
		if dbPath == "" {
			org := orgStateFromIndex(runtimeProjectRoot, p, index)
			machine.SetOrg(&org)
			machine.SetCurrentNamespace(org.Namespace)
		}
	}
	if dbPath != "" {
		loadedStore, org, err := openDBStore(dbPath, projectRoot)
		if err != nil {
			return err
		}
		defer loadedStore.Close()
		store = loadedStore
		machine.SetOrg(&org)
		machine.SetCurrentNamespace(org.Namespace)
	}
	if hasProjectRuntime {
		if err := apextest.RegisterProjectRuntimeForRequest(machine, projectIndex); err != nil {
			return err
		}
	}
	result, execErr := machine.Execute(program)
	logPathForSummary := debugLogPath
	if debugLogMode == "raw" && logPathForSummary == "" {
		logPathForSummary = "-"
	}
	if !jsonOut && !debug && debugLogMode != "raw" && logPathForSummary == "" {
		logPathForSummary = defaultExecLogPath(projectRoot)
	}
	if logPathForSummary != "" {
		log := apexlog.Format(&result, execErr, apexlog.Options{})
		if err := writeDebugLog(logPathForSummary, log, w); err != nil {
			return err
		}
	}
	if execErr != nil {
		return execErr
	}
	if store != nil && !dryRun && machine.Org != nil {
		if err := store.Save(storage.SnapshotRuntimeOrg(machine.Org)); err != nil {
			return err
		}
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
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:   "exec",
			Status:    "passed",
			ExitCode:  0,
			Summary:   execSummary(result, logPathForSummary),
			Artifacts: execArtifacts(logPathForSummary, tracePath),
			Suggestions: []string{
				"glade debug profile --log " + defaultLogSuggestion(logPathForSummary),
				"glade db inspect",
			},
			Data: result,
		})
	}

	if debugLogMode == "raw" && logPathForSummary == "-" {
		return nil
	}
	return writeExecSummary(w, result, logPathForSummary)
}

func defaultExecLogPath(root string) string {
	base := strings.TrimSpace(root)
	if base == "" {
		base = "."
	}
	return filepath.Join(base, ".glade", "logs", "exec-"+time.Now().UTC().Format("20060102T150405Z")+".log")
}

func defaultLogSuggestion(path string) string {
	if strings.TrimSpace(path) == "" {
		return "<log>"
	}
	return path
}

func execSummary(result vm.Result, logPath string) map[string]any {
	return map[string]any{
		"debugEvents": len(result.Debug),
		"soqlQueries": result.Limits.Queries,
		"dml":         result.Limits.DMLStatements,
		"cpuTimeMs":   result.Limits.CPUTimeMS,
		"log":         logPath,
	}
}

func execArtifacts(logPath, tracePath string) []any {
	artifacts := []any{}
	if strings.TrimSpace(logPath) != "" && logPath != "-" {
		artifacts = append(artifacts, map[string]string{"kind": "debugLog", "path": logPath})
	}
	if strings.TrimSpace(tracePath) != "" {
		artifacts = append(artifacts, map[string]string{"kind": "trace", "path": tracePath})
	}
	return artifacts
}

func writeExecSummary(w io.Writer, result vm.Result, logPath string) error {
	fmt.Fprintln(w, "Glade exec")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "✓ Anonymous Apex executed")
	if len(result.Debug) > 0 {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Debug:")
		for _, line := range result.Debug {
			fmt.Fprintln(w, "  USER_DEBUG "+line)
		}
	}
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Limits:")
	fmt.Fprintf(w, "  SOQL queries    %d / 100\n", result.Limits.Queries)
	fmt.Fprintf(w, "  DML statements  %d / 150\n", result.Limits.DMLStatements)
	fmt.Fprintf(w, "  CPU time        %dms / 10000ms\n", result.Limits.CPUTimeMS)
	if strings.TrimSpace(logPath) != "" && logPath != "-" {
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Log:")
		fmt.Fprintln(w, "  "+filepath.ToSlash(logPath))
		fmt.Fprintln(w)
		fmt.Fprintln(w, "Next:")
		fmt.Fprintf(w, "  glade debug profile --log %s\n", filepath.ToSlash(logPath))
		fmt.Fprintln(w, "  glade db inspect")
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

type limitCapFlag struct {
	name  string
	apply func(*vm.LimitCaps, int)
}

var limitCapFlags = []limitCapFlag{
	{name: "limit-queries", apply: func(caps *vm.LimitCaps, value int) { caps.Queries = value }},
	{name: "limit-query-rows", apply: func(caps *vm.LimitCaps, value int) { caps.QueryRows = value }},
	{name: "limit-dml-statements", apply: func(caps *vm.LimitCaps, value int) { caps.DMLStatements = value }},
	{name: "limit-dml-rows", apply: func(caps *vm.LimitCaps, value int) { caps.DMLRows = value }},
	{name: "limit-heap-size", apply: func(caps *vm.LimitCaps, value int) { caps.HeapSize = value }},
	{name: "limit-cpu-ms", apply: func(caps *vm.LimitCaps, value int) { caps.CPUTimeMS = value }},
	{name: "limit-callouts", apply: func(caps *vm.LimitCaps, value int) { caps.Callouts = value }},
	{name: "limit-email-invocations", apply: func(caps *vm.LimitCaps, value int) { caps.EmailInvokes = value }},
	{name: "limit-async-jobs", apply: func(caps *vm.LimitCaps, value int) { caps.AsyncJobs = value }},
	{name: "limit-future-calls", apply: func(caps *vm.LimitCaps, value int) { caps.FutureCalls = value }},
	{name: "limit-queueable-jobs", apply: func(caps *vm.LimitCaps, value int) { caps.QueueableJobs = value }},
	{name: "limit-batch-jobs", apply: func(caps *vm.LimitCaps, value int) { caps.BatchJobs = value }},
	{name: "limit-scheduled-jobs", apply: func(caps *vm.LimitCaps, value int) { caps.ScheduledJobs = value }},
	{name: "limit-sosl-queries", apply: func(caps *vm.LimitCaps, value int) { caps.SOSLQueries = value }},
	{name: "limit-query-locator-rows", apply: func(caps *vm.LimitCaps, value int) { caps.QueryLocatorRows = value }},
	{name: "limit-run-as", apply: func(caps *vm.LimitCaps, value int) { caps.RunAs = value }},
	{name: "limit-savepoints", apply: func(caps *vm.LimitCaps, value int) { caps.Savepoints = value }},
	{name: "limit-savepoint-rollbacks", apply: func(caps *vm.LimitCaps, value int) { caps.SavepointRollbacks = value }},
	{name: "limit-publish-immediate-dml", apply: func(caps *vm.LimitCaps, value int) { caps.PublishImmediateDML = value }},
}

func isLimitCapFlag(name string) bool {
	for _, flag := range limitCapFlags {
		if flag.name == name {
			return true
		}
	}
	return false
}

func parseLimitCapsFromFlags(profile string, flagValue func(string) string) (vm.LimitCaps, bool, error) {
	profile = strings.TrimSpace(profile)
	capsSet := profile != ""
	caps, ok := vm.LimitCapsForProfile(profile)
	if !ok {
		return vm.LimitCaps{}, false, fmt.Errorf("unsupported limit profile %q", profile)
	}
	for _, flag := range limitCapFlags {
		raw := strings.TrimSpace(flagValue(flag.name))
		if raw == "" {
			continue
		}
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 {
			return vm.LimitCaps{}, false, fmt.Errorf("--%s must be a non-negative integer", flag.name)
		}
		flag.apply(&caps, value)
		capsSet = true
	}
	return caps, capsSet, nil
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
		String("format", "").
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
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:  "profile analyze",
			Status:   "passed",
			ExitCode: 0,
			Summary:  map[string]any{"events": report.Events, "limits": report.Limits},
			Data:     report,
			Suggestions: []string{
				"glade debug profile --log <apex.log>",
			},
		})
	}
	switch strings.ToLower(strings.TrimSpace(parsed.String("format"))) {
	case "", "text":
		return profile.WriteText(w, report, tracePath)
	case "markdown":
		return profile.WriteMarkdown(w, report)
	case "pprof":
		return profile.WritePprof(w, report)
	default:
		return fmt.Errorf("--format must be text, markdown, or pprof")
	}
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
	progressMode = progressModeForFlags(parsed.Bool("json"), parsed.Bool("progress"), parsed.Bool("progress-json"), parsed.Bool("no-progress") || parsed.Bool("quiet"))
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
