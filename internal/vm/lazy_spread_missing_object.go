package vm

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/soql"
	"github.com/glade-sh/glade/internal/storage"
)

// A query that names an object the org does not hold fails in the executor
// with the missing-object message. On a lazy org that failure used to complete
// the whole spread and run the query again. The retry can only change the
// answer when a catalog object the spread still owes can resolve the name, so a
// name that no catalog object can answer to is a final miss.

const (
	missingObjectPrefix = "sObject type '"
	missingObjectMarker = "' is not supported."
)

// missingObjectName extracts the object name from the executor's
// missing-object message. It reports false for any other message, including
// text that merely resembles it.
func missingObjectName(message string) (string, bool) {
	if !strings.HasPrefix(message, missingObjectPrefix) {
		return "", false
	}
	rest := message[len(missingObjectPrefix):]
	end := strings.Index(rest, missingObjectMarker)
	if end < 0 {
		return "", false
	}
	name := rest[:end]
	if soql.MissingObjectMessage(name) != message {
		return "", false
	}
	return name, true
}

// catalogStrippedNames holds the lower-case names that namespaced catalog
// objects (Knowledge__kav and the like) answer to once the namespace token is
// dropped. The org's name resolution matches a custom name against them.
var catalogStrippedNames = sync.OnceValue(func() map[string]struct{} {
	stripped := make(map[string]struct{})
	for _, name := range storage.KnownStandardObjectNames() {
		if !strings.Contains(name, "__") {
			continue
		}
		stripped[strings.ToLower(name)] = struct{}{}
		stripped[strings.ToLower(storage.StripAnyNamespaceToken(name))] = struct{}{}
	}
	return stripped
})

// lazyMissingObjectIsFinal reports whether completing the lazy spread cannot
// make name resolve: neither the name nor any namespace variant the org's
// resolution tries is a catalog object, and no namespaced catalog object
// strips to it. Anything uncertain returns false and takes the old path.
func (vm *VM) lazyMissingObjectIsFinal(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" {
		return false
	}
	candidates := []string{name, storage.StripAnyNamespaceToken(name)}
	if namespace := vm.Org.Namespace; namespace != "" {
		candidates = append(candidates,
			storage.NamespaceTokenName(namespace, name),
			storage.StripNamespaceToken(namespace, name),
			storage.StripAnyNamespaceToken(storage.NamespaceTokenName(namespace, name)))
	}
	stripped := catalogStrippedNames()
	for _, candidate := range candidates {
		if _, ok := storage.ResolveKnownStandardObjectName(candidate); ok {
			return false
		}
		if _, ok := stripped[strings.ToLower(strings.TrimSpace(candidate))]; ok {
			return false
		}
	}
	return true
}
