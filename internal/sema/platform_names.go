package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/typesys"
)

const semaAnalysisCanonicalNameLimit = 4 * 1024

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
	if c == nil || c.limit == 0 {
		return normalizeName(name)
	}
	c.mu.RLock()
	canonical, ok := c.names[name]
	c.mu.RUnlock()
	if ok {
		return canonical
	}
	canonical = normalizeName(name)
	if canonical == name {
		return name
	}
	c.mu.Lock()
	if existing, exists := c.names[name]; exists {
		canonical = existing
	} else if len(c.names) < c.limit {
		if c.names == nil {
			c.names = make(map[string]string)
		}
		c.names[name] = canonical
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
)

func semaCanonicalPlatformAlias(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	if typeName == "" {
		return typeName
	}
	base, args := semaGenericBaseAndArgs(typeName)
	if len(args) > 0 {
		canonicalArgs := make([]string, len(args))
		for i, arg := range args {
			canonicalArgs[i] = semaCanonicalPlatformAlias(arg)
		}
		return semaCanonicalPlatformAlias(base) + "<" + strings.Join(canonicalArgs, ",") + ">"
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
