package dml

import "github.com/glade-sh/glade/internal/storage"

// The storage insert boundary is after before-insert triggers. An explicit null
// remains visible in those triggers, then receives the platform currency default.
func (e *Engine) applyCurrencyInsertDefaults(definition storage.ObjectDefinition, record *storage.Record) {
	for name, field := range definition.Fields {
		if current, present := record.GetField(name); present && current.Kind != storage.ValueNull {
			continue
		}
		if value, ok := storage.CurrencyDefaultForField(e.Org, e.systemUserID(), field); ok {
			record.Fields[name] = value
			delete(record.ExplicitNulls, name)
		}
	}
}
