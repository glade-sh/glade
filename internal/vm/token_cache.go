package vm

import (
	"container/list"
	"sync"
	"sync/atomic"
	"unsafe"
)

// Semantic analysis and the runtime compile lex the same method bodies.
// Tokens are immutable once built: the parser and every other consumer only
// reads them (a consumer that keeps tokens copies them first), so one lex
// serves every compile of the same text. The cache is bounded by token bytes
// and evicts least recently used entries.

var lexScanRuns atomic.Int64
var disableTokenCache atomic.Bool

const (
	// Below this size a lex costs less than a cache entry.
	tokenCacheMinSource = 256
	tokenCacheLimit     = 24 << 20
	tokenScratchMax     = 1 << 16
)

type tokenCacheKey struct {
	source      string
	annotations bool
}

type tokenCacheEntry struct {
	key    tokenCacheKey
	tokens []token
	bytes  int64
	elem   *list.Element
}

var tokenCache struct {
	sync.Mutex
	entries map[tokenCacheKey]*tokenCacheEntry
	lru     list.List
	bytes   int64
}

var tokenScratch = sync.Pool{New: func() any {
	s := make([]token, 0, 1024)
	return &s
}}

func tokenBytes(n int) int64 { return int64(n) * int64(unsafe.Sizeof(token{})) }

func resetTokenCache() {
	tokenCache.Lock()
	tokenCache.entries = nil
	tokenCache.lru.Init()
	tokenCache.bytes = 0
	tokenCache.Unlock()
}

func tokenCacheBytes() int64 {
	tokenCache.Lock()
	defer tokenCache.Unlock()
	return tokenCache.bytes
}

func lookupTokens(key tokenCacheKey) ([]token, bool) {
	tokenCache.Lock()
	defer tokenCache.Unlock()
	entry, ok := tokenCache.entries[key]
	if !ok {
		return nil, false
	}
	tokenCache.lru.MoveToFront(entry.elem)
	return entry.tokens, true
}

func storeTokens(key tokenCacheKey, tokens []token) {
	size := tokenBytes(cap(tokens))
	if size > tokenCacheLimit/4 {
		return
	}
	tokenCache.Lock()
	defer tokenCache.Unlock()
	if tokenCache.entries == nil {
		tokenCache.entries = make(map[tokenCacheKey]*tokenCacheEntry)
	}
	if _, exists := tokenCache.entries[key]; exists {
		return
	}
	entry := &tokenCacheEntry{key: key, tokens: tokens, bytes: size}
	entry.elem = tokenCache.lru.PushFront(entry)
	tokenCache.entries[key] = entry
	tokenCache.bytes += size
	for tokenCache.bytes > tokenCacheLimit {
		oldest := tokenCache.lru.Back()
		if oldest == nil {
			break
		}
		victim := oldest.Value.(*tokenCacheEntry)
		tokenCache.lru.Remove(oldest)
		delete(tokenCache.entries, victim.key)
		tokenCache.bytes -= victim.bytes
	}
}

// lexWithAnnotations returns shared, read-only tokens. Callers must not write
// to the slice or its elements; cap equals len so an append copies.
func lexWithAnnotations(source string, annotations bool) ([]token, error) {
	cacheable := len(source) >= tokenCacheMinSource && !disableTokenCache.Load()
	key := tokenCacheKey{source: source, annotations: annotations}
	if cacheable {
		if tokens, ok := lookupTokens(key); ok {
			return tokens, nil
		}
	}
	lexScanRuns.Add(1)
	scratch := tokenScratch.Get().(*[]token)
	scanned, err := lexScan(source, annotations, (*scratch)[:0])
	var exact []token
	if err == nil {
		exact = make([]token, len(scanned))
		copy(exact, scanned)
	}
	if scanned != nil {
		clear(scanned)
		*scratch = scanned[:0]
	}
	if cap(*scratch) <= tokenScratchMax {
		tokenScratch.Put(scratch)
	}
	if err != nil {
		return nil, err
	}
	if cacheable {
		storeTokens(key, exact)
	}
	return exact, nil
}
