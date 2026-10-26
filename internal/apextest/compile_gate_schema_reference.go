package apextest

import (
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
)

// Reuse Apex name binding for static object types and field tokens. Metadata
// validity does not turn strings, comments or dynamic lookups into dependencies.
func compileGateReferencesSchemaObject(file, source string, units compileGateUnits, object string) bool {
	name := strings.ToLower(object)
	targets := units
	targets.fileOf = map[string]string{name: object}
	if prefix := strings.ToLower(units.namespace) + "__"; units.namespace != "" && strings.HasPrefix(name, prefix) {
		targets.fileOf[strings.TrimPrefix(name, prefix)] = object
	}
	// A relationship target need not occur literally (Owner, What, Who or __r).
	if len(compileGateMentions(source, targets.fileOf)) == 0 && !strings.ContainsAny(source, "[.") {
		return false
	}
	walker := compileGateRefWalker{source: source, units: targets, schema: true, seen: make(map[string]bool)}
	if context, ok := units.triggerContexts[filepath.Clean(file)]; ok {
		walker.classes = []compileGateClassScope{{name: context.owner, inner: map[string]bool{}}}
		walker.push(nil)
		walker.bindings[0]["trigger.new"] = "List<" + context.object + ">"
		walker.bindings[0]["trigger.old"] = "List<" + context.object + ">"
		walker.bindings[0]["trigger.newmap"] = "Map<Id," + context.object + ">"
		walker.bindings[0]["trigger.oldmap"] = "Map<Id," + context.object + ">"
	}
	parsed := apexast.ParseSourceAST(file, source)
	for _, node := range parsed.Nodes {
		walker.walk(node)
	}
	// A trigger's target is a declaration identifier, not an Apex type node.
	if strings.EqualFold(filepath.Ext(file), ".trigger") {
		for _, decl := range apexast.ParseSource(file, source).Declarations {
			if decl.Kind == apexast.DeclarationTrigger && targets.fileOf[strings.ToLower(decl.ObjectName)] != "" {
				return true
			}
		}
	}
	return len(walker.refs) > 0
}

// Schema.SObjectType.ObjectName is a schema token, including the optional
// .fields.FieldName suffix. Its object position is distinct from a member of
// an Apex variable or a project class named Schema.
func (w *compileGateRefWalker) schemaObjectToken(node apexast.ASTNode) {
	var parts []string
	var collect func(apexast.ASTNode) bool
	collect = func(n apexast.ASTNode) bool {
		if n.Kind == "identifier" {
			parts = append(parts, w.lowerText(n))
			return true
		}
		if n.Kind != "field_access" {
			return false
		}
		for _, child := range n.Children {
			if !collect(child) {
				return false
			}
		}
		return true
	}
	if collect(node) && len(parts) >= 3 && parts[0] == "schema" && parts[1] == "sobjecttype" && w.namespace(parts[0]) {
		if w.units.fileOf[parts[2]] != "" {
			w.add(parts[2])
		}
	}
}
