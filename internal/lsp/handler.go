package lsp

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/apexlang"
	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/typesys"
)

type Handler struct {
	gen       atomic.Pointer[generation]
	ready     chan struct{}
	readyOnce sync.Once
	loadErr   error

	parserMu     sync.Mutex
	parser       *apexast.Parser
	parserClosed bool

	analyzer   func(typesys.Index) sema.Result
	refresher  func(context.Context, RefreshRequest) (Base, error)
	refreshMu  sync.Mutex
	base       *generation // last disk generation, guarded by refreshMu
	baseConfig Base
	nextSeq    uint64
	// The loaded project for overlay updates, guarded by refreshMu.
	overlayProject       project.Project
	overlayProjectOK     bool
	overlayProjectLoaded bool
	fallback             atomic.Pointer[overlayFallback]
	baseGen              atomic.Pointer[generation] // h.base for readers outside refreshMu
	watchFiles           atomic.Bool                // client supports watched-file registration
	overlayAnalysis      sync.Mutex                 // one overlay analysis at a time; never taken under refreshMu
	validateGeneration   func(typesys.Index) bool

	docMu         sync.RWMutex
	documents     map[DocumentURI]openDocument
	documentPaths map[string]DocumentURI
	diskParse     map[string]ParseDiagnosticsError // guarded by docMu

	shutdown atomic.Bool

	debounce time.Duration
}

// DefaultDiagnosticsDebounce is how long Serve waits after the last
// textDocument/didChange before it publishes diagnostics for that document.
const DefaultDiagnosticsDebounce = 250 * time.Millisecond

type openDocument struct {
	URI     DocumentURI
	Path    string
	Version int
	Text    string
	Digest  [sha256.Size]byte
}

func NewHandler(index typesys.Index) *Handler {
	h := NewPendingHandler()
	h.SetIndex(index)
	return h
}

// NewPendingHandler returns a handler whose project index is not built yet.
// initialize and document sync work at once. Requests that need the index wait
// for SetIndex or Fail.
func NewPendingHandler() *Handler {
	return &Handler{
		ready:         make(chan struct{}),
		parser:        apexast.NewParser(),
		documents:     make(map[DocumentURI]openDocument),
		documentPaths: make(map[string]DocumentURI),
		debounce:      DefaultDiagnosticsDebounce,
	}
}

// SetIndex publishes the project index and releases waiting requests.
func (h *Handler) SetIndex(index typesys.Index) {
	h.SetBase(Base{Index: index, Analyze: h.analyzer})
}

// Fail records an index load error and releases waiting requests with it.
func (h *Handler) Fail(err error) {
	h.readyOnce.Do(func() {
		h.loadErr = err
		close(h.ready)
	})
}

// LoadError returns the error passed to Fail, once the handler is ready.
func (h *Handler) LoadError() error {
	select {
	case <-h.ready:
		return h.loadErr
	default:
		return nil
	}
}

// SetDiagnosticsDebounce sets the didChange debounce used by Serve. Zero
// publishes without delay.
func (h *Handler) SetDiagnosticsDebounce(d time.Duration) {
	h.debounce = d
}

func (h *Handler) waitReady(ctx context.Context) error {
	select {
	case <-h.ready:
		return h.loadErr
	default:
	}
	select {
	case <-h.ready:
		return h.loadErr
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (h *Handler) analysisResult() sema.Result {
	<-h.ready
	if h.loadErr != nil {
		return sema.Result{}
	}
	return h.gen.Load().analysisResult()
}

// SetAnalyzer replaces the whole-project semantic pass, for callers that can
// answer from a cache. Call it before the index is published.
func (h *Handler) SetAnalyzer(analyze func(typesys.Index) sema.Result) {
	h.analyzer = analyze
}

// StartAnalysis runs the semantic pass in the background once the index is
// ready, so the first diagnostics request finds it done or already running.
func (h *Handler) StartAnalysis() {
	go h.analysisResult()
}

func (h *Handler) Shutdown() bool {
	return h.shutdown.Load()
}

func (h *Handler) Initialize(params InitializeParams) InitializeResult {
	h.watchFiles.Store(params.Capabilities.Workspace.DidChangeWatchedFiles.DynamicRegistration)
	return InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync:        TextDocumentSyncOptions{OpenClose: true, Change: textDocumentSyncIncremental, Save: SaveOptions{IncludeText: false}},
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
		ServerInfo: &ServerInfo{Name: "glade"},
	}
}

func (h *Handler) DidOpen(params DidOpenTextDocumentParams) []Notification {
	doc := h.applyOpen(params)
	return h.documentDiagnosticsSync(doc, true)
}

func (h *Handler) applyOpen(params DidOpenTextDocumentParams) openDocument {
	uri := params.TextDocument.URI
	doc := openDocument{
		URI:     uri,
		Path:    pathFromURI(uri),
		Version: params.TextDocument.Version,
		Text:    params.TextDocument.Text,
		Digest:  sha256.Sum256([]byte(params.TextDocument.Text)),
	}
	h.docMu.Lock()
	h.documents[uri] = doc
	h.documentPaths[canonicalDocumentPath(doc.Path)] = uri
	h.docMu.Unlock()
	return doc
}

func (h *Handler) DidChange(params DidChangeTextDocumentParams) ([]Notification, error) {
	doc, err := h.applyChange(params)
	if err != nil {
		return nil, err
	}
	return h.documentDiagnosticsSync(doc, true), nil
}

func (h *Handler) applyChange(params DidChangeTextDocumentParams) (openDocument, error) {
	h.docMu.Lock()
	defer h.docMu.Unlock()
	doc, ok := h.documents[params.TextDocument.URI]
	if !ok {
		doc = openDocument{URI: params.TextDocument.URI, Path: pathFromURI(params.TextDocument.URI)}
	}
	doc.Version = params.TextDocument.Version
	for _, change := range params.ContentChanges {
		next, err := applyTextChange(doc.Text, change)
		if err != nil {
			return openDocument{}, err
		}
		doc.Text = next
	}
	doc.Digest = sha256.Sum256([]byte(doc.Text))
	h.documents[doc.URI] = doc
	h.documentPaths[canonicalDocumentPath(doc.Path)] = doc.URI
	return doc, nil
}

func (h *Handler) DidClose(params DidCloseTextDocumentParams) []Notification {
	h.applyClose(params.TextDocument.URI)
	_ = h.syncOverlay(context.Background(), func(g *generation) { h.gen.Store(g) })
	return h.closeNotifications(params.TextDocument.URI)
}

func (h *Handler) applyClose(uri DocumentURI) {
	h.docMu.Lock()
	if doc, ok := h.documents[uri]; ok {
		delete(h.documentPaths, canonicalDocumentPath(doc.Path))
	}
	delete(h.documents, uri)
	h.docMu.Unlock()
}

func (h *Handler) closeNotifications(uri DocumentURI) []Notification {
	return []Notification{{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  BuildPublishDiagnostics(uri, h.diagnosticsForDocument(uri)),
	}}
}

// currentDiagnostics publishes diagnostics for uri from its latest state: the
// open document if there is one, the closed-document view otherwise.
func (h *Handler) currentDiagnostics(uri DocumentURI) []Notification {
	h.docMu.RLock()
	doc, ok := h.documents[uri]
	h.docMu.RUnlock()
	if !ok {
		return h.closeNotifications(uri)
	}
	return h.documentDiagnostics(doc)
}

func (h *Handler) documentDiagnostics(doc openDocument) []Notification {
	return h.documentDiagnosticsSync(doc, false)
}

// documentDiagnosticsSync publishes parser diagnostics, or semantic ones from
// a generation that reflects the buffer. With sync, a clean buffer first
// brings the overlay up to date, as the synchronous document API does.
func (h *Handler) documentDiagnosticsSync(doc openDocument, sync bool) []Notification {
	var diagnostics []diagnostic.Diagnostic
	h.parserMu.Lock()
	if !h.parserClosed {
		diagnostics = h.parser.ParseSource(doc.Path, doc.Text).Diagnostics
	}
	h.parserMu.Unlock()
	if len(diagnostics) == 0 {
		var publish bool
		diagnostics, publish = h.semanticDiagnostics(doc, sync)
		if !publish {
			return nil
		}
	}
	return []Notification{{
		JSONRPC: "2.0",
		Method:  "textDocument/publishDiagnostics",
		Params:  BuildPublishDiagnostics(doc.URI, diagnostics),
	}}
}

// semanticDiagnostics returns doc's semantic diagnostics and whether to
// publish them. A generation that indexes doc's file but not its text has
// nothing current to say about it, so nothing is published.
func (h *Handler) semanticDiagnostics(doc openDocument, sync bool) ([]diagnostic.Diagnostic, bool) {
	<-h.ready
	if h.loadErr != nil {
		return nil, true
	}
	if sync {
		_ = h.syncOverlay(context.Background(), func(g *generation) { h.gen.Store(g) })
	}
	g := h.gen.Load()
	if _, known := g.index.SourceDigest(doc.Path); known && !reflects(g, doc) {
		fallback := h.fallback.Load()
		if fallback.parserOnlyDocument(doc) {
			return nil, true
		}
		if !fallback.fixedIndex(doc) {
			return nil, false
		}
	}
	var diagnostics []diagnostic.Diagnostic
	for _, diag := range g.analysisResult().Diagnostics {
		if diag.File != "" && (sameDocument(doc.Path, diag.File) || sameDocument(string(doc.URI), diag.File)) {
			diagnostics = append(diagnostics, diag)
		}
	}
	return diagnostics, true
}

func (h *Handler) diagnosticsForDocument(uri DocumentURI) []diagnostic.Diagnostic {
	path := pathFromURI(uri)
	h.docMu.RLock()
	failure, hasFailure := h.diskParse[path]
	h.docMu.RUnlock()
	if hasFailure {
		data, err := os.ReadFile(path)
		if err == nil && sha256.Sum256(data) == failure.Digest {
			return copyDiagnostics(failure.Diagnostics)
		}
	}
	var diagnostics []diagnostic.Diagnostic
	for _, diag := range h.analysisResult().Diagnostics {
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
			message := testCase.Problem.ResultMessage()
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
				Code:     "GLADETEST001",
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
			Source:   "glade",
			Message:  diag.Message,
		}
		if diag.Range != nil {
			lspDiag.Range = toLSPRange(*diag.Range)
		}
		out.Diagnostics = append(out.Diagnostics, lspDiag)
	}
	return out
}

func (h *requestHandler) DocumentSymbols(uri DocumentURI) []DocumentSymbol {
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

func (h *requestHandler) WorkspaceSymbols(query string) []SymbolInformation {
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

func (h *requestHandler) Hover(params HoverParams) *Hover {
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

func (h *requestHandler) HoverForName(name string, hoverRange *Range) *Hover {
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

	h.generation.startAnalysis()
	result, _ := h.analysisIfReady()
	for _, ref := range result.Types {
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

type completionContext int

const (
	completionContextDefault completionContext = iota
	completionContextSOQLSelect
	completionContextSOQLFieldClause
)

func (h *requestHandler) Completion(params CompletionParams) CompletionList {
	context := h.completionContext(params)
	if context.items != nil {
		sortCompletionItems(context.items, completionContextDefault)
		return CompletionList{Items: context.items}
	}
	items, incomplete := filterCompletionCatalog(h.defaultCompletionCatalog(context.kind), context.prefix)
	return CompletionList{Items: items, IsIncomplete: incomplete}
}

// defaultCompletionCatalog returns the sorted whole-project completion list for
// one ordering. The index never changes after it is published, so each ordering
// is built once and shared. Callers must not modify the returned items.
func (h *requestHandler) defaultCompletionCatalog(kind completionContext) []CompletionItem {
	slot := &h.generation.completionCatalogs[kind]
	slot.once.Do(func() { slot.items = h.buildDefaultCompletionCatalog(kind) })
	return slot.items
}

func (h *requestHandler) buildDefaultCompletionCatalog(kind completionContext) []CompletionItem {
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
	sortCompletionItems(items, kind)
	return items
}

type completionResultContext struct {
	kind   completionContext
	items  []CompletionItem
	prefix string
}

func (h *requestHandler) completionContext(params CompletionParams) completionResultContext {
	text, ok := h.documentText(pathFromURI(params.TextDocument.URI))
	if !ok {
		return completionResultContext{}
	}
	offset, err := offsetForPosition(text, params.Position)
	if err != nil {
		return completionResultContext{}
	}
	if items := h.annotationCompletionItems(text, offset); len(items) > 0 {
		return completionResultContext{items: items}
	}
	if items := h.memberCompletionItems(text, offset); len(items) > 0 {
		return completionResultContext{items: items}
	}
	if items := h.testMethodCompletionItems(text, offset); len(items) > 0 {
		return completionResultContext{items: items}
	}
	start, _ := identifierBounds(text, offset)
	typed := text[start:offset]
	prefix := strings.ToLower(text[:offset])
	queryStart := strings.LastIndex(prefix, "[")
	if queryStart < 0 || strings.LastIndex(prefix, "]") > queryStart {
		return completionResultContext{prefix: typed}
	}
	clause := prefix[queryStart+1:]
	if strings.Contains(clause, "select") && !strings.Contains(clause, " from ") {
		return completionResultContext{kind: completionContextSOQLSelect, prefix: typed}
	}
	for _, marker := range []string{" where", " order by", " group by"} {
		if strings.Contains(clause, marker) {
			return completionResultContext{kind: completionContextSOQLFieldClause, prefix: typed}
		}
	}
	return completionResultContext{prefix: typed}
}

func (h *requestHandler) annotationCompletionItems(text string, offset int) []CompletionItem {
	prefix := text[:offset]
	lineStart := strings.LastIndex(prefix, "\n") + 1
	if strings.TrimSpace(prefix[lineStart:]) != "@" {
		return nil
	}
	var out []CompletionItem
	for _, annotation := range apexlang.AllAnnotations() {
		if annotation.Preview {
			continue
		}
		out = append(out, CompletionItem{Label: annotation.Name, Kind: completionItemKindKeyword, Detail: "Apex annotation"})
	}
	return out
}

func (h *requestHandler) testMethodCompletionItems(text string, offset int) []CompletionItem {
	prefix := text[:offset]
	lineStart := strings.LastIndex(prefix, "\n") + 1
	line := strings.ToLower(prefix[lineStart:])
	if !strings.Contains(line, "@istest") {
		return nil
	}
	return []CompletionItem{
		{Label: "testSetup", Kind: completionItemKindKeyword, Detail: "Apex test method modifier"},
		{Label: "static", Kind: completionItemKindKeyword, Detail: "Apex test method modifier"},
		{Label: "void", Kind: completionItemKindKeyword, Detail: "Apex test method modifier"},
	}
}

func (h *requestHandler) memberCompletionItems(text string, offset int) []CompletionItem {
	if offset == 0 || text[offset-1] != '.' {
		return nil
	}
	receiverEnd := offset - 1
	receiverStart := receiverEnd
	for receiverStart > 0 && (isIdentifierByte(text[receiverStart-1]) || text[receiverStart-1] == '.') {
		receiverStart--
	}
	receiver := strings.TrimSpace(text[receiverStart:receiverEnd])
	if receiver == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(receiver), "schema.sobjecttype.") {
		return schemaTokenCompletionItems()
	}
	typ := h.localVariableType(text[:receiverStart], receiver)
	if typ == "" {
		typ = receiver
	}
	if strings.HasPrefix(strings.ToLower(typ), "list<") {
		return methodCompletionItems("List method", "add", "clear", "get", "isEmpty", "iterator", "remove", "set", "size")
	}
	if strings.HasPrefix(strings.ToLower(typ), "map<") {
		return methodCompletionItems("Map method", "clear", "containsKey", "get", "isEmpty", "keySet", "put", "remove", "size", "values")
	}
	if object, ok := h.objectByName(typ); ok {
		return objectFieldCompletionItems(object)
	}
	for _, indexedType := range h.index.Types {
		if strings.EqualFold(indexedType.Name, typ) {
			items := make([]CompletionItem, 0, len(indexedType.Members))
			for _, member := range indexedType.Members {
				items = append(items, CompletionItem{Label: member.Name, Kind: completionKind(member.Kind), Detail: indexedType.Name + "." + member.Name})
			}
			return items
		}
	}
	return nil
}

func (h *requestHandler) localVariableType(prefix, name string) string {
	lines := strings.Split(prefix, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		idx := strings.Index(line, " "+name)
		if idx < 0 {
			continue
		}
		after := idx + 1 + len(name)
		if after < len(line) && isIdentifierByte(line[after]) {
			continue
		}
		before := strings.TrimSpace(line[:idx])
		before = strings.TrimPrefix(before, "public ")
		before = strings.TrimPrefix(before, "private ")
		before = strings.TrimPrefix(before, "protected ")
		before = strings.TrimPrefix(before, "global ")
		before = strings.TrimPrefix(before, "static ")
		before = strings.TrimPrefix(before, "final ")
		return strings.TrimSpace(before)
	}
	return ""
}

func (h *requestHandler) objectByName(name string) (schema.Object, bool) {
	for _, object := range h.index.Objects {
		if strings.EqualFold(object.Name, name) {
			return object, true
		}
	}
	return schema.Object{}, false
}

func objectFieldCompletionItems(object schema.Object) []CompletionItem {
	items := make([]CompletionItem, 0, len(object.Fields))
	for _, field := range object.Fields {
		items = append(items, CompletionItem{
			Label:         field.Name,
			Kind:          completionItemKindField,
			Detail:        object.Name + "." + field.Name,
			Documentation: field.Type,
		})
	}
	return items
}

func methodCompletionItems(detail string, names ...string) []CompletionItem {
	items := make([]CompletionItem, 0, len(names))
	for _, name := range names {
		items = append(items, CompletionItem{Label: name, Kind: completionItemKindMethod, Detail: detail})
	}
	return items
}

func schemaTokenCompletionItems() []CompletionItem {
	return methodCompletionItems("Schema token method", "getDescribe", "newSObject")
}

func (h *requestHandler) Definition(params DefinitionParams) []Location {
	name := h.wordAt(params.TextDocument.URI, params.Position)
	if name == "" {
		return nil
	}
	if loc, ok := h.definitionForName(name); ok {
		return []Location{loc}
	}
	return nil
}

func (h *requestHandler) References(params ReferenceParams) []Location {
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

func (h *requestHandler) PrepareRename(params RenameParams) *Range {
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

func (h *requestHandler) Rename(params RenameParams) *WorkspaceEdit {
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

func (h *requestHandler) SemanticTokensFull(params SemanticTokensParams) SemanticTokens {
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

func (h *requestHandler) hoverForTypeExpression(typeRef string, hoverRange Range) *Hover {
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
