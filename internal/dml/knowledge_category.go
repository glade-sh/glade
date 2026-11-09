package dml

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

func (e *Engine) validateKnowledgeCategoryGroup(record storage.Record) error {
	if !strings.EqualFold(record.Object, "Knowledge__DataCategorySelection") {
		return nil
	}
	group, ok := record.GetField("DataCategoryGroupName")
	if !ok || group.Kind != storage.ValueString {
		return nil
	}
	for _, candidate := range e.Org.Metadata.DataCategoryGroups {
		if strings.EqualFold(candidate.Name, group.String) {
			return nil
		}
	}
	return dmlErrorf("INVALID_DATA_CATEGORY_GROUP_REFERENCE", nil, "invalid data category group referenced")
}
