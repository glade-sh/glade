package storage

// StandardChildRelationship is one child-relationship edge of the standard
// object catalog: Child declares a lookup whose parent-side relationship Name
// lists Child records on each of Parents.
type StandardChildRelationship struct {
	Child   string
	Name    string
	Parents []string
}

// StandardChildRelationships returns every standard catalog child relationship
// in catalog walk order, from the generated table. The table equals
// standardChildRelationshipsFromCatalog, which decodes every catalog member;
// readers that only need the parent-side relationship names use the table and
// skip that decode. Callers must not modify the returned entries.
func StandardChildRelationships() []StandardChildRelationship {
	return standardChildRelationshipTable[:]
}

// standardChildRelationshipsFromCatalog is the reference walk the generated
// table reproduces: every known standard object by canonical name, in
// KnownStandardObjectNames order, with each relationship that names a child
// relationship, in VisitStandardObjectRelationships order for the base feature
// profile.
func standardChildRelationshipsFromCatalog() []StandardChildRelationship {
	var out []StandardChildRelationship
	for _, childObject := range KnownStandardObjectNames() {
		childName := childObject
		if canonical, ok := ResolveKnownStandardObjectName(childObject); ok {
			childName = canonical
		}
		VisitStandardObjectRelationships(childName, nil, func(relationship Relationship) {
			if relationship.ChildRelationship == "" {
				return
			}
			out = append(out, StandardChildRelationship{Child: childName, Name: relationship.ChildRelationship, Parents: append([]string(nil), relationship.ParentObjects...)})
		})
	}
	return out
}
