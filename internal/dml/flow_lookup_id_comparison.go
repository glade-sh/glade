package dml

import "github.com/glade-sh/glade/internal/storage"

func flowLookupIDValuesEqual(field storage.Field, left, right storage.Value) bool {
	// An Apex String input can hold the 18-character representation of a
	// record's stored 15-character Id. Compare identity only for declared Id
	// fields; ordinary text and unsupported operand shapes keep their rules.
	if field.Type == storage.FieldID {
		leftID, rightID := idFromStorageValue(left), idFromStorageValue(right)
		if storage.ValidateID(leftID) == nil && storage.ValidateID(rightID) == nil {
			return storage.IDsEqual(leftID, rightID)
		}
	}
	return storageValuesEqual(field, left, right)
}
