package lsp

import (
	"context"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

// SetRefresher installs the disk-backed loader before the first index is ready.
func (h *Handler) SetRefresher(fn func(context.Context, []string) (typesys.Index, func(typesys.Index) sema.Result, error)) {
	h.refresher = fn
}

// SetGenerationValidator checks whether the analyzed disk inputs are still current.
func (h *Handler) SetGenerationValidator(fn func(typesys.Index) bool) { h.validateGeneration = fn }

// Refresh replaces the generation after the refresher returns a complete index.
func (h *Handler) Refresh(ctx context.Context, paths []string) error {
	return h.refresh(ctx, paths, func(g *generation) { h.gen.Store(g) })
}

func (h *Handler) refresh(ctx context.Context, paths []string, publish func(*generation)) error {
	if err := h.waitReady(ctx); err != nil {
		return err
	}
	h.refreshMu.Lock()
	defer h.refreshMu.Unlock()
	if h.refresher == nil {
		return nil
	}
	previous := h.gen.Load()
	index, analyze, err := h.refresher(ctx, paths)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	publish(newGeneration(index, previous.seq+1, analyze))
	return nil
}
