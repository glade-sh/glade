package storage

import "sync"

type standardDefinitionKey struct {
	name, features string
	catalog        *standardDescribeCatalogV2Cache
	raw            *standardDescribeCatalogV2RawCache
}

type standardDefinitionEntry struct {
	once       sync.Once
	definition ObjectDefinition
}

var standardDefinitions sync.Map

// standardCatalogDefinition memoizes only immutable catalog data. Installed
// describe caches are part of the key so a catalog replacement cannot reuse
// definitions built from the previous catalog.
func standardCatalogDefinition(name string, features []string) ObjectDefinition {
	key := standardDefinitionKey{
		name: name, features: canonicalFeatureSignature(features),
		catalog: standardDescribeCatalogV2ProductionCache,
		raw:     standardDescribeCatalogV2RawProductionCache,
	}
	value, ok := standardDefinitions.Load(key)
	if !ok {
		value, _ = standardDefinitions.LoadOrStore(key, &standardDefinitionEntry{})
	}
	entry := value.(*standardDefinitionEntry)
	entry.once.Do(func() {
		entry.definition = ObjectDefinition{APIName: name}
		ensureStandardObjectFieldsForFeatures(&entry.definition, features)
	})
	return cloneStandardCatalogDefinition(entry.definition)
}

// These definitions start with only an API name, so the overlay can populate
// fields, relationships, record types, metadata, and scalar describe labels.
// Project-owned definitions continue through the normal merge path.
func isBareStandardDefinition(d ObjectDefinition) bool {
	return d.Label == "" && d.PluralLabel == "" && d.KeyPrefix == "" &&
		d.OwnerSharingRules == nil && d.SharingModel == "" && !d.EnableSearch &&
		d.Fields == nil && d.Relations == nil && d.RecordTypes == nil &&
		d.ValidationRules == nil && d.AssignmentRules == nil && d.WorkflowRules == nil && d.FlowRules == nil &&
		d.Indexes == nil && d.Metadata == nil
}

// Catalog-only results contain no project rules or indexes. Clone every nested
// mutable field facet; ObjectDefinition.Clone intentionally shares some facets
// and is not a sufficient ownership boundary for a process-wide catalog.
func cloneStandardCatalogDefinition(d ObjectDefinition) ObjectDefinition {
	out := d
	out.Fields = make(map[string]Field, len(d.Fields))
	for name, field := range d.Fields {
		out.Fields[name] = cloneField(field)
	}
	out.Relations = make([]Relationship, len(d.Relations))
	for i, relation := range d.Relations {
		out.Relations[i] = cloneRelationship(relation)
	}
	if d.Relations == nil {
		out.Relations = nil
	}
	out.RecordTypes = make([]RecordTypeInfo, len(d.RecordTypes))
	for i, recordType := range d.RecordTypes {
		out.RecordTypes[i] = cloneRecordTypeInfo(recordType)
	}
	if d.RecordTypes == nil {
		out.RecordTypes = nil
	}
	out.Metadata = cloneStringMap(d.Metadata)
	return out
}
