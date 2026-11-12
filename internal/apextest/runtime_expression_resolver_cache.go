package apextest

import (
	"sync"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// maxExpressionResolvers bounds retained resolvers. Each keeps its index and a
// lazily built member model alive, so it follows the compiled runtime cache.
const maxExpressionResolvers = maxRuntimeCacheEntries

type expressionResolverEntry struct {
	key      runtimeCacheKey
	resolver vm.ExpressionTypeResolver
}

// newExpressionResolver is a test seam counting resolver constructions.
var newExpressionResolver = sema.RuntimeExpressionTypeResolver

var (
	expressionResolverMu sync.Mutex
	expressionResolvers  []expressionResolverEntry // oldest first
)

// runtimeExpressionResolver returns the expression type resolver for one
// runtime generation. sema builds its type-member model lazily inside the
// resolver, so sharing one resolver per generation key builds that model once
// for every machine, request or run that uses the generation, instead of once
// per machine. The key is the runtime content key of index, so a changed
// source file yields a new resolver. The resolver serializes its own state and
// is safe to share across cloned machines.
func runtimeExpressionResolver(key runtimeCacheKey, index typesys.Index) vm.ExpressionTypeResolver {
	if key == "" {
		return newExpressionResolver(index)
	}
	expressionResolverMu.Lock()
	defer expressionResolverMu.Unlock()
	for i, entry := range expressionResolvers {
		if entry.key == key {
			// Most recently used last.
			expressionResolvers = append(append(expressionResolvers[:i:i], expressionResolvers[i+1:]...), entry)
			return entry.resolver
		}
	}
	resolver := newExpressionResolver(index)
	expressionResolvers = append(expressionResolvers, expressionResolverEntry{key: key, resolver: resolver})
	if len(expressionResolvers) > maxExpressionResolvers {
		expressionResolvers = append([]expressionResolverEntry(nil), expressionResolvers[len(expressionResolvers)-maxExpressionResolvers:]...)
	}
	return resolver
}

func resetExpressionResolvers() {
	expressionResolverMu.Lock()
	expressionResolvers = nil
	expressionResolverMu.Unlock()
}
