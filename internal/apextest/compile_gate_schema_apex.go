package apextest

import (
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/ir"
	"github.com/glade-sh/glade/internal/vm"
)

type compileGateTriggerContext struct{ owner, object string }

func (w *compileGateRefWalker) schemaDeclarationType(node apexast.ASTNode) string {
	if !w.schema {
		return ""
	}
	for _, child := range node.Children {
		if child.Kind == "type_identifier" || child.Kind == "scoped_type_identifier" || strings.HasSuffix(child.Kind, "_type") {
			return w.text(child)
		}
	}
	return ""
}

// Relationship members name schema owners even when those owners never occur
// as source tokens (for example a Contact's Account or a Shipment's Parcel__r).
// Resolve the receiver with the same expression typing used by the semantic
// analyzer; query and Apex access share the metadata relationship catalog.
func (w *compileGateRefWalker) schemaRelationshipAccess(node apexast.ASTNode) {
	if len(node.Children) < 2 {
		return
	}
	if w.units.querySchema == nil {
		w.units.querySchema = &compileGateQuerySchema{}
	}
	w.units.querySchema.initialize(w.units)
	objects := w.schemaReceiverObjects(node.Children[0])
	for _, child := range node.Children[1:] {
		if child.Kind == "identifier" {
			w.schemaQueryObjects(w.schemaQueryParents(objects, w.text(child)))
			return
		}
	}
}

func (w *compileGateRefWalker) schemaReceiverObjects(node apexast.ASTNode) []string {
	// Preserve every possible target at a polymorphic hop, rather than using
	// the analyzer's common field type for What, Who or a queue-capable Owner.
	if node.Kind == "field_access" && len(node.Children) >= 2 {
		objects := w.schemaReceiverObjects(node.Children[0])
		for _, child := range node.Children[1:] {
			if child.Kind == "identifier" {
				if parents := w.schemaQueryParents(objects, w.text(child)); len(parents) > 0 {
					return parents
				}
				break
			}
		}
	}
	variables := make(map[string]string)
	for _, scope := range w.bindings {
		for name, typ := range scope {
			variables[name] = typ
		}
	}
	if w.isVariable("trigger") {
		for _, field := range []string{"trigger.new", "trigger.old", "trigger.newmap", "trigger.oldmap"} {
			delete(variables, field)
		}
	}
	var typ string
	if len(w.classes) > 0 && w.units.expressionType != nil {
		program, err := vm.CompileAnonymous("return " + w.text(node) + ";")
		if err == nil && len(program.Instructions) == 1 {
			expr := program.Instructions[0].Expr
			// Dotted trigger collections are typed scope entries. Preserve
			// that receiver for calls such as Trigger.new.get(0).
			if dot := strings.LastIndex(expr.Callee, "."); expr.Kind == ir.ExprCall && expr.Left == nil && dot > 0 {
				receiver := expr.Callee[:dot]
				if variables[strings.ToLower(receiver)] != "" {
					expr.Left = &ir.Expr{Kind: ir.ExprVariable, Name: receiver}
					expr.Callee = expr.Callee[dot+1:]
				}
			}
			typ = w.units.expressionType(expr, variables, w.classes[len(w.classes)-1].name)
			// Only a query literal admits direct singleton field access. A
			// List<Contact> variable or method return remains a collection.
			if expr.Kind == ir.ExprSOQL && strings.HasPrefix(strings.ToLower(typ), "list<") && strings.HasSuffix(typ, ">") {
				typ = typ[len("List<") : len(typ)-1]
			}
		}
	} else if node.Kind == "identifier" {
		// Trigger bodies have no enclosing class; their declared record
		// variables still carry an unambiguous schema receiver type.
		typ = variables[w.lowerText(node)]
	}
	if typ == "" {
		return nil
	}
	if strings.HasPrefix(strings.ToLower(typ), "schema.") {
		typ = typ[len("Schema."):]
	} else if _, isClass := w.units.types[strings.ToLower(typ)]; isClass || w.innerType(strings.ToLower(typ)) {
		return nil
	}
	return []string{w.schemaQueryObjectName(typ)}
}
