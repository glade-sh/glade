package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/typesys"
)

const semaAnalysisCanonicalNameLimit = 4 * 1024

const (
	semaSharedNameMemoLimit = 16 * 1024
	semaNameMemoMaxBytes    = 1024
)

var semaNormalizedNames = newSemaCanonicalNames(semaSharedNameMemoLimit)

// semaCanonicalNames retains only spellings that require case folding. Lowercase
// ASCII names already pass through normalizeName without allocation and do not
// consume cache entries. The fixed limit prevents a reused Analyzer from
// becoming an unbounded process-global string store.
type semaCanonicalNames struct {
	mu    sync.RWMutex
	names map[string]string
	limit int
}

func newSemaCanonicalNames(limit int) *semaCanonicalNames {
	if limit < 0 {
		limit = 0
	}
	return &semaCanonicalNames{limit: limit}
}

func (c *semaCanonicalNames) canonical(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || !semaNameNeedsCaseFold(name) {
		return name
	}
	if c == nil || c.limit == 0 || len(name) > semaNameMemoMaxBytes {
		return normalizeNameUncached(name)
	}
	c.mu.RLock()
	canonical, ok := c.names[name]
	full := len(c.names) >= c.limit
	c.mu.RUnlock()
	if ok {
		return canonical
	}
	canonical = normalizeNameUncached(name)
	if canonical == name {
		return name
	}
	if full || len(canonical) > semaNameMemoMaxBytes {
		return canonical
	}
	c.mu.Lock()
	if existing, exists := c.names[name]; exists {
		canonical = existing
	} else if len(c.names) < c.limit {
		if c.names == nil {
			c.names = make(map[string]string)
		}
		// Names can be slices of a source file. The changed canonical value
		// already owns its bytes; retain only a copy of the input spelling.
		c.names[strings.Clone(name)] = canonical
	}
	c.mu.Unlock()
	return canonical
}

func (c *semaCanonicalNames) size() int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.names)
}

func semaNameNeedsCaseFold(name string) bool {
	for i := 0; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' || name[i] >= 0x80 {
			return true
		}
	}
	return false
}

var (
	semaPlatformAliasOnce sync.Once
	semaPlatformAliasMap  map[string]string
	semaPlatformNames     = &semaPlatformNameMemo{limit: semaSharedNameMemoLimit}
)

// Both results depend only on the exact input and immutable platform tables.
// Limit entries and bytes, and own retained strings so source slices cannot
// keep entire files alive after an analysis finishes.
type semaPlatformNameMemo struct {
	mu        sync.RWMutex
	aliases   map[string]string
	receivers map[string]bool
	limit     int
}

func (c *semaPlatformNameMemo) canonicalAlias(typeName string) string {
	if c == nil || c.limit <= 0 || typeName == "" || len(typeName) > semaNameMemoMaxBytes {
		return semaCanonicalPlatformAliasUncached(typeName)
	}
	c.mu.RLock()
	canonical, ok := c.aliases[typeName]
	full := len(c.aliases) >= c.limit
	c.mu.RUnlock()
	if ok {
		return canonical
	}
	canonical = semaCanonicalPlatformAliasUncached(typeName)
	if full || len(canonical) > semaNameMemoMaxBytes {
		return canonical
	}
	c.mu.Lock()
	if existing, exists := c.aliases[typeName]; exists {
		canonical = existing
	} else if len(c.aliases) < c.limit {
		if c.aliases == nil {
			c.aliases = make(map[string]string)
		}
		key := strings.Clone(typeName)
		if canonical == typeName {
			canonical = key
		} else {
			canonical = strings.Clone(canonical)
		}
		c.aliases[key] = canonical
	}
	c.mu.Unlock()
	return canonical
}

func (c *semaPlatformNameMemo) knownReceiver(typeName string) bool {
	if c == nil || c.limit <= 0 || typeName == "" || len(typeName) > semaNameMemoMaxBytes {
		return semaKnownPlatformTypeReceiverUncached(typeName)
	}
	c.mu.RLock()
	known, ok := c.receivers[typeName]
	full := len(c.receivers) >= c.limit
	c.mu.RUnlock()
	if ok {
		return known
	}
	known = semaKnownPlatformTypeReceiverUncached(typeName)
	if full {
		return known
	}
	c.mu.Lock()
	if existing, exists := c.receivers[typeName]; exists {
		known = existing
	} else if len(c.receivers) < c.limit {
		if c.receivers == nil {
			c.receivers = make(map[string]bool)
		}
		c.receivers[strings.Clone(typeName)] = known
	}
	c.mu.Unlock()
	return known
}

func semaCanonicalPlatformAlias(typeName string) string {
	return semaPlatformNames.canonicalAlias(typeName)
}

func semaCanonicalPlatformAliasUncached(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		canonicalArgs := make([]string, len(args))
		for i, arg := range args {
			canonicalArgs[i] = semaCanonicalPlatformAliasUncached(arg)
		}
		return semaCanonicalPlatformAliasUncached(base) + "<" + strings.Join(canonicalArgs, ",") + ">"
	}
	if canonical, ok := semaPlatformAlias(typeName); ok {
		return canonical
	}
	return typeName
}

func semaExplicitPlatformQualifiedName(typeName string) bool {
	typeName = strings.TrimSpace(typeName)
	if !strings.Contains(typeName, ".") {
		return false
	}
	canonical := semaCanonicalPlatformAlias(typeName)
	if strings.EqualFold(canonical, typeName) {
		return false
	}
	root, _, _ := strings.Cut(typeName, ".")
	switch normalizeName(root) {
	case "system", "schema", "apexpages":
		return true
	default:
		return false
	}
}

func ensureSemaPlatformAliases() {
	semaPlatformAliasOnce.Do(func() {
		aliases := map[string]string{}
		for _, name := range typesys.StandardSystemNamespaceTypeNames() {
			aliases[normalizeName("System."+name)] = name
		}
		for _, name := range typesys.StandardSchemaNamespaceTypeNames() {
			aliases[normalizeName(name)] = "Schema." + name
			aliases[normalizeName("System."+name)] = "Schema." + name
		}
		aliases[normalizeName("ApexPages.PageReference")] = "PageReference"
		aliases[normalizeName("APEX_OBJECT")] = "Object"
		aliases[normalizeName("System.APEX_OBJECT")] = "Object"
		semaPlatformAliasMap = aliases
	})
}

func semaPlatformAlias(typeName string) (string, bool) {
	ensureSemaPlatformAliases()
	canonical, ok := semaPlatformAliasMap[normalizeName(typeName)]
	return canonical, ok
}

func semaPlatformAliases() map[string]string {
	ensureSemaPlatformAliases()
	aliases := make(map[string]string, len(semaPlatformAliasMap))
	for key, canonical := range semaPlatformAliasMap {
		aliases[key] = canonical
	}
	return aliases
}
