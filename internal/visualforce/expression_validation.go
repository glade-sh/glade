package visualforce

import (
	"fmt"
	"regexp"
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
	declaredTypes       map[string]bool // the project's interfaces and enums
	objects             map[string]schema.Object
	namespace           string
	tabFiles            []string
	controller          string
	standardController  string
	bindings            map[string]string
	sourceBindings      bool
	controllerless      bool
	formControllers     []apexast.Declaration
	standardFormActions map[string]bool
	ajaxAssignments     map[*MarkupNode]bool
}

// expressionValidationSources is the project-wide input of expression
// validation: the Apex class declarations, the resource registry and the
// schema. It does not depend on which pages a load validates, so one load
// sequence computes it once (see loadCache).
type expressionValidationSources struct {
	registry storage.MetadataRegistry
	classes  map[string]apexast.Declaration
	// declaredTypes holds the project's interfaces and enums, which a component
	// attribute may name like a class.
	declaredTypes map[string]bool
	objects       map[string]schema.Object
	namespace     string
	tabFiles      []string
}

func loadExpressionValidationSources(p project.Project) (*expressionValidationSources, error) {
	src := &expressionValidationSources{classes: map[string]apexast.Declaration{}, declaredTypes: map[string]bool{}, objects: map[string]schema.Object{}, namespace: strings.TrimSpace(p.Namespace), tabFiles: p.TabFiles}
	if len(p.ApexFiles) != 0 {
		parser := apexast.NewParser()
		defer parser.Close()
		for _, path := range p.ApexFiles {
			apexSourceParses.Add(1)
			file, err := parser.ParseFile(path)
			if err != nil {
				return nil, err
			}
			for _, declaration := range file.Declarations {
				if declaration.Kind == apexast.DeclarationClass {
					src.classes[strings.ToLower(declaration.Name)] = declaration
				}
				if declaration.Kind == apexast.DeclarationInterface || declaration.Kind == apexast.DeclarationEnum {
					src.declaredTypes[strings.ToLower(declaration.Name)] = true
				}
			}
		}
	}
	if len(src.classes) == 0 {
		// A page without a controller can still reference a declared custom
		// object through $Action. Load only object definitions on this path;
		// unrelated field and resource metadata are not prerequisites.
		if len(p.ObjectFiles) != 0 {
			objects, err := schema.LoadProject(project.Project{Root: p.Root, Namespace: p.Namespace, ObjectFiles: p.ObjectFiles})
			if err != nil {
				return nil, err
			}
			for _, object := range objects.Objects {
				src.objects[strings.ToLower(object.Name)] = object
			}
		}
		return src, nil
	}
	// Load only metadata used by expressions. Unrelated resource types must not
	// become new prerequisites for indexing Visualforce pages.
	registry, err := resource.LoadProject(project.Project{
		Root: p.Root, Namespace: p.Namespace,
		LabelFiles: p.LabelFiles, TranslationFiles: p.TranslationFiles,
		StaticResourceFiles: p.StaticResourceFiles, StaticResourceMetas: p.StaticResourceMetas, MessageChannelFiles: p.MessageChannelFiles,
		ManagedPackageDependencies: p.ManagedPackageDependencies,
	})
	if err != nil {
		return nil, err
	}
	src.registry = registry
	objects, err := schema.LoadProject(p)
	if err != nil {
		return nil, err
	}
	for _, object := range objects.Objects {
		src.objects[strings.ToLower(object.Name)] = object
	}
	mergeDependencyExpressionFields(src.objects, p, map[string]bool{})
	inheritExpressionActivityFields(src.objects)
	return src, nil
}

// mergeDependencyExpressionFields adds the custom fields that loaded
// dependency projects declare on standard objects, and on objects the
// project itself declares, to those objects. An installed dependency's fields
// exist in the org, so a page that binds one (for example an Activity lookup
// on an Event row) compiles natively. A dependency object the project does not
// declare stays out of the index, so its paths keep the deferred validation
// (its own source may be partial).
func mergeDependencyExpressionFields(objects map[string]schema.Object, p project.Project, seen map[string]bool) {
	for _, dep := range p.ManagedPackageDependencies {
		if dep.Status != "loaded" || dep.Project == nil {
			continue
		}
		key := strings.ToLower(dep.Project.Root + "\x00" + dep.Namespace)
		if seen[key] {
			continue
		}
		seen[key] = true
		depSchema, err := schema.LoadProject(*dep.Project)
		if err != nil {
			// A dependency whose metadata does not load contributes nothing,
			// as before; the dependency's own pages report it.
			continue
		}
		for _, object := range depSchema.Objects {
			name := strings.ToLower(object.Name)
			existing, declared := objects[name]
			if !declared {
				// Activity holds the shared Task and Event custom fields.
				if _, standard := storage.StandardObjectDefinition(object.Name); !standard && name != "activity" {
					continue
				}
				existing = schema.Object{Name: object.Name}
			}
			fields := append([]schema.Field(nil), existing.Fields...)
			for _, field := range object.Fields {
				present := false
				for _, current := range fields {
					if strings.EqualFold(current.Name, field.Name) {
						present = true
						break
					}
				}
				if !present {
					fields = append(fields, field)
				}
			}
			existing.Fields = fields
			objects[name] = existing
		}
		mergeDependencyExpressionFields(objects, *dep.Project, seen)
	}
}

// inheritExpressionActivityFields gives Task and Event the custom fields
// declared on Activity, as Apex and runtime describe already do. A concrete
// object that redeclares one of those fields keeps its own fields unchanged
// (M002/M003 reject that shape; no precedence is assigned here).
func inheritExpressionActivityFields(objects map[string]schema.Object) {
	activity, ok := objects["activity"]
	if !ok || len(activity.Fields) == 0 {
		return
	}
	for _, target := range []string{"Task", "Event"} {
		key := strings.ToLower(target)
		object, exists := objects[key]
		if !exists {
			object = schema.Object{Name: target}
		}
		conflict := false
		for _, field := range activity.Fields {
			for _, concrete := range object.Fields {
				if strings.EqualFold(field.Name, concrete.Name) {
					conflict = true
				}
			}
		}
		if conflict {
			continue
		}
		object.Fields = append(append([]schema.Field(nil), object.Fields...), activity.Fields...)
		objects[key] = object
	}
}

func newExpressionValidationContext(p project.Project, index *Index) (*expressionValidationContext, error) {
	return newExpressionValidationContextWith(p, index, nil)
}

// newExpressionValidationContextWith builds the context from cached sources
// when a loadCache is supplied. The context's own fields (bindings, controller,
// form state) are per page and set by validate.
func newExpressionValidationContextWith(p project.Project, index *Index, cache *loadCache) (*expressionValidationContext, error) {
	src, err := cache.expressionSources(p)
	if err != nil {
		return nil, err
	}
	return &expressionValidationContext{index: index, registry: src.registry, classes: src.classes, declaredTypes: src.declaredTypes, objects: src.objects, namespace: src.namespace, tabFiles: src.tabFiles}, nil
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
	local.controllerless = local.controller == "" && root.Attribute("standardcontroller") == "" && root.Attribute("extensions") == ""
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
	local.ajaxAssignments = ajaxFunctionAssignmentNodes(tree)
	var walk func(*MarkupNode, map[string]string) error
	walk = func(node *MarkupNode, bindings map[string]string) error {
		current := local
		current.bindings = bindings
		if err := validateFormControllerBinding(node, &current); err != nil {
			return err
		}
		if err := current.validateFieldComponentBinding(node, sourceName); err != nil {
			return err
		}
		if err := validateAjaxControllerBinding(node, &current); err != nil {
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
			if err := current.validateTextTemplate(node, sourceName, root.Name); err != nil {
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
			// Native rows (vf_corpus_shapes): an <apex:variable> is visible to
			// the siblings that follow it and their descendants, not to earlier
			// siblings and not outside its parent.
			if child.Namespace == "apex" && child.Name == "variable" {
				if name := strings.ToLower(strings.TrimSpace(child.Attribute("var"))); name != "" {
					copy := make(map[string]string, len(bindings)+1)
					for key, typ := range bindings {
						copy[key] = typ
					}
					copy[name] = ""
					bindings = copy
				}
			}
		}
		return nil
	}
	return walk(tree, local.bindings)
}

var scriptRepeatTag = regexp.MustCompile(`(?is)<apex:repeat\b[^>]*>|</apex:repeat\s*>`)
var scriptRepeatVar = regexp.MustCompile(`(?is)\bvar\s*=\s*["']([^"']+)["']`)

// validateTextTemplate validates the expressions of a text node. The content of
// a script element is raw text, so an <apex:repeat> inside it reaches the
// validator as text; the org still repeats it, and its variable is bound for the
// expressions up to the closing tag (rows ScriptRepeat* of testdata/vf_component_ids).
func (ctx *expressionValidationContext) validateTextTemplate(node *MarkupNode, sourceName, rootName string) error {
	text := node.Text
	if !strings.Contains(strings.ToLower(text), "<apex:repeat") {
		return ctx.validateTemplate(text, node, "", sourceName, rootName)
	}
	current := *ctx
	var stack []map[string]string
	bindings := ctx.bindings
	last := 0
	for _, loc := range scriptRepeatTag.FindAllStringIndex(text, -1) {
		current.bindings = bindings
		if err := current.validateTemplate(text[last:loc[0]], node, "", sourceName, rootName); err != nil {
			return err
		}
		last = loc[1]
		tag := text[loc[0]:loc[1]]
		if strings.HasPrefix(tag, "</") {
			if len(stack) > 0 {
				bindings, stack = stack[len(stack)-1], stack[:len(stack)-1]
			}
			continue
		}
		// The tag's own attributes are validated with the enclosing bindings.
		if err := current.validateTemplate(tag, node, "", sourceName, rootName); err != nil {
			return err
		}
		stack = append(stack, bindings)
		if match := scriptRepeatVar.FindStringSubmatch(tag); match != nil {
			next := make(map[string]string, len(bindings)+1)
			for name, typ := range bindings {
				next[name] = typ
			}
			next[strings.ToLower(match[1])] = ""
			bindings = next
		}
	}
	current.bindings = bindings
	return current.validateTemplate(text[last:], node, "", sourceName, rootName)
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
		// A field without accessors is not a page property: the org reports
		// "Unknown property" (rows ExprField*, PageMsgStrengthExprField).
		if member.Kind == apexast.DeclarationField {
			continue
		}
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
		// e_function_param_unknown accepts an assignment destination without
		// reading it. Preserve the shared syntax validation above.
		if attribute == "assignto" && ctx.ajaxAssignments[node] {
			offset = end + 1
			continue
		}
		if _, err := ctx.expressionType(expr); err != nil {
			return err
		}
		if err := ctx.rejectBarePicklist(expr); err != nil {
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
		// Only these operators have native picklist operand captures.
		switch expr.op {
		case "==", "!=", "+", "&", "<":
			for _, operand := range []Expression{expr.left, expr.right} {
				if err := ctx.rejectBarePicklist(operand); err != nil {
					return "", err
				}
			}
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
		if strings.EqualFold(expr.name, "ISPICKVAL") {
			return ctx.isPickValType(expr)
		}
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
	for i, arg := range args {
		if name == "URLFOR" && i == 0 && len(args) == 2 && (ctx.sourceBindings || ctx.controllerless) {
			if accepted, err := ctx.customTabURLTarget(arg, args[1]); err != nil {
				return "", err
			} else if accepted {
				continue
			}
		}
		typ, err := ctx.expressionType(arg)
		if err != nil {
			return "", err
		}
		// Only the captured selector shapes are rejected; other Boolean
		// expressions retain their pre-ISPICKVAL behavior.
		if name == "CASE" && i == 0 && ctx.rejectedCaseSelector(arg) {
			return "", fmt.Errorf("Incorrect argument type for function 'CASE()'.")
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
		// $Label.site.* is a closed system namespace; it needs no controller
		// or project source to validate.
		if root == "$label" && len(parts) >= 2 && strings.EqualFold(parts[1], "site") {
			if len(parts) != 3 {
				return missing()
			}
			if _, ok := resource.SiteSystemLabel(parts[2]); !ok {
				return missing()
			}
			return "Text", nil
		}
		// $User, $Profile and $Organization resolve against the standard
		// definitions and need no controller: native rows reject an unknown
		// field on a controller-less page too.
		if root == "$user" || root == "$profile" || root == "$organization" {
			return ctx.globalIdentifierType(root, parts, missing)
		}
		// Controller-less pages resolve action names against project object
		// metadata; they have no controller source to wait for.
		if root == "$action" && ctx.controllerless {
			return ctx.actionReferenceType(parts, missing)
		}
		// LoadProject also indexes projects whose controllers and metadata are
		// supplied later by a VM. Validate reference existence only when the
		// page's controller/extension source participates in this project.
		if !ctx.sourceBindings {
			return "", nil
		}
		switch root {
		case "$action":
			return ctx.actionReferenceType(parts, missing)
		case "$remoteaction":
			return "", ctx.validateRemoteActionReference(parts)
		case "$asset":
			if len(parts) == 2 && strings.EqualFold(parts[1], "SLDS") {
				return "Text", nil
			}
			return missing()
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
			if len(parts) < 3 {
				return missing()
			}
			object, found := ctx.objects[strings.ToLower(parts[1])]
			if !found && len(parts) == 3 && ctx.namespace != "" && strings.Count(parts[1], "__") == 1 {
				// Resolve only an owned hierarchy setting's local name. Explicit
				// qualifiers and already resolved objects keep their identity.
				candidate := ctx.objects[strings.ToLower(ctx.namespace+"__"+parts[1])]
				if strings.EqualFold(candidate.CustomSettingsType, "Hierarchy") {
					object = candidate
				}
			}
			if object.CustomSettingsType == "" {
				return missing()
			}
			return "", nil
		case "$messagechannel":
			return ctx.messageChannelType(parts)
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
			if ctx.sameOwnName(field.Name, parts[1]) {
				return "", nil
			}
			// A custom lookup declared in project metadata is also a member
			// by its relationship name (Field__r), as on the standard
			// controller path.
			if len(field.ReferenceTo) != 0 && ctx.sameOwnName(storage.ParentRelationshipName(storage.Field{
				APIName: field.Name, Type: storage.FieldReference, RelationshipName: field.RelationshipName,
			}), parts[1]) {
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

// sameOwnName reports whether a declared custom name is the requested one. A
// namespaced package's source names its own fields, relationships and objects
// without the namespace prefix while the project index qualifies them (rows
// NsRel* and NsAttr* of testdata/vf_component_ids).
func (ctx *expressionValidationContext) sameOwnName(declared, requested string) bool {
	if strings.EqualFold(declared, requested) {
		return true
	}
	return ctx.namespace != "" && strings.EqualFold(declared, ctx.namespace+"__"+requested)
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
		if ctx.sameOwnName(field.Name, parts[0]) {
			return nil
		}
		if len(field.ReferenceTo) != 0 && ctx.sameOwnName(storage.ParentRelationshipName(storage.Field{
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

// $User does not expose every User picklist to Visualforce formulas.
func visualforceHiddenUserPicklist(field string) bool {
	switch strings.ToLower(field) {
	case "emailencodingkey", "digestfrequency", "defaultgroupnotificationfrequency":
		return true
	}
	return false
}

func (ctx *expressionValidationContext) messageChannelType(parts []string) (string, error) {
	if len(parts) == 1 {
		return "", fmt.Errorf("Invalid use of global variable 'ApexPagesELAdapterContext.$MessageChannel'. Expected field selector is missing.")
	}
	name := parts[1]
	if len(name) > len("__c") && name[len(name)-len("__c"):] == "__c" {
		local := name[:len(name)-len("__c")]
		for _, channel := range ctx.registry.MessageChannels {
			if strings.EqualFold(channel.Name, local) {
				return "Text", nil
			}
		}
	}
	return "", fmt.Errorf("We couldn\u2019t find Lightning Message Channel %s. Check your spelling and try again.", name)
}

// A picklist global is legal only as the first ISPICKVAL argument and inside
// TEXT; used as a plain value it fails to compile (captured for a printed
// value, an operator operand and the ISPICKVAL literal argument).
func (ctx *expressionValidationContext) rejectBarePicklist(expr Expression) error {
	identifier, ok := expr.(visualforceIdentifierExpr)
	if !ok || len(identifier.parts) != 2 || (!strings.EqualFold(identifier.parts[0], "$User") && !strings.EqualFold(identifier.parts[0], "$Profile")) {
		return nil
	}
	if field, ok := ctx.globalField(expr); ok && field.Type == storage.FieldPicklist {
		return fmt.Errorf("Field %s is a picklist field. Picklist fields are only supported in certain functions.", field.APIName)
	}
	return nil
}

// rejectedCaseSelector recognizes the captured ISPICKVAL, true, equality,
// and checkbox selector shapes without rejecting unrelated Boolean functions.
func (ctx *expressionValidationContext) rejectedCaseSelector(expr Expression) bool {
	switch value := expr.(type) {
	case visualforceFunctionExpr:
		return strings.EqualFold(value.name, "ISPICKVAL")
	case literalExpr:
		return value.value.Kind == vm.ValueBool && value.value.Bool
	case binaryExpr:
		return value.op == "=="
	case visualforceIdentifierExpr:
		if len(value.parts) != 2 {
			return false
		}
		typ := ctx.bindings[strings.ToLower(value.parts[0])]
		if ctx.standardControllerPath(value.parts) {
			typ = ctx.standardController
		}
		for _, field := range ctx.objects[strings.ToLower(typ)].Fields {
			if strings.EqualFold(field.Name, value.parts[1]) && strings.EqualFold(field.Type, "Checkbox") {
				return true
			}
		}
	}
	return false
}

// isPickValType checks ISPICKVAL(picklist, 'literal'). Record fields reach a
// formula as Text, so only picklist globals ($User, $Profile) qualify.
func (ctx *expressionValidationContext) isPickValType(expr visualforceFunctionExpr) (string, error) {
	spelling := expr.spelling
	if spelling == "" {
		spelling = strings.ToUpper(expr.name)
	}
	if len(expr.args) != 2 {
		return "", fmt.Errorf("Incorrect number of parameters for function 'ISPICKVAL()'. Expected 2, received %d", len(expr.args))
	}
	first, err := ctx.expressionType(expr.args[0])
	if err != nil {
		return "", err
	}
	if field, ok := ctx.globalField(expr.args[0]); !ok || field.Type != storage.FieldPicklist {
		return "", fmt.Errorf("Incorrect parameter type for function '%s()'. Expected Picklist, received %s", spelling, ctx.isPickValOperandType(expr.args[0], first))
	}
	second, err := ctx.expressionType(expr.args[1])
	if err != nil {
		return "", err
	}
	if err := ctx.rejectBarePicklist(expr.args[1]); err != nil {
		return "", err
	}
	if literal, isLiteral := expr.args[1].(literalExpr); !isLiteral || literal.value.Kind != vm.ValueString {
		return "", fmt.Errorf("Incorrect parameter type for function '%s()'. Expected Text Literal, received %s", spelling, ctx.isPickValOperandType(expr.args[1], second))
	}
	return "Boolean", nil
}

// isPickValOperandType names an operand's type the way the compiler reports it.
func (ctx *expressionValidationContext) isPickValOperandType(arg Expression, inferred string) string {
	if literal, ok := arg.(literalExpr); ok {
		switch literal.value.Kind {
		case vm.ValueNull:
			return "Null"
		case vm.ValueBool:
			return "Boolean"
		case vm.ValueInt, vm.ValueDecimal:
			return "Number"
		}
		return "Text"
	}
	if field, ok := ctx.globalField(arg); ok {
		if field.Type == storage.FieldID {
			return "Lookup(User)"
		}
		if typ := expressionStorageFieldType(field); typ != "" {
			return typ
		}
		return "Text"
	}
	if identifier, ok := arg.(visualforceIdentifierExpr); ok && ctx.standardControllerPath(identifier.parts) && len(identifier.parts) == 2 {
		for _, field := range ctx.objects[strings.ToLower(ctx.standardController)].Fields {
			if strings.EqualFold(field.Name, identifier.parts[1]) {
				switch strings.ToLower(field.Type) {
				case "number", "currency", "percent":
					return "Number"
				case "checkbox":
					return "Boolean"
				}
			}
		}
	}
	if inferred == "" {
		return "Text"
	}
	return inferred
}

// globalField resolves a $User, $Profile or $Organization field from the
// standard definitions. It does not depend on a controller: native rows for
// controller-less pages and components accept the same picklist uses.
func (ctx *expressionValidationContext) globalField(expr Expression) (storage.Field, bool) {
	identifier, ok := expr.(visualforceIdentifierExpr)
	if !ok || len(identifier.parts) != 2 {
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

func (ctx *expressionValidationContext) globalIdentifierType(root string, parts []string, missing func() (string, error)) (string, error) {
	if len(parts) < 2 {
		return "", nil
	}
	definition, ok := storage.StandardObjectDefinition(root[1:])
	if !ok {
		if !ctx.sourceBindings {
			return "", nil
		}
		return missing()
	}
	if root == "$user" && len(parts) == 2 && (strings.EqualFold(parts[1], "UITheme") || strings.EqualFold(parts[1], "UIThemeDisplayed")) {
		// Documented $User globals that the sObject describe does not carry.
		return "Text", nil
	}
	fieldName, ok := storage.ResolveFieldName(definition, "", parts[1])
	if !ok && !ctx.sourceBindings && (strings.Contains(parts[1], "__") || len(parts) > 2) {
		// Without project source, custom fields and relationship paths the
		// standard definition cannot see stay accepted.
		return "", nil
	}
	if !ok || (root == "$user" && (visualforceHiddenUserPicklist(fieldName) || strings.EqualFold(fieldName, "PortalRole"))) ||
		(root == "$organization" && (strings.EqualFold(fieldName, "OrganizationType") || strings.EqualFold(fieldName, "LanguageLocaleKey"))) {
		return "", fmt.Errorf("Field %s does not exist. Check spelling.", strings.Join(parts[1:], "."))
	}
	return expressionStorageFieldType(definition.Fields[fieldName]), nil
}

// actionVerbs are the $Action verbs the org compiles for a standard or custom
// object (rows Action* of testdata/vf_component_ids).
var actionVerbs = map[string]bool{"new": true, "view": true, "edit": true, "delete": true, "list": true}

// actionReferenceType validates $Action.<Object>.<Verb>. A custom object's Tab
// action needs its tab metadata and is validated with URLFOR (customTabURLTarget).
func (ctx *expressionValidationContext) actionReferenceType(parts []string, missing func() (string, error)) (string, error) {
	if len(parts) != 3 {
		return missing()
	}
	verb := strings.ToLower(parts[2])
	_, standard := storage.StandardObjectDefinition(parts[1])
	if standard && (actionVerbs[verb] || verb == "tab") {
		return "Text", nil
	}
	custom := false
	if _, ok := ctx.objects[strings.ToLower(parts[1])]; ok {
		custom = true
	} else if ctx.namespace != "" && strings.Count(parts[1], "__") == 1 {
		_, custom = ctx.objects[strings.ToLower(ctx.namespace+"__"+parts[1])]
	}
	if !custom && !ctx.controllerless && len(ctx.objects) == 0 && strings.HasSuffix(strings.ToLower(parts[1]), "__c") {
		custom = true // no project schema was indexed for this page
	}
	if custom && actionVerbs[verb] {
		return "Text", nil
	}
	return missing()
}
