package visualforce

import (
	"crypto/sha256"
	"sync"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/apexast"
)

// apexParseMemoEnabled lets a long-lived caller (the test daemon) keep the
// declarations parsed for expression validation across project generations.
// Entries are keyed by path and source text, so an edited, deleted or renamed
// file simply asks for a different key.
var apexParseMemoEnabled atomic.Bool

// EnableApexParseMemo turns on the declaration memo.
func EnableApexParseMemo() { apexParseMemoEnabled.Store(true) }

// apexParseMemoLimit and apexParseMemoByteLimit bound the memo by entries
// and by the source bytes parsed into it; past either it starts over.
const (
	apexParseMemoLimit     = 1 << 15
	apexParseMemoByteLimit = 256 << 20
)

type apexParseKey struct {
	path   string
	digest [sha256.Size]byte
}

var apexParseMemo struct {
	mu      sync.Mutex
	entries map[apexParseKey][]apexast.Declaration
	bytes   int
}

// parseApexDeclarations returns the declarations of one Apex source. Callers
// treat them as read-only.
func parseApexDeclarations(parser *apexast.Parser, path, source string) []apexast.Declaration {
	if !apexParseMemoEnabled.Load() {
		apexSourceParses.Add(1)
		return parser.ParseSource(path, source).Declarations
	}
	key := apexParseKey{path: path, digest: sha256.Sum256([]byte(source))}
	apexParseMemo.mu.Lock()
	declarations, ok := apexParseMemo.entries[key]
	apexParseMemo.mu.Unlock()
	if ok {
		return declarations
	}
	apexSourceParses.Add(1)
	declarations = parser.ParseSource(path, source).Declarations
	apexParseMemo.mu.Lock()
	if apexParseMemo.entries == nil || len(apexParseMemo.entries) >= apexParseMemoLimit || apexParseMemo.bytes+len(source) > apexParseMemoByteLimit {
		apexParseMemo.entries = make(map[apexParseKey][]apexast.Declaration)
		apexParseMemo.bytes = 0
	}
	if _, present := apexParseMemo.entries[key]; !present {
		apexParseMemo.bytes += len(source)
	}
	apexParseMemo.entries[key] = declarations
	apexParseMemo.mu.Unlock()
	return declarations
}
