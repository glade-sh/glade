package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/apexlog"
	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/debuglog"
	"github.com/glade-sh/glade/internal/profile"
	"github.com/glade-sh/glade/internal/typesys"
)

func runDebug(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: glade debug parse|profile|explain|editor|repro|replay --log <path> [--json]")
	}

	subcommand := args[0]
	subcommandArgs, progressMode := splitDebugProgressArgs(args[1:])
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressMode})
	switch subcommand {
	case "parse":
		return runDebugParse(ctx, subcommandArgs, w, renderer)
	case "profile":
		return runDebugProfile(ctx, subcommandArgs, w, renderer)
	case "explain":
		return runDebugExplain(ctx, subcommandArgs, w, renderer)
	case "editor":
		return runDebugEditor(ctx, subcommandArgs, w, renderer)
	case "repro":
		return runDebugRepro(ctx, subcommandArgs, w, renderer)
	case "replay":
		return runDebugReplay(ctx, subcommandArgs, w, renderer)
	default:
		return errors.New("usage: glade debug parse|profile|explain|editor|repro|replay --log <path> [--json]")
	}
}

func splitDebugProgressArgs(args []string) ([]string, cliui.ProgressMode) {
	progress := false
	progressJSON := false
	noProgress := false
	jsonOut := false
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		switch arg {
		case "--progress":
			progress = true
		case "--progress-json":
			progressJSON = true
		case "--no-progress", "--quiet":
			noProgress = true
		case "--json":
			jsonOut = true
			filtered = append(filtered, arg)
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, progressModeForFlags(jsonOut, progress, progressJSON, noProgress)
}

func runDebugParse(ctx context.Context, args []string, w io.Writer, renderer cliui.Renderer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, _, err := parseDebugCommandArgs(args)
	if err != nil {
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "debug parse", Label: "Reading log"})
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "debug complete"})
	return writeIndentedJSON(w, log)
}

func runDebugProfile(ctx context.Context, args []string, w io.Writer, renderer cliui.Renderer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, jsonOut, format, err := parseDebugProfileArgs(args)
	if err != nil {
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "debug profile", Label: "Reading log"})
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "debug profile", Label: "Analyzing trace", Current: 1, Total: 2})
	doc := apexlog.TraceDocument(log)
	report := profile.Analyze(doc)
	renderer.Finish(cliui.Result{OK: true, Label: "debug complete"})
	if jsonOut {
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:  "debug profile",
			Status:   "passed",
			ExitCode: 0,
			Summary:  map[string]any{"events": report.Events, "limits": report.Limits},
			Data:     report,
			Suggestions: []string{
				"glade debug explain --log " + logPath + " --project .",
			},
		})
	}
	if format == "markdown" {
		return profile.WriteMarkdown(w, report)
	}
	return profile.WriteText(w, report, logPath)
}

func runDebugExplain(ctx context.Context, args []string, w io.Writer, renderer cliui.Renderer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, jsonOut, minConfidence, projectRoot, err := parseDebugExplainArgs(args)
	if err != nil {
		return err
	}

	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "debug explain", Label: "Reading log"})
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}

	_, index, err := loadProjectIndexWithProgress(projectRoot, "debug explain", renderer)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}

	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "debug explain", Label: "Annotating log", Current: 2, Total: 3})
	annotated, err := debuglog.Annotate(log, index, 5)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "debug complete"})
	if jsonOut {
		return debuglog.WriteJSON(w, annotated)
	}
	return debuglog.WriteText(w, annotated, minConfidence)
}

func runDebugEditor(ctx context.Context, args []string, w io.Writer, renderer cliui.Renderer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, jsonOut, minConfidence, maxCandidates, projectRoot, err := parseDebugEditorArgs(args)
	if err != nil {
		return err
	}

	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "debug editor", Label: "Reading log"})
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}

	index := typesys.Index{}
	var objectFiles []string
	var fieldFiles []string
	if projectRoot != "" {
		projectState, loadedIndex, err := loadProjectIndexWithProgress(projectRoot, "debug editor", renderer)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
			return err
		}
		index = loadedIndex
		objectFiles = projectState.ObjectFiles
		fieldFiles = projectState.FieldFiles
	}

	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "debug editor", Label: "Analyzing log", Current: 2, Total: 3})
	analysis := debuglog.BuildEditorAnalysis(log, index, debuglog.EditorOptions{
		LogFile:       logPath,
		ProjectRoot:   projectRoot,
		ObjectFiles:   objectFiles,
		FieldFiles:    fieldFiles,
		MinConfidence: minConfidence,
		MaxCandidates: maxCandidates,
	})
	renderer.Finish(cliui.Result{OK: true, Label: "debug complete"})
	if jsonOut {
		return writeIndentedJSON(w, analysis)
	}
	_, err = fmt.Fprintf(w, "Glade debug editor\nLog: %s\nEntries: %d\nFolds: %d\nLinks: %d\nVariables: %d\nDiagnostics: %d\nNext:\n  Open this file in VS Code with language Apex Log.\n",
		logPath,
		len(analysis.Entries),
		len(analysis.Folds),
		len(analysis.Links),
		len(analysis.Variables),
		len(analysis.Diagnostics),
	)
	return err
}

func runDebugRepro(ctx context.Context, args []string, w io.Writer, renderer cliui.Renderer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, _, minConfidence, projectRoot, err := parseDebugExplainArgs(args)
	if err != nil {
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "debug repro", Label: "Reading log"})
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	_, index, err := loadProjectIndexWithProgress(projectRoot, "debug repro", renderer)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "debug repro", Label: "Annotating log", Current: 2, Total: 3})
	annotated, err := debuglog.Annotate(log, index, 5)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	source, err := debuglog.SynthesizeTest(annotated, minConfidence)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "debug complete"})
	_, err = io.WriteString(w, source)
	return err
}

func runDebugReplay(ctx context.Context, args []string, w io.Writer, renderer cliui.Renderer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, jsonOut, minConfidence, projectRoot, entryIndex, err := parseDebugReplayArgs(args)
	if err != nil {
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "debug replay", Label: "Reading log"})
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	_, index, err := loadProjectIndexWithProgress(projectRoot, "debug replay", renderer)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "debug replay", Label: "Building replay", Current: 2, Total: 3})
	annotated, err := debuglog.Annotate(log, index, 5)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	plan, err := debuglog.SynthesizeReplayWithOptions(annotated, debuglog.ReplayOptions{MinConfidence: minConfidence, EntryIndex: entryIndex, UseEntryIndex: entryIndex >= 0, SourceIndex: debuglog.BuildSourceIndex(index)})
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "debug failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "debug complete"})
	if jsonOut {
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:  "debug replay",
			Status:   "passed",
			ExitCode: 0,
			Summary: map[string]any{
				"entryPoint": plan.EntryPoint,
				"warnings":   len(plan.Warnings),
			},
			Data: plan,
		})
	}
	_, err = io.WriteString(w, plan.Source)
	return err
}

func parseDebugEditorArgs(args []string) (logPath string, jsonOut bool, minConfidence float64, maxCandidates int, projectRoot string, err error) {
	minConfidence = 0.35
	maxCandidates = 5
	logSeen := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--log":
			if i+1 >= len(args) {
				return "", false, 0, 0, "", errors.New("--log requires a path")
			}
			logPath = args[i+1]
			logSeen = true
			i++
		case "--project":
			if i+1 >= len(args) {
				return "", false, 0, 0, "", errors.New("--project requires a value")
			}
			projectRoot = args[i+1]
			i++
		case "--min-confidence":
			if i+1 >= len(args) {
				return "", false, 0, 0, "", errors.New("--min-confidence requires a value")
			}
			var parseErr error
			minConfidence, parseErr = parseDebugFloat(args[i+1], "--min-confidence")
			if parseErr != nil {
				return "", false, 0, 0, "", parseErr
			}
			i++
		case "--max-candidates":
			if i+1 >= len(args) {
				return "", false, 0, 0, "", errors.New("--max-candidates requires a value")
			}
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(args[i+1]))
			if parseErr != nil || parsed <= 0 {
				return "", false, 0, 0, "", errors.New("--max-candidates must be a positive integer")
			}
			maxCandidates = parsed
			i++
		case "--json":
			jsonOut = true
		default:
			return "", false, 0, 0, "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if !logSeen {
		return "", false, 0, 0, "", errors.New("--log is required")
	}
	return logPath, jsonOut, minConfidence, maxCandidates, projectRoot, nil
}

func parseDebugReplayArgs(args []string) (logPath string, jsonOut bool, minConfidence float64, projectRoot string, entryIndex int, err error) {
	projectRoot = "."
	minConfidence = 0.50
	entryIndex = -1
	logSeen := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--log":
			if i+1 >= len(args) {
				return "", false, 0, "", -1, errors.New("--log requires a path")
			}
			logPath = args[i+1]
			logSeen = true
			i++
		case "--project":
			if i+1 >= len(args) {
				return "", false, 0, "", -1, errors.New("--project requires a value")
			}
			projectRoot = args[i+1]
			i++
		case "--min-confidence":
			if i+1 >= len(args) {
				return "", false, 0, "", -1, errors.New("--min-confidence requires a value")
			}
			var parseErr error
			minConfidence, parseErr = parseDebugFloat(args[i+1], "--min-confidence")
			if parseErr != nil {
				return "", false, 0, "", -1, parseErr
			}
			i++
		case "--entry-index":
			if i+1 >= len(args) {
				return "", false, 0, "", -1, errors.New("--entry-index requires a value")
			}
			parsed, parseErr := strconv.Atoi(strings.TrimSpace(args[i+1]))
			if parseErr != nil || parsed < 0 {
				return "", false, 0, "", -1, errors.New("--entry-index must be a non-negative integer")
			}
			entryIndex = parsed
			i++
		case "--json":
			jsonOut = true
		default:
			return "", false, 0, "", -1, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if !logSeen {
		return "", false, 0, "", -1, errors.New("--log is required")
	}
	return logPath, jsonOut, minConfidence, projectRoot, entryIndex, nil
}

func parseDebugCommandArgs(args []string) (logPath string, jsonOut bool, err error) {
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--log":
			if i+1 >= len(args) {
				return "", false, errors.New("--log requires a path")
			}
			logPath = args[i+1]
			i++
		case "--json":
			jsonOut = true
		default:
			return "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if logPath == "" {
		return "", jsonOut, errors.New("--log is required")
	}
	return logPath, jsonOut, nil
}

func parseDebugProfileArgs(args []string) (logPath string, jsonOut bool, format string, err error) {
	format = "text"
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--log":
			if i+1 >= len(args) {
				return "", false, "", errors.New("--log requires a path")
			}
			logPath = args[i+1]
			i++
		case "--json":
			jsonOut = true
		case "--format":
			if i+1 >= len(args) {
				return "", false, "", errors.New("--format requires a value")
			}
			format = strings.ToLower(strings.TrimSpace(args[i+1]))
			i++
		default:
			return "", false, "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if logPath == "" {
		return "", jsonOut, format, errors.New("--log is required")
	}
	switch format {
	case "text", "markdown":
	default:
		return "", false, "", fmt.Errorf("--format must be text or markdown")
	}
	return logPath, jsonOut, format, nil
}

func parseDebugExplainArgs(args []string) (logPath string, jsonOut bool, minConfidence float64, projectRoot string, err error) {
	projectRoot = "."
	minConfidence = 0.50
	logSeen := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--log":
			if i+1 >= len(args) {
				return "", false, 0, "", errors.New("--log requires a path")
			}
			logPath = args[i+1]
			logSeen = true
			i++
		case "--project":
			if i+1 >= len(args) {
				return "", false, 0, "", errors.New("--project requires a value")
			}
			projectRoot = args[i+1]
			i++
		case "--min-confidence":
			if i+1 >= len(args) {
				return "", false, 0, "", errors.New("--min-confidence requires a value")
			}
			var parseErr error
			minConfidence, parseErr = parseDebugFloat(args[i+1], "--min-confidence")
			if parseErr != nil {
				return "", false, 0, "", parseErr
			}
			i++
		case "--json":
			jsonOut = true
		default:
			return "", false, 0, "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if !logSeen {
		return "", false, 0, "", errors.New("--log is required")
	}
	return logPath, jsonOut, minConfidence, projectRoot, nil
}

func parseDebugLogFile(path string) (apexlog.Log, error) {
	if path == "-" {
		return apexlog.Parse(os.Stdin)
	}
	file, err := os.Open(path)
	if err != nil {
		return apexlog.Log{}, err
	}
	defer file.Close()
	return apexlog.Parse(file)
}

func parseDebugFloat(raw, flag string) (float64, error) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a number", flag)
	}
	return value, nil
}

// writeIndentedJSON serializes a payload with stable pretty formatting.
func writeIndentedJSON(w io.Writer, value any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}
