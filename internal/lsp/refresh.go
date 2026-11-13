package lsp

import (
	"context"

	"github.com/glade-sh/glade/internal/typesys"
)

// SetRefresher installs the disk-backed loader before the first index is ready.
func (h *Handler) SetRefresher(fn func(context.Context, RefreshRequest) (Base, error)) {
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
	b, err := h.refresher(ctx, RefreshRequest{Changed: paths})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	h.installBaseLocked(b, publish)
	// The base is current even when the overlay cannot follow it; documents
	// it does not reflect publish nothing until a later sync succeeds.
	if err := h.syncOverlayLocked(ctx, publish); err != nil && ctx.Err() != nil {
		return err
	}
	return nil
}
