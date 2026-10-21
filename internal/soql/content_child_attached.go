package soql

import (
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// CD03: an Id-only AttachedContentDocuments subquery on a custom object returns
// the linked child rather than an absent relationship or an empty list. Reuse
// link selection so parent scoping and record deletion follow the existing path.
func executeContentChildAttachedQuery(org storage.OrgState, parentDefinition storage.ObjectDefinition, parent storage.Record, child ChildQuery, cache *childRelationshipQueryCache, context ExecutionContext) ([]storage.Record, bool, error) {
	if !isContentChildAttachedQuery(parentDefinition, child) {
		return nil, false, nil
	}
	// An explicit/project edge retains its existing child-object resolution.
	if _, _, resolved := loadedChildRelationship(org, parentDefinition, child.Relationship); resolved {
		return nil, false, nil
	}
	child.Relationship = "ContentDocumentLinks"
	records, err := executeChildRelationshipQuery(org, parentDefinition, parent, child, cache, context)
	if err != nil {
		return nil, true, err
	}
	for i := range records {
		records[i].Object = "AttachedContentDocument"
	}
	return records, true, nil
}

// Both reference validation and preparation resolve the same captured view.
func contentChildRelationshipForQuery(org storage.OrgState, parent storage.ObjectDefinition, child ChildQuery, cache *childRelationshipQueryCache) (string, storage.Relationship, bool) {
	if isContentChildAttachedQuery(parent, child) {
		if name, relation, ok := loadedChildRelationship(org, parent, child.Relationship); ok {
			return name, relation, true
		}
		return childRelationshipCached(org, parent, "ContentDocumentLinks", cache)
	}
	return childRelationshipCached(org, parent, child.Relationship, cache)
}

func isContentChildAttachedQuery(parent storage.ObjectDefinition, child ChildQuery) bool {
	return strings.EqualFold(child.Relationship, "AttachedContentDocuments") &&
		strings.HasSuffix(strings.ToLower(parent.APIName), "__c") &&
		!storage.IsCustomSettingDefinition(parent) && IsContentChildAttachedIDQuery(child.Query)
}
