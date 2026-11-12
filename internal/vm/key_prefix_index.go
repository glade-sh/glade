package vm

import (
	"strings"
	"sync/atomic"

	"github.com/glade-sh/glade/internal/storage"
)

// keyPrefixScans counts full passes over an org's objects, for tests.
var keyPrefixScans atomic.Int64

// keyPrefixIndex maps each key prefix to the first object name, in name
// order, that holds it: the answer a scan of the sorted names reaches. One
// index serves every org clone of a frozen runtime generation.
type keyPrefixIndex struct {
	objects int
	names   map[string]string
}

func buildKeyPrefixIndex(objects map[string]storage.ObjectState) *keyPrefixIndex {
	index := &keyPrefixIndex{objects: len(objects), names: make(map[string]string, len(objects))}
	for name, state := range objects {
		prefix := state.Definition.KeyPrefix
		if first, ok := index.names[prefix]; !ok || name < first {
			index.names[prefix] = name
		}
	}
	return index
}

// orgObjectNameForKeyPrefix answers from the generation's index when the org
// is stamped. Every in-place definition writer clears the stamp, which
// detaches the generation and with it the index. Two guards cover a writer
// that does not: an object count that differs from the indexed one, and a hit
// whose object no longer holds the prefix, both fall back to the scan.
func orgObjectNameForKeyPrefix(org *storage.OrgState, prefix string) (string, bool) {
	if strings.TrimSpace(org.RuntimeSchemaStamp) == "" {
		return scanOrgObjectNameForKeyPrefix(org.Objects, prefix)
	}
	index := org.RuntimeGenerationCacheValue("vm:key-prefix-index", func() any {
		return buildKeyPrefixIndex(org.Objects)
	}).(*keyPrefixIndex)
	if index.objects != len(org.Objects) {
		return scanOrgObjectNameForKeyPrefix(org.Objects, prefix)
	}
	name, ok := index.names[prefix]
	if !ok {
		return "", false
	}
	if org.Objects[name].Definition.KeyPrefix != prefix {
		return scanOrgObjectNameForKeyPrefix(org.Objects, prefix)
	}
	return name, true
}
