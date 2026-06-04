package gladecli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/glade-sh/glade/internal/surfaceledger"
)

func runCompatSurface(args []string, w io.Writer) error {
	if len(args) == 0 || isHelpArg(args[0]) {
		return errors.New(surfaceUsage())
	}
	switch args[0] {
	case "refresh":
		return runCompatSurfaceRefresh(args[1:], w)
	case "sources":
		return runCompatSurfaceSources(args[1:], w)
	case "docs":
		return runCompatSurfaceDocs(args[1:], w)
	case "org":
		return runCompatSurfaceOrg(args[1:], w)
	case "glade":
		return runCompatSurfaceGlade(args[1:], w)
	case "evidence":
		return runCompatSurfaceEvidence(args[1:], w)
	case "ledger":
		return runCompatSurfaceLedger(args[1:], w)
	case "packet":
		return runCompatSurfacePacket(args[1:], w)
	case "gaps":
		return runCompatSurfaceGaps(args[1:], w)
	case "explain":
		return runCompatSurfaceExplain(args[1:], w)
	case "check":
		return runCompatSurfaceCheck(args[1:], w)
	default:
		return errors.New(surfaceUsage())
	}
}

func surfaceUsage() string {
	return "usage: glade compat surface refresh|sources|docs|org|glade|evidence|ledger|packet|gaps|explain|check [flags]"
}

func runCompatSurfaceSources(args []string, w io.Writer) error {
	docs := ""
	output := ""
	check := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--docs":
			i++
			var err error
			docs, err = argValue(args, i, "--docs")
			if err != nil {
				return err
			}
		case "--output":
			i++
			var err error
			output, err = argValue(args, i, "--output")
			if err != nil {
				return err
			}
		case "--check":
			i++
			var err error
			check, err = argValue(args, i, "--check")
			if err != nil {
				return err
			}
		case "--json":
			jsonOut = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	requested := 0
	for _, set := range []bool{jsonOut, output != "", check != ""} {
		if set {
			requested++
		}
	}
	if requested > 1 {
		return errors.New("use only one of --json, --output, or --check")
	}
	if docs == "" {
		docs = os.Getenv("GLADE_SALESFORCE_DOCS_SOURCE")
	}
	report, err := surfaceledger.AuditSourceUniverse(docs)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(report)
	}
	markdown := surfaceledger.SourceAuditMarkdown(report)
	if output != "" {
		if err := os.WriteFile(output, []byte(markdown), 0o644); err != nil {
			return err
		}
	}
	if check != "" {
		existing, err := os.ReadFile(check)
		if err != nil {
			return err
		}
		if string(existing) != markdown {
			return fmt.Errorf("surface sources drift: run `glade compat surface sources --docs %s --output %s`", docs, check)
		}
		if !surfaceledger.SourceAuditComplete(report) {
			return fmt.Errorf("surface sources check failed: missing source families: %d, partial source families: %d, missingLocalMarkdown: %d, unmanifestedMarkdown: %d", report.MissingRequired, report.PartialRequired, report.MissingLocalMarkdown, report.UnmanifestedMarkdown)
		}
		fmt.Fprintf(w, "%s: up to date\n", check)
	}
	fmt.Fprintln(w, "surface sources: ok")
	fmt.Fprintf(w, "atlas: pinned=%d missing=%d partial=%d\n", report.AtlasPinned, report.AtlasMissing, report.AtlasPartial)
	fmt.Fprintf(w, "nonAtlas: lwc=%s siteReferences=%s\n", report.LWCStatus, report.SiteReferencesStatus)
	fmt.Fprintf(w, "files: manifest=%s searchIndex=%s missingLocalMarkdown=%d\n", presentWord(report.ManifestPresent), presentWord(report.SearchIndexPresent), report.MissingLocalMarkdown)
	if check != "" {
		fmt.Fprintf(w, "check: %s\n", check)
	}
	return nil
}

func presentWord(ok bool) string {
	if ok {
		return "present"
	}
	return "missing"
}

func runCompatSurfaceRefresh(args []string, w io.Writer) error {
	options := surfaceledger.RefreshOptions{OutputDir: filepath.Join("docs", "generated", "salesforce")}
	jsonOut := false
	dryRun := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--docs":
			i++
			if i >= len(args) {
				return errors.New("--docs requires a value")
			}
			options.DocsSource = args[i]
		case "--tooling-completions":
			i++
			if i >= len(args) {
				return errors.New("--tooling-completions requires a value")
			}
			options.ToolingCompletions = args[i]
		case "--target-org":
			i++
			if i >= len(args) {
				return errors.New("--target-org requires a value")
			}
			options.TargetOrg = args[i]
		case "--out":
			i++
			if i >= len(args) {
				return errors.New("--out requires a value")
			}
			options.OutputDir = args[i]
		case "--release":
			i++
			if i >= len(args) {
				return errors.New("--release requires a value")
			}
			options.Release = args[i]
		case "--diff-from":
			i++
			if i >= len(args) {
				return errors.New("--diff-from requires a value")
			}
			options.DiffFrom = args[i]
		case "--json":
			jsonOut = true
		case "--dry-run":
			dryRun = true
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	if options.DocsSource == "" {
		options.DocsSource = os.Getenv("GLADE_SALESFORCE_DOCS_SOURCE")
	}
	if options.DocsSource == "" {
		return errors.New("--docs is required unless GLADE_SALESFORCE_DOCS_SOURCE is set")
	}
	if options.ToolingCompletions == "" && options.TargetOrg == "" {
		if defaultPath := defaultSalesforceToolingCompletionsSource(); fileExists(defaultPath) {
			options.ToolingCompletions = defaultPath
		}
	}
	if dryRun {
		out, err := os.MkdirTemp("", "glade-surface-ledger-*")
		if err != nil {
			return err
		}
		options.OutputDir = out
	}
	result, err := surfaceledger.Refresh(options)
	if err != nil {
		return err
	}
	if jsonOut {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	}
	fmt.Fprintln(w, "surface refresh: ok")
	if dryRun {
		fmt.Fprintf(w, "dryRunOut=%s\n", result.OutputDir)
	}
	fmt.Fprintf(w, "inputs: docs=%s org=%s glade=standard-symbols evidence=fixtures\n", options.DocsSource, orgInputName(options))
	fmt.Fprintf(w, "implemented=%d partial=%d passive=%d explicitUnsupported=%d\n", result.Summary.Implemented, result.Summary.Partial, result.Summary.Passive, result.Summary.ExplicitUnsupported)
	fmt.Fprintf(w, "gaps: missingShape=%d missingBehavior=%d missingEvidence=%d\n", result.Summary.Gaps[surfaceledger.GapMissingShape], result.Summary.Gaps[surfaceledger.GapMissingBehavior], result.Summary.Gaps[surfaceledger.GapMissingEvidence])
	fmt.Fprintf(w, "failures: parser=%d docsOrgMismatch=%d staleGlade=%d passiveServiceRisk=%d\n", result.Summary.Failures["parser"], result.Summary.Failures[surfaceledger.GapDocsOrgMismatch], result.Summary.Failures[surfaceledger.GapStaleGladeShape], result.Summary.Failures[surfaceledger.GapPassiveServiceRisk])
	fmt.Fprintf(w, "reports: %s\n", filepath.Join(result.OutputDir, "SURFACE_DASHBOARD.md"))
	return nil
}

func orgInputName(options surfaceledger.RefreshOptions) string {
	if options.TargetOrg != "" {
		return options.TargetOrg
	}
	if options.ToolingCompletions != "" {
		return filepath.Base(options.ToolingCompletions)
	}
	return "not-queried"
}

func runCompatSurfaceDocs(args []string, w io.Writer) error {
	source, output, jsonOut, err := parseSourceOutputJSON(args, "--source")
	if err != nil {
		return err
	}
	rows, err := surfaceledger.BuildDocsSnapshot(source)
	if err != nil {
		return err
	}
	return writeRows(rows, output, jsonOut, w)
}

func runCompatSurfaceOrg(args []string, w io.Writer) error {
	path, output, jsonOut, err := parseSourceOutputJSON(args, "--tooling-completions")
	if err != nil {
		return err
	}
	if path == "" {
		path = defaultSalesforceToolingCompletionsSource()
	}
	rows, err := surfaceledger.BuildOrgSnapshotFromToolingCompletions(path)
	if err != nil {
		return err
	}
	return writeRows(rows, output, jsonOut, w)
}

func runCompatSurfaceGlade(args []string, w io.Writer) error {
	output := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
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
	return writeRows(surfaceledger.BuildGladeSnapshot(), output, jsonOut, w)
}

func runCompatSurfaceEvidence(args []string, w io.Writer) error {
	output := ""
	jsonOut := false
	var fixtures []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--output":
			i++
			if i >= len(args) {
				return errors.New("--output requires a value")
			}
			output = args[i]
		case "--json":
			jsonOut = true
		default:
			fixtures = append(fixtures, args[i])
		}
	}
	rows, err := surfaceledger.BuildEvidenceSnapshot(fixtures)
	if err != nil {
		return err
	}
	return writeRows(rows, output, jsonOut, w)
}

func runCompatSurfaceLedger(args []string, w io.Writer) error {
	var docsPath, orgPath, gladePath, evidencePath, output string
	var err error
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--docs":
			i++
			docsPath, err = argValue(args, i, "--docs")
			if err != nil {
				return err
			}
		case "--org":
			i++
			orgPath, err = argValue(args, i, "--org")
			if err != nil {
				return err
			}
		case "--glade":
			i++
			gladePath, err = argValue(args, i, "--glade")
			if err != nil {
				return err
			}
		case "--evidence":
			i++
			evidencePath, err = argValue(args, i, "--evidence")
			if err != nil {
				return err
			}
		case "--output":
			i++
			output, err = argValue(args, i, "--output")
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	docsRows, err := surfaceledger.ReadRowsJSON(docsPath)
	if err != nil {
		return err
	}
	orgRows, err := surfaceledger.ReadRowsJSON(orgPath)
	if err != nil {
		return err
	}
	gladeRows, err := surfaceledger.ReadRowsJSON(gladePath)
	if err != nil {
		return err
	}
	evidenceRows, err := surfaceledger.ReadRowsJSON(evidencePath)
	if err != nil {
		return err
	}
	ledger := surfaceledger.Merge(docsRows, orgRows, gladeRows, evidenceRows)
	surfaceledger.AssignPriorities(ledger.Rows)
	ledger.Summary = surfaceledger.Summarize(ledger.Rows)
	if output != "" {
		var buf stringsBuilder
		if err := surfaceledger.WriteLedgerJSON(&buf, ledger); err != nil {
			return err
		}
		return os.WriteFile(output, []byte(buf.String()), 0o644)
	}
	return surfaceledger.WriteLedgerJSON(w, ledger)
}

func runCompatSurfacePacket(args []string, w io.Writer) error {
	ledgerPath := ""
	area := ""
	output := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ledger":
			i++
			var err error
			ledgerPath, err = argValue(args, i, "--ledger")
			if err != nil {
				return err
			}
		case "--area":
			i++
			var err error
			area, err = argValue(args, i, "--area")
			if err != nil {
				return err
			}
		case "--out", "--output":
			i++
			var err error
			output, err = argValue(args, i, args[i-1])
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	packet, ok := surfaceledger.AreaPacketByName(area)
	if !ok {
		return fmt.Errorf("unknown surface area %q", area)
	}
	ledger, err := surfaceledger.ReadLedgerJSON(ledgerPath)
	if err != nil {
		return err
	}
	markdown := surfaceledger.PacketMarkdown(ledger, packet)
	if output != "" {
		if err := os.MkdirAll(filepath.Dir(output), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(output, []byte(markdown), 0o644); err != nil {
			return err
		}
		fmt.Fprintf(w, "surface packet: %s -> %s\n", packet.Name, output)
		return nil
	}
	fmt.Fprint(w, markdown)
	return nil
}

func runCompatSurfaceGaps(args []string, w io.Writer) error {
	ledgerPath, output, err := parseLedgerOutput(args)
	if err != nil {
		return err
	}
	ledger, err := surfaceledger.ReadLedgerJSON(ledgerPath)
	if err != nil {
		return err
	}
	md := surfaceledger.GapsMarkdown(ledger)
	if output != "" {
		return os.WriteFile(output, []byte(md), 0o644)
	}
	fmt.Fprint(w, md)
	return nil
}

func runCompatSurfaceExplain(args []string, w io.Writer) error {
	ledgerPath := ""
	id := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ledger":
			i++
			var err error
			ledgerPath, err = argValue(args, i, "--ledger")
			if err != nil {
				return err
			}
		case "--id":
			i++
			var err error
			id, err = argValue(args, i, "--id")
			if err != nil {
				return err
			}
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	ledger, err := surfaceledger.ReadLedgerJSON(ledgerPath)
	if err != nil {
		return err
	}
	md := surfaceledger.ExplainMarkdown(ledger, id)
	if md == "" {
		return fmt.Errorf("surface %q not found", id)
	}
	fmt.Fprint(w, md)
	return nil
}

func runCompatSurfaceCheck(args []string, w io.Writer) error {
	ledgerPath := ""
	options := surfaceledger.CheckOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ledger":
			i++
			var err error
			ledgerPath, err = argValue(args, i, "--ledger")
			if err != nil {
				return err
			}
		case "--max-missing-shape":
			i++
			value, err := parseIntArg(args, i, "--max-missing-shape")
			if err != nil {
				return err
			}
			options.MaxMissingShape = value
		case "--max-missing-behavior":
			i++
			value, err := parseIntArg(args, i, "--max-missing-behavior")
			if err != nil {
				return err
			}
			options.MaxMissingBehavior = value
		case "--max-parser-failures":
			i++
			value, err := parseIntArg(args, i, "--max-parser-failures")
			if err != nil {
				return err
			}
			options.MaxParserFailures = value
		default:
			return fmt.Errorf("unknown flag %q", args[i])
		}
	}
	ledger, err := surfaceledger.ReadLedgerJSON(ledgerPath)
	if err != nil {
		return err
	}
	if err := surfaceledger.CheckLedger(ledger, options); err != nil {
		return err
	}
	fmt.Fprintln(w, "surface check: ok")
	return nil
}

func parseSourceOutputJSON(args []string, sourceFlag string) (string, string, bool, error) {
	source := ""
	output := ""
	jsonOut := false
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case sourceFlag:
			i++
			if i >= len(args) {
				return "", "", false, fmt.Errorf("%s requires a value", sourceFlag)
			}
			source = args[i]
		case "--output":
			i++
			if i >= len(args) {
				return "", "", false, errors.New("--output requires a value")
			}
			output = args[i]
		case "--json":
			jsonOut = true
		default:
			return "", "", false, fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return source, output, jsonOut, nil
}

func parseLedgerOutput(args []string) (string, string, error) {
	ledger := ""
	output := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--ledger":
			i++
			var err error
			ledger, err = argValue(args, i, "--ledger")
			if err != nil {
				return "", "", err
			}
		case "--output":
			i++
			var err error
			output, err = argValue(args, i, "--output")
			if err != nil {
				return "", "", err
			}
		default:
			return "", "", fmt.Errorf("unknown flag %q", args[i])
		}
	}
	return ledger, output, nil
}

func writeRows(rows []surfaceledger.SurfaceLedgerRow, output string, jsonOut bool, w io.Writer) error {
	if output != "" {
		var buf stringsBuilder
		if err := surfaceledger.WriteRowsJSON(&buf, rows); err != nil {
			return err
		}
		return os.WriteFile(output, []byte(buf.String()), 0o644)
	}
	if jsonOut {
		return surfaceledger.WriteRowsJSON(w, rows)
	}
	fmt.Fprintf(w, "rows=%d\n", len(rows))
	return nil
}

func argValue(args []string, i int, flag string) (string, error) {
	if i >= len(args) {
		return "", fmt.Errorf("%s requires a value", flag)
	}
	return args[i], nil
}

func parseIntArg(args []string, i int, flag string) (int, error) {
	value, err := argValue(args, i, flag)
	if err != nil {
		return 0, err
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s requires an integer", flag)
	}
	return parsed, nil
}

type stringsBuilder struct {
	data []byte
}

func (b *stringsBuilder) Write(data []byte) (int, error) {
	b.data = append(b.data, data...)
	return len(data), nil
}

func (b *stringsBuilder) String() string {
	return string(b.data)
}
