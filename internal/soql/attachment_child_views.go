package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type attachmentViewPlanKey struct {
	parent, view string
}

type attachmentViewPlan struct {
	loaded          bool
	importedObject  string
	sourcesPrepared bool
	sources         []ChildQuery
}

// The Id-only attachment views on ordinary custom records read from their
// backing child relationships. Run those selections through the normal child
// path so they share its parent matching and visibility rules.
func executeAttachmentViewQuery(org storage.OrgState, definition storage.ObjectDefinition, parent storage.Record, child ChildQuery, cache *childRelationshipQueryCache, context ExecutionContext) ([]storage.Record, bool, error) {
	var viewName string
	switch {
	case strings.EqualFold(child.Relationship, "NotesAndAttachments"):
		viewName = "NoteAndAttachment"
	case strings.EqualFold(child.Relationship, "CombinedAttachments"):
		viewName = "CombinedAttachment"
	default:
		return nil, false, nil
	}
	name := definition.APIName
	if len(name) < 3 || !strings.EqualFold(name[len(name)-3:], "__c") ||
		storage.StripAnyNamespaceToken(definition.APIName) != definition.APIName ||
		storage.IsCustomSettingDefinition(definition) || !IsContentChildAttachedIDQuery(child.Query) {
		return nil, false, nil
	}
	plan := planAttachmentView(org, definition, child.Relationship, viewName, cache)
	if plan.loaded {
		return nil, false, nil
	}

	// Fixture imports can populate the view table while its relationship is
	// still resolved by the platform catalog. Use the stored rows for that
	// parent, with ordinary child visibility and projection.
	if plan.importedObject != "" {
		viewObject := org.Objects[plan.importedObject]
		index := childRelationshipIndex(org, plan.importedObject, viewObject, "ParentId", cache)
		for _, id := range index[storage.IDIndexKey(parent.ID)] {
			row := viewObject.Records[storage.ID(id)]
			if parentID, ok := recordValue(org, viewObject.Definition, row, "ParentId"); ok && equalValues(parentID, storage.IDValue(parent.ID)) {
				return nil, false, nil
			}
		}
	}

	result := make([]storage.Record, 0)
	parentReadable := context.ChildFieldReadable != nil && context.ChildFieldReadable(viewName, "ParentId")
	if plan.sourcesPrepared && len(plan.sources) == 0 {
		return result, true, nil
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
	sources := plan.sources
	if !plan.sourcesPrepared {
		note, attachment, link := child, child, child
		note.Relationship = "Notes"
		attachment.Relationship = "Attachments"
		sources = []ChildQuery{note, attachment}
		if viewName == "CombinedAttachment" {
			link.Relationship = "ContentDocumentLinks"
			link.Query.Fields = []string{"Id", "ContentDocumentId"}
			sources = append(sources, link)
		}
	}
	for _, source := range sources {
		prepared, err := prepareChildRelationshipQuery(org, definition, source, cache, context, true)
		if err != nil {
			return nil, true, err
		}
		if prepared.emptyAttachmentSource {
			continue
		}
		if !plan.sourcesPrepared {
			plan.sources = append(plan.sources, source)
		}
		rows, err := executePreparedChildRelationshipRows(org, definition, parent, source, cache, context, prepared)
		if err != nil {
			return nil, true, err
		}
		file := source.Relationship == "ContentDocumentLinks"
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
	plan.sourcesPrepared = true
	return result, true, nil
}

func planAttachmentView(org storage.OrgState, definition storage.ObjectDefinition, relationship, view string, cache *childRelationshipQueryCache) *attachmentViewPlan {
	key := attachmentViewPlanKey{parent: definition.APIName, view: view}
	if cache != nil {
		if plan := cache.attachmentViews[key]; plan != nil {
			return plan
		}
	}
	// Reference validation already resolves the loaded and platform edges.
	// Reuse its schema result instead of scanning the standard spread again.
	resolution := childRelationshipResolutionCached(org, definition, relationship, cache)
	plan := &attachmentViewPlan{loaded: resolution.loaded}
	if org.StandardSpread != "" && resolution.objectCount != len(org.Objects) {
		// Lazy materialization during validation can expose more loaded edges.
		_, _, plan.loaded = loadedChildRelationship(org, definition, relationship)
	}
	if !plan.loaded {
		if name, ok := storage.ResolveObjectName(org, view); ok && len(org.Objects[name].Records) > 0 {
			plan.importedObject = name
		}
	}
	if cache != nil {
		if cache.attachmentViews == nil {
			cache.attachmentViews = make(map[attachmentViewPlanKey]*attachmentViewPlan)
		}
		cache.attachmentViews[key] = plan
	}
	return plan
}
