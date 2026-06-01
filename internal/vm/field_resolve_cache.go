package vm

import (
	"sync"

	"github.com/glade-sh/glade/internal/storage"
)

type fieldResolveLookupKey struct {
	ObjectName      string
	Namespace       string
	FieldName       string
	FieldCount      int
	RelationCount   int
	RecordTypeCount int
}

type fieldResolveLookup struct {
	FieldName string
	OK        bool
}

type fieldResolveLookupCache struct {
	mu      sync.RWMutex
	entries map[fieldResolveLookupKey]fieldResolveLookup
}

func newFieldResolveLookupCache() *fieldResolveLookupCache {
	return &fieldResolveLookupCache{entries: make(map[fieldResolveLookupKey]fieldResolveLookup)}
}

func (c *fieldResolveLookupCache) load(key fieldResolveLookupKey) (fieldResolveLookup, bool) {
	if c == nil {
		return fieldResolveLookup{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	value, ok := c.entries[key]
	return value, ok
}

func (c *fieldResolveLookupCache) store(key fieldResolveLookupKey, value fieldResolveLookup) fieldResolveLookup {
	if c == nil {
		return value
	}
	c.mu.Lock()
	c.entries[key] = value
	c.mu.Unlock()
	return value
}

func (vm *VM) resolveFieldName(definition storage.ObjectDefinition, name string) (string, bool) {
	namespace := ""
	if vm != nil && vm.Org != nil {
		namespace = vm.Org.Namespace
	}
	if vm == nil || definition.APIName == "" {
		return storage.ResolveFieldName(definition, namespace, name)
	}
	if vm.fieldResolveCache == nil {
		vm.fieldResolveCache = newFieldResolveLookupCache()
	}
	key := fieldResolveLookupKey{
		ObjectName:      definition.APIName,
		Namespace:       namespace,
		FieldName:       name,
		FieldCount:      len(definition.Fields),
		RelationCount:   len(definition.Relations),
		RecordTypeCount: len(definition.RecordTypes),
	}
	if cached, ok := vm.fieldResolveCache.load(key); ok {
		return cached.FieldName, cached.OK
	}
	resolved, ok := storage.ResolveFieldName(definition, namespace, name)
	cached := vm.fieldResolveCache.store(key, fieldResolveLookup{FieldName: resolved, OK: ok})
	return cached.FieldName, cached.OK
}
