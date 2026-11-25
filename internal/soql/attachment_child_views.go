package soql

import (
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

type attachmentViewPlanKey struct {
	parent, view string
}

const (
	combinedAttachmentsRelationship = "CombinedAttachments"
	notesAndAttachmentsRelationship = "NotesAndAttachments"
)

type attachmentRelationshipSchema struct {
	runtimeGeneration uint64
	schemaGeneration  uint64
	runtimeStamp      string
	objectCount       int
	spreadSource      string
	spreadInventory   string
	spreadFeatures    string
	declarations      string
}

type attachmentRelationshipSchemaSnapshot struct {
	objects   map[string]storage.ObjectState
	namespace string
	schema    attachmentRelationshipSchema
}

func (cache *childRelationshipQueryCache) currentAttachmentRelationshipSchema(org storage.OrgState) attachmentRelationshipSchema {
	// The storage mutation boundary advances the schema and runtime
	// generations. External org replacement can detach just the runtime one.
	schema := attachmentRelationshipSchema{
		runtimeGeneration: org.RuntimeGenerationID(),
		schemaGeneration:  org.SchemaGeneration(),
		runtimeStamp:      org.RuntimeSchemaStamp,
		objectCount:       len(org.Objects),
		spreadSource:      org.StandardSpread,
	}
	if org.StandardSpread != "" {
		schema.spreadFeatures = strings.Join(org.Features, "\x00")
	}
	// Validation, planning and preparation can reuse the declaration scan
	// within a query. The map identity distinguishes lazy views of equal size.
	snapshot := cache.attachmentSchema
	if snapshot != nil && snapshot.namespace == org.Namespace &&
		reflect.ValueOf(snapshot.objects).Pointer() == reflect.ValueOf(org.Objects).Pointer() {
		current := snapshot.schema
		current.spreadInventory, current.declarations = "", ""
		if current == schema {
			return snapshot.schema
		}
	}
	if org.StandardSpread != "" {
		// Source-backed clones can share a generation yet install different
		// catalog objects. Compare their loaded inventories even at equal size.
		schema.spreadInventory = strings.Join(slices.Sorted(maps.Keys(org.Objects)), "\x00")
	}
	schema.declarations = attachmentRelationshipDeclarations(org)
	cache.attachmentSchema = &attachmentRelationshipSchemaSnapshot{objects: org.Objects, namespace: org.Namespace, schema: schema}
	return schema
}

// ExecuteWithCache also accepts org values whose caller edits declarations
// directly. Compare named overrides using the same name rules as the resolver.
// Derived names and catalog changes follow the storage schema generation.
func attachmentRelationshipDeclarations(org storage.OrgState) string {
	var objects []string
	for name, object := range org.Objects {
		definition := object.Definition
		var edges strings.Builder
		for _, relation := range definition.Relations {
			declared := relation.ChildRelationship
			if !isAttachmentRelationshipDeclaration(org.Namespace, declared) {
				continue
			}
			edges.WriteString(declared)
			edges.WriteByte(0)
			edges.WriteString(relation.Field)
			edges.WriteByte(0)
			for _, parent := range relation.ParentObjects {
				edges.WriteString(parent)
				edges.WriteByte(0)
			}
			edges.WriteByte(0)
		}
		if edges.Len() != 0 {
			// Resolution prefers the richer definition when matching edges tie.
			objects = append(objects, strings.Join([]string{
				name, definition.APIName, strconv.Itoa(len(definition.Fields) + len(definition.Relations)), edges.String(),
			}, "\x00"))
		}
	}
	slices.Sort(objects)
	return strings.Join(objects, "\x00")
}

func isAttachmentRelationshipDeclaration(namespace, declared string) bool {
	if len(declared) == len(combinedAttachmentsRelationship) {
		return strings.EqualFold(declared, combinedAttachmentsRelationship) || strings.EqualFold(declared, notesAndAttachmentsRelationship)
	}
	// A longer spelling can match through a namespace token or __r suffix.
	return len(declared) > len(combinedAttachmentsRelationship) && strings.Contains(declared, "__") &&
		(childRelationshipNameMatches(namespace, declared, combinedAttachmentsRelationship) || childRelationshipNameMatches(namespace, declared, notesAndAttachmentsRelationship))
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

func attachmentViewName(definition storage.ObjectDefinition, child ChildQuery) string {
	var viewName string
	switch strings.ToLower(child.Relationship) {
	case "notesandattachments":
		viewName = "NoteAndAttachment"
	case "combinedattachments":
		viewName = "CombinedAttachment"
	default:
		return ""
	}
	if !strings.HasSuffix(strings.ToLower(definition.APIName), "__c") ||
		storage.StripAnyNamespaceToken(definition.APIName) != definition.APIName ||
		storage.IsCustomSettingDefinition(definition) || !IsContentChildAttachedIDQuery(child.Query) {
		return ""
	}
	return viewName
}

// The Id-only attachment views on ordinary custom records read from their
// backing child relationships. Run those selections through the normal child
// path so they share its parent matching and visibility rules.
func executeAttachmentViewQuery(org storage.OrgState, definition storage.ObjectDefinition, parent storage.Record, child ChildQuery, cache *childRelationshipQueryCache, context ExecutionContext) ([]storage.Record, bool, error) {
	viewName := attachmentViewName(definition, child)
	if viewName == "" {
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
	resolution := childRelationshipResolutionCached(org, definition, relationship, cache, true)
	plan := &attachmentViewPlan{loaded: resolution.loaded}
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
