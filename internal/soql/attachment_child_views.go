package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// The Id-only attachment views on ordinary custom records read from their
// backing child relationships. Run those selections through the normal child
// path so they share its parent matching and visibility rules.
func executeAttachmentViewQuery(org storage.OrgState, definition storage.ObjectDefinition, parent storage.Record, child ChildQuery, cache *childRelationshipQueryCache, context ExecutionContext) ([]storage.Record, bool, error) {
	var viewName string
	switch strings.ToLower(child.Relationship) {
	case "notesandattachments":
		viewName = "NoteAndAttachment"
	case "combinedattachments":
		viewName = "CombinedAttachment"
	default:
		return nil, false, nil
	}
	if !strings.HasSuffix(strings.ToLower(definition.APIName), "__c") ||
		storage.StripAnyNamespaceToken(definition.APIName) != definition.APIName ||
		storage.IsCustomSettingDefinition(definition) || !IsContentChildAttachedIDQuery(child.Query) {
		return nil, false, nil
	}
	if _, _, loaded := loadedChildRelationship(org, definition, child.Relationship); loaded {
		return nil, false, nil
	}

	sources := []string{"Notes", "Attachments"}
	if viewName == "CombinedAttachment" {
		sources = append(sources, "ContentDocumentLinks")
	}
	// Keep the implicit projections separate from sibling subqueries on the
	// same source relationships, which can select different fields.
	if cache != nil {
		if cache.attachmentSources == nil {
			cache.attachmentSources = newChildRelationshipQueryCache(cache.execution)
			cache.attachmentSources.indexes = cache.indexes
		}
		cache = cache.attachmentSources
	}
	result := make([]storage.Record, 0)
	parentReadable := context.ChildFieldReadable != nil && context.ChildFieldReadable(viewName, "ParentId")
	for _, relationship := range sources {
		source := child
		source.Relationship = relationship
		file := relationship == "ContentDocumentLinks"
		if file {
			source.Query.Fields = []string{"Id", "ContentDocumentId"}
		}
		rows, err := executeChildRelationshipQuery(org, definition, parent, source, cache, context)
		if err != nil {
			return nil, true, err
		}
		for _, row := range rows {
			id := row.ID
			if file {
				id = row.Fields["ContentDocumentId"].ID
				if id == "" {
					continue
				}
			}
			fields := map[string]storage.Value{"Id": storage.IDValue(id)}
			if parentReadable {
				fields["ParentId"] = storage.IDValue(parent.ID)
			}
			result = append(result, storage.Record{ID: id, Object: viewName, Fields: fields})
		}
	}
	return result, true, nil
}
