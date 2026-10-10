package storage

import (
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"
)

// objectNameIndexKey is the objectNameCache key of the case-folded object-name
// index. Its type keeps it apart from the resolved-name string keys.
type objectNameIndexKey struct{}

// objectNameIndex groups the object names of one Objects map by their
// strings.EqualFold class, so ResolveObjectName does not scan every object for
// a name that is not an exact key. It is valid only for the map it was built
// from and only while that map has the same number of objects. Additions raise
// the count, and the index is then extended with the added names. Deletions go
// through DeleteObject, which drops the index. Clones of the org get a copy of
// the index for their own map.
type objectNameIndex struct {
	objects map[string]ObjectState
	count   int
	base    *objectNameTables // shared with clones; never modified
	added   *objectNameTables // names added to this map after base; may be nil
	misses  *sync.Map         // objectNameMiss -> struct{}
}

type objectNameTables struct {
	names          map[string]struct{}
	byFold         map[string][]string // fold(name) -> names, sorted
	byStrippedFold map[string][]string // fold(StripAnyNamespaceToken(name)) -> names, sorted
}

// objectNameMiss is a name that did not resolve under a namespace. Misses
// depend only on the object names and the namespace, so they live with the
// index and are dropped with it.
type objectNameMiss struct {
	namespace string
	name      string
}

// objectNameIndexBuilds counts full index builds so tests can prove reuse.
var objectNameIndexBuilds atomic.Uint64

func buildObjectNameIndex(objects map[string]ObjectState) *objectNameIndex {
	objectNameIndexBuilds.Add(1)
	names := make([]string, 0, len(objects))
	for name := range objects {
		names = append(names, name)
	}
	return &objectNameIndex{
		objects: objects,
		count:   len(objects),
		base:    newObjectNameTables(names),
		misses:  &sync.Map{},
	}
}

// extend returns an index for objects, which holds every name of index plus
// the names added since. It reuses the base tables.
func (index *objectNameIndex) extend(objects map[string]ObjectState) *objectNameIndex {
	var added []string
	for name := range objects {
		if _, ok := index.base.names[name]; !ok {
			added = append(added, name)
		}
	}
	return &objectNameIndex{
		objects: objects,
		count:   len(objects),
		base:    index.base,
		added:   newObjectNameTables(added),
		misses:  &sync.Map{},
	}
}

func newObjectNameTables(names []string) *objectNameTables {
	tables := &objectNameTables{
		names:          make(map[string]struct{}, len(names)),
		byFold:         make(map[string][]string, len(names)),
		byStrippedFold: make(map[string][]string, len(names)),
	}
	for _, name := range names {
		tables.names[name] = struct{}{}
		key := objectNameFoldKey(name)
		tables.byFold[key] = append(tables.byFold[key], name)
		if stripped := StripAnyNamespaceToken(name); stripped != name {
			key = objectNameFoldKey(stripped)
		}
		tables.byStrippedFold[key] = append(tables.byStrippedFold[key], name)
	}
	for _, names := range tables.byFold {
		if len(names) > 1 {
			sort.Strings(names)
		}
	}
	for _, names := range tables.byStrippedFold {
		if len(names) > 1 {
			sort.Strings(names)
		}
	}
	return tables
}

func (index *objectNameIndex) indexes(objects map[string]ObjectState) bool {
	return index != nil && len(objects) == index.count && sameObjectMap(index.objects, objects)
}

func sameObjectMap(left, right map[string]ObjectState) bool {
	return reflect.ValueOf(left).UnsafePointer() == reflect.ValueOf(right).UnsafePointer()
}

// objectNameIndexFor returns the org's index, building it on first use,
// extending it after additions, and rebuilding it for another map. It returns
// nil when the org has no cache.
func objectNameIndexFor(org OrgState) *objectNameIndex {
	if org.objectNameCache == nil {
		return nil
	}
	var index *objectNameIndex
	if cached, ok := org.objectNameCache.Load(objectNameIndexKey{}); ok {
		index, _ = cached.(*objectNameIndex)
	}
	switch {
	case index.indexes(org.Objects):
		return index
	case index != nil && len(org.Objects) > index.count && sameObjectMap(index.objects, org.Objects):
		index = index.extend(org.Objects)
	default:
		index = buildObjectNameIndex(org.Objects)
	}
	org.objectNameCache.Store(objectNameIndexKey{}, index)
	return index
}

// PrimeObjectNameIndex builds the org's object-name index before first use. A
// template org primed this way hands the index to each of its clones, which
// would otherwise build one apiece.
func PrimeObjectNameIndex(org OrgState) {
	objectNameIndexFor(org)
}

// newObjectNameCacheFrom returns a fresh cache for a clone whose Objects map
// has the same names as the source map. It carries the source's index, if it
// is current, so the clone does not rebuild it. Resolved names and misses are
// not carried.
func newObjectNameCacheFrom(source *sync.Map, sourceObjects, cloneObjects map[string]ObjectState) *sync.Map {
	out := &sync.Map{}
	if source == nil || len(sourceObjects) != len(cloneObjects) {
		return out
	}
	cached, ok := source.Load(objectNameIndexKey{})
	if !ok {
		return out
	}
	index, _ := cached.(*objectNameIndex)
	if !index.indexes(sourceObjects) {
		return out
	}
	out.Store(objectNameIndexKey{}, &objectNameIndex{
		objects: cloneObjects,
		count:   index.count,
		base:    index.base,
		added:   index.added,
		misses:  &sync.Map{},
	})
	return out
}

// DeleteObject removes an object from the org and drops the org's name cache.
// Product code deletes objects only through this function, because the cache
// assumes that a map with an unchanged count holds unchanged names.
func DeleteObject(org *OrgState, name string) {
	if org == nil {
		return
	}
	delete(org.Objects, name)
	if org.objectNameCache != nil {
		org.objectNameCache.Clear()
	}
}

// firstObjectNameFolding returns the smallest object name that equals one of
// names under strings.EqualFold.
func firstObjectNameFolding(org OrgState, index *objectNameIndex, names ...string) (string, bool) {
	var first string
	found := false
	consider := func(candidate string) {
		if !found || candidate < first {
			first, found = candidate, true
		}
	}
	if index == nil {
		for candidate := range org.Objects {
			for _, name := range names {
				if strings.EqualFold(candidate, name) {
					consider(candidate)
					break
				}
			}
		}
		return first, found
	}
	for _, name := range names {
		key := objectNameFoldKey(name)
		if candidates := index.base.byFold[key]; len(candidates) > 0 {
			consider(candidates[0])
		}
		if index.added != nil {
			if candidates := index.added.byFold[key]; len(candidates) > 0 {
				consider(candidates[0])
			}
		}
	}
	return first, found
}

// objectNamesStrippingTo returns the object names whose namespace-stripped
// form equals name under strings.EqualFold, sorted. The result may be shared
// and must not be modified.
func objectNamesStrippingTo(org OrgState, index *objectNameIndex, name string) []string {
	if index != nil {
		key := objectNameFoldKey(name)
		names := index.base.byStrippedFold[key]
		if index.added == nil || len(index.added.byStrippedFold[key]) == 0 {
			return names
		}
		merged := append(append([]string(nil), names...), index.added.byStrippedFold[key]...)
		sort.Strings(merged)
		return merged
	}
	var out []string
	for candidate := range org.Objects {
		if strings.EqualFold(StripAnyNamespaceToken(candidate), name) {
			out = append(out, candidate)
		}
	}
	sort.Strings(out)
	return out
}

// objectNameFoldKey maps s to a key that is equal for two strings exactly when
// strings.EqualFold reports them equal: each rune becomes the smallest rune of
// its simple case-folding orbit, and an invalid byte becomes U+FFFD.
func objectNameFoldKey(s string) string {
	i := 0
	for ; i < len(s); i++ {
		c := s[i]
		if c >= utf8.RuneSelf || ('a' <= c && c <= 'z') {
			break
		}
	}
	if i == len(s) {
		return s
	}
	var b strings.Builder
	b.Grow(len(s))
	b.WriteString(s[:i])
	for _, r := range s[i:] {
		switch {
		case r < utf8.RuneSelf:
			if 'a' <= r && r <= 'z' {
				r -= 'a' - 'A'
			}
			b.WriteByte(byte(r))
		default:
			b.WriteRune(smallestFoldRune(r))
		}
	}
	return b.String()
}

func smallestFoldRune(r rune) rune {
	smallest := r
	for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
		smallest = min(smallest, f)
	}
	return smallest
}
