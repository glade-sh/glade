package lsp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/apexast"
	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/testreport"
	"github.com/open-aer/oaer/internal/typesys"
)

type Handler struct {
	index     typesys.Index
	analysis  sema.Result
	documents map[DocumentURI]openDocument
	shutdown  bool
}

type openDocument struct {
	URI     DocumentURI
	Path    string
	Version int
	Text    string
}

func NewHandler(index typesys.Index) *Handler {
	return &Handler{
		index:     index,
		analysis:  sema.Analyze(index),
		documents: make(map[DocumentURI]openDocument),
	}
}

func (h *Handler) Shutdown() bool {
	return h.shutdown
}

func (h *Handler) Initialize(_ InitializeParams) InitializeResult {
	return InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:        textDocumentSyncIncremental,
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			HoverProvider:           true,
			CompletionProvider: CompletionOptions{
				ResolveProvider:   false,
				TriggerCharacters: []string{".", "_"},
			},
			DefinitionProvider:     true,
			ReferencesProvider:     true,
			RenameProvider:         RenameOptions{PrepareProvider: true},
			SemanticTokensProvider: SemanticTokensOptions{Legend: semanticTokensLegend(), Full: true},
		},
		ServerInfo: &ServerInfo{Name: "oaer"},
	}
}

func (h *Handler) DidOpen(params DidOpenTextDocumentParams) []Notification {
	uri := params.TextDocument.URI
	doc := openDocument{
		URI:     uri,
		Path:    pathFromURI(uri),
		Version: params.TextDocument.Version,
		Text:    params.TextDocument.Text,
	}
	h.documents[uri] = doc
	return h.documentDiagnostics(doc)
}

func (h *Handler) DidChange(params DidChangeTextDocumentParams) ([]Notification, error) {
	doc, ok := h.documents[params.TextDocument.URI]
	if !ok {
		doc = openDocument{URI: params.TextDocument.URI, Path: pathFromURI(params.TextDocument.URI)}
	}
	doc.Version = params.TextDocument.Version
	for _, change := range params.ContentChanges {
		next, err := applyTextChange(doc.Text, change)
		if err != nil {
			return nil, err
		}
		doc.Text = next
	}
	h.documents[doc.URI] = doc
	return h.documentDiagnostics(doc), nil
}

func (h *Handler) DidClose(params DidCloseTextDocumentParams) []Notification {
	delete(h.documents, params.TextDocument.URI)
	return []Notification{{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  BuildPublishDiagnostics(params.TextDocument.URI, h.diagnosticsForDocument(params.TextDocument.URI)),
	}}
}

func (h *Handler) documentDiagnostics(doc openDocument) []Notification {
	parser := apexast.NewParser()
	file := parser.ParseSource(doc.Path, doc.Text)
	diagnostics := file.Diagnostics
	if len(diagnostics) == 0 {
		diagnostics = h.diagnosticsForDocument(doc.URI)
	}
	return []Notification{{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  BuildPublishDiagnostics(doc.URI, diagnostics),
	}}
}

func (h *Handler) diagnosticsForDocument(uri DocumentURI) []diagnostic.Diagnostic {
	path := pathFromURI(uri)
	var diagnostics []diagnostic.Diagnostic
	for _, diag := range h.analysis.Diagnostics {
		if diag.File != "" && (sameDocument(path, diag.File) || sameDocument(string(uri), diag.File)) {
			diagnostics = append(diagnostics, diag)
		}
	}
	return diagnostics
}

func (h *Handler) PublishDiagnostics(diagnostics []diagnostic.Diagnostic) []Notification {
	grouped := make(map[string][]diagnostic.Diagnostic)
	for _, diag := range diagnostics {
		if diag.File == "" {
			continue
		}
		uri := string(uriFromPath(diag.File))
		grouped[uri] = append(grouped[uri], diag)
	}

	uris := make([]string, 0, len(grouped))
	for uri := range grouped {
		uris = append(uris, uri)
	}
	sort.Strings(uris)

	notifications := make([]Notification, 0, len(uris))
	for _, uri := range uris {
		notifications = append(notifications, Notification{
			JSONRPC: "2.0",
			Method:  "textDocument/publishDiagnostics",
			Params:  BuildPublishDiagnostics(DocumentURI(uri), grouped[uri]),
		})
	}
	return notifications
}

func (h *Handler) PublishTestDiagnostics(run testreport.Run) []Notification {
	return h.PublishDiagnostics(TestDiagnostics(run))
}

func TestDiagnostics(run testreport.Run) []diagnostic.Diagnostic {
	var diagnostics []diagnostic.Diagnostic
	for _, suite := range run.Suites {
		for _, testCase := range suite.Cases {
			if testCase.Status == testreport.StatusPass || testCase.Status == testreport.StatusSkipped || testCase.Problem == nil {
				continue
			}
			frame := firstProblemFrame(testCase.Problem.Stack)
			if frame.File == "" {
				continue
			}
			message := testCase.Problem.Message
			if message == "" {
				message = string(testCase.Status)
			}
			name := testCase.ClassName
			if testCase.MethodName != "" {
				name += "." + testCase.MethodName
			}
			if name != "" {
				message = name + ": " + message
			}
			diag := diagnostic.Diagnostic{
				Severity: diagnostic.Error,
				Code:     "OAERTEST001",
				Message:  message,
				File:     frame.File,
			}
			if frame.Line > 0 {
				start := diagnostic.Position{Line: frame.Line, Column: frame.Column}
				if start.Column <= 0 {
					start.Column = 1
				}
				diag.Range = &diagnostic.Range{
					Start: start,
					End:   diagnostic.Position{Line: start.Line, Column: start.Column + 1},
				}
			}
			diagnostics = append(diagnostics, diag)
		}
	}
	return diagnostics
}

func firstProblemFrame(frames []testreport.StackFrame) testreport.StackFrame {
	for _, frame := range frames {
		if frame.File != "" {
			return frame
		}
	}
	return testreport.StackFrame{}
}

func BuildPublishDiagnostics(uri DocumentURI, diagnostics []diagnostic.Diagnostic) PublishDiagnosticsParams {
	out := PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: make([]Diagnostic, 0, len(diagnostics)),
	}
	for _, diag := range diagnostics {
		if diag.File != "" && !sameDocument(string(uri), diag.File) {
			continue
		}
		lspDiag := Diagnostic{
			Severity: lspSeverity(diag.Severity),
			Code:     diag.Code,
			Source:   "oaer",
			Message:  diag.Message,
		}
		if diag.Range != nil {
			lspDiag.Range = toLSPRange(*diag.Range)
		}
		out.Diagnostics = append(out.Diagnostics, lspDiag)
	}
	return out
}

func (h *Handler) DocumentSymbols(uri DocumentURI) []DocumentSymbol {
	path := pathFromURI(uri)
	var out []DocumentSymbol

	for _, typ := range h.index.Types {
		if !sameDocument(path, typ.File) && !sameDocument(string(uri), typ.File) {
			continue
		}
		symbol := DocumentSymbol{
			Name:           typ.Name,
			Detail:         string(typ.Kind),
			Kind:           symbolKind(typ.Kind),
			Range:          toLSPRange(typ.Range),
			SelectionRange: toLSPRange(typ.Range),
		}
		for _, member := range typ.Members {
			detail := string(member.Kind)
			if member.Type != "" {
				detail = member.Type
			}
			symbol.Children = append(symbol.Children, DocumentSymbol{
				Name:           member.Name,
				Detail:         detail,
				Kind:           symbolKind(member.Kind),
				Range:          toLSPRange(member.Range),
				SelectionRange: toLSPRange(member.Range),
			})
		}
		out = append(out, symbol)
	}

	for _, trigger := range h.index.Triggers {
		if !sameDocument(path, trigger.File) && !sameDocument(string(uri), trigger.File) {
			continue
		}
		out = append(out, DocumentSymbol{
			Name:           trigger.Name,
			Detail:         trigger.ObjectName,
			Kind:           symbolKindEvent,
			Range:          toLSPRange(trigger.Range),
			SelectionRange: toLSPRange(trigger.Range),
		})
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (h *Handler) WorkspaceSymbols(query string) []SymbolInformation {
	query = strings.ToLower(strings.TrimSpace(query))
	matches := func(name string) bool {
		return query == "" || strings.Contains(strings.ToLower(name), query)
	}

	var out []SymbolInformation
	for _, typ := range h.index.Types {
		if !matches(typ.Name) {
			continue
		}
		out = append(out, SymbolInformation{
			Name: typ.Name,
			Kind: symbolKind(typ.Kind),
			Location: Location{
				URI:   uriFromPath(typ.File),
				Range: toLSPRange(typ.Range),
			},
		})
	}
	for _, trigger := range h.index.Triggers {
		if !matches(trigger.Name) {
			continue
		}
		out = append(out, SymbolInformation{
			Name:          trigger.Name,
			Kind:          symbolKindEvent,
			ContainerName: trigger.ObjectName,
			Location: Location{
				URI:   uriFromPath(trigger.File),
				Range: toLSPRange(trigger.Range),
			},
		})
	}
	for _, object := range h.index.Objects {
		if matches(object.Name) {
			out = append(out, SymbolInformation{
				Name: object.Name,
				Kind: symbolKindObject,
				Location: Location{
					URI:   schemaURI(object.Name),
					Range: Range{},
				},
			})
		}
		for _, field := range object.Fields {
			if !matches(field.Name) {
				continue
			}
			out = append(out, SymbolInformation{
				Name:          field.Name,
				Kind:          symbolKindField,
				ContainerName: object.Name,
				Location: Location{
					URI:   schemaURI(object.Name, "fields", field.Name),
					Range: Range{},
				},
			})
		}
	}

	sortSymbols(out)
	return out
}

func (h *Handler) Hover(params HoverParams) *Hover {
	path := pathFromURI(params.TextDocument.URI)
	for _, typ := range h.index.Types {
		if !sameDocument(path, typ.File) && !sameDocument(string(params.TextDocument.URI), typ.File) {
			continue
		}
		for _, member := range typ.Members {
			if !containsPosition(member.Range, params.Position) {
				continue
			}
			if hover := h.hoverForTypeExpression(member.Type, toLSPRange(member.Range)); hover != nil {
				return hover
			}
			return &Hover{
				Contents: MarkupContent{Kind: "markdown", Value: fmt.Sprintf("`%s %s`", member.Kind, member.Name)},
				Range:    rangePtr(toLSPRange(member.Range)),
			}
		}
		if containsPosition(typ.Range, params.Position) {
			return h.HoverForName(typ.Name, rangePtr(toLSPRange(typ.Range)))
		}
	}

	for _, trigger := range h.index.Triggers {
		if (!sameDocument(path, trigger.File) && !sameDocument(string(params.TextDocument.URI), trigger.File)) || !containsPosition(trigger.Range, params.Position) {
			continue
		}
		if trigger.ObjectName != "" {
			return h.HoverForName(trigger.ObjectName, rangePtr(toLSPRange(trigger.Range)))
		}
	}
	return nil
}

func (h *Handler) HoverForName(name string, hoverRange *Range) *Hover {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}

	for _, typ := range h.index.Types {
		if strings.EqualFold(typ.Name, name) {
			return &Hover{
				Contents: MarkupContent{
					Kind:  "markdown",
					Value: apexTypeHover(typ),
				},
				Range: hoverRange,
			}
		}
	}
	for _, object := range h.index.Objects {
		if strings.EqualFold(object.Name, name) {
			return &Hover{
				Contents: MarkupContent{
					Kind:  "markdown",
					Value: objectHover(object),
				},
				Range: hoverRange,
			}
		}
	}

	for _, ref := range h.analysis.Types {
		if !strings.EqualFold(ref.Name, name) {
			continue
		}
		return &Hover{
			Contents: MarkupContent{
				Kind:  "markdown",
				Value: fmt.Sprintf("**%s**\n\n%s type", ref.Name, typeKindLabel(ref.Kind)),
			},
			Range: hoverRange,
		}
	}
	return nil
}

func (h *Handler) Completion(_ CompletionParams) CompletionList {
	items := make([]CompletionItem, 0, len(h.index.Types)+len(h.index.Objects))
	seen := make(map[string]bool)
	add := func(item CompletionItem) {
		key := item.Label + "\x00" + item.Detail
		if seen[key] {
			return
		}
		seen[key] = true
		items = append(items, item)
	}
	for _, typ := range h.index.Types {
		add(CompletionItem{
			Label:  typ.Name,
			Kind:   completionKind(typ.Kind),
			Detail: string(typ.Kind),
		})
		for _, member := range typ.Members {
			add(CompletionItem{
				Label:  member.Name,
				Kind:   completionKind(member.Kind),
				Detail: typ.Name + "." + member.Name,
			})
		}
	}
	for _, object := range h.index.Objects {
		detail := "SObject"
		if object.Label != "" {
			detail = "SObject: " + object.Label
		}
		add(CompletionItem{
			Label:         object.Name,
			Kind:          completionItemKindStruct,
			Detail:        detail,
			Documentation: objectHover(object),
		})
		for _, field := range object.Fields {
			add(CompletionItem{
				Label:         field.Name,
				Kind:          completionItemKindField,
				Detail:        object.Name + "." + field.Name,
				Documentation: field.Type,
			})
		}
	}
	for _, keyword := range []string{"class", "trigger", "interface", "enum", "public", "private", "protected", "global", "static", "void", "return", "new", "for", "if", "else"} {
		add(CompletionItem{Label: keyword, Kind: completionItemKindKeyword, Detail: "Apex keyword"})
	}
	sortCompletionItems(items)
	return CompletionList{Items: items}
}

func (h *Handler) Definition(params DefinitionParams) []Location {
	name := h.wordAt(params.TextDocument.URI, params.Position)
	if name == "" {
		return nil
	}
	if loc, ok := h.definitionForName(name); ok {
		return []Location{loc}
	}
	return nil
}

func (h *Handler) References(params ReferenceParams) []Location {
	name := h.wordAt(params.TextDocument.URI, params.Position)
	if name == "" {
		return nil
	}
	locations := h.referenceLocations(name)
	if params.Context.IncludeDeclaration {
		return locations
	}
	if def, ok := h.definitionForName(name); ok {
		filtered := locations[:0]
		for _, loc := range locations {
			if loc.URI == def.URI && loc.Range == def.Range {
				continue
			}
			filtered = append(filtered, loc)
		}
		return filtered
	}
	return locations
}

func (h *Handler) PrepareRename(params RenameParams) *Range {
	name := h.wordAt(params.TextDocument.URI, params.Position)
	if name == "" {
		return nil
	}
	if _, ok := h.definitionForName(name); !ok {
		return nil
	}
	wordRange, ok := h.wordRangeAt(params.TextDocument.URI, params.Position)
	if !ok {
		return nil
	}
	return &wordRange
}

func (h *Handler) Rename(params RenameParams) *WorkspaceEdit {
	if strings.TrimSpace(params.NewName) == "" {
		return nil
	}
	if h.PrepareRename(params) == nil {
		return nil
	}
	locations := h.referenceLocations(h.wordAt(params.TextDocument.URI, params.Position))
	if len(locations) == 0 {
		return nil
	}
	edit := &WorkspaceEdit{Changes: make(map[DocumentURI][]TextEdit)}
	for _, loc := range locations {
		edit.Changes[loc.URI] = append(edit.Changes[loc.URI], TextEdit{Range: loc.Range, NewText: params.NewName})
	}
	return edit
}

func (h *Handler) SemanticTokensFull(params SemanticTokensParams) SemanticTokens {
	var raw []semanticToken
	path := pathFromURI(params.TextDocument.URI)
	for _, typ := range h.index.Types {
		if !sameDocument(path, typ.File) && !sameDocument(string(params.TextDocument.URI), typ.File) {
			continue
		}
		raw = append(raw, semanticTokenFromRange(typ.Name, typ.Range, semanticTokenType(typ.Kind)))
		for _, member := range typ.Members {
			raw = append(raw, semanticTokenFromRange(member.Name, member.Range, semanticTokenType(member.Kind)))
		}
	}
	for _, trigger := range h.index.Triggers {
		if !sameDocument(path, trigger.File) && !sameDocument(string(params.TextDocument.URI), trigger.File) {
			continue
		}
		raw = append(raw, semanticTokenFromRange(trigger.Name, trigger.Range, semanticTokenEvent))
	}
	sort.Slice(raw, func(i, j int) bool {
		if raw[i].Line == raw[j].Line {
			return raw[i].Start < raw[j].Start
		}
		return raw[i].Line < raw[j].Line
	})
	return SemanticTokens{Data: encodeSemanticTokens(raw)}
}

func (h *Handler) hoverForTypeExpression(typeRef string, hoverRange Range) *Hover {
	for _, name := range candidateTypeNames(typeRef) {
		if hover := h.HoverForName(name, rangePtr(hoverRange)); hover != nil {
			return hover
		}
	}
	return nil
}

func candidateTypeNames(typeRef string) []string {
	if typeRef == "" {
		return nil
	}
	fields := strings.FieldsFunc(typeRef, func(r rune) bool {
		return !(r == '_' || r == '.' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'))
	})
	out := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == "" || strings.EqualFold(field, "list") || strings.EqualFold(field, "set") || strings.EqualFold(field, "map") {
			continue
		}
		out = append(out, field)
	}
	return out
}

func apexTypeHover(typ typesys.TypeSymbol) string {
	modifiers := strings.Join(typ.Modifiers, " ")
	if modifiers != "" {
		modifiers += " "
	}
	return fmt.Sprintf("```apex\n%s%s %s\n```\n\nSource: `%s`", modifiers, typ.Kind, typ.Name, typ.File)
}

func objectHover(object schema.Object) string {
	lines := []string{fmt.Sprintf("**%s**", object.Name), "SObject"}
	if object.Label != "" {
		lines = append(lines, "Label: "+object.Label)
	}
	if len(object.Fields) > 0 {
		lines = append(lines, fmt.Sprintf("Fields: %d", len(object.Fields)))
	}
	return strings.Join(lines, "\n\n")
}

func typeKindLabel(kind sema.TypeKind) string {
	switch kind {
	case sema.TypeApex:
		return "Apex"
	case sema.TypeSchema:
		return "SObject"
	case sema.TypePlatform:
		return "Platform"
	case sema.TypeBuiltin:
		return "Builtin"
	default:
		return "Known"
	}
}

func rangePtr(r Range) *Range {
	return &r
}
