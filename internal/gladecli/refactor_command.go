package gladecli

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/cliui"
	"github.com/glade-sh/glade/internal/flagparse"
	"github.com/glade-sh/glade/internal/refactor"
)

type refactorRenameFlags struct {
	root    string
	symbol  string
	file    string
	line    int
	column  int
	to      string
	write   bool
	dryRun  bool
	jsonOut bool
}

type refactorRenameData struct {
	Symbol string                      `json:"symbol"`
	From   string                      `json:"from"`
	To     string                      `json:"to"`
	DryRun bool                        `json:"dryRun"`
	Count  int                         `json:"count"`
	Edits  []refactorRenameEditSummary `json:"edits"`
}

type refactorRenameEditSummary struct {
	File        string `json:"file"`
	Line        int    `json:"line"`
	Column      int    `json:"column"`
	Original    string `json:"original"`
	Replacement string `json:"replacement"`
}

func runRefactor(args []string, w io.Writer) error {
	if len(args) == 0 {
		return errors.New("usage: glade refactor rename --project <root> --symbol <name> --to <name> [--dry-run|--write] [--json]")
	}
	switch args[0] {
	case "rename":
		return runRefactorRename(args[1:], w)
	default:
		return fmt.Errorf("unknown refactor command %q", args[0])
	}
}

func runRefactorRename(args []string, w io.Writer) error {
	flags, err := parseRefactorRenameFlags(args)
	if err != nil {
		return err
	}
	index, err := loadIndex(flags.root)
	if err != nil {
		return err
	}
	plan, err := refactor.PlanRename(index, refactor.RenameOptions{
		Symbol: flags.symbol,
		File:   flags.file,
		Line:   flags.line,
		Column: flags.column,
		To:     flags.to,
		DryRun: flags.dryRun,
	})
	if err != nil {
		return err
	}
	plan.DryRun = flags.dryRun
	if flags.write {
		if err := refactor.Apply(plan); err != nil {
			return err
		}
	}
	data := refactorRenamePlanData(plan)
	if flags.jsonOut {
		return writeCLIJSONEnvelope(w, cliJSONEnvelope{
			Command:  "refactor rename",
			Status:   "passed",
			ExitCode: 0,
			Project:  index.Project,
			Data:     data,
		})
	}
	writeRefactorRenameText(w, data)
	return nil
}

func parseRefactorRenameFlags(args []string) (refactorRenameFlags, error) {
	parsed, err := flagparse.New("glade refactor rename").
		String("project", "p").
		String("symbol", "").
		String("file", "").
		String("line", "").
		String("column", "").
		String("to", "").
		Bool("dry-run", "").
		Bool("write", "").
		Bool("json", "j").
		Parse(args)
	if err != nil {
		return refactorRenameFlags{}, err
	}
	line, err := parseRefactorPositiveInt(parsed.String("line"), "line")
	if err != nil {
		return refactorRenameFlags{}, err
	}
	column, err := parseRefactorPositiveInt(parsed.String("column"), "column")
	if err != nil {
		return refactorRenameFlags{}, err
	}
	flags := refactorRenameFlags{
		root:    ".",
		symbol:  strings.TrimSpace(parsed.String("symbol")),
		file:    strings.TrimSpace(parsed.String("file")),
		line:    line,
		column:  column,
		to:      strings.TrimSpace(parsed.String("to")),
		write:   parsed.Bool("write"),
		dryRun:  !parsed.Bool("write"),
		jsonOut: parsed.Bool("json"),
	}
	if parsed.String("project") != "" {
		flags.root = parsed.String("project")
	}
	if parsed.Bool("dry-run") && parsed.Bool("write") {
		return refactorRenameFlags{}, errors.New("glade refactor rename accepts only one of --dry-run or --write")
	}
	if parsed.Bool("dry-run") {
		flags.dryRun = true
	}
	hasSymbol := flags.symbol != ""
	hasLocation := flags.file != "" || flags.line != 0 || flags.column != 0
	if hasSymbol == hasLocation {
		return refactorRenameFlags{}, errors.New("usage: glade refactor rename --project <root> --symbol <name> --to <name> [--dry-run|--write] [--json] | --file <path> --line <n> --column <n> --to <name> [--dry-run|--write] [--json]")
	}
	if hasLocation && (flags.file == "" || flags.line <= 0 || flags.column <= 0) {
		return refactorRenameFlags{}, errors.New("usage: glade refactor rename --file <path> --line <n> --column <n> --to <name> [--project <root>] [--dry-run|--write] [--json]")
	}
	if flags.to == "" {
		return refactorRenameFlags{}, errors.New("glade refactor rename requires --to <name>")
	}
	return flags, nil
}

func parseRefactorPositiveInt(value, name string) (int, error) {
	if value == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("--%s must be a positive integer", name)
	}
	return n, nil
}

func refactorRenamePlanData(plan refactor.RenamePlan) refactorRenameData {
	edits := make([]refactorRenameEditSummary, 0, len(plan.Edits))
	for _, edit := range plan.Edits {
		edits = append(edits, refactorRenameEditSummary{
			File:        cliui.ProjectRelativePath(plan.ProjectRoot, edit.File),
			Line:        edit.Range.Start.Line,
			Column:      edit.Range.Start.Column,
			Original:    edit.Original,
			Replacement: edit.Replacement,
		})
	}
	return refactorRenameData{
		Symbol: string(plan.Symbol.ID),
		From:   plan.From,
		To:     plan.To,
		DryRun: plan.DryRun,
		Count:  len(edits),
		Edits:  edits,
	}
}

func writeRefactorRenameText(w io.Writer, data refactorRenameData) {
	fmt.Fprintln(w, "Rename")
	fmt.Fprintf(w, "  symbol: %s\n", data.Symbol)
	fmt.Fprintf(w, "  from: %s\n", data.From)
	fmt.Fprintf(w, "  to: %s\n", data.To)
	if data.DryRun {
		fmt.Fprintln(w, "  mode: dry-run")
	} else {
		fmt.Fprintln(w, "  mode: write")
	}
	fmt.Fprintf(w, "  edits: %d\n", data.Count)
	for _, edit := range data.Edits {
		fmt.Fprintf(w, "  %s:%d:%d %s -> %s\n", edit.File, edit.Line, edit.Column, edit.Original, edit.Replacement)
	}
}
