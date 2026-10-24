package apextest

import (
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sobject"
	"github.com/glade-sh/glade/internal/storage"
)

type compileGateQueryScope struct {
	objects []string
	aliases map[string][]string
}

func (w *compileGateRefWalker) walkSchemaQuery(node apexast.ASTNode) {
	if w.units.querySchema == nil {
		w.units.querySchema = &compileGateQuerySchema{}
	}
	w.units.querySchema.initialize(w.units)
	w.walkSchemaQueryScope(node, compileGateQueryScope{}, false)
}

// Query identifiers bind in the query's own object scope. In particular a
// parent hop names its reference targets, not an Apex class or local variable.
func (w *compileGateRefWalker) walkSchemaQueryScope(node apexast.ASTNode, scope compileGateQueryScope, childQuery bool) {
	switch node.Kind {
	case "bound_apex_expression":
		return // The ordinary Apex walker handles binds separately.
	case "soql_query_body":
		for _, from := range node.Children {
			if from.Kind != "from_clause" {
				continue
			}
			scope = w.schemaQueryFrom(from, scope, childQuery)
			break
		}
		childQuery = false
		w.schemaQueryObjects(scope.objects)
	case "sobject_return":
		scope = compileGateQueryScope{}
		for _, c := range node.Children {
			if c.Kind == "identifier" {
				scope.objects = []string{w.schemaQueryObjectName(w.text(c))}
				break
			}
		}
		w.schemaQueryObjects(scope.objects)
	case "type_of_clause":
		w.schemaQueryTypeof(node, scope)
		return
	case "field_identifier":
		w.schemaQueryField(scope, w.text(node))
		return
	}
	for _, c := range node.Children {
		isChild := childQuery
		if c.Kind == "subquery" {
			isChild = node.Kind == "select_clause"
		}
		w.walkSchemaQueryScope(c, scope, isChild)
	}
}

func (w *compileGateRefWalker) schemaQueryObjects(objects []string) {
	for _, object := range objects {
		if name := strings.ToLower(object); w.units.fileOf[name] != "" {
			w.add(name)
		}
	}
}

func (w *compileGateRefWalker) schemaQueryObjectName(name string) string {
	name = strings.TrimSpace(name)
	if w.units.querySchema != nil {
		if canonical := w.units.querySchema.names[strings.ToLower(name)]; canonical != "" {
			return canonical
		}
		if w.units.namespace != "" {
			if canonical := w.units.querySchema.names[strings.ToLower(w.units.namespace+"__"+name)]; canonical != "" {
				return canonical
			}
		}
	}
	if object, ok := w.units.objects[strings.ToLower(name)]; ok {
		return object.Name
	}
	if w.units.namespace != "" {
		if object, ok := w.units.objects[strings.ToLower(w.units.namespace+"__"+name)]; ok {
			return object.Name
		}
	}
	if canonical, ok := storage.ResolveKnownStandardObjectName(name); ok {
		return canonical
	}
	return name
}

func (w *compileGateRefWalker) schemaQueryNameMatches(declared, query string) bool {
	return strings.EqualFold(declared, query) || w.units.namespace != "" && strings.EqualFold(declared, w.units.namespace+"__"+query)
}

func (w *compileGateRefWalker) schemaQueryParents(objects []string, relationship string) []string {
	var targets []string
	seen := map[string]bool{}
	add := func(names []string) {
		for _, name := range names {
			name = w.schemaQueryObjectName(name)
			key := strings.ToLower(name)
			if !seen[key] {
				seen[key] = true
				targets = append(targets, name)
			}
		}
	}
	for _, object := range objects {
		matched := false
		visit := func(relation storage.Relationship) {
			if w.schemaQueryNameMatches(relation.ParentRelationship, relationship) {
				matched = true
				add(relation.ParentObjects)
			}
		}
		for _, relation := range w.units.querySchema.relationships[strings.ToLower(object)] {
			visit(relation)
		}
		storage.VisitStandardObjectRelationships(object, w.units.features, visit)
		if !matched && len(w.units.features) > 0 {
			for _, relation := range w.units.querySchema.featureRelationships(object, w.units.features) {
				visit(relation)
			}
		}
	}
	return targets
}

func (w *compileGateRefWalker) schemaQueryField(scope compileGateQueryScope, path string) {
	parts := strings.Split(path, ".")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	objects := scope.objects
	if len(parts) > 1 {
		if bound, ok := scope.aliases[strings.ToLower(parts[0])]; ok {
			objects = bound
			parts = parts[1:]
		} else if len(objects) == 1 && w.schemaQueryNameMatches(objects[0], parts[0]) {
			parts = parts[1:]
		}
	}
	for _, part := range parts[:len(parts)-1] {
		objects = w.schemaQueryParents(objects, part)
		// Gate policy applies to static users of an invalid object, including an
		// intermediate parent; the leaf remains for the semantic field checker.
		w.schemaQueryObjects(objects)
	}
}

func (w *compileGateRefWalker) schemaQueryChildren(parents []string, relationship string) []string {
	var children []string
	for _, parent := range parents {
		for object, relations := range w.units.querySchema.relationships {
			for _, relation := range relations {
				if !w.schemaQueryNameMatches(relation.ChildRelationship, relationship) {
					continue
				}
				for _, target := range relation.ParentObjects {
					if strings.EqualFold(w.schemaQueryObjectName(target), parent) {
						children = append(children, w.schemaQueryObjectName(object))
					}
				}
			}
		}
		if child, _, ok := storage.LookupPlatformChildRelationship(storage.ObjectDefinition{APIName: parent}, relationship); ok {
			children = append(children, child)
		}
	}
	return children
}

func (w *compileGateRefWalker) schemaQueryTypeof(node apexast.ASTNode, scope compileGateQueryScope) {
	var targets []string
	for _, c := range node.Children {
		if c.Kind == "identifier" {
			targets = w.schemaQueryParents(scope.objects, w.text(c))
			break
		}
	}
	selected := map[string]bool{}
	for _, c := range node.Children {
		if c.Kind == "when_expression" {
			for _, n := range c.Children {
				if n.Kind == "identifier" {
					selected[strings.ToLower(w.schemaQueryObjectName(w.text(n)))] = true
					break
				}
			}
		}
	}
	for _, c := range node.Children {
		var objects []string
		switch c.Kind {
		case "when_expression":
			for _, n := range c.Children {
				if n.Kind == "identifier" {
					// WHEN explicitly names its object even when a generated
					// relationship catalog omits custom activity targets.
					objects = []string{w.schemaQueryObjectName(w.text(n))}
					break
				}
			}
		case "else_expression":
			for _, target := range targets {
				if !selected[strings.ToLower(target)] {
					objects = append(objects, target)
				}
			}
		default:
			continue
		}
		w.schemaQueryObjects(objects)
		for _, n := range c.Children {
			if n.Kind == "field_list" {
				for _, field := range n.Children {
					w.schemaQueryField(compileGateQueryScope{objects: objects}, w.text(field))
				}
			}
		}
	}
}

func (w *compileGateRefWalker) schemaQueryFrom(from apexast.ASTNode, outer compileGateQueryScope, child bool) compileGateQueryScope {
	scope := compileGateQueryScope{aliases: map[string][]string{}}
	for _, binding := range from.Children {
		var path, alias string
		switch binding.Kind {
		case "storage_identifier":
			path = w.text(binding)
		case "storage_alias":
			for _, n := range binding.Children {
				if n.Kind == "storage_identifier" {
					path = w.text(n)
				} else if n.Kind == "identifier" {
					alias = w.lowerText(n)
				}
			}
		default:
			continue
		}
		parts := strings.Split(path, ".")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		var objects []string
		if scope.objects == nil {
			objects = []string{w.schemaQueryObjectName(parts[0])}
			if child {
				objects = w.schemaQueryChildren(outer.objects, parts[len(parts)-1])
			}
			scope.objects = objects
		} else {
			objects = scope.aliases[strings.ToLower(parts[0])]
			if objects == nil && w.schemaQueryNameMatches(scope.objects[0], parts[0]) {
				objects = scope.objects
			}
			for _, part := range parts[1:] {
				objects = w.schemaQueryParents(objects, part)
				w.schemaQueryObjects(objects)
			}
		}
		if alias != "" {
			scope.aliases[alias] = objects
		}
	}
	return scope
}

// Build once per gate's immutable index, shared by every diagnostic/file walk.
// The full describe registry preserves inherited Activity fields and generated
// system relationships; no metadata is added to the published typesys index.
type compileGateQuerySchema struct {
	once           sync.Once
	relationships  map[string][]storage.Relationship
	names          map[string]string
	featureParents sync.Map
}

// Feature fields can add a parent absent from the base relationship catalog.
// Cache that object's feature-aware definition lazily within the gate snapshot.
func (q *compileGateQuerySchema) featureRelationships(object string, features []string) []storage.Relationship {
	canonical, ok := storage.ResolveKnownStandardObjectName(object)
	if !ok {
		return nil
	}
	key := strings.ToLower(canonical)
	if cached, ok := q.featureParents.Load(key); ok {
		return cached.([]storage.Relationship)
	}
	definition := storage.ObjectDefinition{APIName: canonical}
	storage.EnsureStandardObjectFieldsForFeatures(&definition, features)
	cached, _ := q.featureParents.LoadOrStore(key, definition.Relations)
	return cached.([]storage.Relationship)
}

func (q *compileGateQuerySchema) initialize(units compileGateUnits) {
	q.once.Do(func() {
		registry := sobject.BuildDescribeRegistry(schema.Schema{Objects: units.schemaObjects})
		q.relationships = make(map[string][]storage.Relationship, len(registry.Objects))
		q.names = make(map[string]string, len(registry.Objects))
		for name, describe := range registry.Objects {
			q.names[strings.ToLower(name)] = name
			definition := sobject.ToObjectDefinition(describe)
			storage.EnsureStandardObjectFieldsForFeatures(&definition, units.features)
			q.relationships[strings.ToLower(name)] = definition.Relations
		}
	})
}
