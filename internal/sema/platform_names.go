package sema

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/typesys"
)

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
	if canonical, ok := semaPlatformAliases()[normalizeName(typeName)]; ok {
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

func semaPlatformAliases() map[string]string {
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
	return semaPlatformAliasMap
}
