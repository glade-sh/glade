package visualforce

import (
	"fmt"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/apexast"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type expressionValidationContext struct {
	index               *Index
	registry            storage.MetadataRegistry
	classes             map[string]apexast.Declaration
	objects             map[string]schema.Object
	controller          string
	standardController  string
	bindings            map[string]string
	sourceBindings      bool
	formControllers     []apexast.Declaration
	standardFormActions map[string]bool
}

func newExpressionValidationContext(p project.Project, index *Index) (*expressionValidationContext, error) {
	ctx := &expressionValidationContext{index: index, classes: map[string]apexast.Declaration{}, objects: map[string]schema.Object{}}
	if len(p.ApexFiles) != 0 {
		parser := apexast.NewParser()
		defer parser.Close()
		for _, path := range p.ApexFiles {
			file, err := parser.ParseFile(path)
			if err != nil {
				return nil, err
			}
			for _, declaration := range file.Declarations {
				if declaration.Kind == apexast.DeclarationClass {
					ctx.classes[strings.ToLower(declaration.Name)] = declaration
				}
			}
		}
	}
	if len(ctx.classes) == 0 {
		return ctx, nil
	}
	// Load only metadata used by expressions. Unrelated resource types must not
	// become new prerequisites for indexing Visualforce pages.
	registry, err := resource.LoadProject(project.Project{
		Root: p.Root, Namespace: p.Namespace,
		LabelFiles: p.LabelFiles, TranslationFiles: p.TranslationFiles,
		StaticResourceFiles: p.StaticResourceFiles, StaticResourceMetas: p.StaticResourceMetas,
		ManagedPackageDependencies: p.ManagedPackageDependencies,
	})
	if err != nil {
		return nil, err
	}
	ctx.registry = registry
	objects, err := schema.LoadProject(p)
	if err != nil {
		return nil, err
	}
	for _, object := range objects.Objects {
		ctx.objects[strings.ToLower(object.Name)] = object
	}
	return ctx, nil
}

// Expression validation is separate from markup/metadata checks. Bindings
// are checked against available source declarations; an absent controller source
// remains a runtime/controller concern rather than an invented property error.
func (ctx *expressionValidationContext) validate(tree *MarkupNode, sourceName string) error {
	local := *ctx
	local.bindings = map[string]string{}
	root := tree
	if root.Namespace != "apex" {
		for _, child := range tree.Children {
			if child.Namespace == "apex" && (child.Name == "page" || child.Name == "component") {
				root = child
				break
			}
		}
	}
	local.controller = root.Attribute("controller")
	local.formControllers = nil
	local.standardFormActions = nil
	for _, name := range append(splitCSV(root.Attribute("extensions")), local.controller) {
		if class, ok := local.classes[strings.ToLower(name)]; ok {
			local.formControllers = append(local.formControllers, class)
		}
	}
	for _, class := range append([]string{local.controller}, splitCSV(root.Attribute("extensions"))...) {
		if _, known := local.classes[strings.ToLower(class)]; known {
			local.sourceBindings = true
		}
		local.addBindings(class, map[string]bool{})
	}
	if standard := root.Attribute("standardcontroller"); standard != "" {
		// Standard-controller records may use schema supplied by the runtime.
		// Their fields are not custom-controller source declarations.
		local.bindings[strings.ToLower(standard)] = ""
		if root.Attribute("recordsetvar") == "" {
			local.standardController = standard
		}
		local.standardFormActions = formStandardControllerActions(root.Attribute("recordsetvar") != "")
	}
	if variable := root.Attribute("recordsetvar"); variable != "" {
		local.bindings[strings.ToLower(variable)] = ""
	}
	if root.Name == "component" {
		for _, child := range root.Children {
			if child.Namespace == "apex" && child.Name == "attribute" {
				local.bindings[strings.ToLower(child.Attribute("name"))] = child.Attribute("type")
			}
		}
	}
	var walk func(*MarkupNode, map[string]string) error
	walk = func(node *MarkupNode, bindings map[string]string) error {
		current := local
		current.bindings = bindings
		if err := validateFormControllerBinding(node, &current); err != nil {
			return err
		}
		for _, name := range sortedExpressionAttributeNames(node.Attributes) {
			raw := node.Attribute(name)
			// A null facet name retains the captured page-validation failure.
			if node.Namespace == "apex" && node.Name == "facet" && name == "name" && strings.EqualFold(strings.TrimSpace(raw), "{!NULL}") && root.Name == "page" {
				return fmt.Errorf("Failed validation: ApexPage")
			}
			if err := current.validateTemplate(raw, node, name, sourceName, root.Name); err != nil {
				return err
			}
		}
		if node.Type == MarkupNodeText {
			if err := current.validateTemplate(node.Text, node, "", sourceName, root.Name); err != nil {
				return err
			}
		}
		if variable := node.Attribute("var"); variable != "" {
			if typ, bind := current.repetitionVariableBinding(node); bind {
				copy := make(map[string]string, len(bindings)+1)
				for name, typ := range bindings {
					copy[name] = typ
				}
				copy[strings.ToLower(variable)] = typ
				bindings = copy
			}
		}
		for _, child := range node.Children {
			if err := walk(child, bindings); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(tree, local.bindings)
}

func sortedExpressionAttributeNames(attributes map[string]string) []string {
	names := make([]string, 0, len(attributes))
	for name := range attributes {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (ctx *expressionValidationContext) addBindings(name string, seen map[string]bool) {
	key := strings.ToLower(name)
	if seen[key] {
		return
	}
	seen[key] = true
	class, ok := ctx.classes[key]
	if !ok {
		return
	}
	ctx.addBindings(class.SuperClass, seen)
	for _, member := range class.Members {
		name := member.Name
		if member.Kind == apexast.DeclarationMethod && len(member.Parameters) == 0 && strings.HasPrefix(strings.ToLower(name), "get") && len(name) > 3 {
			name = name[3:]
		}
		ctx.bindings[strings.ToLower(name)] = member.Type
	}
}

func (ctx *expressionValidationContext) validateTemplate(raw string, node *MarkupNode, attribute, sourceName, rootName string) error {
	for offset := 0; offset < len(raw); {
		next := strings.Index(raw[offset:], "{!")
		if next < 0 {
			return nil
		}
		start := offset + next + 2
		end := findExpressionTemplateEnd(raw, start)
		if end < 0 {
			return nil
		}
		text := strings.TrimSpace(raw[start:end])
		// Captured root_page_boolean_output/root_component_boolean_output cases
		// reject standalone true in an otherwise unbound output value. Other
		// standalone literals retain their behavior until captured.
		if node.Namespace == "apex" && node.Name == "outputtext" && attribute == "value" && strings.EqualFold(text, "true") && ctx.controller == "" && len(ctx.bindings) == 0 {
			message := fmt.Sprintf("Unknown property '%s'", text)
			if rootName == "page" {
				message += " referenced in " + sourceName
			}
			return fmt.Errorf("%s", message)
		}
		expr, err := parseExpression(text)
		if err != nil {
			message := "Syntax error"
			if err == errNoExpression {
				message = "Syntax error.  Found 'end of formula'"
			} else if strings.Contains(err.Error(), "missing ')'") {
				message = "Syntax error.  Missing ')'"
			}
			return fmt.Errorf("%s", message)
		}
		if _, err := ctx.expressionType(expr); err != nil {
			return err
		}
		offset = end + 1
	}
	return nil
}

func (ctx *expressionValidationContext) expressionType(expr Expression) (string, error) {
	switch expr := expr.(type) {
	case literalExpr:
		switch expr.value.Kind {
		case vm.ValueString:
			return "Text", nil
		case vm.ValueInt, vm.ValueDecimal:
			return "Number", nil
		case vm.ValueBool:
			return "Boolean", nil
		}
	case visualforceIdentifierExpr:
		return ctx.identifierType(expr.parts)
	case identifierExpr:
		return ctx.identifierType(expr.parts)
	case unaryExpr:
		return ctx.expressionType(expr.value)
	case binaryExpr:
		left, err := ctx.expressionType(expr.left)
		if err != nil {
			return "", err
		}
		right, err := ctx.expressionType(expr.right)
		if err != nil {
			return "", err
		}
		if (expr.op == "==" || expr.op == "!=" || expr.op == "+") && left != "" && right != "" && left != right {
			op := expr.op
			if op == "==" {
				op = "="
			}
			return "", fmt.Errorf("Incorrect parameter type for operator '%s'. Expected %s, received %s", op, left, right)
		}
		if expr.op == "==" || expr.op == "!=" || expr.op == "&&" || expr.op == "||" || strings.Contains(expr.op, "<") || strings.Contains(expr.op, ">") {
			return "Boolean", nil
		}
		if expr.op == "&" {
			return "Text", nil
		}
		return left, nil
	case visualforceFunctionExpr:
		return ctx.functionType(expr.name, expr.args)
	case functionExpr:
		return ctx.functionType(expr.name, expr.args)
	case indexExpr:
		keyType, err := ctx.expressionType(expr.key)
		if err != nil {
			return "", err
		}
		if parts, static := standardStaticFieldPath(expr.target); static && len(parts) == 1 && ctx.standardControllerPath(parts) && keyType == "Number" {
			return "", fmt.Errorf("Incorrect parameter type for subscript. Expected Text, received Number")
		}
		return ctx.expressionType(expr.target)
	case memberExpr:
		if parts, static := standardStaticFieldPath(expr); static && ctx.standardControllerPath(parts) {
			return ctx.identifierType(parts)
		}
		_, err := ctx.expressionType(expr.target)
		return "", err
	case methodCallExpr:
		for _, arg := range expr.args {
			if _, err := ctx.expressionType(arg); err != nil {
				return "", err
			}
		}
	case parameterMapExpr:
		for _, value := range expr.values {
			if _, err := ctx.expressionType(value); err != nil {
				return "", err
			}
		}
	}
	return "", nil
}

type visualforceFunctionSignature struct {
	min, max int
	result   string
}

func (ctx *expressionValidationContext) functionType(name string, args []Expression) (string, error) {
	name = strings.ToUpper(name)
	signatures := map[string]visualforceFunctionSignature{
		"ABS": {1, 1, "Number"}, "CEILING": {1, 1, "Number"}, "FLOOR": {1, 1, "Number"}, "ROUND": {2, 2, "Number"},
		"MAX": {1, -1, "Number"}, "MIN": {1, -1, "Number"}, "MOD": {2, 2, "Number"}, "SQRT": {1, 1, "Number"}, "EXP": {1, 1, "Number"}, "LN": {1, 1, "Number"},
		"LEN": {1, 1, "Number"}, "UPPER": {1, 1, "Text"}, "LOWER": {1, 1, "Text"}, "TRIM": {1, 1, "Text"},
		"LEFT": {2, 2, "Text"}, "RIGHT": {2, 2, "Text"}, "MID": {3, 3, "Text"}, "FIND": {2, 3, "Number"},
		"CONTAINS": {2, 2, "Boolean"}, "BEGINS": {2, 2, "Boolean"}, "SUBSTITUTE": {3, 3, "Text"},
		"TEXT": {1, 1, "Text"}, "VALUE": {1, 1, "Number"}, "ISBLANK": {1, 1, "Boolean"}, "ISNULL": {1, 1, "Boolean"},
		"BLANKVALUE": {2, 2, ""}, "NULLVALUE": {2, 2, ""}, "CASE": {4, -1, ""}, "IF": {3, 3, ""},
		"AND": {1, -1, "Boolean"}, "OR": {1, -1, "Boolean"}, "NOT": {1, 1, "Boolean"},
		"DATE": {3, 3, "Date"}, "DATEVALUE": {1, 1, "Date"}, "DATETIMEVALUE": {1, 1, "Datetime"},
		"DAY": {1, 1, "Number"}, "MONTH": {1, 1, "Number"}, "YEAR": {1, 1, "Number"}, "TODAY": {0, 0, "Date"}, "NOW": {0, 0, "Datetime"},
		"LPAD": {2, 3, "Text"}, "RPAD": {2, 3, "Text"}, "URLFOR": {1, 4, "Text"},
		"JSENCODE": {1, 1, "Text"}, "HTMLENCODE": {1, 1, "Text"}, "JSINHTMLENCODE": {1, 1, "Text"},
		"URLENCODE": {1, 1, "Text"}, "URLDECODE": {1, 1, "Text"}, "CASESAFEID": {1, 1, "Text"},
	}
	signature, ok := signatures[name]
	if !ok {
		return "", fmt.Errorf("Unknown function %s. Check spelling.", name)
	}
	if len(args) < signature.min || (signature.max >= 0 && len(args) > signature.max) {
		expected := signature.min
		if name == "URLFOR" {
			expected = 2
			if len(args) >= 5 {
				expected = 5
			}
		}
		return "", fmt.Errorf("Incorrect number of parameters for function '%s()'. Expected %d, received %d", name, expected, len(args))
	}
	for _, arg := range args {
		typ, err := ctx.expressionType(arg)
		if err != nil {
			return "", err
		}
		if (name == "JSENCODE" || name == "HTMLENCODE" || name == "JSINHTMLENCODE") && typ != "" && typ != "Text" {
			return "", fmt.Errorf("Incorrect argument type for function '%s()'.", name)
		}
		// The captured picklist boundaries are LEN($User.LocaleSidKey) and
		// LEN($User.TimeZoneSidKey). Do not reject picklists in TEXT or other
		// conversions that previously accepted them.
		if name == "LEN" {
			if field, ok := ctx.globalField(arg); ok && field.Type == storage.FieldPicklist {
				return "", fmt.Errorf("Field %s is a picklist field. Picklist fields are only supported in certain functions.", field.APIName)
			}
		}
	}
	if name == "URLFOR" && len(args) > 0 {
		if literal, ok := args[0].(literalExpr); ok && literal.value.Kind == vm.ValueString && literal.value.Text == "" {
			return "", fmt.Errorf("Invalid target parameter for function URLFOR")
		}
	}
	return signature.result, nil
}

func (ctx *expressionValidationContext) identifierType(parts []string) (string, error) {
	if len(parts) == 0 {
		return "", nil
	}
	path, root := strings.Join(parts, "."), strings.ToLower(parts[0])
	missing := func() (string, error) { return "", fmt.Errorf("Field %s does not exist. Check spelling.", path) }
	if strings.HasPrefix(root, "$") {
		// LoadProject also indexes projects whose controllers and metadata are
		// supplied later by a VM. Validate reference existence only when the
		// page's controller/extension source participates in this project.
		if !ctx.sourceBindings {
			return "", nil
		}
		switch root {
		case "$remoteaction":
			return "", ctx.validateRemoteActionReference(parts)
		case "$label":
			if len(parts) < 2 {
				return missing()
			}
			_, status := resource.ResolveLabel(ctx.registry, "", "", parts[len(parts)-1])
			if status == resource.LabelLookupMissing {
				return missing()
			}
			return "Text", nil
		case "$resource":
			for _, resource := range ctx.registry.StaticResources {
				if len(parts) > 1 && strings.EqualFold(resource.Name, parts[1]) {
					return "Text", nil
				}
			}
			if len(parts) > 1 {
				return "", fmt.Errorf("Static Resource named %s does not exist. Check spelling.", parts[1])
			}
			return missing()
		case "$page":
			if len(parts) > 1 && ctx.index.HasPageReference(parts[1]) {
				return "Text", nil
			}
			if len(parts) > 1 {
				return "", fmt.Errorf("Page %s does not exist", parts[1])
			}
			return missing()
		case "$api":
			if len(parts) > 1 && strings.EqualFold(parts[1], "Version") {
				return "", fmt.Errorf("Field %s does not exist. Check spelling.", parts[1])
			}
			return "", nil
		case "$setup":
			if len(parts) < 3 || ctx.objects[strings.ToLower(parts[1])].CustomSettingsType == "" {
				return missing()
			}
			return "", nil
		case "$user", "$profile", "$organization":
			if len(parts) < 2 {
				return "", nil
			}
			definition, ok := storage.StandardObjectDefinition(parts[0][1:])
			if !ok {
				return missing()
			}
			fieldName, ok := storage.ResolveFieldName(definition, "", parts[1])
			if !ok {
				return "", fmt.Errorf("Field %s does not exist. Check spelling.", parts[1])
			}
			return expressionStorageFieldType(definition.Fields[fieldName]), nil
		default:
			if !supportedVisualforceGlobal(parts[0]) {
				return missing()
			}
			return "", nil
		}
	}
	typ, ok := ctx.bindings[root]
	if !ok {
		if _, known := ctx.classes[strings.ToLower(ctx.controller)]; known {
			return "", fmt.Errorf("Unknown property '%s.%s'", ctx.controller, parts[0])
		}
		if ctx.sourceBindings && ctx.standardController != "" && len(parts) > 1 {
			return "", fmt.Errorf("Unknown property '%sStandardController.%s'", ctx.standardController, parts[0])
		}
		return "", nil
	}
	if ctx.standardControllerPath(parts) {
		return "", ctx.validateStandardControllerFieldPath(ctx.standardController, parts[1:])
	}
	if len(parts) > 1 {
		for _, field := range ctx.objects[strings.ToLower(typ)].Fields {
			if strings.EqualFold(field.Name, parts[1]) {
				return "", nil
			}
		}
		if definition, known := storage.StandardObjectDefinition(typ); known {
			fieldName, ok := storage.ResolveFieldName(definition, "", parts[1])
			if !ok {
				// Typed repetition rows also expose relationship objects. Keep
				// their member paths accepted as before row typing, resolving
				// the relationship from reference metadata rather than names.
				for _, field := range definition.Fields {
					if field.Type == storage.FieldReference && strings.EqualFold(storage.ParentRelationshipName(field), parts[1]) {
						return "", nil
					}
				}
				return "", fmt.Errorf("Invalid field %s for SObject %s", parts[1], definition.APIName)
			}
			return expressionStorageFieldType(definition.Fields[fieldName]), nil
		}
		return "", nil
	}
	switch strings.ToLower(typ) {
	case "string", "id":
		return "Text", nil
	case "integer", "long", "decimal", "double":
		return "Number", nil
	case "boolean":
		return "Boolean", nil
	}
	return "", nil
}

func (ctx *expressionValidationContext) standardControllerPath(parts []string) bool {
	if !ctx.sourceBindings || ctx.standardController == "" || len(parts) == 0 || !strings.EqualFold(parts[0], ctx.standardController) {
		return false
	}
	// A repetition variable with the same name uses its own declared row type.
	typ, bound := ctx.bindings[strings.ToLower(parts[0])]
	return bound && typ == ""
}

// Validate direct fields and reference relationship paths without assigning a
// new formula type to the record binding. Unknown runtime-supplied objects and
// polymorphic targets retain the existing deferred validation.
func (ctx *expressionValidationContext) validateStandardControllerFieldPath(objectName string, parts []string) error {
	if len(parts) == 0 {
		return nil
	}
	for _, field := range ctx.objects[strings.ToLower(objectName)].Fields {
		if strings.EqualFold(field.Name, parts[0]) {
			return nil
		}
		if len(field.ReferenceTo) != 0 && strings.EqualFold(storage.ParentRelationshipName(storage.Field{
			APIName: field.Name, Type: storage.FieldReference, RelationshipName: field.RelationshipName,
		}), parts[0]) {
			if len(field.ReferenceTo) == 1 {
				return ctx.validateStandardControllerFieldPath(field.ReferenceTo[0], parts[1:])
			}
			return nil
		}
	}
	definition, known := storage.StandardObjectDefinition(objectName)
	if !known {
		return nil
	}
	if _, exists := storage.ResolveFieldName(definition, "", parts[0]); exists {
		return nil
	}
	for _, field := range definition.Fields {
		if field.Type == storage.FieldReference && strings.EqualFold(storage.ParentRelationshipName(field), parts[0]) {
			if len(field.ReferenceTo) == 1 {
				return ctx.validateStandardControllerFieldPath(field.ReferenceTo[0], parts[1:])
			}
			return nil
		}
	}
	return fmt.Errorf("Invalid field %s for SObject %s", parts[0], definition.APIName)
}

func expressionStorageFieldType(field storage.Field) string {
	switch field.Type {
	case storage.FieldID, storage.FieldString, storage.FieldPicklist, storage.FieldReference:
		return "Text"
	case storage.FieldBoolean:
		return "Boolean"
	case storage.FieldInteger, storage.FieldDecimal:
		return "Number"
	}
	return ""
}

func (ctx *expressionValidationContext) globalField(expr Expression) (storage.Field, bool) {
	identifier, ok := expr.(visualforceIdentifierExpr)
	if !ok || !ctx.sourceBindings || len(identifier.parts) != 2 {
		return storage.Field{}, false
	}
	root := strings.ToLower(identifier.parts[0])
	if root != "$user" && root != "$profile" && root != "$organization" {
		return storage.Field{}, false
	}
	definition, ok := storage.StandardObjectDefinition(root[1:])
	if !ok {
		return storage.Field{}, false
	}
	name, ok := storage.ResolveFieldName(definition, "", identifier.parts[1])
	return definition.Fields[name], ok
}
