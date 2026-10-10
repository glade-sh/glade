package visualforce

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// ComponentDiagnostic retains the resource that failed source compilation.
// A rejected component and each page importing it have separate diagnostics.
type ComponentDiagnostic struct {
	File     string
	Resource string
	Type     string
	Message  string
}

type ComponentValidationError struct {
	Diagnostics []ComponentDiagnostic
}

func (e *ComponentValidationError) Error() string {
	return e.Diagnostics[0].Message
}

type customComponentDefinition struct {
	name   string
	source string
	tree   *MarkupNode
	err    error
}

type customComponentValidation struct {
	definitions map[string]customComponentDefinition
	expressions *expressionValidationContext
	index       Index
	knownTypes  map[string]bool
	namespace   string
}

func validateCustomComponentProject(p project.Project) error {
	v := customComponentValidation{definitions: map[string]customComponentDefinition{}, knownTypes: map[string]bool{}, namespace: p.Namespace}
	for _, typ := range typesys.StandardPlatformSymbolView() {
		v.knownTypes[strings.ToLower(typ.Name)] = true
	}
	for _, path := range p.VisualforceComponentFiles {
		name := nameFromPath(path, ".component")
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree, parseErr := parseMarkupTree(string(source), name)
		v.definitions[strings.ToLower(name)] = customComponentDefinition{name: name, source: string(source), tree: tree, err: parseErr}
		v.index.Components = append(v.index.Components, Component{Name: name, File: path})
	}
	v.index.sortAndBuildLookups()
	expressions, err := newExpressionValidationContext(p, &v.index)
	if err != nil {
		return err
	}
	v.expressions = expressions
	diagnostics := []ComponentDiagnostic{}
	add := func(path, resource string, err error) {
		if err != nil {
			diagnostics = append(diagnostics, ComponentDiagnostic{File: path, Resource: resource, Type: "Error", Message: err.Error()})
		}
	}
	for _, path := range p.VisualforceComponentFiles {
		def := v.definitions[strings.ToLower(nameFromPath(path, ".component"))]
		add(path, "ApexComponent", v.definitionError(def, def.name, map[string]bool{}))
	}
	for _, path := range p.VisualforcePageFiles {
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name := nameFromPath(path, ".page")
		tree, err := parseMarkupTree(string(source), name)
		if err == nil {
			err = v.invocationError(tree, string(source), name, map[string]bool{})
		} else {
			err = componentCompilationError(err, string(source), name)
		}
		add(path, "ApexPage", err)
	}
	if len(diagnostics) == 0 {
		return nil
	}
	sort.SliceStable(diagnostics, func(i, j int) bool {
		a, b := diagnostics[i], diagnostics[j]
		if a.Resource != b.Resource {
			return a.Resource < b.Resource
		}
		return a.File < b.File
	})
	return &ComponentValidationError{Diagnostics: diagnostics}
}

func (v *customComponentValidation) definitionError(def customComponentDefinition, displayName string, active map[string]bool) error {
	if active[strings.ToLower(def.name)] {
		return nil // The existing project checks remain responsible for cycles.
	}
	if def.err != nil {
		err := componentCompilationError(def.err, def.source, displayName)
		return fmt.Errorf("%s", strings.ReplaceAll(err.Error(), def.name, displayName))
	}
	active = copyComponentValidationPath(active, strings.ToLower(def.name))
	var validate func(*MarkupNode) error
	validate = func(node *MarkupNode) error {
		if isApexStructureTag(node, "component") {
			if controller := node.Attribute("controller"); controller != "" && len(v.expressions.classes) != 0 {
				class, exists := v.expressions.classes[strings.ToLower(controller)]
				if !exists {
					return fmt.Errorf("Apex class '%s' does not exist", controller)
				}
				if err := visualforceControllerConstructor(class, "", v.expressions.classes); err != nil {
					return err
				}
			}
			for _, child := range node.Children {
				if isApexStructureTag(child, "attribute") {
					if err := v.attributeError(child, node.Attribute("controller"), def.source, displayName); err != nil {
						return err
					}
				}
			}
		}
		if isApexStructureTag(node, "componentBody") {
			for _, name := range sortedExpressionAttributeNames(node.Attributes) {
				if name != "id" && name != "rendered" {
					return fmt.Errorf("Unsupported attribute %s in <apex:componentBody>%s", name, componentDiagnosticLocation(node, def.source, displayName))
				}
			}
		}
		for _, child := range node.Children {
			if err := validate(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := validate(def.tree); err != nil {
		return err
	}
	return v.invocationError(def.tree, def.source, displayName, active)
}

func (v *customComponentValidation) attributeError(node *MarkupNode, controller, source, displayName string) error {
	for _, name := range []string{"name", "type"} {
		if node.Attribute(name) == "" {
			return fmt.Errorf("Missing required attribute %s in <apex:attribute>%s", name, componentDiagnosticLocation(node, source, displayName))
		}
	}
	if _, description := node.Attributes["description"]; !description {
		present := false
		for _, child := range node.Children {
			present = present || isApexStructureTag(child, "description")
		}
		if !present {
			return fmt.Errorf("Missing required description attribute or component in <attribute>")
		}
	}
	if typ := node.Attribute("type"); !v.knownAttributeType(typ) {
		return fmt.Errorf("Apex class '%s' does not exist", strings.TrimSuffix(typ, "[]"))
	}
	componentName := strings.TrimPrefix(displayName, "c__")
	if strings.Contains(node.Attribute("default"), "{!") {
		return fmt.Errorf("Literal value is required for attribute %s in <c:%s>", node.Attribute("name"), componentName)
	}
	assign := strings.TrimSpace(node.Attribute("assignTo"))
	if assign == "" {
		return nil
	}
	if !strings.HasPrefix(assign, "{!") || findExpressionTemplateEnd(assign, 2) != len(assign)-1 {
		return fmt.Errorf("Formula expression is required for attribute assignTo in <apex:attribute>%s", componentDiagnosticLocation(node, source, displayName))
	}
	class, exists := v.expressions.classes[strings.ToLower(controller)]
	if !exists {
		return nil // VM-provided source-free controllers are checked at rendering.
	}
	target := strings.TrimSpace(assign[2 : len(assign)-1])
	member, writable, exists := componentControllerProperty(class, target, v.expressions.classes, map[string]bool{})
	if !exists {
		return fmt.Errorf("Unknown property '%s.%s'", controller, target)
	}
	if !writable {
		return fmt.Errorf("Read only property 'c:%s.%s'", componentName, target)
	}
	if !componentBindingTypesEqual(node.Attribute("type"), member.Type) {
		return fmt.Errorf("Type mismatch for <apex:attribute assignTo>. Value binding to a property of type %s is required, property specified (%s) is of type %s.", node.Attribute("type"), target, member.Type)
	}
	return nil
}

func (v *customComponentValidation) knownAttributeType(typ string) bool {
	typ = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(typ)), "system.")
	typ = strings.TrimSuffix(typ, "[]")
	if v.knownTypes[typ] {
		return true
	}
	if _, known := v.expressions.classes[typ]; known {
		return true
	}
	if _, known := v.expressions.objects[typ]; known {
		return true
	}
	_, known := storage.StandardObjectDefinition(typ)
	return known
}

func (v *customComponentValidation) invocationError(node *MarkupNode, source, sourceName string, active map[string]bool) error {
	if node.Type == MarkupNodeElement && node.Namespace != "" && node.Namespace != "apex" {
		name := node.Name
		if node.Namespace != "c" && !strings.EqualFold(node.Namespace, v.namespace) {
			name = node.Namespace + "__" + name
		}
		def, exists := v.definitions[strings.ToLower(name)]
		if !exists {
			return validateMarkupComponents(node, &v.index, v.namespace)
		}
		if err := v.definitionError(def, "c__"+strings.ToLower(def.name), active); err != nil {
			return err
		}
		if !active[strings.ToLower(def.name)] {
			attributes := componentDeclarationNodes(def.tree)
			known := map[string]*MarkupNode{}
			for _, attr := range attributes {
				known[strings.ToLower(attr.Attribute("name"))] = attr
				if strings.EqualFold(attr.Attribute("required"), "true") {
					if _, supplied := node.Attributes[strings.ToLower(attr.Attribute("name"))]; !supplied {
						return fmt.Errorf("Missing required attribute %s in <%s>%s", attr.Attribute("name"), node.RawName, componentDiagnosticLocation(node, source, sourceName))
					}
				}
			}
			for _, name := range sortedExpressionAttributeNames(node.Attributes) {
				if name == "id" || name == "rendered" {
					continue
				}
				attr, declared := known[name]
				if !declared {
					return fmt.Errorf("Unsupported attribute %s in <%s>%s", name, node.RawName, componentDiagnosticLocation(node, source, sourceName))
				}
				value := node.Attribute(name)
				if !strings.Contains(value, "{!") && invalidComponentLiteral(value, attr.Attribute("type")) {
					return fmt.Errorf("Wrong type for attribute <%s:%s %s=%q>. Expected %s, found String", node.Namespace, node.Name, name, value, attr.Attribute("type"))
				}
			}
			// A self-closing invocation has no body. The shared fragment parser
			// can attach following siblings when a custom tag name contains an
			// underscore; source syntax remains authoritative for this check.
			if componentBodyDefinitionCount(def.tree) == 0 && !componentInvocationSelfClosing(node, source) {
				for _, child := range componentBodyNodes(node) {
					if child.Type == MarkupNodeElement || strings.TrimSpace(child.Text) != "" {
						return fmt.Errorf("Component <%s:%s> definition does not contain <apex:componentBody> so it cannot be used with any child tags.", node.Namespace, node.Name)
					}
				}
			}
		}
	}
	for _, child := range node.Children {
		if err := v.invocationError(child, source, sourceName, active); err != nil {
			return err
		}
	}
	return nil
}

func componentDeclarationNodes(tree *MarkupNode) []*MarkupNode {
	var declarations []*MarkupNode
	for _, root := range tree.Children {
		if isApexStructureTag(root, "component") {
			for _, child := range root.Children {
				if isApexStructureTag(child, "attribute") {
					declarations = append(declarations, child)
				}
			}
		}
	}
	return declarations
}

func componentControllerProperty(class apexast.Declaration, name string, classes map[string]apexast.Declaration, seen map[string]bool) (apexast.Declaration, bool, bool) {
	if seen[strings.ToLower(class.Name)] {
		return apexast.Declaration{}, false, false
	}
	seen[strings.ToLower(class.Name)] = true
	for _, member := range class.Members {
		if strings.EqualFold(member.Name, name) && (member.Kind == apexast.DeclarationField || member.Kind == apexast.DeclarationProperty) {
			writable := member.Kind == apexast.DeclarationField
			for _, accessor := range member.Accessors {
				writable = writable || strings.EqualFold(accessor.Kind, "set")
			}
			return member, writable && visualforceControllerMemberVisible(member), true
		}
		if member.Kind == apexast.DeclarationMethod && len(member.Parameters) == 0 && strings.EqualFold(member.Name, "get"+name) {
			return member, false, true
		}
	}
	if parent, exists := classes[strings.ToLower(class.SuperClass)]; exists {
		return componentControllerProperty(parent, name, classes, seen)
	}
	return apexast.Declaration{}, false, false
}

func invalidComponentLiteral(value, typeName string) bool {
	switch strings.ToLower(typeName) {
	case "integer":
		_, err := strconv.ParseInt(value, 10, 32)
		return err != nil
	case "date":
		_, err := time.Parse("2006-01-02", value)
		return err != nil
	}
	return false
}

func copyComponentValidationPath(active map[string]bool, name string) map[string]bool {
	out := make(map[string]bool, len(active)+1)
	for name, value := range active {
		out[name] = value
	}
	out[name] = true
	return out
}

// Native component declaration/import diagnostics locate the character after
// the opening tag. The fragment parser retains the start and element-name end;
// use the source here without changing other families' diagnostic locations.
func componentDiagnosticLocation(node *MarkupNode, source, sourceName string) string {
	if end := componentOpeningTagEnd(node, source); end >= 0 {
		endLine, endColumn := lineColumnAt(source, end+1)
		location := ""
		if sourceName != "" {
			location = " in " + sourceName
		}
		return fmt.Sprintf("%s at line %d column %d", location, endLine, endColumn)
	}
	return markupDiagnosticLocation(node, sourceName)
}

func componentInvocationSelfClosing(node *MarkupNode, source string) bool {
	end := componentOpeningTagEnd(node, source)
	return end > 0 && source[end-1] == '/'
}

func componentOpeningTagEnd(node *MarkupNode, source string) int {
	line, column := 1, 1
	for offset, ch := range source {
		if line == node.Line && column == node.Column {
			if ch == '<' {
				return findStartTagEnd(source, offset+1)
			}
			break
		}
		if ch == '\n' {
			line, column = line+1, 1
		} else {
			column++
		}
	}
	return -1
}

type componentBodyOutsideError struct {
	node       *MarkupNode
	sourceName string
}

func (e *componentBodyOutsideError) Error() string {
	return "<apex:componentBody> cannot be used inside <apex:page> in the markup" + markupDiagnosticLocation(e.node, e.sourceName)
}

func componentCompilationError(err error, source, displayName string) error {
	var outside *componentBodyOutsideError
	if errors.As(err, &outside) {
		return fmt.Errorf("<apex:componentBody> cannot be used inside <apex:page> in the markup%s", componentDiagnosticLocation(outside.node, source, displayName))
	}
	return err
}
