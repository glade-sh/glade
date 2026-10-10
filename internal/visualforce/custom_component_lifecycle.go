package visualforce

import (
	"os"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/vm"
)

type preparedCustomComponent struct {
	tree         *MarkupNode
	controller   vm.Value
	constructErr error
}

type preparedCustomComponentQueue struct {
	mu        sync.Mutex
	instances []preparedCustomComponent
}

// Keys are nodes from one RenderPage request, including its component trees.
// The page hook releases every key on return, including render errors. Keeping
// these values here avoids putting private lifecycle state in markup or Apex.
var preparedCustomComponents sync.Map

func takePreparedCustomComponent(node *MarkupNode) (preparedCustomComponent, bool) {
	value, ok := preparedCustomComponents.Load(node)
	if !ok {
		return preparedCustomComponent{}, false
	}
	queue := value.(*preparedCustomComponentQueue)
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.instances) == 0 {
		return preparedCustomComponent{}, false
	}
	instance := queue.instances[0]
	queue.instances = queue.instances[1:]
	return instance, true
}

type customComponentPageInitialization struct {
	machine      *vm.VM
	page         Page
	root         *RenderContext
	bootstrapped bool
	instances    map[*MarkupNode][]preparedCustomComponent
}

// Native assign_* and order_* rows initialize the component controller before
// evaluating its attributes, assign once during page initialization, then
// assign again during rendering. Literal and NULL bindings do not construct
// an unused page controller; repeats construct it to obtain their row values.
func bootstrapCustomComponentPageControllers(machine *vm.VM, page Page, tree *MarkupNode, req PageRenderRequest) (vm.Value, []vm.Value, vm.Value, func(), error) {
	release := func() {}
	if req.ViewState != nil || len(req.FormValues) != 0 || strings.TrimSpace(req.Action) != "" || strings.TrimSpace(page.Action) != "" || page.StandardController != "" || len(page.Extensions) != 0 || !customComponentSubtree(tree, &req.VFIndex) || !customComponentInitializationSupported(tree, &req.VFIndex) {
		controller, extensions, standard, err := bootstrapControllers(machine, page, req.ViewState)
		return controller, extensions, standard, release, err
	}
	if page.Controller != "" {
		if _, registered := visualforceVMClass(machine, page.Controller); !registered {
			controller, extensions, standard, err := bootstrapControllers(machine, page, req.ViewState)
			return controller, extensions, standard, release, err
		}
	}
	markup, err := os.ReadFile(page.File)
	if err != nil {
		return vm.Null, nil, vm.Null, release, err
	}
	restoreCustomComponentSiblings(tree, string(markup), &req.VFIndex)
	namespace := strings.TrimSpace(req.Project.Namespace)
	if namespace == "" && req.Org != nil {
		namespace = strings.TrimSpace(req.Org.Namespace)
	}
	root := &RenderContext{
		VM:         machine,
		VFIndex:    &req.VFIndex,
		Project:    req.Project,
		PageMeta:   page,
		PageName:   req.PageName,
		PageURL:    req.PageURL,
		Scope:      NewScopeStack(),
		Expression: &ExpressionContext{VM: machine, CurrentPage: machine.CurrentPage(), ProjectNamespace: namespace},
	}
	root.ensureExpression()
	initialization := customComponentPageInitialization{
		machine:   machine,
		page:      page,
		root:      root,
		instances: make(map[*MarkupNode][]preparedCustomComponent),
	}
	if err := initialization.walk(tree, root); err != nil {
		return vm.Null, nil, vm.Null, release, err
	}
	// Output getters are not evaluated in the initialization pass. Construct a
	// controller needed by ordinary page output only after preparing components.
	if initialization.pageOutputNeedsController(tree) {
		if err := initialization.bootstrapPage(); err != nil {
			return vm.Null, nil, vm.Null, release, err
		}
	}
	for node, instances := range initialization.instances {
		preparedCustomComponents.Store(node, &preparedCustomComponentQueue{instances: instances})
	}
	release = func() {
		for node := range initialization.instances {
			preparedCustomComponents.Delete(node)
		}
	}
	return root.Expression.Controller, root.Expression.Extensions, root.Expression.StandardController, release, nil
}

func (initialization *customComponentPageInitialization) bootstrapPage() error {
	if initialization.bootstrapped {
		return nil
	}
	controller, extensions, standard, err := bootstrapControllers(initialization.machine, initialization.page, nil)
	if err != nil {
		return err
	}
	initialization.bootstrapped = true
	initialization.root.Expression.Controller = controller
	initialization.root.Expression.Extensions = extensions
	initialization.root.Expression.StandardController = standard
	return nil
}

func (initialization *customComponentPageInitialization) prepareBinding(raw string, ctx *RenderContext) error {
	if ctx == initialization.root && initialization.bindingNeedsController(raw) {
		return initialization.bootstrapPage()
	}
	return nil
}

func (initialization *customComponentPageInitialization) walk(node *MarkupNode, ctx *RenderContext) error {
	if node == nil || node.Type != MarkupNodeElement {
		return nil
	}
	if node.Namespace == "apex" && node.Name == "componentbody" && ctx.ComponentParent != nil {
		for _, child := range ctx.ComponentBody {
			if err := initialization.walk(child, ctx.ComponentParent); err != nil {
				return err
			}
		}
		return nil
	}
	component, custom := indexedCustomComponent(node, ctx.VFIndex)
	if !custom && !customComponentInitializationSubtree(node, ctx) {
		return nil
	}
	if node.Namespace != "" {
		if err := initialization.prepareBinding(node.Attribute("rendered"), ctx); err != nil {
			return err
		}
		rendered, err := visualforceComponentShouldRender(node, ctx)
		if err != nil || !rendered {
			return err
		}
	}
	if custom {
		return initialization.component(node, ctx, component)
	}
	if node.Namespace == "apex" && node.Name == "repeat" {
		if err := initialization.prepareBinding(node.Attribute("value"), ctx); err != nil {
			return err
		}
		items, err := evaluateListExpression(node.Attribute("value"), ctx)
		if err != nil {
			return err
		}
		for i, item := range items {
			ctx.Scope.PushFrame()
			ctx.Scope.Set(node.Attribute("var"), item)
			ctx.Scope.Set(node.Attribute("indexvar"), vm.Int(int64(i)))
			err := initialization.children(node, ctx)
			ctx.Scope.PopFrame()
			if err != nil {
				return err
			}
		}
		return nil
	}
	return initialization.children(node, ctx)
}

func (initialization *customComponentPageInitialization) children(node *MarkupNode, ctx *RenderContext) error {
	for _, child := range node.Children {
		if err := initialization.walk(child, ctx); err != nil {
			return err
		}
	}
	return nil
}

func (initialization *customComponentPageInitialization) component(node *MarkupNode, ctx *RenderContext, component Component) error {
	markup, err := os.ReadFile(component.File)
	if err != nil {
		return err
	}
	tree, err := ParseMarkupTree(string(markup))
	if err != nil {
		return err
	}
	restoreCustomComponentSiblings(tree, string(markup), ctx.VFIndex)
	instance := preparedCustomComponent{tree: tree}
	if component.Controller != "" {
		instance.controller, instance.constructErr = initialization.machine.ConstructController(component.Controller)
	}
	for _, attribute := range component.Attributes {
		raw, supplied := componentAttributeValue(node.Attributes, attribute.Name)
		if supplied {
			if err := initialization.prepareBinding(raw, ctx); err != nil {
				return err
			}
		}
	}
	values, err := evaluatedTypedComponentAttributes(node, ctx, component.Attributes)
	if err != nil {
		return err
	}
	if err := validateRequiredComponentAttributes(component.Attributes, node.Attributes); err != nil {
		return err
	}
	childCtx := *ctx
	childCtx.ComponentAttrs = make(map[string]string, len(values))
	childCtx.componentValues = values
	childCtx.ComponentParent = ctx
	childCtx.ComponentBody = componentBodyNodes(node)
	childCtx.ComponentFacets = componentFacets(node)
	for name, value := range values {
		childCtx.ComponentAttrs[name] = visualforceFormulaText(value)
	}
	variables := childCtx.ComponentAttrsToVariables()
	expression := *ctx.Expression
	if len(variables) != 0 {
		expression.Variables = variables
	}
	childCtx.Expression = &expression
	if component.Controller != "" {
		if instance.constructErr != nil {
			if componentHasAssignedValue(component.Attributes, childCtx.ComponentAttrs) {
				return instance.constructErr
			}
		} else {
			if err := applyComponentAssignTo(initialization.machine, &instance.controller, component.Attributes, values); err != nil {
				return contextualComponentSetterError(err, component.Name)
			}
			childCtx.Expression = &ExpressionContext{VM: initialization.machine, Controller: instance.controller, Variables: variables, Scope: ctx.Scope}
		}
	}
	initialization.instances[node] = append(initialization.instances[node], instance)
	return initialization.children(tree, &childCtx)
}

func indexedCustomComponent(node *MarkupNode, index *Index) (Component, bool) {
	if node == nil || node.Type != MarkupNodeElement || index == nil || node.Namespace == "" || node.Namespace == "apex" {
		return Component{}, false
	}
	name := node.Name
	if node.Namespace != "c" {
		name = node.Namespace + "__" + node.Name
	}
	component, ok := index.Component(name)
	if !ok {
		component, ok = index.Component(node.Name)
	}
	return component, ok
}

func customComponentSubtree(node *MarkupNode, index *Index) bool {
	if _, ok := indexedCustomComponent(node, index); ok {
		return true
	}
	if node != nil {
		for _, child := range node.Children {
			if customComponentSubtree(child, index) {
				return true
			}
		}
	}
	return false
}

func customComponentInitializationSubtree(node *MarkupNode, ctx *RenderContext) bool {
	if node == nil {
		return false
	}
	if _, custom := indexedCustomComponent(node, ctx.VFIndex); custom {
		return true
	}
	if node.Namespace == "apex" && node.Name == "componentbody" && ctx.ComponentParent != nil {
		for _, child := range ctx.ComponentBody {
			if customComponentInitializationSubtree(child, ctx.ComponentParent) {
				return true
			}
		}
		return false
	}
	for _, child := range node.Children {
		if customComponentInitializationSubtree(child, ctx) {
			return true
		}
	}
	return false
}

func customComponentInitializationSupported(node *MarkupNode, index *Index) bool {
	if node == nil {
		return true
	}
	if node.Namespace == "apex" && customComponentSubtree(node, index) {
		switch node.Name {
		case "datatable", "pageblocktable", "datalist", "composition":
			return false
		}
	}
	for _, child := range node.Children {
		if !customComponentInitializationSupported(child, index) {
			return false
		}
	}
	return true
}

// Native order_twins, order_reverse_twins and order_output_after use custom
// names containing underscores. The shared HTML parser can put their following
// siblings in the custom node's body. A source self-closing invocation has no
// body; restore only indexed custom components, without changing HTML parsing.
func restoreCustomComponentSiblings(parent *MarkupNode, source string, index *Index) {
	if parent == nil {
		return
	}
	for i := 0; i < len(parent.Children); i++ {
		child := parent.Children[i]
		if _, custom := indexedCustomComponent(child, index); custom && componentInvocationSelfClosing(child, source) && len(child.Children) != 0 {
			siblings := child.Children
			child.Children = nil
			children := make([]*MarkupNode, 0, len(parent.Children)+len(siblings))
			children = append(children, parent.Children[:i+1]...)
			children = append(children, siblings...)
			children = append(children, parent.Children[i+1:]...)
			parent.Children = children
		}
		restoreCustomComponentSiblings(child, source, index)
	}
}

func (initialization *customComponentPageInitialization) pageOutputNeedsController(node *MarkupNode) bool {
	if node == nil {
		return false
	}
	if raw, ok := node.Attributes["rendered"]; ok && !strings.Contains(raw, "{!") {
		switch strings.ToLower(strings.TrimSpace(raw)) {
		case "true", "1", "on":
		default:
			return false
		}
	}
	if initialization.bindingNeedsController(node.Text) {
		return true
	}
	for _, raw := range node.Attributes {
		if initialization.bindingNeedsController(raw) {
			return true
		}
	}
	for _, child := range node.Children {
		if initialization.pageOutputNeedsController(child) {
			return true
		}
	}
	return false
}

func (initialization *customComponentPageInitialization) bindingNeedsController(raw string) bool {
	for offset := 0; offset < len(raw); {
		start := strings.Index(raw[offset:], "{!")
		if start < 0 {
			return false
		}
		start += offset + 2
		end := findExpressionTemplateEnd(raw, start)
		if end < 0 {
			return false
		}
		expression, err := parseExpression(strings.TrimSpace(raw[start:end]))
		if err == nil && customComponentExpressionNeedsController(expression, initialization.page.Controller, initialization.machine) {
			return true
		}
		offset = end + 1
	}
	return false
}

// Check declarations rather than evaluating a getter to discover whether the
// page controller is needed. This keeps getters out of the initialization scan.
func customComponentExpressionNeedsController(expression Expression, controller string, machine *vm.VM) bool {
	identifier := func(parts []string) bool {
		if len(parts) == 0 || controller == "" || strings.HasPrefix(parts[0], "$") {
			return false
		}
		name := parts[0]
		if strings.EqualFold(name, "currentpage") {
			return false
		}
		if strings.EqualFold(name, "this") {
			return true
		}
		for className := controller; className != ""; {
			class, ok := visualforceVMClass(machine, className)
			if !ok {
				break
			}
			for key, field := range class.Fields {
				if !field.Static && (strings.EqualFold(key, name) || strings.EqualFold(field.Name, name)) {
					return true
				}
			}
			for _, method := range class.Methods {
				methodName := method.Name
				if dot := strings.LastIndex(methodName, "."); dot >= 0 {
					methodName = methodName[dot+1:]
				}
				if !method.IsStatic && len(method.Params) == 0 && strings.EqualFold(methodName, "get"+name) {
					return true
				}
			}
			className = class.SuperClass
		}
		return false
	}
	needs := func(child Expression) bool {
		return customComponentExpressionNeedsController(child, controller, machine)
	}
	var arguments []Expression
	switch typed := expression.(type) {
	case identifierExpr:
		return identifier(typed.parts)
	case visualforceIdentifierExpr:
		return identifier(typed.parts)
	case binaryExpr:
		return needs(typed.left) || needs(typed.right)
	case unaryExpr:
		return needs(typed.value)
	case indexExpr:
		return needs(typed.target) || needs(typed.key)
	case memberExpr:
		return needs(typed.target)
	case methodCallExpr:
		if needs(typed.target) {
			return true
		}
		arguments = typed.args
	case visualforceFunctionExpr:
		arguments = typed.args
	case functionExpr:
		arguments = typed.args
	case parameterMapExpr:
		arguments = typed.values
	}
	for _, argument := range arguments {
		if needs(argument) {
			return true
		}
	}
	return false
}
