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
	if attachmentViewRelationshipLoaded(org, definition, child.Relationship, cache) {
		return nil, false, nil
	}

	// Fixture imports can populate the view table while its relationship is
	// still resolved by the platform catalog. Use the stored rows for that
	// parent, with ordinary child visibility and projection.
	if viewObjectName, loaded := storage.ResolveObjectName(org, viewName); loaded {
		viewObject := org.Objects[viewObjectName]
		if len(viewObject.Records) > 0 {
			index := childRelationshipIndex(org, viewObjectName, viewObject, "ParentId", cache)
			for _, id := range index[storage.IDIndexKey(parent.ID)] {
				row := viewObject.Records[storage.ID(id)]
				if parentID, ok := recordValue(org, viewObject.Definition, row, "ParentId"); ok && equalValues(parentID, storage.IDValue(parent.ID)) {
					return nil, false, nil
				}
			}
		}
	}

	sources := []string{"Notes", "Attachments"}
	if viewName == "CombinedAttachment" {
		sources = append(sources, "ContentDocumentLinks")
	}
	// Keep the platform backing edges and their implicit projections separate
	// from sibling subqueries, which can resolve custom relationships or select
	// different fields.
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
		rows, err := executeChildRelationshipRows(org, definition, parent, source, cache, context, true)
		if err != nil {
			return nil, true, err
		}
		for _, row := range rows {
			id := row.ID
			if file {
				id = idFromValue(row.Fields["ContentDocumentId"])
				if id == "" {
					continue
				}
			}
			if context.ChildRecordVisible != nil {
				candidate := storage.Record{
					ID: id, Object: viewName,
					Fields: map[string]storage.Value{"Id": storage.IDValue(id), "ParentId": storage.IDValue(parent.ID)},
				}
				if !context.ChildRecordVisible(viewName, candidate) {
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

func attachmentViewRelationshipLoaded(org storage.OrgState, definition storage.ObjectDefinition, relationship string, cache *childRelationshipQueryCache) bool {
	key := childRelationshipCacheKey(org.Namespace, definition.APIName, relationship)
	if cache != nil {
		if loaded, found := cache.loadedAttachmentViews[key]; found {
			return loaded
		}
	}
	_, _, loaded := loadedChildRelationship(org, definition, relationship)
	if cache != nil {
		if cache.loadedAttachmentViews == nil {
			cache.loadedAttachmentViews = make(map[string]bool)
		}
		cache.loadedAttachmentViews[key] = loaded
	}
	return loaded
}
