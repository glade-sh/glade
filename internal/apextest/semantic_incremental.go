package apextest

import (
	"context"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

type semanticIncrementalKey struct{}

// WithIncrementalSemantic returns a context whose semantic gate analyzes
// through state. A long-lived caller keeps one state per project so an edit
// that changes only method bodies re-checks only the affected types. The
// result equals a plain analysis, so cache identities are unchanged.
func WithIncrementalSemantic(ctx context.Context, state *sema.IncrementalState) context.Context {
	if state == nil {
		return ctx
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, semanticIncrementalKey{}, state)
}

func analyzeSemantic(ctx context.Context, index typesys.Index, options sema.AnalyzeOptions) sema.Result {
	if ctx != nil {
		if state, ok := ctx.Value(semanticIncrementalKey{}).(*sema.IncrementalState); ok && state != nil && options.BuildArtifacts != nil {
			result, _ := state.Analyze(index, options)
			return result
		}
	}
	return sema.AnalyzeWithOptions(index, options)
}
