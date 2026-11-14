package lsp

import (
	"context"
	"crypto/sha256"
	"errors"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"

	"github.com/glade-sh/glade/internal/typesys"
)

// ParseDiagnosticsError reports the exact disk contents that prevented a refresh.
type ParseDiagnosticsError struct {
	Path        string
	Digest      [sha256.Size]byte
	Diagnostics []diagnostic.Diagnostic
}

func (e *ParseDiagnosticsError) Error() string { return "saved source has parse diagnostics" }

// SetRefresher installs the disk-backed loader before the first index is ready.
func (h *Handler) SetRefresher(fn func(context.Context, RefreshRequest) (Base, error)) {
	h.refresher = fn
}

// SetGenerationValidator checks whether the analyzed disk inputs are still current.
func (h *Handler) SetGenerationValidator(fn func(typesys.Index) bool) { h.validateGeneration = fn }

// Refresh replaces the generation after the refresher returns a complete index.
func (h *Handler) Refresh(ctx context.Context, paths []string) error {
	return h.refreshRequest(ctx, RefreshRequest{Changed: paths}, func(g *generation) { h.gen.Store(g) })
}

func (h *Handler) refreshRequest(ctx context.Context, request RefreshRequest, publish func(*generation)) error {
	return h.refreshRequestValidated(ctx, request, publish, nil)
}

var errStaleGeneration = errors.New("stale disk generation")

func (h *Handler) refreshRequestValidated(ctx context.Context, request RefreshRequest, publish func(*generation), validate func(typesys.Index) bool) error {
	if err := h.waitReady(ctx); err != nil {
		return err
	}
	h.refreshMu.Lock()
	defer h.refreshMu.Unlock()
	if h.refresher == nil {
		return nil
	}
	b, err := h.refresher(ctx, request)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if validate != nil {
		candidate := newGeneration(b.Index, 0, b.Analyze)
		result := make(chan sema.Result, 1)
		go func() { result <- candidate.analysisResult() }()
		var analyzed sema.Result
		select {
		case analyzed = <-result:
		case <-ctx.Done():
			return ctx.Err()
		}
		if !validate(b.Index) {
			return errStaleGeneration
		}
		b.Analyze = func(typesys.Index) sema.Result { return analyzed }
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
