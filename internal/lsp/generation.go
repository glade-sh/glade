package lsp

import (
	"sync"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

type generation struct {
	index              typesys.Index
	artifacts          *typesys.BuildArtifacts
	overlay            *typesys.SourceOverlay
	baseSeq            uint64
	seq                uint64
	analyzer           func(typesys.Index) sema.Result
	analysisOnce       sync.Once
	analysisStart      sync.Once
	analysisDone       chan struct{}
	analysis           sema.Result
	occurrenceOnce     sync.Once
	occurrences        occurrenceIndex
	completionCatalogs [3]struct {
		once  sync.Once
		items []CompletionItem
	}
}

func newGeneration(index typesys.Index, seq uint64, analyze func(typesys.Index) sema.Result) *generation {
	if analyze == nil {
		analyze = sema.Analyze
	}
	return &generation{index: index, seq: seq, analyzer: analyze, analysisDone: make(chan struct{})}
}
func (g *generation) analysisResult() sema.Result {
	g.analysisOnce.Do(func() { g.analysis = g.analyzer(g.index); close(g.analysisDone) })
	return g.analysis
}
func (g *generation) startAnalysis() {
	g.analysisStart.Do(func() { go g.analysisResult() })
}
func (g *generation) analysisIfReady() (sema.Result, bool) {
	select {
	case <-g.analysisDone:
		return g.analysis, true
	default:
		return sema.Result{}, false
	}
}

type requestHandler struct {
	*Handler
	*generation
}

func (h *Handler) snapshot() *requestHandler {
	return &requestHandler{Handler: h, generation: h.gen.Load()}
}
func (h *Handler) DocumentSymbols(uri DocumentURI) []DocumentSymbol {
	return h.snapshot().DocumentSymbols(uri)
}
func (h *Handler) WorkspaceSymbols(query string) []SymbolInformation {
	return h.snapshot().WorkspaceSymbols(query)
}
func (h *Handler) Hover(p HoverParams) *Hover { return h.snapshot().Hover(p) }
func (h *Handler) HoverForName(name string, r *Range) *Hover {
	return h.snapshot().HoverForName(name, r)
}
func (h *Handler) Completion(p CompletionParams) CompletionList { return h.snapshot().Completion(p) }
func (h *Handler) defaultCompletionCatalog(k completionContext) []CompletionItem {
	return h.snapshot().defaultCompletionCatalog(k)
}
func (h *Handler) Definition(p DefinitionParams) []Location { return h.snapshot().Definition(p) }
func (h *Handler) References(p ReferenceParams) []Location  { return h.snapshot().References(p) }
func (h *Handler) PrepareRename(p RenameParams) *Range      { return h.snapshot().PrepareRename(p) }
func (h *Handler) Rename(p RenameParams) *WorkspaceEdit     { return h.snapshot().Rename(p) }
func (h *Handler) SemanticTokensFull(p SemanticTokensParams) SemanticTokens {
	return h.snapshot().SemanticTokensFull(p)
}
