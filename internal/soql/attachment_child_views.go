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
	importedIndex   map[storage.ID][]string
	sourcesPrepared bool
	sources         []attachmentViewSource
}

type attachmentViewSource struct {
	child ChildQuery
	index map[storage.ID][]string
}

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
	plan := planAttachmentView(org, definition, child.Relationship, viewName, cache)
	if plan.loaded {
		return nil, false, nil
	}

	// Fixture imports can populate the view table while its relationship is
	// still resolved by the platform catalog. Use the stored rows for that
	// parent, with ordinary child visibility and projection.
	parentKey := storage.IDIndexKey(parent.ID)
	if imported := plan.importedIndex[parentKey]; len(imported) > 0 {
		viewObject := org.Objects[plan.importedObject]
		for _, id := range imported {
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
		sources = []attachmentViewSource{{child: note}, {child: attachment}}
		if viewName == "CombinedAttachment" {
			link.Relationship = "ContentDocumentLinks"
			link.Query.Fields = []string{"Id", "ContentDocumentId"}
			sources = append(sources, attachmentViewSource{child: link})
		}
	}
	for _, source := range sources {
		if !plan.sourcesPrepared {
			prepared, err := prepareChildRelationshipQuery(org, definition, source.child, cache, context, true)
			if err != nil {
				return nil, true, err
			}
			if prepared.emptyAttachmentSource {
				continue
			}
			field := prepared.relation.Field
			// The ordinary row executor uses its parent index when there is
			// no matching storage index. Reuse that index to skip misses.
			if object, loaded := org.Objects[prepared.childObjectName]; loaded && !isSystemRelationshipField(field) {
				if _, indexed := storage.LookupIndex(object, field, storage.IDValue(parent.ID)); !indexed {
					source.index = childRelationshipIndex(org, prepared.childObjectName, object, field, cache)
				}
			}
			plan.sources = append(plan.sources, source)
		}
		if source.index != nil && len(source.index[parentKey]) == 0 {
			continue
		}
		rows, err := executeChildRelationshipRows(org, definition, parent, source.child, cache, context, true)
		if err != nil {
			return nil, true, err
		}
		file := source.child.Relationship == "ContentDocumentLinks"
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
			plan.importedIndex = childRelationshipIndex(org, name, org.Objects[name], "ParentId", cache)
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
