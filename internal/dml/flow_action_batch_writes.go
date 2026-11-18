package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// recordBatchUpdateFields records explicit filtered writes to this interview's
// own row, including writes that do not change the stored value.
func (frame *flowFrame) recordBatchUpdateFields(source, update storage.Record) {
	if frame == nil || source.ID == "" || !storage.IDsEqual(source.ID, update.ID) || !strings.EqualFold(source.Object, update.Object) {
		return
	}
	if frame.batchUpdatedFields == nil {
		frame.batchUpdatedFields = make(map[string]bool)
	}
	for field := range update.Fields {
		frame.batchUpdatedFields[field] = true
	}
	for field, set := range update.ExplicitNulls {
		if set {
			frame.batchUpdatedFields[field] = true
		}
	}
}
