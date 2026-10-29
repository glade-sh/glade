package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type flowUnstoredSubflowInputFieldError struct{ reference string }

func (e *flowUnstoredSubflowInputFieldError) Error() string {
	return "dml: flow subflow input field was not stored: " + e.reference
}

// A selected-field Get Records output must retain the distinction between an
// unstored field and a stored field whose value is null across a subflow call.
func flowSubflowInputFieldError(reference string, frame *flowFrame, namespace string) error {
	if frame == nil {
		return nil
	}
	name, path, nested := strings.Cut(strings.TrimSpace(reference), ".")
	output, found := frame.subflowInputs[flowFrameKey(name)]
	if !nested || !found || len(output.queriedFields) == 0 {
		return nil
	}
	field, known := storage.ResolveFieldName(output.definition, namespace, path)
	if !known || strings.EqualFold(field, "Id") {
		return nil
	}
	if record, present := frame.records[flowFrameKey(name)]; present {
		if _, stored := sourceRecordFieldValue(record, field); stored {
			return nil
		}
	}
	for _, selected := range output.queriedFields {
		loaded, ok := storage.ResolveFieldName(output.definition, namespace, selected)
		if ok && strings.EqualFold(loaded, field) {
			return nil
		}
	}
	return &flowUnstoredSubflowInputFieldError{reference: reference}
}
