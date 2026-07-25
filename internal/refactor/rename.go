package refactor

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	apexparser "github.com/glade-sh/apex-parser"
	"github.com/glade-sh/glade/internal/codeintel"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

type RenameOptions struct {
	Symbol   string             `json:"symbol,omitempty"`
	SymbolID codeintel.SymbolID `json:"symbolId,omitempty"`
	File     string             `json:"file,omitempty"`
	Line     int                `json:"line,omitempty"`
	Column   int                `json:"column,omitempty"`
	To       string             `json:"to"`
	DryRun   bool               `json:"dryRun,omitempty"`
}

type RenamePlan struct {
	ProjectRoot string           `json:"projectRoot,omitempty"`
	Symbol      codeintel.Symbol `json:"symbol"`
	From        string           `json:"from"`
	To          string           `json:"to"`
	DryRun      bool             `json:"dryRun"`
	Edits       []FileEdit       `json:"edits"`
}

type FileEdit struct {
	File         string           `json:"file"`
	Range        diagnostic.Range `json:"range"`
	Original     string           `json:"original"`
	Replacement  string           `json:"replacement"`
	StartOffset  int              `json:"startOffset"`
	EndOffset    int              `json:"endOffset"`
	OriginalHash string           `json:"originalHash"`
}

var apexIdentifierRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func PlanRename(index typesys.Index, opts RenameOptions) (RenamePlan, error) {
	to := strings.TrimSpace(opts.To)
	if to == "" {
		return RenamePlan{}, errors.New("rename target is required")
	}
	graph := codeintel.Build(index, codeintel.Options{IncludeUnresolved: true, UseCache: true})
	symbol, err := resolveRenameSymbol(graph, opts)
	if err != nil {
		return RenamePlan{}, err
	}
	if err := validateRenameTarget(symbol, to); err != nil {
		return RenamePlan{}, err
	}
	if err := ensureNoUnresolvedReferences(graph, symbol); err != nil {
		return RenamePlan{}, err
	}
	refs := graph.References(symbol.ID, true)
	if len(refs) == 0 {
		return RenamePlan{}, fmt.Errorf("symbol %q has no resolved references", symbol.Name)
	}
	edits, err := buildRenameEdits(index.Project.Root, symbol, refs, to)
	if err != nil {
		return RenamePlan{}, err
	}
	if len(edits) == 0 {
		return RenamePlan{}, fmt.Errorf("symbol %q has no writable references", symbol.Name)
	}
	if err := ensureNoOverlappingEdits(edits); err != nil {
		return RenamePlan{}, err
	}
	return RenamePlan{
		ProjectRoot: index.Project.Root,
		Symbol:      symbol,
		From:        symbol.Name,
		To:          to,
		DryRun:      opts.DryRun,
		Edits:       edits,
	}, nil
}

func Apply(plan RenamePlan) error {
	if plan.Symbol.File == "" && (plan.Symbol.Kind == codeintel.SymbolSObject || plan.Symbol.Kind == codeintel.SymbolSObjectField) {
		return fmt.Errorf("schema rename write requires a metadata declaration location for %s", plan.Symbol.Name)
	}
	if err := ensureNoOverlappingEdits(plan.Edits); err != nil {
		return err
	}
	byFile := make(map[string][]FileEdit)
	for _, edit := range plan.Edits {
		byFile[edit.File] = append(byFile[edit.File], edit)
	}
	files := make([]string, 0, len(byFile))
	for file := range byFile {
		files = append(files, file)
	}
	sort.Strings(files)
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			return err
		}
		hash := contentHash(data)
		for _, edit := range byFile[file] {
			if edit.OriginalHash != "" && edit.OriginalHash != hash {
				return fmt.Errorf("%s changed between plan and write", file)
			}
		}
		edits := append([]FileEdit(nil), byFile[file]...)
		sort.Slice(edits, func(i, j int) bool {
			return edits[i].StartOffset > edits[j].StartOffset
		})
		out := append([]byte(nil), data...)
		for _, edit := range edits {
			if edit.StartOffset < 0 || edit.EndOffset > len(out) || edit.StartOffset > edit.EndOffset {
				return fmt.Errorf("edit range for %s is outside file bounds", file)
			}
			if string(out[edit.StartOffset:edit.EndOffset]) != edit.Original {
				return fmt.Errorf("%s changed between plan and write", file)
			}
			next := make([]byte, 0, len(out)-len(edit.Original)+len(edit.Replacement))
			next = append(next, out[:edit.StartOffset]...)
			next = append(next, edit.Replacement...)
			next = append(next, out[edit.EndOffset:]...)
			out = next
		}
		if err := os.WriteFile(file, out, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func resolveRenameSymbol(graph codeintel.Graph, opts RenameOptions) (codeintel.Symbol, error) {
	if opts.SymbolID != "" {
		symbol, ok := graph.Definition(opts.SymbolID)
		if !ok {
			return codeintel.Symbol{}, fmt.Errorf("symbol id %q not found", opts.SymbolID)
		}
		return ensureRenameSupported(symbol)
	}
	query := strings.TrimSpace(opts.Symbol)
	if query != "" {
		if symbol, ok := graph.Definition(codeintel.SymbolID(query)); ok {
			return ensureRenameSupported(symbol)
		}
		var matches []codeintel.Symbol
		for _, symbol := range graph.SortedSymbols() {
			if renameSymbolMatchesQuery(graph, symbol, query) {
				if supportedRenameKind(symbol.Kind) {
					matches = append(matches, symbol)
				}
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
	if opts.File != "" || opts.Line != 0 || opts.Column != 0 {
		if opts.File == "" || opts.Line <= 0 || opts.Column <= 0 {
			return codeintel.Symbol{}, errors.New("location rename requires --file, --line, and --column")
		}
		id, ok := findRenameSymbolAtLocation(graph, opts.File, opts.Line, opts.Column)
		if !ok {
			return codeintel.Symbol{}, fmt.Errorf("no symbol found at %s:%d:%d", opts.File, opts.Line, opts.Column)
		}
		symbol, ok := graph.Definition(id)
		if !ok {
			return codeintel.Symbol{}, fmt.Errorf("no definition found for symbol at %s:%d:%d", opts.File, opts.Line, opts.Column)
		}
		return ensureRenameSupported(symbol)
	}
	return codeintel.Symbol{}, errors.New("rename requires --symbol or --file --line --column")
}

func ensureRenameSupported(symbol codeintel.Symbol) (codeintel.Symbol, error) {
	if !supportedRenameKind(symbol.Kind) {
		return codeintel.Symbol{}, fmt.Errorf("rename does not support symbol kind %s", symbol.Kind)
	}
	return symbol, nil
}

func supportedRenameKind(kind codeintel.SymbolKind) bool {
	switch kind {
	case codeintel.SymbolApexType, codeintel.SymbolApexMember, codeintel.SymbolSObject, codeintel.SymbolSObjectField:
		return true
	default:
		return false
	}
}

func renameSymbolMatchesQuery(graph codeintel.Graph, symbol codeintel.Symbol, query string) bool {
	if symbol.Name == query {
		return true
	}
	parts := strings.Split(query, ".")
	if len(parts) != 2 || parts[1] != symbol.Name {
		return false
	}
	switch symbol.Kind {
	case codeintel.SymbolSObjectField, codeintel.SymbolApexMember:
		container, ok := graph.Definition(symbol.Container)
		return ok && container.Name == parts[0]
	default:
		return false
	}
}

func findRenameSymbolAtLocation(graph codeintel.Graph, file string, line, column int) (codeintel.SymbolID, bool) {
	normalized := normalizeRenamePath(file)
	var best codeintel.SymbolID
	bestWidth := int(^uint(0) >> 1)
	for _, use := range graph.Uses {
		if use.SymbolID == "" || !use.Resolved {
			continue
		}
		if sameRenamePath(graph.ProjectRoot, use.File, normalized) && rangeContains(use.Range, line, column) {
			if width := rangeWidth(use.Range); width < bestWidth {
				best = use.SymbolID
				bestWidth = width
			}
		}
	}
	for _, symbol := range graph.SortedSymbols() {
		if sameRenamePath(graph.ProjectRoot, symbol.File, normalized) && rangeContains(symbol.Range, line, column) {
			if width := rangeWidth(symbol.Range); width < bestWidth {
				best = symbol.ID
				bestWidth = width
			}
		}
	}
	return best, best != ""
}

func validateRenameTarget(symbol codeintel.Symbol, to string) error {
	switch symbol.Kind {
	case codeintel.SymbolSObject, codeintel.SymbolSObjectField:
		// Schema/API names may contain consecutive underscores (e.g. Amount__c).
		// Do not apply Apex source-identifier shape rules here.
		if !apexIdentifierRE.MatchString(to) {
			return fmt.Errorf("invalid Apex identifier %q", to)
		}
		if !validSchemaSuffix(symbol.Name, to) {
			return fmt.Errorf("invalid custom schema suffix for %s -> %s", symbol.Name, to)
		}
		return nil
	default:
		if err := apexparser.ValidateSourceIdentifier(to); err != nil {
			return err
		}
		methodName := symbol.Kind == codeintel.SymbolApexMember && symbol.Metadata["declarationKind"] == "method"
		if apexparser.IsReservedSourceIdentifier(to, methodName) {
			return fmt.Errorf("Invalid identifier: %s", to)
		}
		return nil
	}
}

func validSchemaSuffix(from, to string) bool {
	fromSuffix := schemaSuffix(from)
	toSuffix := schemaSuffix(to)
	if fromSuffix == "" {
		return toSuffix == ""
	}
	return fromSuffix == toSuffix
}

func schemaSuffix(name string) string {
	for _, suffix := range []string{"__mdt", "__b", "__e", "__x", "__c"} {
		if strings.HasSuffix(name, suffix) {
			return suffix
		}
	}
	return ""
}

func ensureNoUnresolvedReferences(graph codeintel.Graph, symbol codeintel.Symbol) error {
	for _, use := range graph.Uses {
		if use.Resolved {
			continue
		}
		if strings.EqualFold(use.Name, symbol.Name) {
			return fmt.Errorf("unresolved reference %q at %s:%d:%d blocks rename", use.Name, use.File, use.Range.Start.Line, use.Range.Start.Column)
		}
	}
	return nil
}

func buildRenameEdits(projectRoot string, symbol codeintel.Symbol, refs []codeintel.Use, replacement string) ([]FileEdit, error) {
	var edits []FileEdit
	hashes := make(map[string]string)
	contents := make(map[string][]byte)
	for _, ref := range refs {
		if !ref.Resolved || ref.SymbolID != symbol.ID || ref.File == "" {
			continue
		}
		if ref.Kind == codeintel.UseQuery && rangeWidth(ref.Range) != len(ref.Name) {
			return nil, fmt.Errorf("%s:%d:%d: query reference %q has query-level range; rename is unsafe", ref.File, ref.Range.Start.Line, ref.Range.Start.Column, ref.Name)
		}
		file := sourcePath(projectRoot, ref.File)
		data, ok := contents[file]
		if !ok {
			var err error
			data, err = os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			contents[file] = data
			hashes[file] = contentHash(data)
		}
		start, end, err := editOffsets(data, ref.Range, ref.Name)
		if err != nil {
			return nil, fmt.Errorf("%s:%d:%d: %w", file, ref.Range.Start.Line, ref.Range.Start.Column, err)
		}
		edits = append(edits, FileEdit{
			File:         file,
			Range:        rangeFromOffsets(string(data), start, end),
			Original:     string(data[start:end]),
			Replacement:  replacement,
			StartOffset:  start,
			EndOffset:    end,
			OriginalHash: hashes[file],
		})
	}
	edits = dedupeExactEdits(edits)
	sort.Slice(edits, func(i, j int) bool {
		if edits[i].File == edits[j].File {
			return edits[i].StartOffset < edits[j].StartOffset
		}
		return edits[i].File < edits[j].File
	})
	return edits, nil
}

func dedupeExactEdits(edits []FileEdit) []FileEdit {
	seen := make(map[string]struct{}, len(edits))
	out := make([]FileEdit, 0, len(edits))
	for _, edit := range edits {
		key := fmt.Sprintf("%s:%d:%d:%s", edit.File, edit.StartOffset, edit.EndOffset, edit.Replacement)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, edit)
	}
	return out
}

func editOffsets(data []byte, r diagnostic.Range, name string) (int, int, error) {
	start, end := r.Start.Offset, r.End.Offset
	if start >= 0 && end > start && end <= len(data) {
		if string(data[start:end]) == name {
			return start, end, nil
		}
		if start < len(data) {
			searchEnd := end
			if searchEnd > len(data) {
				searchEnd = len(data)
			}
			if idx := strings.Index(string(data[start:searchEnd]), name); idx >= 0 {
				return start + idx, start + idx + len(name), nil
			}
		}
	}
	if r.Start.Line <= 0 || r.Start.Column <= 0 {
		return 0, 0, fmt.Errorf("missing offset for %q", name)
	}
	lineStart, ok := offsetForLineColumn(string(data), r.Start.Line, r.Start.Column)
	if !ok {
		return 0, 0, fmt.Errorf("range outside file for %q", name)
	}
	lineEnd := lineStart + len(name)
	if lineEnd <= len(data) && string(data[lineStart:lineEnd]) == name {
		return lineStart, lineEnd, nil
	}
	windowEnd := lineStart + max(0, rangeWidth(r))
	if windowEnd > len(data) || windowEnd <= lineStart {
		windowEnd = min(len(data), lineStart+len(name)+160)
	}
	if idx := strings.Index(string(data[lineStart:windowEnd]), name); idx >= 0 {
		return lineStart + idx, lineStart + idx + len(name), nil
	}
	return 0, 0, fmt.Errorf("planned text %q does not match file", name)
}

func ensureNoOverlappingEdits(edits []FileEdit) error {
	byFile := make(map[string][]FileEdit)
	for _, edit := range edits {
		byFile[edit.File] = append(byFile[edit.File], edit)
	}
	for file, fileEdits := range byFile {
		sort.Slice(fileEdits, func(i, j int) bool {
			return fileEdits[i].StartOffset < fileEdits[j].StartOffset
		})
		for i := 1; i < len(fileEdits); i++ {
			if fileEdits[i].StartOffset < fileEdits[i-1].EndOffset {
				return fmt.Errorf("overlapping edits in %s", file)
			}
		}
	}
	return nil
}

func rangeFromOffsets(source string, start, end int) diagnostic.Range {
	return diagnostic.Range{
		Start: positionForOffset(source, start),
		End:   positionForOffset(source, end),
	}
}

func positionForOffset(source string, offset int) diagnostic.Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}
	line, column := 1, 1
	for i, ch := range source {
		if i >= offset {
			break
		}
		if ch == '\n' {
			line++
			column = 1
			continue
		}
		column++
	}
	return diagnostic.Position{Line: line, Column: column, Offset: offset}
}

func offsetForLineColumn(source string, line, column int) (int, bool) {
	if line <= 0 || column <= 0 {
		return 0, false
	}
	currentLine, currentColumn := 1, 1
	for offset, ch := range source {
		if currentLine == line && currentColumn == column {
			return offset, true
		}
		if ch == '\n' {
			currentLine++
			currentColumn = 1
			continue
		}
		currentColumn++
	}
	if currentLine == line && currentColumn == column {
		return len(source), true
	}
	return 0, false
}

func sameRenamePath(projectRoot, candidate, normalizedQuery string) bool {
	normalizedCandidate := normalizeRenamePath(candidate)
	if normalizedCandidate == normalizedQuery {
		return true
	}
	if projectRoot == "" {
		return false
	}
	rel, err := filepath.Rel(projectRoot, sourcePath(projectRoot, candidate))
	if err != nil {
		return false
	}
	return normalizeRenamePath(rel) == normalizedQuery
}

func sourcePath(root, path string) string {
	if filepath.IsAbs(path) || root == "" {
		return path
	}
	return filepath.Join(root, path)
}

func normalizeRenamePath(path string) string {
	return filepath.ToSlash(filepath.Clean(path))
}

func rangeContains(r diagnostic.Range, line, column int) bool {
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

func rangeWidth(r diagnostic.Range) int {
	if r.Start.Offset != 0 || r.End.Offset != 0 {
		return r.End.Offset - r.Start.Offset
	}
	return (r.End.Line-r.Start.Line)*10000 + (r.End.Column - r.Start.Column)
}

func contentHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
