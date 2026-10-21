package visualforce

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/storage"
)

// field and input rendering's entity/unknown-field controls require a field component to bind a
// metadata-resolvable SObject field. This check does not change formula typing
// for outputText, controller methods or other component attributes.
func (ctx *expressionValidationContext) validateFieldComponentBinding(node *MarkupNode, pageName string) error {
	tag := presentationCompileTag(node.RawName)
	if tag != "inputField" && tag != "outputField" {
		return nil
	}
	if tag == "inputField" && ctx.standardController != "" {
		label := strings.TrimSpace(node.Attribute("label"))
		if strings.HasPrefix(label, "{!") && strings.HasSuffix(label, "}") {
			label = strings.TrimSpace(label[2 : len(label)-1])
			if strings.EqualFold(label, "true") {
				return fmt.Errorf("Unknown property '%sStandardController.%s'", ctx.standardController, label)
			}
		}
	}
	raw := node.Attribute("value")
	if !strings.HasPrefix(raw, "{!") || findExpressionTemplateEnd(raw, 2) != len(raw)-1 {
		return nil // Source-token validation owns this diagnostic's location.
	}
	expr, err := parseExpression(strings.TrimSpace(raw[2 : len(raw)-1]))
	if err != nil {
		return nil // Preserve the shared syntax diagnostic.
	}
	var parts []string
	switch value := expr.(type) {
	case identifierExpr:
		parts = value.parts
	case visualforceIdentifierExpr:
		parts = value.parts
	case indexExpr, memberExpr:
		// standardController's captured dynamic field bindings are resolved at runtime. The
		// shared expression/projection validator owns their receiver and index.
		return nil
	}
	if len(parts) < 2 {
		return fmt.Errorf("Could not resolve the entity from <apex:%s> value binding '%s'.  <apex:%s> can only be used with SObjects, or objects that are Visualforce field component resolvable.", tag, raw, tag)
	}
	objectType, bound := ctx.bindings[strings.ToLower(parts[0])]
	if ctx.standardControllerPath(parts) {
		objectType = ctx.standardController
	}
	if !bound || objectType == "" {
		return nil // The existing controller/projection validator owns this root.
	}
	for index, part := range parts[1:] {
		found, target, known := ctx.fieldComponentMember(objectType, part)
		if !known {
			return nil // Runtime-supplied and polymorphic objects remain deferred.
		}
		if !found {
			return fmt.Errorf("Could not resolve field '%s' from <apex:%s> value binding '%s' in page %s", part, tag, raw, pageName)
		}
		if index != len(parts)-2 {
			if target == "" {
				return nil
			}
			objectType = target
		}
	}
	return nil
}

func (ctx *expressionValidationContext) fieldComponentMember(objectType, member string) (found bool, target string, known bool) {
	object, indexed := ctx.objects[strings.ToLower(objectType)]
	if indexed {
		for _, field := range object.Fields {
			if strings.EqualFold(field.Name, member) {
				return true, "", true
			}
			if len(field.ReferenceTo) == 1 && strings.EqualFold(storage.ParentRelationshipName(storage.Field{APIName: field.Name, Type: storage.FieldReference, RelationshipName: field.RelationshipName}), member) {
				return true, field.ReferenceTo[0], true
			}
		}
		// A standard-object metadata file can declare only a subset of fields.
		// Preserve its explicit members, then consult the standard catalog for
		// native OwnerId/Owner.Name and other omitted standard fields.
	}
	definition, ok := storage.StandardObjectDefinition(objectType)
	if !ok {
		return false, "", indexed
	}
	if _, found := storage.ResolveFieldName(definition, "", member); found {
		return true, "", true
	}
	for _, field := range definition.Fields {
		if len(field.ReferenceTo) == 1 && strings.EqualFold(storage.ParentRelationshipName(field), member) {
			return true, field.ReferenceTo[0], true
		}
	}
	return false, "", true
}
