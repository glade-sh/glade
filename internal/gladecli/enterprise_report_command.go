package gladecli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/enterprise"
	"github.com/glade-sh/glade/internal/enterpriseassess"
	"github.com/glade-sh/glade/internal/enterprisecruft"
	"github.com/glade-sh/glade/internal/enterprisegraph"
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/refactorproof"
)

func runInspectGraph(ctx context.Context, args []string, w io.Writer, progressW io.Writer) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(args) > 0 && isHelpArg(args[0]) {
		fmt.Fprintln(w, "usage: glade inspect graph [--project <root>] [--json]")
		return nil
	}
	parsed, err := flagparse.New("glade inspect graph").
		String("project", "p").
		Bool("json", "j").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		Parse(args)
	if err != nil {
		return err
	}
	root := "."
	if parsed.String("project") != "" {
		root = parsed.String("project")
	}
	progressMode := progressModeForFlags(parsed.Bool("json"), parsed.Bool("progress"), parsed.Bool("progress-json"), parsed.Bool("no-progress") || parsed.Bool("quiet"))
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: progressMode})
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "inspect graph", Label: "Loading context"})
	ctxData, err := enterprise.LoadContext(root)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "inspect graph failed"})
		return err
	}
	renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "inspect graph", Label: "Building graph", Current: 1, Total: 2})
	graph, err := enterprisegraph.Build(ctxData)
	if err != nil {
		renderer.Finish(cliui.Result{OK: false, Label: "inspect graph failed"})
		return err
	}
	renderer.Finish(cliui.Result{OK: true, Label: "inspect graph complete"})
	if !parsed.Bool("json") {
		fmt.Fprintf(w, "nodes: %d\n", len(graph.Nodes))
		fmt.Fprintf(w, "edges: %d\n", len(graph.Edges))
		return nil
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(graph)
}

type enterpriseReportOptions struct {
	Root            string
	Format          string
	Out             string
	IncludeMetadata bool
	IncludeTests    bool
	Strict          bool
	Since           string
	TracePath       string
	FailOnAPIBreak  bool
	ProgressMode    cliui.ProgressMode
}

func runEnterpriseReport(ctx context.Context, command string, args []string, w io.Writer, progressW io.Writer) error {
	opts, err := parseEnterpriseReportOptions(command, args)
	if err != nil {
		return err
	}
	renderer := cliui.NewRenderer(cliui.RendererOptions{Stderr: progressW, Mode: opts.ProgressMode})
	switch command {
	case "assess":
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "report " + command, Label: "Loading context"})
		ctxData, err := enterprise.LoadContext(opts.Root)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "report " + command, Label: "Building graph", Current: 1, Total: 3})
		graph, err := enterprisegraph.Build(ctxData)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "report " + command, Label: "Analyzing report", Current: 2, Total: 3})
		report := enterpriseassess.Assess(ctxData, graph, enterpriseassess.Options{
			IncludeMetadata: opts.IncludeMetadata,
			IncludeTests:    opts.IncludeTests,
			Strict:          opts.Strict,
		})
		report.Command = enterpriseCommandLine("report", command, args)
		if err := writeEnterpriseReport(w, report, opts.Format, opts.Out); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Finish(cliui.Result{OK: report.Status != enterprise.StatusFail, Label: "report complete"})
		return enterpriseReportStatusError(report, "enterprise assessment failed")
	case "cruft":
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "report " + command, Label: "Loading context"})
		ctxData, err := enterprise.LoadContext(opts.Root)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "report " + command, Label: "Building graph", Current: 1, Total: 3})
		graph, err := enterprisegraph.Build(ctxData)
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseTick, Phase: "report " + command, Label: "Analyzing report", Current: 2, Total: 3})
		report := enterprisecruft.Scan(ctxData, graph)
		report.Command = enterpriseCommandLine("report", command, args)
		if err := writeEnterpriseReport(w, report, opts.Format, opts.Out); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Finish(cliui.Result{OK: report.Status != enterprise.StatusFail, Label: "report complete"})
		return enterpriseReportStatusError(report, "enterprise cruft report failed")
	case "refactor-proof":
		renderer.Render(cliui.Event{Kind: cliui.EventPhaseStart, Phase: "report " + command, Label: "Running proof"})
		result, err := refactorproof.Prove(ctx, refactorproof.Options{
			Root:           opts.Root,
			Since:          opts.Since,
			TracePath:      opts.TracePath,
			FailOnAPIBreak: opts.FailOnAPIBreak,
		})
		if err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		result.Report.Command = enterpriseCommandLine("report", command, args)
		if err := writeEnterpriseReport(w, result.Report, opts.Format, opts.Out); err != nil {
			renderer.Finish(cliui.Result{OK: false, Label: "report failed"})
			return err
		}
		renderer.Finish(cliui.Result{OK: result.Report.Status != enterprise.StatusFail, Label: "report complete"})
		return enterpriseReportStatusError(result.Report, "refactor proof failed")
	default:
		return fmt.Errorf("unknown enterprise report command %q", command)
	}
}

func enterpriseReportStatusError(report enterprise.Report, message string) error {
	if report.Status == enterprise.StatusFail {
		return errors.New(message)
	}
	return nil
}

func parseEnterpriseReportOptions(command string, args []string) (enterpriseReportOptions, error) {
	if len(args) > 0 && isHelpArg(args[0]) {
		return enterpriseReportOptions{}, fmt.Errorf("usage: glade report %s [--project <root>] [--format json|html|md] [--out <path>]", command)
	}
	parsed, err := flagparse.New("glade report "+command).
		String("project", "p").
		String("format", "").
		String("out", "o").
		Bool("include-metadata", "").
		Bool("include-tests", "").
		Bool("strict", "").
		String("since", "").
		String("trace", "").
		Bool("fail-on-api-break", "").
		Bool("progress", "").
		Bool("progress-json", "").
		Bool("no-progress", "").
		Bool("quiet", "q").
		Parse(args)
	if err != nil {
		return enterpriseReportOptions{}, err
	}
	opts := enterpriseReportOptions{Root: ".", Format: "json", Since: "HEAD"}
	if parsed.String("project") != "" {
		opts.Root = parsed.String("project")
	}
	if parsed.String("format") != "" {
		opts.Format = strings.ToLower(strings.TrimSpace(parsed.String("format")))
	}
	opts.Out = parsed.String("out")
	opts.IncludeMetadata = parsed.Bool("include-metadata")
	opts.IncludeTests = parsed.Bool("include-tests")
	opts.Strict = parsed.Bool("strict")
	if parsed.String("since") != "" {
		opts.Since = parsed.String("since")
	}
	opts.TracePath = strings.TrimSpace(parsed.String("trace"))
	opts.FailOnAPIBreak = parsed.Bool("fail-on-api-break")
	opts.ProgressMode = progressModeForFlags(opts.Format == "json", parsed.Bool("progress"), parsed.Bool("progress-json"), parsed.Bool("no-progress") || parsed.Bool("quiet"))
	if err := validateEnterpriseReportOptions(command, opts, parsed.String("since") != ""); err != nil {
		return enterpriseReportOptions{}, err
	}
	return opts, nil
}

func validateEnterpriseReportOptions(command string, opts enterpriseReportOptions, sinceSet bool) error {
	switch command {
	case "assess":
		if sinceSet || opts.TracePath != "" || opts.FailOnAPIBreak {
			return errors.New("glade report assess only accepts --project, --format, --out, --include-metadata, --include-tests, and --strict")
		}
	case "cruft":
		if opts.IncludeMetadata || opts.IncludeTests || opts.Strict || sinceSet || opts.TracePath != "" || opts.FailOnAPIBreak {
			return errors.New("glade report cruft only accepts --project, --format, and --out")
		}
	case "refactor-proof":
		if opts.IncludeMetadata || opts.IncludeTests || opts.Strict {
			return errors.New("glade report refactor-proof only accepts --project, --format, --out, --since, --trace, and --fail-on-api-break")
		}
	}
	return nil
}

func writeEnterpriseReport(w io.Writer, report enterprise.Report, format, out string) error {
	if format == "" {
		format = "json"
	}
	switch format {
	case "json", "html", "md":
	default:
		return errors.New("--format must be json, html, or md")
	}
	var writer io.Writer = w
	var file *os.File
	if out != "" {
		if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
			return err
		}
		created, err := os.Create(out)
		if err != nil {
			return err
		}
		defer created.Close()
		file = created
		writer = file
	}
	switch format {
	case "json":
		if err := enterprise.WriteJSON(writer, report); err != nil {
			return err
		}
	case "html":
		if err := enterprise.WriteHTML(writer, report); err != nil {
			return err
		}
	case "md":
		if err := enterprise.WriteMarkdown(writer, report); err != nil {
			return err
		}
	}
	if out != "" {
		fmt.Fprintf(w, "report: %s\n", out)
	}
	return nil
}

func enterpriseCommandLine(root, command string, args []string) string {
	parts := append([]string{"glade", root, command}, args...)
	return strings.Join(parts, " ")
}
