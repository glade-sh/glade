package apextest

import (
	"encoding/json"
	"hash/fnv"
	"strconv"
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
	// The runtime key hashes source contents, not every field the member
	// model reads (artifact and generated types carry no source snapshot), so
	// the identity also covers the resolver's own inputs.
	key += runtimeCacheKey("\x00" + memoizedExpressionResolverInputs(index))
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

type expressionFingerprint struct {
	types     []typesys.TypeSymbol // retained so the backing array cannot be reused
	namespace string
	value     string
}

var (
	expressionFingerprintMu sync.Mutex
	expressionFingerprints  []expressionFingerprint // oldest first
)

// memoizedExpressionResolverInputs computes the fingerprint once per index
// generation. An index is an immutable generation: copies of it share the
// Types backing array, so the array's address and length identify it. The
// array is retained by the memo, so its address cannot be reused by another
// index while the entry lives. The memo is bounded like the resolvers.
func memoizedExpressionResolverInputs(index typesys.Index) string {
	if len(index.Types) == 0 {
		return expressionResolverInputs(index)
	}
	expressionFingerprintMu.Lock()
	for _, e := range expressionFingerprints {
		if len(e.types) == len(index.Types) && &e.types[0] == &index.Types[0] && e.namespace == index.Project.Namespace {
			expressionFingerprintMu.Unlock()
			return e.value
		}
	}
	expressionFingerprintMu.Unlock()
	value := expressionResolverInputs(index)
	expressionFingerprintMu.Lock()
	defer expressionFingerprintMu.Unlock()
	expressionFingerprints = append(expressionFingerprints, expressionFingerprint{index.Types, index.Project.Namespace, value})
	if len(expressionFingerprints) > maxExpressionResolvers {
		expressionFingerprints = append([]expressionFingerprint(nil), expressionFingerprints[len(expressionFingerprints)-maxExpressionResolvers:]...)
	}
	return value
}

// expressionResolverInputs fingerprints everything the resolver reads from the
// index: every type symbol with its members, and the project namespace.
func expressionResolverInputs(index typesys.Index) string {
	h := fnv.New128a()
	enc := json.NewEncoder(h)
	_ = enc.Encode(index.Types)
	_, _ = h.Write([]byte(index.Project.Namespace))
	for _, typ := range index.Types {
		// Excluded from the symbol's JSON form but read by the model.
		_, _ = h.Write([]byte(strconv.FormatBool(typ.ConstructorsAuthoritative)))
		if typ.EnumHashBase != nil {
			_, _ = h.Write([]byte(strconv.FormatInt(*typ.EnumHashBase, 10)))
		}
	}
	return string(h.Sum(nil))
}

func resetExpressionResolvers() {
	expressionResolverMu.Lock()
	expressionResolvers = nil
	expressionResolverMu.Unlock()
	expressionFingerprintMu.Lock()
	expressionFingerprints = nil
	expressionFingerprintMu.Unlock()
}
