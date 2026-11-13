package lsp

import (
	"context"
	"crypto/sha256"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// Overlay size caps. A document past a cap gets parser diagnostics only.
const (
	maxOverlayDocumentBytes = 4 << 20
	maxOverlayTotalBytes    = 64 << 20
)

// Base binds a disk generation to its project and analysis functions.
//
// Analyze is the disk analysis and may use persistent caches. AnalyzeOverlay
// analyzes a generation that holds unsaved editor text and must stay in memory.
// RebuildOverlay is the optional full rebuild used when the one-file updater
// declines an overlay. Nil fields take memory-only defaults.
type Base struct {
	Index          typesys.Index
	Project        project.Project
	Artifacts      *typesys.BuildArtifacts
	Analyze        func(typesys.Index) sema.Result
	AnalyzeOverlay func(typesys.Index, *typesys.BuildArtifacts) sema.Result
	RebuildOverlay func(context.Context, *typesys.SourceOverlay) (typesys.Index, typesys.BuildArtifacts, error)
}

// RefreshRequest lists the disk paths a refresh must account for.
type RefreshRequest struct{ Changed, Created, Deleted []string }

// overlayFallback records open documents the last overlay sync could not
// represent. Values are buffer digests, so a later edit leaves the set.
type overlayFallback struct {
	// fixed: no rebuild hook and the updater declined. The handler keeps its
	// fixed-index diagnostics for these, as before overlays existed.
	fixed map[string][sha256.Size]byte
	// parserOnly: past a size cap. Only parser diagnostics are published.
	parserOnly map[string][sha256.Size]byte
}

func (f *overlayFallback) fixedIndex(doc openDocument) bool {
	return f != nil && digestMatches(f.fixed, doc)
}

func (f *overlayFallback) parserOnlyDocument(doc openDocument) bool {
	return f != nil && digestMatches(f.parserOnly, doc)
}

func digestMatches(m map[string][sha256.Size]byte, doc openDocument) bool {
	digest, ok := m[canonicalDocumentPath(doc.Path)]
	return ok && digest == doc.Digest
}

// SetBase publishes the first disk generation and releases waiting requests.
func (h *Handler) SetBase(b Base) {
	h.readyOnce.Do(func() {
		h.refreshMu.Lock()
		defer h.refreshMu.Unlock()
		h.installBaseLocked(b, func(g *generation) { h.gen.Store(g) })
		close(h.ready)
	})
}

// installBaseLocked records a new disk generation. Callers hold refreshMu.
func (h *Handler) installBaseLocked(b Base, publish func(*generation)) *generation {
	h.nextSeq++
	g := newGeneration(b.Index, h.nextSeq, b.Analyze)
	g.artifacts = b.Artifacts
	g.baseSeq = g.seq
	h.base, h.baseConfig = g, b
	h.overlayProject, h.overlayProjectOK = b.Project, b.Project.Root != ""
	h.overlayProjectLoaded = h.overlayProjectOK
	h.fallback.Store(nil)
	publish(g)
	return g
}

// reflects reports whether g was built from exactly doc's text.
func reflects(g *generation, doc openDocument) bool {
	digest, ok := g.index.SourceDigest(doc.Path)
	return ok && digest == doc.Digest
}

func isApexSourcePath(path string) bool {
	lower := strings.ToLower(path)
	return strings.HasSuffix(lower, ".cls") || strings.HasSuffix(lower, ".trigger")
}

// overlayTarget computes the overlay the open documents call for. A buffer
// equal to its disk digest, unknown to the base, or past a size cap adds no
// text. A buffer with parse errors keeps the text it had in current's overlay.
func (h *Handler) overlayTarget(current *generation) (*typesys.SourceOverlay, map[string][sha256.Size]byte) {
	h.docMu.RLock()
	docs := make([]openDocument, 0, len(h.documents))
	for _, doc := range h.documents {
		docs = append(docs, doc)
	}
	h.docMu.RUnlock()
	sort.Slice(docs, func(i, j int) bool { return docs[i].Path < docs[j].Path })
	texts := map[string]string{}
	var capped map[string][sha256.Size]byte
	total := 0
	for _, doc := range docs {
		if !isApexSourcePath(doc.Path) {
			continue
		}
		baseDigest, ok := h.base.index.SourceDigest(doc.Path)
		if !ok || baseDigest == doc.Digest {
			continue
		}
		if len(doc.Text) > maxOverlayDocumentBytes || total+len(doc.Text) > maxOverlayTotalBytes {
			if capped == nil {
				capped = map[string][sha256.Size]byte{}
			}
			capped[canonicalDocumentPath(doc.Path)] = doc.Digest
			continue
		}
		text := doc.Text
		h.parserMu.Lock()
		clean := !h.parserClosed && len(h.parser.ParseSource(doc.Path, text).Diagnostics) == 0
		h.parserMu.Unlock()
		if !clean {
			if current == nil || current.overlay == nil || !current.overlay.Has(doc.Path) {
				continue
			}
			raw, err := current.overlay.ReadFile(doc.Path)
			if err != nil {
				continue
			}
			text = string(raw)
		}
		total += len(text)
		texts[doc.Path] = text
	}
	return typesys.NewSourceOverlay(texts), capped
}

// resolveOverlayProjectLocked returns the loaded project the one-file updater
// needs. A handler built from an index alone loads it once from the index root.
func (h *Handler) resolveOverlayProjectLocked() (project.Project, bool) {
	if !h.overlayProjectLoaded {
		h.overlayProjectLoaded = true
		if root := h.base.index.Project.Root; root != "" {
			if p, err := project.Load(root); err == nil {
				h.overlayProject, h.overlayProjectOK = p, true
			}
		}
	}
	return h.overlayProject, h.overlayProjectOK
}

func (h *Handler) syncOverlay(ctx context.Context, publish func(*generation)) error {
	if err := h.waitReady(ctx); err != nil {
		return err
	}
	h.refreshMu.Lock()
	defer h.refreshMu.Unlock()
	return h.syncOverlayLocked(ctx, publish)
}

// syncOverlayLocked makes the current generation match the open buffers:
// the base when no buffer differs from disk, otherwise the base plus an
// overlay. Callers hold refreshMu. A cancelled sync stores nothing.
func (h *Handler) syncOverlayLocked(ctx context.Context, publish func(*generation)) error {
	if h.base == nil {
		return nil
	}
	current := h.gen.Load()
	target, capped := h.overlayTarget(current)
	start := current
	if start == nil || start.baseSeq != h.base.seq {
		start = h.base
	}
	paths := map[string]bool{}
	for _, path := range h.openDocumentPaths() {
		paths[path] = true
	}
	// Closed documents leave the overlay; their files stay in the index.
	if start.overlay != nil {
		for _, typ := range start.index.Types {
			if start.overlay.Has(typ.File) {
				paths[typ.File] = true
			}
		}
		for _, trigger := range start.index.Triggers {
			if start.overlay.Has(trigger.File) {
				paths[trigger.File] = true
			}
		}
	}
	changed := make([]string, 0, len(paths))
	for path := range paths {
		a, aok := target.Digest(path)
		var b [sha256.Size]byte
		bok := false
		if start.overlay != nil {
			b, bok = start.overlay.Digest(path)
		}
		if aok != bok || (aok && a != b) {
			changed = append(changed, path)
		}
	}
	setFallback := func(fixed map[string][sha256.Size]byte) {
		if len(fixed) == 0 && len(capped) == 0 {
			h.fallback.Store(nil)
			return
		}
		h.fallback.Store(&overlayFallback{fixed: fixed, parserOnly: capped})
	}
	if len(changed) == 0 && start == current {
		setFallback(nil)
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if target.Len() == 0 {
		setFallback(nil)
		publish(h.base)
		return nil
	}
	sort.Strings(changed)
	index := start.index
	exact := false
	if p, ok := h.resolveOverlayProjectLocked(); ok {
		exact = true
		for _, path := range changed {
			if err := ctx.Err(); err != nil {
				return err
			}
			var updated bool
			index, updated = typesys.TryUpdateApexFileWithOverlay(index, path, p, target)
			if !updated {
				exact = false
				break
			}
		}
	}
	var artifacts typesys.BuildArtifacts
	if exact {
		if err := ctx.Err(); err != nil {
			return err
		}
		previous, err := h.startArtifactsLocked(start)
		if err == nil {
			artifacts, err = typesys.RefreshBuildArtifactsWithOverlay(index, previous, target)
		}
		exact = err == nil
	}
	if !exact {
		if h.baseConfig.RebuildOverlay == nil {
			fixed := map[string][sha256.Size]byte{}
			for _, path := range changed {
				if digest, ok := target.Digest(path); ok {
					fixed[canonicalDocumentPath(path)] = h.bufferDigest(path, digest)
				}
			}
			setFallback(fixed)
			return nil
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		var err error
		index, artifacts, err = h.baseConfig.RebuildOverlay(ctx, target)
		if err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	analyze := h.baseConfig.AnalyzeOverlay
	if analyze == nil {
		analyze = func(i typesys.Index, a *typesys.BuildArtifacts) sema.Result {
			return sema.AnalyzeWithOptions(i, sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: a})
		}
	}
	h.nextSeq++
	captured := &artifacts
	g := newGeneration(index, h.nextSeq, nil)
	// One overlay analysis runs at a time. A generation superseded before its
	// turn is abandoned unanalyzed, so typing never queues a backlog.
	g.analyzer = func(i typesys.Index) sema.Result {
		h.overlayAnalysis.Lock()
		defer h.overlayAnalysis.Unlock()
		if h.gen.Load() != g {
			g.abandoned.Store(true)
			return sema.Result{}
		}
		return analyze(i, captured)
	}
	g.artifacts = captured
	g.overlay = target
	g.baseSeq = h.base.seq
	setFallback(nil)
	publish(g)
	return nil
}

// bufferDigest returns the open buffer's digest for path. It differs from
// the overlay digest when a broken buffer keeps its last clean text.
func (h *Handler) bufferDigest(path string, fallback [sha256.Size]byte) [sha256.Size]byte {
	h.docMu.RLock()
	defer h.docMu.RUnlock()
	if uri, ok := h.documentPaths[canonicalDocumentPath(path)]; ok {
		return h.documents[uri].Digest
	}
	return fallback
}

func (h *Handler) openDocumentPaths() []string {
	h.docMu.RLock()
	defer h.docMu.RUnlock()
	paths := make([]string, 0, len(h.documents))
	for _, doc := range h.documents {
		paths = append(paths, doc.Path)
	}
	return paths
}

// startArtifactsLocked returns the arena an overlay update adopts from. A base
// installed without artifacts gets one built once from its index.
func (h *Handler) startArtifactsLocked(start *generation) (*typesys.BuildArtifacts, error) {
	if start.artifacts != nil {
		return start.artifacts, nil
	}
	if start != h.base {
		return nil, nil
	}
	artifacts, err := typesys.RefreshBuildArtifactsWithOverlay(h.base.index, nil, typesys.NewSourceOverlay(nil))
	if err != nil {
		return nil, err
	}
	h.base.artifacts = &artifacts
	return h.base.artifacts, nil
}
