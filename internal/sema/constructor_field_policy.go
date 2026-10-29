package sema

import (
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

// Constructor admission uses the same effective schema as the runtime, built
// from declarations captured before source inference. Build the complete
// registry once so inherited Activity fields and generated objects keep their
// normal schema relationships; cache immutable results for parallel analysis.
func (a *Analyzer) standardConstructorDefinition(name string, features []string, model *semaTypeMemberView) storage.ObjectDefinition {
	if _, standard := storage.ResolveKnownStandardObjectName(name); !standard {
		return storage.ObjectDefinition{APIName: name}
	}
	key := normalizeName(name)
	base := model.state.base
	base.constructorSchemaOnce.Do(func() {
		registry := sobject.BuildDescribeRegistry(schema.Schema{Objects: a.queryDeclaredObjects})
		for name, describe := range registry.Objects {
			definition := sobject.ToObjectDefinition(describe)
			storage.EnsureStandardObjectFieldsForFeatures(&definition, features)
			base.constructorDefinitions.Store(normalizeName(name), definition)
		}
	})
	if cached, ok := base.constructorDefinitions.Load(key); ok {
		return cached.(storage.ObjectDefinition)
	}
	definition := storage.ObjectDefinition{APIName: name}
	storage.EnsureStandardObjectFieldsForFeatures(&definition, features)
	actual, _ := base.constructorDefinitions.LoadOrStore(key, definition)
	return actual.(storage.ObjectDefinition)
}
