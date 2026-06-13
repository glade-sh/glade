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
	"github.com/glade-sh/glade/internal/debuglog"
	"github.com/glade-sh/glade/internal/profile"
)

func runDebug(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) == 0 {
		return errors.New("usage: glade debug parse|profile|explain|repro --log <path> [--json]")
	}

	subcommand := args[0]
	subcommandArgs := args[1:]
	switch subcommand {
	case "parse":
		return runDebugParse(ctx, subcommandArgs, w)
	case "profile":
		return runDebugProfile(ctx, subcommandArgs, w)
	case "explain":
		return runDebugExplain(ctx, subcommandArgs, w)
	case "repro":
		return runDebugRepro(ctx, subcommandArgs, w)
	default:
		return errors.New("usage: glade debug parse|profile|explain|repro --log <path> [--json]")
	}
}

func runDebugParse(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, _, err := parseDebugCommandArgs(args)
	if err != nil {
		return err
	}
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		return err
	}
	return writeIndentedJSON(w, log)
}

func runDebugProfile(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, jsonOut, format, err := parseDebugProfileArgs(args)
	if err != nil {
		return err
	}
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		return err
	}
	doc := apexlog.TraceDocument(log)
	report := profile.Analyze(doc)
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

func runDebugExplain(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, jsonOut, minConfidence, projectRoot, err := parseDebugExplainArgs(args)
	if err != nil {
		return err
	}

	log, err := parseDebugLogFile(logPath)
	if err != nil {
		return err
	}

	index, err := loadIndex(projectRoot)
	if err != nil {
		return err
	}

	annotated, err := debuglog.Annotate(log, index, 5)
	if err != nil {
		return err
	}
	if jsonOut {
		return debuglog.WriteJSON(w, annotated)
	}
	return debuglog.WriteText(w, annotated, minConfidence)
}

func runDebugRepro(ctx context.Context, args []string, w io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	logPath, _, minConfidence, projectRoot, err := parseDebugExplainArgs(args)
	if err != nil {
		return err
	}
	log, err := parseDebugLogFile(logPath)
	if err != nil {
		return err
	}
	index, err := loadIndex(projectRoot)
	if err != nil {
		return err
	}
	annotated, err := debuglog.Annotate(log, index, 5)
	if err != nil {
		return err
	}
	source, err := debuglog.SynthesizeTest(annotated, minConfidence)
	if err != nil {
		return err
	}
	_, err = io.WriteString(w, source)
	return err
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
