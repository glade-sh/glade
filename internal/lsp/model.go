package lsp

import (
	"net/url"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/diagnostic"
)

type DocumentURI string

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   DocumentURI `json:"uri"`
	Range Range       `json:"range"`
}

type TextDocumentIdentifier struct {
	URI DocumentURI `json:"uri"`
}

type TextDocumentItem struct {
	URI        DocumentURI `json:"uri"`
	LanguageID string      `json:"languageId,omitempty"`
	Version    int         `json:"version,omitempty"`
	Text       string      `json:"text"`
}

type VersionedTextDocumentIdentifier struct {
	URI     DocumentURI `json:"uri"`
	Version int         `json:"version,omitempty"`
}

type InitializeParams struct {
	RootURI string `json:"rootUri,omitempty"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   *ServerInfo        `json:"serverInfo,omitempty"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

type ServerCapabilities struct {
	TextDocumentSync        int                   `json:"textDocumentSync,omitempty"`
	DocumentSymbolProvider  bool                  `json:"documentSymbolProvider"`
	WorkspaceSymbolProvider bool                  `json:"workspaceSymbolProvider"`
	HoverProvider           bool                  `json:"hoverProvider"`
	CompletionProvider      CompletionOptions     `json:"completionProvider"`
	DefinitionProvider      bool                  `json:"definitionProvider"`
	ReferencesProvider      bool                  `json:"referencesProvider"`
	RenameProvider          RenameOptions         `json:"renameProvider"`
	SemanticTokensProvider  SemanticTokensOptions `json:"semanticTokensProvider"`
}

type CompletionOptions struct {
	ResolveProvider   bool     `json:"resolveProvider"`
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

type RenameOptions struct {
	PrepareProvider bool `json:"prepareProvider"`
}

type SemanticTokensOptions struct {
	Legend SemanticTokensLegend `json:"legend"`
	Full   bool                 `json:"full"`
}

type SemanticTokensLegend struct {
	TokenTypes     []string `json:"tokenTypes"`
	TokenModifiers []string `json:"tokenModifiers"`
}

type PublishDiagnosticsParams struct {
	URI         DocumentURI  `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Diagnostic struct {
	Range    Range  `json:"range"`
	Severity int    `json:"severity,omitempty"`
	Code     string `json:"code,omitempty"`
	Source   string `json:"source,omitempty"`
	Message  string `json:"message"`
}

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           int              `json:"kind"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

type SymbolInformation struct {
	Name          string   `json:"name"`
	Kind          int      `json:"kind"`
	Location      Location `json:"location"`
	ContainerName string   `json:"containerName,omitempty"`
}

type HoverParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

type CompletionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument,omitempty"`
	Position     Position               `json:"position,omitempty"`
}

type DefinitionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

type ReferenceParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	Context      ReferenceContext       `json:"context,omitempty"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

type RenameParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
	NewName      string                 `json:"newName"`
}

type SemanticTokensParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type SemanticTokens struct {
	Data []int `json:"data"`
}

type TextEdit struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

type WorkspaceEdit struct {
	Changes map[DocumentURI][]TextEdit `json:"changes"`
}

type CompletionList struct {
	IsIncomplete bool             `json:"isIncomplete"`
	Items        []CompletionItem `json:"items"`
}

type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind,omitempty"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
}

type Notification struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
}

const (
	textDocumentSyncFull        = 1
	textDocumentSyncIncremental = 2

	diagnosticSeverityError       = 1
	diagnosticSeverityWarning     = 2
	diagnosticSeverityInformation = 3

	symbolKindClass       = 5
	symbolKindMethod      = 6
	symbolKindProperty    = 7
	symbolKindField       = 8
	symbolKindConstructor = 9
	symbolKindEnum        = 10
	symbolKindInterface   = 11
	symbolKindObject      = 19
	symbolKindEvent       = 24

	completionItemKindClass     = 7
	completionItemKindMethod    = 2
	completionItemKindField     = 5
	completionItemKindProperty  = 10
	completionItemKindInterface = 8
	completionItemKindEnum      = 13
	completionItemKindKeyword   = 14
	completionItemKindStruct    = 22
)

func uriFromPath(path string) DocumentURI {
	if path == "" {
		return ""
	}
	if hasScheme(path) {
		return DocumentURI(path)
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	return DocumentURI((&url.URL{Scheme: "file", Path: filepath.ToSlash(path)}).String())
}

func pathFromURI(uri DocumentURI) string {
	raw := string(uri)
	if raw == "" || !hasScheme(raw) {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "file" {
		return raw
	}
	return filepath.FromSlash(parsed.Path)
}

func schemaURI(parts ...string) DocumentURI {
	escaped := make([]string, 0, len(parts))
	for _, part := range parts {
		escaped = append(escaped, url.PathEscape(part))
	}
	return DocumentURI("glade-schema:///" + strings.Join(escaped, "/"))
}

func hasScheme(value string) bool {
	i := strings.Index(value, ":")
	if i <= 0 {
		return false
	}
	for _, r := range value[:i] {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '+' || r == '-' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func sameDocument(a, b string) bool {
	if a == "" || b == "" {
		return false
	}
	if a == b {
		return true
	}
	ap := filepath.Clean(pathFromURI(DocumentURI(a)))
	bp := filepath.Clean(pathFromURI(DocumentURI(b)))
	if abs, err := filepath.Abs(ap); err == nil {
		ap = abs
	}
	if abs, err := filepath.Abs(bp); err == nil {
		bp = abs
	}
	return ap == bp
}

func toLSPRange(r diagnostic.Range) Range {
	return Range{
		Start: toLSPPosition(r.Start),
		End:   toLSPPosition(r.End),
	}
}

func toLSPPosition(p diagnostic.Position) Position {
	line := p.Line
	if line > 0 {
		line--
	}
	character := p.Column
	if character > 0 {
		character--
	}
	return Position{Line: line, Character: character}
}

func toDiagnosticPosition(p Position) diagnostic.Position {
	return diagnostic.Position{Line: p.Line + 1, Column: p.Character + 1}
}

func containsPosition(r diagnostic.Range, p Position) bool {
	pos := toDiagnosticPosition(p)
	if r.Start.Line == 0 && r.End.Line == 0 {
		return false
	}
	if comparePosition(pos, r.Start) < 0 {
		return false
	}
	if r.End.Line == 0 {
		return true
	}
	return comparePosition(pos, r.End) < 0
}

func comparePosition(a, b diagnostic.Position) int {
	if a.Line != b.Line {
		if a.Line < b.Line {
			return -1
		}
		return 1
	}
	if a.Column != b.Column {
		if a.Column < b.Column {
			return -1
		}
		return 1
	}
	return 0
}

func lspSeverity(severity diagnostic.Severity) int {
	switch severity {
	case diagnostic.Error:
		return diagnosticSeverityError
	case diagnostic.Warning:
		return diagnosticSeverityWarning
	case diagnostic.Info:
		return diagnosticSeverityInformation
	default:
		return diagnosticSeverityInformation
	}
}

func symbolKind(kind apexast.DeclarationKind) int {
	switch kind {
	case apexast.DeclarationClass:
		return symbolKindClass
	case apexast.DeclarationInterface:
		return symbolKindInterface
	case apexast.DeclarationEnum:
		return symbolKindEnum
	case apexast.DeclarationMethod:
		return symbolKindMethod
	case apexast.DeclarationConstructor:
		return symbolKindConstructor
	case apexast.DeclarationField:
		return symbolKindField
	case apexast.DeclarationProperty:
		return symbolKindProperty
	case apexast.DeclarationTrigger:
		return symbolKindEvent
	default:
		return symbolKindObject
	}
}

func completionKind(kind apexast.DeclarationKind) int {
	switch kind {
	case apexast.DeclarationClass:
		return completionItemKindClass
	case apexast.DeclarationInterface:
		return completionItemKindInterface
	case apexast.DeclarationEnum:
		return completionItemKindEnum
	case apexast.DeclarationMethod, apexast.DeclarationConstructor:
		return completionItemKindMethod
	case apexast.DeclarationField:
		return completionItemKindField
	case apexast.DeclarationProperty:
		return completionItemKindProperty
	default:
		return completionItemKindClass
	}
}

func sortSymbols(symbols []SymbolInformation) {
	sort.Slice(symbols, func(i, j int) bool {
		if symbols[i].Name == symbols[j].Name {
			return symbols[i].ContainerName < symbols[j].ContainerName
		}
		return symbols[i].Name < symbols[j].Name
	})
}

func sortCompletionItems(items []CompletionItem, context completionContext) {
	sort.Slice(items, func(i, j int) bool {
		if context != completionContextDefault {
			leftRank := completionItemContextRank(items[i], context)
			rightRank := completionItemContextRank(items[j], context)
			if leftRank != rightRank {
				return leftRank < rightRank
			}
		}
		return items[i].Label < items[j].Label
	})
}

func completionItemContextRank(item CompletionItem, context completionContext) int {
	switch context {
	case completionContextSOQLSelect:
		if item.Kind == completionItemKindField {
			return 0
		}
	}
	return 1
}
