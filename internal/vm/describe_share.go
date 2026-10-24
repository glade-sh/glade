package vm

import (
	"maps"
	"reflect"
	"strings"
	"sync"
	"weak"

	"github.com/glade-sh/glade/internal/storage"
)

// describeShareCache holds the schema-derived parts of SObject describes for
// one runtime template generation: prepared object definitions and SObject
// field maps with their field tokens. PrimeMetadataSchema creates it, every
// clone of the primed base shares the pointer, and clearMetadataCaches drops
// it. A VM uses an entry only while its installed org still carries the stamp
// the cache was created for and the caller passes the org's own definition
// (the one the entry was built from). A clone whose schema changed builds
// privately, exactly as an unprimed VM does.
//
// The cache holds its entries weakly and each VM holds the entries it used
// (describeShareDefinitions, sharedDescribeFieldMaps). Clones running at the
// same time share one build; an entry that no live VM holds is freed instead
// of staying for the rest of the run (a full standard-schema sweep builds about
// 80 MB of prepared definitions and 450 MB of field maps).
//
// Entries are never written after they are built. The describe root, its
// "fields" wrapper and every other per-test value stay private to each VM.
// Field maps are read-only to Apex in this runtime, and code reaches a field map
// or a field token only through privateDescribeFieldMap, which first swaps a
// private copy into that VM's wrapper. Tokens that Apex code can write are
// therefore never shared between clones.
type describeShareCache struct {
	stamp       string
	mu          sync.Mutex
	definitions map[string]weak.Pointer[describeShareDefinition]
	fieldMaps   map[describeShareFieldMapKey]weak.Pointer[describeShareFieldMap]
}

// Entries are built under their own lock by the first VM that needs them. A
// build that panics leaves the entry unbuilt, so the next VM builds it again.
type describeShareDefinition struct {
	mu         sync.Mutex
	built      bool
	source     uintptr
	definition storage.ObjectDefinition
}

// describeShareFieldMapKey names one field map build. The map depends on the
// object, its prepared definition and the caller namespace (describe field
// names). The per-VM describe cache key carries the permission identity; keying
// on it too keeps one field map per per-VM describe root, as without sharing.
type describeShareFieldMapKey struct {
	object          string
	callerNamespace string
	describeKey     describeCacheKey
}

type describeShareFieldMap struct {
	mu     sync.Mutex
	built  bool
	source uintptr
	fields Value
}

func newDescribeShareCache(stamp string) *describeShareCache {
	return &describeShareCache{
		stamp:       stamp,
		definitions: make(map[string]weak.Pointer[describeShareDefinition]),
		fieldMaps:   make(map[describeShareFieldMapKey]weak.Pointer[describeShareFieldMap]),
	}
}

// describeShareSource identifies an org object definition by its field map.
// Frozen runtime clones share definition maps with their template; replacing
// a definition (Clone, EnsureMutableObjectDefinition) allocates a new map.
func describeShareSource(definition storage.ObjectDefinition) uintptr {
	if definition.Fields == nil {
		return 0
	}
	return reflect.ValueOf(definition.Fields).Pointer()
}

// sharedDescribeFor returns the shared describe cache when name is an object of
// the installed org, definition is that object's definition, and the org still
// carries the cache's schema stamp.
func (vm *VM) sharedDescribeFor(name string, definition storage.ObjectDefinition) (*describeShareCache, uintptr) {
	if vm == nil || vm.describeShare == nil || vm.Org == nil {
		return nil, 0
	}
	share := vm.describeShare
	if share.stamp == "" || strings.TrimSpace(vm.Org.RuntimeSchemaStamp) != share.stamp {
		return nil, 0
	}
	object, ok := vm.Org.Objects[name]
	if !ok {
		return nil, 0
	}
	source := describeShareSource(object.Definition)
	if source == 0 || source != describeShareSource(definition) {
		return nil, 0
	}
	return share, source
}

// sharedPreparedDefinition returns the shared prepared definition for name,
// building it with build on first use. ok is false when the entry was built
// from another definition; the caller then prepares its own.
func (vm *VM) sharedPreparedDefinition(c *describeShareCache, name string, source uintptr, build func() storage.ObjectDefinition) (storage.ObjectDefinition, bool) {
	c.mu.Lock()
	entry := c.definitions[name].Value()
	if entry == nil {
		entry = &describeShareDefinition{source: source}
		c.definitions[name] = weak.Make(entry)
	}
	c.mu.Unlock()
	if entry.source != source {
		return storage.ObjectDefinition{}, false
	}
	vm.describeShareDefinitions = append(vm.describeShareDefinitions, entry)
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if !entry.built {
		entry.definition = build()
		entry.built = true
	}
	return entry.definition, true
}

// sharedFieldMap returns the shared field map for key, building it with build
// on first use, and records it as held by this VM. ok is false when the entry
// was built from another definition.
func (vm *VM) sharedFieldMap(c *describeShareCache, key describeShareFieldMapKey, source uintptr, build func() Value) (Value, bool) {
	c.mu.Lock()
	entry := c.fieldMaps[key].Value()
	if entry == nil {
		entry = &describeShareFieldMap{source: source}
		c.fieldMaps[key] = weak.Make(entry)
	}
	c.mu.Unlock()
	if entry.source != source {
		return Null, false
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if !entry.built {
		entry.fields = build()
		entry.built = true
	}
	if identity := describeFieldMapIdentity(entry.fields); identity != 0 {
		if vm.sharedDescribeFieldMaps == nil {
			vm.sharedDescribeFieldMaps = make(map[uintptr]*describeShareFieldMap)
		}
		vm.sharedDescribeFieldMaps[identity] = entry
	}
	return entry.fields, true
}

func describeFieldMapIdentity(fields Value) uintptr {
	if fields.Map == nil {
		return 0
	}
	return reflect.ValueOf(fields.Map).Pointer()
}

// privateDescribeFieldMap returns the map behind a describe wrapper (any
// object with a "map" field) before Apex code can reach it. A field map shared
// between runtime clones is replaced in the wrapper by this VM's own copy, so
// every holder of the wrapper in this VM sees the same map and field tokens,
// as when the VM built them itself, and no other VM sees this VM's writes.
func (vm *VM) privateDescribeFieldMap(owner Value) (Value, bool) {
	mapField, ok := owner.Fields["map"]
	if !ok || vm == nil || len(vm.sharedDescribeFieldMaps) == 0 || mapField.Kind != ValueMap {
		return mapField, ok
	}
	if _, shared := vm.sharedDescribeFieldMaps[describeFieldMapIdentity(mapField)]; !shared {
		return mapField, true
	}
	private := copySharedDescribeFieldMap(mapField)
	owner.Fields["map"] = private
	vm.rememberPublishedObjectFieldRefs(owner, mapField, private)
	return private, true
}

// copySharedDescribeFieldMap copies a shared field map and its field tokens.
// Entries that shared one token (the aliases of a field) share one copy.
func copySharedDescribeFieldMap(shared Value) Value {
	out := shared
	out.Ref = newValueRef()
	tokens := make(map[uintptr]Value, len(shared.Map)/2+1)
	copyToken := func(token Value) Value {
		if token.Kind != ValueObject || token.Fields == nil {
			return token
		}
		identity := reflect.ValueOf(token.Fields).Pointer()
		if copied, ok := tokens[identity]; ok {
			return copied
		}
		copied := token
		copied.Ref = newValueRef()
		copied.Fields = maps.Clone(token.Fields)
		tokens[identity] = copied
		return copied
	}
	out.Map = make(map[string]Value, len(shared.Map))
	for key, token := range shared.Map {
		out.Map[key] = copyToken(token)
	}
	out.MapKeys = make(map[string]Value, len(shared.MapKeys))
	for key, value := range shared.MapKeys {
		out.MapKeys[key] = value
	}
	out.MapOrder = append([]string(nil), shared.MapOrder...)
	return out
}
