package lsp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/open-aer/oaer/internal/diagnostic"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/sema"
	"github.com/open-aer/oaer/internal/typesys"
)

type Handler struct {
	index    typesys.Index
	analysis sema.Result
	shutdown bool
}

func NewHandler(index typesys.Index) *Handler {
	return &Handler{
		index:    index,
		analysis: sema.Analyze(index),
	}
}

func (h *Handler) Shutdown() bool {
	return h.shutdown
}

func (h *Handler) Initialize(_ InitializeParams) InitializeResult {
	return InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:        textDocumentSyncFull,
			DocumentSymbolProvider:  true,
			WorkspaceSymbolProvider: true,
			HoverProvider:           true,
			CompletionProvider: CompletionOptions{
				ResolveProvider:   false,
				TriggerCharacters: []string{".", "_"},
			},
		},
		ServerInfo: &ServerInfo{Name: "oaer"},
	}
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
	for _, typ := range h.index.Types {
		items = append(items, CompletionItem{
			Label:  typ.Name,
			Kind:   completionKind(typ.Kind),
			Detail: string(typ.Kind),
		})
	}
	for _, object := range h.index.Objects {
		detail := "SObject"
		if object.Label != "" {
			detail = "SObject: " + object.Label
		}
		items = append(items, CompletionItem{
			Label:         object.Name,
			Kind:          completionItemKindStruct,
			Detail:        detail,
			Documentation: objectHover(object),
		})
	}
	sortCompletionItems(items)
	return CompletionList{Items: items}
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
