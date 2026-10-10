package visualforce

import (
	"archive/zip"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

type RenderContext struct {
	VM                        *vm.VM
	PageName                  string
	PageURL                   string
	PageMeta                  Page
	VFIndex                   *Index
	Project                   project.Project
	Expression                *ExpressionContext
	Scope                     *ScopeStack
	Defines                   map[string]*MarkupNode
	ComponentAttrs            map[string]string
	componentValues           map[string]vm.Value
	componentTypes            map[string]string
	Metrics                   *RenderMetrics
	Debug                     bool
	LightningOut              bool
	LightningBootstrap        *lwcbrowser.PageConfig
	ComponentBody             []*MarkupNode
	ComponentFacets           map[string]*MarkupNode
	ComponentParent           *RenderContext
	ComponentReferences       *componentReferenceResolver
	inputFieldSectionLabel    bool
	presentationHead          []string
	collectPresentationHead   bool
	presentationBlockID       string
	presentationSingleSection bool
	presentationChartRendered bool
	repetitionVariables       []string
	formLifecycle             *formLifecycle
	repeatPath                []string
	repeatCommandScripts      map[string]bool
}

func RenderMarkupTree(node *MarkupNode, ctx *RenderContext) (string, error) {
	if node == nil {
		return "", nil
	}
	if ctx == nil {
		ctx = &RenderContext{}
	}
	ctx.ensureExpression()
	previousChart := ctx.presentationChartRendered
	ctx.presentationChartRendered = false
	defer func() { ctx.presentationChartRendered = previousChart }()
	previousReferences := ctx.ComponentReferences
	previousScope := ctx.Expression.ComponentReferenceScope
	resolver := newComponentReferenceResolver(node, previousScope)
	ctx.ComponentReferences = resolver
	ctx.Expression.ComponentReferenceScope = &componentReferenceScope{resolver: resolver, container: resolver.root}
	defer func() {
		ctx.ComponentReferences = previousReferences
		ctx.Expression.ComponentReferenceScope = previousScope
	}()
	out, err := renderMarkupNode(node, ctx)
	if err != nil {
		return "", err
	}
	return out, nil
}

func (ctx *RenderContext) ensureExpression() {
	if ctx == nil {
		return
	}
	if ctx.Expression == nil {
		ctx.Expression = &ExpressionContext{}
	}
	if ctx.Expression.VM == nil && ctx.VM != nil {
		ctx.Expression.VM = ctx.VM
	}
	if ctx.Scope == nil {
		ctx.Scope = NewScopeStack()
	}
	if ctx.Expression.Scope == nil {
		ctx.Expression.Scope = ctx.Scope
	}
	if ctx.Defines == nil {
		ctx.Defines = make(map[string]*MarkupNode)
	}
}

func (ctx *RenderContext) countComponent(name string) {
	if ctx == nil || ctx.Metrics == nil {
		return
	}
	key := strings.ToLower(strings.TrimSpace(name))
	ctx.Metrics.ComponentCounts[key]++
}

func renderMarkupNode(node *MarkupNode, ctx *RenderContext) (string, error) {
	return renderMarkupNodeWithHiddenPlaceholder(node, ctx, false)
}

// Only direct children of layout-none output panels preserve hidden placeholders.
// Scope, counting, and rendered-expression evaluation stay on the shared path.
func renderMarkupNodeWithHiddenPlaceholder(node *MarkupNode, ctx *RenderContext, preserveHidden bool) (string, error) {
	previousScope := ctx.Expression.ComponentReferenceScope
	if ctx.ComponentReferences != nil {
		if scope := ctx.ComponentReferences.scopes[node]; scope != nil {
			ctx.Expression.ComponentReferenceScope = scope
		}
	}
	defer func() {
		ctx.Expression.ComponentReferenceScope = previousScope
	}()
	switch node.Type {
	case MarkupNodeText:
		if ctx.Metrics != nil {
			ctx.Metrics.ExpressionEvals++
		}
		return RenderVisualforceText(node.Text, ctx.Expression)
	case MarkupNodeElement:
		return renderElement(node, ctx, preserveHidden)
	default:
		return "", nil
	}
}

func renderElement(node *MarkupNode, ctx *RenderContext, preserveHidden bool) (string, error) {
	if node == nil {
		return "", nil
	}
	component := strings.TrimSpace(node.Name)
	namespace := strings.ToLower(strings.TrimSpace(node.Namespace))
	ctx.countComponent(namespace + ":" + component)
	if namespace != "" {
		shouldRender, err := visualforceComponentShouldRender(node, ctx)
		if err != nil {
			return "", err
		}
		if !shouldRender {
			if preserveHidden {
				attrs := ""
				if id := visualforceComponentClientID(node, ctx); id != "" {
					attrs = ` id="` + html.EscapeString(id) + `"`
				}
				return "<span" + attrs + ` style="display: none;"></span>`, nil
			}
			return "", nil
		}
		if spec, ok := StandardComponentSpec(namespace, component); ok {
			if spec.Render == nil {
				return renderUnsupportedComponent(node, spec)
			}
			if namespace == "apex" && component == "outputtext" {
				return renderComponentAttributeOutput(node, ctx, spec.Render)
			}
			return spec.Render(node, ctx)
		}
	}
	if namespace == "c" || (namespace != "" && namespace != "apex") {
		return renderCustomComponent(node, ctx)
	}
	return renderHTMLPassthrough(node, ctx)
}

func renderUnsupportedComponent(node *MarkupNode, spec ComponentSpec) (string, error) {
	name := strings.TrimSpace(spec.Name)
	if name == "" && node != nil {
		name = strings.TrimSpace(node.Namespace + ":" + node.Name)
	}
	reason := strings.TrimSpace(spec.Reason)
	if reason == "" {
		return "", vm.NewUnsupportedFeatureError(fmt.Sprintf("unsupported Visualforce component %s", name))
	}
	return "", vm.NewUnsupportedFeatureError(fmt.Sprintf("unsupported Visualforce component %s: %s", name, reason))
}

func renderChildren(node *MarkupNode, ctx *RenderContext) (string, error) {
	builder := strings.Builder{}
	for _, child := range node.Children {
		rendered, err := renderMarkupNode(child, ctx)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func renderApexPage(node *MarkupNode, ctx *RenderContext) (string, error) {
	previousHead, previousCollect := ctx.presentationHead, ctx.collectPresentationHead
	ctx.presentationHead, ctx.collectPresentationHead = nil, true
	defer func() { ctx.presentationHead, ctx.collectPresentationHead = previousHead, previousCollect }()
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	title := strings.TrimSpace(node.Attribute("title"))
	head := ""
	body := children
	if title != "" {
		head = "<title>" + html.EscapeString(title) + "</title>"
	}
	standardStyles, err := RenderExpressionTemplate(node.Attribute("standardStylesheets"), ctx.Expression)
	if err != nil {
		return "", err
	}
	if node.Attribute("standardStylesheets") == "" || strings.EqualFold(strings.TrimSpace(standardStyles), "true") {
		head += `<link rel="stylesheet" href="/styles/glade-visualforce.css" />`
	}
	head += strings.Join(ctx.presentationHead, "") + VisualforceAjaxScript() + renderPageRemotingScript(node, ctx)
	return "<!DOCTYPE html><html><head>" + head + "</head><body>" + body + "</body></html>", nil
}

func renderApexOutput(node *MarkupNode, ctx *RenderContext, outputField bool) (string, error) {
	raw, hasValue := node.Attributes["value"]
	if outputField && !hasValue {
		raw = ""
	}
	if outputField {
		if value, ok := renderFieldOutput(ctx, raw); ok {
			return "<span" + componentIDAttr(node, ctx) + ">" + value + "</span>", nil
		}
	}
	literalValue := hasValue && !strings.Contains(raw, "{!")
	value := raw
	if hasValue {
		var rendered string
		var err error
		formKey := strings.ToLower(strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(raw), "{!"), "}")))
		if outputField || (ctx.formLifecycle != nil && ctx.formLifecycle.properties[formKey] != "") {
			rendered, err = renderFormExpression(raw, ctx)
		} else {
			rendered, err = renderRepetitionOutputTemplate(raw, ctx)
		}
		if err != nil {
			var field *repetitionFieldError
			if errors.As(err, &field) {
				return "", contextualRepetitionFieldError(err, raw, node.RawName, ctx.PageName)
			}
			var formula *formulaEvaluationError
			if errors.As(err, &formula) {
				err = contextualVisualforceFormulaError(err, formula, raw, node.RawName, ctx.PageName)
			}
			return "", err
		}
		value = rendered
	} else if !outputField {
		rawChildren, err := renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
		value = rawChildren
	}
	escape := !strings.EqualFold(strings.TrimSpace(node.Attribute("escape")), "false")
	renderedValue := EscapeVisualforceOutput(value, escape)
	if literalValue && !escape {
		renderedValue = html.EscapeString(value)
	}
	attrs := outputTextSpanAttrs(node, ctx)
	if !outputField {
		var err error
		attrs, err = presentationAttributes(node, ctx, false, true, "styleClass", "style", "title")
		if err != nil {
			return "", err
		}
	}
	if outputField || attrs != "" {
		return "<span" + attrs + ">" + renderedValue + "</span>", nil
	}
	return renderedValue, nil
}

func renderApexOutputFormat(node *MarkupNode, ctx *RenderContext) (string, error) {
	format, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	if format == "" {
		format, err = renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
	}
	for i, param := range outputFormatParams(node, ctx) {
		format = strings.ReplaceAll(format, fmt.Sprintf("{%d}", i), param)
	}
	escape := !strings.EqualFold(strings.TrimSpace(node.Attribute("escape")), "false")
	return "<span" + componentIDAttr(node, ctx) + ">" + EscapeVisualforceOutput(format, escape) + "</span>", nil
}

func outputTextSpanAttrs(node *MarkupNode, ctx *RenderContext) string {
	var attrs []string
	if id := visualforceExplicitComponentClientID(node, ctx); id != "" {
		attrs = append(attrs, `id="`+html.EscapeString(id)+`"`)
	}
	if className := strings.TrimSpace(firstNonEmpty(node.Attribute("styleClass"), node.Attribute("class"))); className != "" {
		attrs = append(attrs, `class="`+html.EscapeString(className)+`"`)
	}
	if style := strings.TrimSpace(node.Attribute("style")); style != "" {
		attrs = append(attrs, `style="`+html.EscapeString(style)+`"`)
	}
	if len(attrs) == 0 {
		return ""
	}
	return " " + strings.Join(attrs, " ")
}

func outputFormatParams(node *MarkupNode, ctx *RenderContext) []string {
	params := make([]string, 0)
	for _, child := range node.Children {
		if child.Type != MarkupNodeElement || !strings.EqualFold(child.Namespace, "apex") || !strings.EqualFold(child.Name, "param") {
			continue
		}
		value := firstNonEmpty(child.Attribute("value"), child.Attribute("assignTo"))
		rendered, err := RenderExpressionTemplate(value, ctx.Expression)
		if err != nil || strings.TrimSpace(value) == "" {
			rendered, _ = renderChildren(child, ctx)
		}
		params = append(params, rendered)
	}
	return params
}

func renderApexOutputLabel(node *MarkupNode, ctx *RenderContext) (string, error) {
	value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(value) == "" {
		value, err = renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
	}
	return "<label" + componentIDAttr(node, ctx) + ` class="vfLabel">` + html.EscapeString(value) + "</label>", nil
}

func renderApexContainer(node *MarkupNode, tag string, className string, ctx *RenderContext) (string, error) {
	children, eventAttrs, err := renderApexContainerChildren(node, ctx)
	if err != nil {
		return "", err
	}
	attrs := componentIDAttr(node, ctx) + eventAttrs
	if className == "" {
		return "<" + tag + attrs + ">" + children + "</" + tag + ">", nil
	}
	return "<" + tag + attrs + " class=\"" + className + "\">" + children + "</" + tag + ">", nil
}

func renderApexContainerChildren(node *MarkupNode, ctx *RenderContext) (string, string, error) {
	var children strings.Builder
	var eventAttrs strings.Builder
	// Parent binding is unambiguous for one leaf actionSupport child; richer or
	// multiple child variants keep the existing component-render path.
	parentSupportCount := 0
	for _, child := range node.Children {
		if isApexActionSupportNode(child) && strings.TrimSpace(child.Attribute("rerender")) != "" {
			parentSupportCount++
		}
	}
	for _, child := range node.Children {
		if parentSupportCount == 1 && isApexActionSupportNode(child) && len(child.Children) == 0 && strings.TrimSpace(child.Attribute("rerender")) != "" {
			ctx.countComponent("apex:actionSupport")
			shouldRender, err := visualforceComponentShouldRender(child, ctx)
			if err != nil {
				return "", "", err
			}
			if !shouldRender {
				continue
			}
			event := strings.TrimSpace(child.Attribute("event"))
			if event == "" {
				event = "change"
			}
			hook := VisualforceAjaxLinkHookWithStatus(
				strings.TrimSpace(child.Attribute("action")),
				strings.TrimSpace(child.Attribute("rerender")),
				strings.TrimSpace(child.Attribute("status")),
			)
			eventAttrs.WriteByte(' ')
			eventAttrs.WriteString(html.EscapeString(visualforceEventAttributeName(event)))
			eventAttrs.WriteString(`="`)
			eventAttrs.WriteString(html.EscapeString(hook))
			eventAttrs.WriteByte('"')
			continue
		}
		previousSectionLabel := ctx.inputFieldSectionLabel
		ctx.inputFieldSectionLabel = strings.EqualFold(node.Namespace, "apex") &&
			strings.EqualFold(node.Name, "pageBlockSection") &&
			strings.EqualFold(child.Namespace, "apex") && strings.EqualFold(child.Name, "inputField")
		rendered, err := renderMarkupNode(child, ctx)
		ctx.inputFieldSectionLabel = previousSectionLabel
		if err != nil {
			return "", "", err
		}
		children.WriteString(rendered)
	}
	return children.String(), eventAttrs.String(), nil
}

func renderInputFieldLabel(raw string, ctx *ExpressionContext) (string, error) {
	if strings.HasPrefix(raw, "{!") && findExpressionTemplateEnd(raw, 2) == len(raw)-1 {
		expr, err := parseExpression(strings.TrimSpace(raw[2 : len(raw)-1]))
		if err != nil {
			return "", err
		}
		if global := unsupportedVisualforceGlobal(expr); global != "" {
			return "", vm.NewUnsupportedFeatureError(fmt.Sprintf("%s: unsupported Visualforce global", global))
		}
		value, err := evaluateExpressionNode(expr, ctx)
		if err != nil {
			return "", err
		}
		if value.Kind == vm.ValueNull {
			return "", fmt.Errorf("inputField label cannot be null")
		}
		if value.Kind == vm.ValueString {
			return value.Text, nil
		}
		return value.String(), nil
	}
	return RenderExpressionTemplate(raw, ctx)
}

func isApexActionSupportNode(node *MarkupNode) bool {
	return node != nil && node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") && strings.EqualFold(node.Name, "actionSupport")
}

func renderApexOutputPanel(node *MarkupNode, ctx *RenderContext) (string, error) {
	switch strings.ToLower(strings.TrimSpace(node.Attribute("layout"))) {
	case "", "inline":
		return renderPresentationPanel(node, "span", ctx)
	case "none":
		var children strings.Builder
		for _, child := range node.Children {
			rendered, err := renderMarkupNodeWithHiddenPlaceholder(child, ctx, true)
			if err != nil {
				return "", err
			}
			children.WriteString(rendered)
		}
		return children.String(), nil
	default:
		return renderPresentationPanel(node, "div", ctx)
	}
}

func renderPresentationPanel(node *MarkupNode, tag string, ctx *RenderContext) (string, error) {
	attrs, err := presentationAttributes(node, ctx, true, true, "styleClass", "style", "title", "onclick")
	if err != nil {
		return "", err
	}
	children, events, err := renderApexContainerChildren(node, ctx)
	if err != nil {
		return "", err
	}
	return "<" + tag + attrs + events + ">" + children + "</" + tag + ">", nil
}

func renderApexLink(node *MarkupNode, ctx *RenderContext) (string, error) {
	href := node.Attribute("value")
	if href == "" {
		href = "#"
	}
	renderedHref, err := RenderExpressionTemplate(href, ctx.Expression)
	if err != nil {
		return "", err
	}
	child, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(child) == "" {
		child = html.EscapeString(renderedHref)
	}
	return "<a" + componentIDAttr(node, ctx) + ` href="` + html.EscapeString(renderedHref) + `">` + child + "</a>", nil
}

func renderApexForm(node *MarkupNode, ctx *RenderContext) (string, error) {
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	action := "/apex/"
	if ctx != nil {
		if strings.TrimSpace(ctx.PageURL) != "" {
			action = ctx.PageURL
		} else if strings.TrimSpace(ctx.PageName) != "" {
			action += ctx.PageName
		}
	}
	attrs := strings.Builder{}
	if id := visualforceComponentClientID(node, ctx); id != "" {
		attrs.WriteString(` id="`)
		attrs.WriteString(html.EscapeString(id))
		attrs.WriteString(`" name="`)
		attrs.WriteString(html.EscapeString(id))
		attrs.WriteString(`"`)
	}
	attrs.WriteString(` method="post" action="`)
	attrs.WriteString(html.EscapeString(action))
	attrs.WriteString(`"`)
	enctype := strings.TrimSpace(node.Attribute("enctype"))
	if enctype == "" && visualforceFormContainsInputFile(node) {
		enctype = "multipart/form-data"
	}
	if enctype != "" {
		attrs.WriteString(` enctype="`)
		attrs.WriteString(html.EscapeString(enctype))
		attrs.WriteString(`"`)
	}
	formMarker := ""
	if ctx.formLifecycle != nil {
		formMarker = `<input type="hidden" name="__vf_form" value="` + html.EscapeString(visualforceComponentClientID(node, ctx)) + `" />`
	}
	return "<form" + attrs.String() + ">" + formMarker + `<input type="hidden" name="` + ViewStateActionFieldName() + `" value="" />` + children + "</form>", nil
}

func visualforceFormContainsInputFile(node *MarkupNode) bool {
	if node == nil {
		return false
	}
	if node.Type == MarkupNodeElement && strings.EqualFold(node.Namespace, "apex") && strings.EqualFold(node.Name, "inputFile") {
		return true
	}
	for _, child := range node.Children {
		if visualforceFormContainsInputFile(child) {
			return true
		}
	}
	return false
}

func renderApexInputText(node *MarkupNode, ctx *RenderContext, inputType string) (string, error) {
	name := formRenderedInputName(node, ctx)
	clientID := visualforceExplicitComponentClientID(node, ctx)
	value, err := renderFormInputValue(node, ctx)
	if err != nil {
		return "", err
	}
	attrs := `<input type="` + html.EscapeString(inputType) + `" name="` + html.EscapeString(name) + `"`
	if clientID != "" {
		attrs += ` id="` + html.EscapeString(clientID) + `"`
	}
	return attrs + formBooleanAttrs(node, ctx) + ` value="` + html.EscapeString(value) + `" />`, nil
}

func renderApexInputTextarea(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := formRenderedInputName(node, ctx)
	clientID := visualforceExplicitComponentClientID(node, ctx)
	value, err := renderFormInputValue(node, ctx)
	if err != nil {
		return "", err
	}
	attrs := strings.Builder{}
	attrs.WriteString(` name="`)
	attrs.WriteString(html.EscapeString(name))
	attrs.WriteString(`"`)
	if clientID != "" {
		attrs.WriteString(` id="`)
		attrs.WriteString(html.EscapeString(clientID))
		attrs.WriteString(`"`)
	}
	for _, attr := range []string{"rows", "cols"} {
		if raw := strings.TrimSpace(node.Attribute(attr)); raw != "" {
			attrs.WriteString(` `)
			attrs.WriteString(attr)
			attrs.WriteString(`="`)
			attrs.WriteString(html.EscapeString(raw))
			attrs.WriteString(`"`)
		}
	}
	return `<textarea` + attrs.String() + formBooleanAttrs(node, ctx) + `>` + html.EscapeString(value) + `</textarea>`, nil
}

func renderApexInputCheckbox(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := formRenderedInputName(node, ctx)
	clientID := visualforceExplicitComponentClientID(node, ctx)
	checked := ""
	checkboxValue := "true"
	selected := isTruthyExpression(node.Attribute("selected"), ctx)
	if node.Attribute("value") != "" {
		checkboxValue = "on"
		value, err := renderFormInputValue(node, ctx)
		if err != nil {
			return "", err
		}
		selected = strings.EqualFold(value, "true")
	}
	if selected {
		checked = ` checked="checked"`
	}
	escapedName := html.EscapeString(name)
	checkboxID := ""
	if clientID != "" {
		checkboxID = ` id="` + html.EscapeString(clientID) + `"`
	}
	return `<input type="hidden" name="` + escapedName + `" value="false" />` +
		`<input type="checkbox" name="` + escapedName + `"` + checkboxID + formBooleanAttrs(node, ctx) + ` value="` + checkboxValue + `"` + checked + " />", nil
}

func renderApexInputField(node *MarkupNode, ctx *RenderContext) (string, error) {
	if binding, ok := resolveFieldBinding(ctx, node.Attribute("value")); ok && binding.ObjectName != "" {
		raw := strings.TrimSpace(node.Attribute("value"))
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "{!"), "}"))
		if len(strings.Split(raw, ".")) != 2 ||
			(!strings.EqualFold(binding.FieldName, "Id") && !strings.EqualFold(binding.FieldName, "Name") && !binding.Authorized) {
			return "", fmt.Errorf("Visualforce inputField %q is not supported by the bounded record projection", raw)
		}
	}
	name := inputFieldName(node)
	clientID := visualforceExplicitComponentClientID(node, ctx)
	id := firstNonEmpty(clientID, name)
	labelPrefix := ""
	if node.HasAttribute("label") {
		label, err := renderInputFieldLabel(node.Attribute("label"), ctx.Expression)
		if err != nil {
			return "", err
		}
		if label != "" && ctx.inputFieldSectionLabel {
			labelPrefix = `<label for="` + html.EscapeString(id) + `">` + html.EscapeString(label) + `</label>`
		}
	} else if ctx.inputFieldSectionLabel {
		if binding, ok := resolveFieldBinding(ctx, node.Attribute("value")); ok {
			if label := strings.TrimSpace(binding.Field.Label); label != "" {
				labelPrefix = `<label for="` + html.EscapeString(id) + `">` + html.EscapeString(label) + `</label>`
			}
		}
	}
	requiredValue, err := RenderExpressionTemplate(node.Attribute("required"), ctx.Expression)
	if err != nil {
		return "", err
	}
	required := truthyExpressionValue(requiredValue)
	var rendered string
	if input, ok := renderFieldInput(ctx, node.Attribute("value"), id, required); ok {
		rendered = input
	} else {
		value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
		if err != nil {
			return "", err
		}
		idAttr := fieldInputIDAttr(id)
		rendered = `<input type="text" class="inputField" name="` + html.EscapeString(name) + `"` + idAttr + ` value="` + html.EscapeString(value) + `"` + fieldInputRequiredAttr(required) + ` />`
	}
	list := strings.TrimSpace(node.Attribute("list"))
	if list == "" {
		return labelPrefix + rendered, nil
	}
	listIDBase := visualforceComponentClientID(node, ctx)
	if listIDBase == "" {
		listIDBase = id
	}
	listID := listIDBase + "-list"
	input, ok := appendInputDatalistReference(rendered, listID)
	if !ok {
		return labelPrefix + rendered, nil
	}
	options, err := inputFieldDatalistOptions(list, ctx)
	if err != nil {
		return "", err
	}
	return labelPrefix + input + renderInputDatalist(listID, options), nil
}

func inputFieldDatalistOptions(raw string, ctx *RenderContext) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "{!") || !strings.HasSuffix(raw, "}") {
		return commaSeparatedDatalistValues(raw), nil
	}
	value, ok, err := evaluateRenderExpressionValue(raw, ctx)
	if err != nil || !ok {
		return nil, err
	}
	if value.Kind == vm.ValueString {
		return commaSeparatedDatalistValues(value.Text), nil
	}
	values := []vm.Value{value}
	if value.Kind == vm.ValueList {
		values = value.List
	}
	options := make([]string, 0, len(values))
	for _, item := range values {
		if item.Kind == vm.ValueObject {
			machine := ctx.VM
			if machine == nil && ctx.Expression != nil {
				machine = ctx.Expression.VM
			}
			if machine == nil || strings.TrimSpace(item.Type) == "" {
				return nil, vm.NewUnsupportedFeatureError("Visualforce inputField list object string conversion")
			}
			pageURL := ""
			if ctx != nil {
				pageURL = ctx.PageURL
			}
			converted, updated, result, err := machine.InvokeVisualforceActionOnController(item, item.Type, "toString", pageURL, nil)
			if err != nil {
				return nil, err
			}
			if result.Error != nil {
				return nil, vm.UnsupportedFeature(result.Error.Message)
			}
			if !result.Success {
				return nil, vm.NewUnsupportedFeatureError("Visualforce inputField list object string conversion")
			}
			if ctx.Expression != nil {
				writeBackVisualforceReceiver(ctx.Expression, item, updated)
			}
			item = converted
		}
		options = append(options, item.String())
	}
	return options, nil
}

func commaSeparatedDatalistValues(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		if value := strings.TrimSpace(part); value != "" {
			values = append(values, value)
		}
	}
	return values
}

func appendInputDatalistReference(rendered, listID string) (string, bool) {
	if strings.Count(rendered, "<input ") != 1 || !strings.HasSuffix(rendered, " />") {
		return rendered, false
	}
	end := len(rendered) - len(" />")
	return rendered[:end] + ` list="` + html.EscapeString(listID) + `"` + rendered[end:], true
}

func renderInputDatalist(id string, options []string) string {
	var rendered strings.Builder
	rendered.WriteString(`<datalist id="`)
	rendered.WriteString(html.EscapeString(id))
	rendered.WriteString(`">`)
	for _, option := range options {
		escaped := html.EscapeString(option)
		rendered.WriteString(`<option value="`)
		rendered.WriteString(escaped)
		rendered.WriteString(`">`)
		rendered.WriteString(escaped)
		rendered.WriteString(`</option>`)
	}
	rendered.WriteString(`</datalist>`)
	return rendered.String()
}

func inputFieldName(node *MarkupNode) string {
	if name := strings.TrimSpace(node.Attribute("id")); name != "" {
		return name
	}
	value := strings.TrimSpace(node.Attribute("value"))
	if strings.HasPrefix(value, "{!") && strings.HasSuffix(value, "}") {
		value = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(value, "{!"), "}"))
	}
	if value != "" {
		parts := strings.Split(value, ".")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return fieldName(node)
}

func renderApexCommandButton(node *MarkupNode, ctx *RenderContext) (string, error) {
	action := strings.TrimSpace(node.Attribute("action"))
	label, err := RenderExpressionTemplate(firstNonEmpty(node.Attribute("value"), node.Attribute("title")), ctx.Expression)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(label) == "" {
		label = action
	}
	if label == "" {
		label = "Submit"
	}
	attrs := ` type="submit"`
	if isTruthyExpression(node.Attribute("disabled"), ctx) {
		attrs += ` disabled="disabled"`
	}
	if id := visualforceExplicitComponentClientID(node, ctx); id != "" {
		attrs += ` id="` + html.EscapeString(id) + `" name="` + html.EscapeString(id) + `"`
	}
	script, marker, selectHook := repeatedCommandSubmission(node, ctx)
	attrs += marker
	attrs += ` value="` + html.EscapeString(label) + `" data-action="` + html.EscapeString(action) + `"`
	if rerender := strings.TrimSpace(node.Attribute("rerender")); rerender != "" {
		hook := selectHook + VisualforceAjaxSubmitHookWithStatus(action, rerender, strings.TrimSpace(node.Attribute("status")))
		return script + `<input` + attrs + ` onclick="` + html.EscapeString(hook) + `" />`, nil
	}
	hook := selectHook + `if(this.form&&this.form.elements['` + ViewStateActionFieldName() + `']){this.form.elements['` + ViewStateActionFieldName() + `'].value=` + jsStringLiteral(action) + `;}`
	return script + `<input` + attrs + ` onclick="` + html.EscapeString(hook) + `" />`, nil
}

func renderApexCommandLink(node *MarkupNode, ctx *RenderContext) (string, error) {
	action := strings.TrimSpace(node.Attribute("action"))
	label, err := RenderExpressionTemplate(firstNonEmpty(node.Attribute("value"), node.Attribute("title")), ctx.Expression)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(label) == "" {
		label = action
	}
	script, marker, selectHook := repeatedCommandSubmission(node, ctx)
	if rerender := strings.TrimSpace(node.Attribute("rerender")); rerender != "" {
		hook := selectHook + VisualforceAjaxLinkHookWithStatus(action, rerender, strings.TrimSpace(node.Attribute("status")))
		return script + `<a` + componentIDAttr(node, ctx) + marker + ` href="#" onclick="` + html.EscapeString(hook) + `">` + html.EscapeString(label) + `</a>`, nil
	}
	hook := selectHook + `var f=this.closest('form')||document.forms[0];if(f&&f.elements['` + ViewStateActionFieldName() + `']){f.elements['` + ViewStateActionFieldName() + `'].value=` + jsStringLiteral(action) + `;f.submit();}return false;`
	return script + `<a` + componentIDAttr(node, ctx) + marker + ` href="#" onclick="` + html.EscapeString(hook) + `">` + html.EscapeString(label) + `</a>`, nil
}

func renderApexSelectList(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := fieldName(node)
	selected, err := selectSelectedValues(node, ctx)
	if err != nil {
		return "", err
	}
	options, err := selectOptionNodes(node, ctx)
	if err != nil {
		return "", err
	}
	size := node.Attribute("size")
	if size == "" {
		size = fmt.Sprint(len(options))
	} else if size, err = RenderExpressionTemplate(size, ctx.Expression); err != nil {
		return "", err
	}
	var builder strings.Builder
	builder.WriteString(`<select name="` + html.EscapeString(name) + `"`)
	builder.WriteString(componentIDAttr(node, ctx))
	builder.WriteString(` size="` + html.EscapeString(size) + `"`)
	if isTruthyExpression(node.Attribute("multiselect"), ctx) {
		builder.WriteString(` multiple="multiple"`)
	}
	builder.WriteString(formBooleanAttrs(node, ctx))
	for _, attr := range []struct{ source, target string }{{"styleClass", "class"}, {"style", "style"}} {
		if raw := node.Attribute(attr.source); raw != "" {
			value, err := RenderExpressionTemplate(raw, ctx.Expression)
			if err != nil {
				return "", err
			}
			builder.WriteString(` ` + attr.target + `="` + html.EscapeString(value) + `"`)
		}
	}
	builder.WriteString(`>`)
	for _, option := range options {
		builder.WriteString(`<option value="` + html.EscapeString(option.value) + `"`)
		if selected[option.value] {
			builder.WriteString(` selected="selected"`)
		}
		if option.disabled {
			builder.WriteString(` disabled="disabled"`)
		}
		builder.WriteString(`>` + selectOptionLabelContent(option) + `</option>`)
	}
	builder.WriteString(`</select>`)
	if message := selectValidationMessage(node, ctx); message != "" {
		builder.WriteString(`<span class="selectionError">` + html.EscapeString(message) + `</span>`)
	}
	return builder.String(), nil
}

func renderApexRepeat(node *MarkupNode, ctx *RenderContext) (string, error) {
	items, err := repetitionItems(node, ctx)
	if err != nil {
		return "", err
	}
	varName := strings.TrimSpace(node.Attribute("var"))
	indexName := strings.TrimSpace(node.Attribute("indexvar"))
	builder := strings.Builder{}
	for i, item := range items {
		repeatID := strings.TrimSpace(node.Attribute("id"))
		if ctx.ComponentReferences != nil {
			repeatID = ctx.ComponentReferences.clientIDs[node]
		}
		ctx.repeatPath = append(ctx.repeatPath, repeatID+":"+strconv.Itoa(i))
		ctx.Scope.PushFrame()
		if varName != "" {
			ctx.Scope.Set(varName, item)
		}
		if indexName != "" {
			ctx.Scope.Set(indexName, vm.Int(int64(i)))
		}
		rendered, renderErr := renderRepetitionChildren(node, ctx, varName)
		ctx.Scope.PopFrame()
		ctx.repeatPath = ctx.repeatPath[:len(ctx.repeatPath)-1]
		if renderErr != nil {
			return "", renderErr
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func renderApexDataTable(node *MarkupNode, ctx *RenderContext, pageBlockStyle bool) (string, error) {
	rows, err := repetitionItems(node, ctx)
	if err != nil {
		return "", err
	}
	className := "dataTable"
	if pageBlockStyle {
		className = "list"
	}
	builder := strings.Builder{}
	builder.WriteString(`<table class="`)
	builder.WriteString(className)
	builder.WriteString(`"><thead><tr>`)
	columns, err := dataTableColumns(node, ctx, true)
	if err != nil {
		return "", err
	}
	for _, col := range columns {
		ctx.Scope.PushFrame()
		col.bind(ctx.Scope)
		header := dataTableColumnHeader(col.node, node, rows, ctx, pageBlockStyle, len(col.bindings) != 0)
		ctx.Scope.PopFrame()
		builder.WriteString(`<th colspan="1">`)
		builder.WriteString(html.EscapeString(header))
		builder.WriteString(`</th>`)
	}
	builder.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		// r_{dataTable,pageBlockTable}_null_members: null row objects are
		// absent rows, whereas empty String values still produce a cell.
		if row.Kind == vm.ValueNull {
			continue
		}
		ctx.Scope.PushFrame()
		varName := strings.TrimSpace(node.Attribute("var"))
		if varName != "" {
			ctx.Scope.Set(varName, row)
		}
		builder.WriteString(`<tr>`)
		for _, col := range columns {
			ctx.Scope.PushFrame()
			col.bind(ctx.Scope)
			cell, err := renderRepetitionColumn(col, ctx, varName)
			ctx.Scope.PopFrame()
			if err != nil {
				ctx.Scope.PopFrame()
				return "", err
			}
			builder.WriteString(`<td colspan="1">`)
			builder.WriteString(cell)
			builder.WriteString(`</td>`)
		}
		builder.WriteString(`</tr>`)
		ctx.Scope.PopFrame()
	}
	builder.WriteString(`</tbody></table>`)
	return builder.String(), nil
}

func renderApexPanelGrid(node *MarkupNode, ctx *RenderContext) (string, error) {
	columns := panelGridColumns(node.Attribute("columns"))
	cells := panelGridCellNodes(node)
	builder := strings.Builder{}
	attrs, err := presentationAttributes(node, ctx, false, true, "styleClass", "style", "title", "onclick")
	if err != nil {
		return "", err
	}
	builder.WriteString("<table" + attrs + ">\n")
	if err := renderPanelGridFacet(&builder, node, ctx, "caption", "caption", node.Attribute("captionClass"), columns); err != nil {
		return "", err
	}
	if err := renderPanelGridFacet(&builder, node, ctx, "header", "thead", node.Attribute("headerClass"), columns); err != nil {
		return "", err
	}
	if err := renderPanelGridFacet(&builder, node, ctx, "footer", "tfoot", node.Attribute("footerClass"), columns); err != nil {
		return "", err
	}
	builder.WriteString("<tbody>\n")
	for i, child := range cells {
		if i%columns == 0 {
			builder.WriteString("<tr>\n")
		}
		rendered, err := renderMarkupNode(child, ctx)
		if err != nil {
			return "", err
		}
		builder.WriteString(`<td>`)
		builder.WriteString(rendered)
		builder.WriteString("</td>\n")
		if i%columns == columns-1 {
			builder.WriteString("</tr>\n")
		}
	}
	if len(cells) > 0 && len(cells)%columns != 0 {
		builder.WriteString("</tr>\n")
	}
	builder.WriteString("</tbody>\n</table>\n")
	return builder.String(), nil
}

func panelGridColumns(raw string) int {
	var columns int
	if _, err := fmt.Sscanf(strings.TrimSpace(raw), "%d", &columns); err != nil || columns < 1 {
		return 1
	}
	return columns
}

func panelGridCellNodes(node *MarkupNode) []*MarkupNode {
	cells := make([]*MarkupNode, 0, len(node.Children))
	for _, child := range node.Children {
		if child.Type == MarkupNodeText && strings.TrimSpace(child.Text) == "" {
			continue
		}
		if child.Type == MarkupNodeElement && strings.EqualFold(child.Namespace, "apex") && strings.EqualFold(child.Name, "facet") {
			continue
		}
		cells = append(cells, child)
	}
	return cells
}

func renderPanelGridFacet(builder *strings.Builder, node *MarkupNode, ctx *RenderContext, name, sectionTag, className string, columns int) error {
	rendered, err := renderNamedFacet(node, ctx, name)
	if err != nil || rendered == "" {
		return err
	}
	classAttr := ""
	if strings.TrimSpace(className) != "" {
		classAttr = ` class="` + html.EscapeString(strings.TrimSpace(className)) + `"`
	}
	switch sectionTag {
	case "caption":
		builder.WriteString(`<caption`)
		builder.WriteString(classAttr)
		builder.WriteString(`>`)
		builder.WriteString(rendered)
		builder.WriteString(`</caption>`)
	case "thead":
		builder.WriteString("<thead>\n<tr><th")
		builder.WriteString(classAttr)
		builder.WriteString(` colspan="`)
		builder.WriteString(fmt.Sprintf("%d", columns))
		builder.WriteString(`" scope="colgroup">`)
		builder.WriteString(rendered)
		builder.WriteString("</th></tr>\n</thead>\n")
	case "tfoot":
		builder.WriteString("<tfoot>\n<tr><td")
		builder.WriteString(classAttr)
		builder.WriteString(` colspan="`)
		builder.WriteString(fmt.Sprintf("%d", columns))
		builder.WriteString(`">`)
		builder.WriteString(rendered)
		builder.WriteString("</td></tr>\n</tfoot>\n")
	}
	return nil
}

func renderApexPageBlockTable(node *MarkupNode, ctx *RenderContext) (string, error) {
	table, err := renderApexDataTable(node, ctx, true)
	if err != nil {
		return "", err
	}
	return `<div class="pbBody">` + table + `</div>`, nil
}

func renderApexDataList(node *MarkupNode, ctx *RenderContext) (string, error) {
	rows, err := repetitionItems(node, ctx)
	if err != nil {
		return "", err
	}
	builder := strings.Builder{}
	builder.WriteString(`<ul class="dataList">`)
	varName := strings.TrimSpace(node.Attribute("var"))
	for _, row := range rows {
		if row.Kind == vm.ValueNull {
			continue
		}
		ctx.Scope.WithFrame(func() {
			if varName != "" {
				ctx.Scope.Set(varName, row)
			}
			item, renderErr := renderRepetitionChildren(node, ctx, varName)
			if renderErr != nil {
				err = renderErr
				return
			}
			builder.WriteString(`<li>`)
			builder.WriteString(item)
			builder.WriteString(`</li>`)
		})
		if err != nil {
			return "", err
		}
	}
	builder.WriteString(`</ul>`)
	return builder.String(), nil
}

func renderApexDetail(node *MarkupNode, ctx *RenderContext) (string, error) {
	objectType := strings.TrimSpace(node.Attribute("subject"))
	if objectType == "" {
		objectType = "Record"
	}
	recordValue := ctx.Expression.Controller
	if ctx.Expression.StandardController.Kind == vm.ValueObject {
		if rec, ok := ctx.Expression.StandardController.Fields["record"]; ok {
			recordValue = rec
		}
	}
	builder := strings.Builder{}
	builder.WriteString(`<div class="detailBlock" data-object="`)
	builder.WriteString(html.EscapeString(objectType))
	builder.WriteString(`">`)
	if recordValue.Kind == vm.ValueObject {
		for field, value := range recordValue.Fields {
			builder.WriteString(`<div class="detailRow"><span class="label">`)
			builder.WriteString(html.EscapeString(field))
			builder.WriteString(`</span><span class="value">`)
			builder.WriteString(html.EscapeString(value.String()))
			builder.WriteString(`</span></div>`)
		}
	}
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	builder.WriteString(children)
	builder.WriteString(`</div>`)
	return builder.String(), nil
}

func renderApexPageMessages(node *MarkupNode, ctx *RenderContext) (string, error) {
	if ctx.VM == nil {
		return "", nil
	}
	builder := strings.Builder{}
	builder.WriteString(`<div class="pageMessages">`)
	for _, message := range ctx.VM.PageMessages() {
		// Messages associated with a component are not
		// part of the global pageMessages/messages component's queue.
		if target, ok := message.Fields["componentLabel"]; ok && target.Kind == vm.ValueString && target.Text != "" {
			continue
		}
		if ctx.formLifecycle != nil {
			builder.WriteString(renderFormPageMessage(message))
		} else {
			builder.WriteString(renderPageMessage(message))
		}
	}
	builder.WriteString(`</div>`)
	return builder.String(), nil
}

func renderApexPageMessage(node *MarkupNode, ctx *RenderContext) (string, error) {
	severity := strings.ToLower(strings.TrimSpace(node.Attribute("severity")))
	summary, err := RenderExpressionTemplate(node.Attribute("summary"), ctx.Expression)
	if err != nil {
		return "", err
	}
	detail, err := RenderExpressionTemplate(node.Attribute("detail"), ctx.Expression)
	if err != nil {
		return "", err
	}
	if summary != "" && detail != "" {
		summary = strings.TrimSpace(summary + " " + detail)
	} else if detail != "" {
		summary = detail
	}
	if summary == "" {
		summary, err = renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
	}
	return `<div class="message ` + html.EscapeString(severity) + `">` + html.EscapeString(summary) + `</div>`, nil
}

func renderApexMessage(node *MarkupNode, ctx *RenderContext) (string, error) {
	target := strings.TrimSpace(node.Attribute("for"))
	summary, err := RenderExpressionTemplate(firstNonEmpty(node.Attribute("summary"), node.Attribute("detail")), ctx.Expression)
	if err != nil {
		return "", err
	}
	if summary == "" {
		summary, err = renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
	}
	return `<div class="message" data-for="` + html.EscapeString(target) + `">` + html.EscapeString(summary) + `</div>`, nil
}

func renderPageMessage(message vm.Value) string {
	severity := "info"
	summary := message.String()
	if message.Kind == vm.ValueObject {
		if raw, ok := message.Fields["severity"]; ok {
			severity = strings.ToLower(raw.String())
		}
		if raw, ok := message.Fields["summary"]; ok {
			summary = raw.String()
		}
	}
	return `<div class="message ` + html.EscapeString(severity) + `">` + html.EscapeString(summary) + `</div>`
}

func renderApexActionSupport(node *MarkupNode, ctx *RenderContext) (string, error) {
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	target := strings.TrimSpace(node.Attribute("rerender"))
	if target == "" {
		return children, nil
	}
	action := strings.TrimSpace(node.Attribute("action"))
	event := strings.TrimSpace(node.Attribute("event"))
	if event == "" {
		event = "change"
	}
	hook := VisualforceAjaxLinkHookWithStatus(action, target, strings.TrimSpace(node.Attribute("status")))
	return `<span class="actionSupport" data-event="` + html.EscapeString(event) + `" data-rerender="` + html.EscapeString(target) + `" ` + html.EscapeString(visualforceEventAttributeName(event)) + `="` + html.EscapeString(hook) + `">` + children + `</span>`, nil
}

func visualforceEventAttributeName(event string) string {
	event = strings.ToLower(strings.TrimSpace(event))
	if event == "" {
		event = "change"
	}
	if strings.HasPrefix(event, "on") {
		return event
	}
	return "on" + event
}

func renderApexActionFunction(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := strings.TrimSpace(node.Attribute("name"))
	if name == "" {
		return "", nil
	}
	action := strings.TrimSpace(node.Attribute("action"))
	rerender := strings.TrimSpace(node.Attribute("rerender"))
	status := strings.TrimSpace(node.Attribute("status"))
	params, err := visualforceAjaxParams(node, ctx)
	if err != nil {
		return "", err
	}
	hook := VisualforceAjaxFunctionCall(action, rerender, status, params)
	return `<script data-action="` + html.EscapeString(action) + `" data-rerender="` + html.EscapeString(rerender) + `">function ` + html.EscapeString(name) + `(` + html.EscapeString(VisualforceAjaxFunctionArgs(params)) + `){` + hook + `}</script>`, nil
}

func renderApexActionRegion(node *MarkupNode, ctx *RenderContext) (string, error) {
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	region := strings.TrimSpace(node.Attribute("id"))
	return `<span class="actionRegion" data-region="` + html.EscapeString(region) + `" data-vf-region="` + html.EscapeString(region) + `">` + children + `</span>`, nil
}

func renderApexActionStatus(node *MarkupNode, ctx *RenderContext) (string, error) {
	statusID := strings.TrimSpace(node.Attribute("id"))
	start, err := renderNamedFacet(node, ctx, "start")
	if err != nil {
		return "", err
	}
	stop, err := renderNamedFacet(node, ctx, "stop")
	if err != nil {
		return "", err
	}
	if start == "" {
		start, err = RenderExpressionTemplate(node.Attribute("startText"), ctx.Expression)
		if err != nil {
			return "", err
		}
	}
	if stop == "" {
		stop, err = RenderExpressionTemplate(node.Attribute("stopText"), ctx.Expression)
		if err != nil {
			return "", err
		}
	}
	if start == "" && stop == "" {
		stop, err = renderChildren(node, ctx)
		if err != nil {
			return "", err
		}
	}
	return `<span class="actionStatus" data-status="` + html.EscapeString(statusID) + `"><span class="actionStatusStart" hidden="hidden">` + start + `</span><span class="actionStatusStop">` + stop + `</span></span>`, nil
}

func renderApexActionPoller(node *MarkupNode, _ *RenderContext) (string, error) {
	interval := normalizePollerInterval(node.Attribute("interval"))
	enabled := strings.TrimSpace(node.Attribute("enabled"))
	if enabled == "" {
		enabled = "true"
	}
	return `<span class="actionPoller" data-action="` + html.EscapeString(node.Attribute("action")) + `" data-rerender="` + html.EscapeString(node.Attribute("rerender")) + `" data-interval="` + html.EscapeString(interval) + `" data-enabled="` + html.EscapeString(strings.ToLower(enabled)) + `"></span>`, nil
}

func renderApexVariable(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := strings.TrimSpace(node.Attribute("var"))
	if name == "" {
		return "", nil
	}
	valueText, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	ctx.Scope.Set(name, vm.String(valueText))
	return "", nil
}

func renderApexStylesheet(node *MarkupNode, ctx *RenderContext) (string, error) {
	href, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	link := `<link rel="stylesheet" type="text/css" href="` + html.EscapeString(href) + `" />`
	if ctx.collectPresentationHead {
		ctx.presentationHead = append(ctx.presentationHead, link)
		return "", nil
	}
	return link, nil
}

func renderApexIncludeScript(node *MarkupNode, ctx *RenderContext) (string, error) {
	src, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	return `<script src="` + html.EscapeString(src) + `"></script>`, nil
}

func renderApexIncludeLightning(_ *MarkupNode, ctx *RenderContext) (string, error) {
	if ctx != nil {
		ctx.LightningOut = true
		if ctx.LightningBootstrap != nil {
			return lwcbrowser.BootstrapHTML(*ctx.LightningBootstrap), nil
		}
		if strings.EqualFold(strings.TrimSpace(ctx.PageName), "setup") {
			return lightningOutUnavailableNotice(), nil
		}
	}
	return "", nil
}

func lightningOutUnavailableNotice() string {
	return `<div class="glade-vf-lightning-notice" style="margin:1rem;padding:0.75rem 1rem;border:1px solid #c9c9c9;background:#fff8e6;font:14px/1.4 system-ui,sans-serif;">` +
		"Lightning Out is not available in local Visualforce preview. The page markup renders, but $Lightning components will not boot." +
		`</div>`
}

func renderApexSLDS(_ *MarkupNode, _ *RenderContext) (string, error) {
	return `<link rel="stylesheet" type="text/css" href="https://cdn.jsdelivr.net/npm/@salesforce-ux/design-system@2.25.3/assets/styles/salesforce-lightning-design-system.min.css" />`, nil
}

func renderHTMLPassthrough(node *MarkupNode, ctx *RenderContext) (string, error) {
	if node == nil {
		return "", nil
	}
	tag := strings.ToLower(strings.TrimSpace(node.Name))
	if tag == "" || tag == "_vfroot" || tag == "html" || tag == "head" || tag == "body" {
		return renderChildren(node, ctx)
	}
	attrs, err := renderHTMLAttributes(node, ctx)
	if err != nil {
		return "", err
	}
	children, err := renderHTMLPassthroughChildren(tag, node, ctx)
	if err != nil {
		return "", err
	}
	if isVoidHTMLElement(tag) {
		return "<" + tag + attrs + " />", nil
	}
	return "<" + tag + attrs + ">" + children + "</" + tag + ">", nil
}

func renderHTMLPassthroughChildren(tag string, node *MarkupNode, ctx *RenderContext) (string, error) {
	if isRawTextHTMLElement(tag) {
		return renderRawTextChildren(node, ctx)
	}
	return renderChildren(node, ctx)
}

func renderRawTextChildren(node *MarkupNode, ctx *RenderContext) (string, error) {
	builder := strings.Builder{}
	for _, child := range node.Children {
		if child.Type == MarkupNodeText {
			rendered, err := RenderVisualforceRawText(child.Text, ctx.Expression)
			if err != nil {
				return "", err
			}
			builder.WriteString(rendered)
			continue
		}
		rendered, err := renderMarkupNode(child, ctx)
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func renderHTMLAttributes(node *MarkupNode, ctx *RenderContext) (string, error) {
	if node == nil || len(node.Attributes) == 0 {
		return "", nil
	}
	builder := strings.Builder{}
	keys := make([]string, 0, len(node.Attributes))
	for key := range node.Attributes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := node.Attributes[key]
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		builder.WriteString(` `)
		builder.WriteString(key)
		if value != "" {
			// Attribute bindings use the same controller/extension and repeat
			// scope as text and component attributes. Escape after evaluation.
			value, err := RenderExpressionTemplate(value, ctx.Expression)
			if err != nil {
				return "", err
			}
			builder.WriteString(`="`)
			builder.WriteString(html.EscapeString(value))
			builder.WriteString(`"`)
		}
	}
	return builder.String(), nil
}

func isRawTextHTMLElement(tag string) bool {
	switch strings.ToLower(tag) {
	case "script", "style":
		return true
	default:
		return false
	}
}

func isVoidHTMLElement(tag string) bool {
	switch strings.ToLower(tag) {
	case "area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "param", "source", "track", "wbr":
		return true
	default:
		return false
	}
}

func renderApexImage(node *MarkupNode, ctx *RenderContext) (string, error) {
	src, err := RenderExpressionTemplate(firstNonEmpty(node.Attribute("value"), node.Attribute("url")), ctx.Expression)
	if err != nil {
		return "", err
	}
	alt, err := RenderExpressionTemplate(node.Attribute("alt"), ctx.Expression)
	if err != nil {
		return "", err
	}
	attrs, err := presentationAttributes(node, ctx, false, true, "width", "height", "title", "styleClass", "style")
	if err != nil {
		return "", err
	}
	return `<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) + `"` + attrs + ` />`, nil
}

func renderApexIframe(node *MarkupNode, ctx *RenderContext) (string, error) {
	src, err := RenderExpressionTemplate(firstNonEmpty(node.Attribute("src"), node.Attribute("value")), ctx.Expression)
	if err != nil {
		return "", err
	}
	attrs := strings.Builder{}
	attrs.WriteString(` src="`)
	attrs.WriteString(html.EscapeString(src))
	attrs.WriteString(`"`)
	for _, attr := range []string{"width", "height", "title", "scrolling", "frameborder"} {
		if raw := strings.TrimSpace(node.Attribute(attr)); raw != "" {
			attrs.WriteString(` `)
			attrs.WriteString(strings.ToLower(attr))
			attrs.WriteString(`="`)
			attrs.WriteString(html.EscapeString(raw))
			attrs.WriteString(`"`)
		}
	}
	return `<iframe` + attrs.String() + `></iframe>`, nil
}

func renderApexComposition(node *MarkupNode, ctx *RenderContext) (string, error) {
	templateName := strings.TrimSpace(node.Attribute("template"))
	if templateName == "" || ctx.VFIndex == nil {
		return renderChildren(node, ctx)
	}
	templatePage, ok := ctx.VFIndex.Page(templateName)
	if !ok {
		return renderChildren(node, ctx)
	}
	templateMarkup, err := os.ReadFile(templatePage.File)
	if err != nil {
		return "", err
	}
	templateTree, err := ParseMarkupTree(string(templateMarkup))
	if err != nil {
		return "", err
	}
	childCtx := *ctx
	childCtx.Defines = make(map[string]*MarkupNode)
	for _, child := range node.Children {
		if child.Type == MarkupNodeElement && child.Name == "define" {
			region := strings.TrimSpace(child.Attribute("name"))
			if region != "" {
				childCtx.Defines[region] = child
			}
		}
	}
	return RenderMarkupTree(templateTree, &childCtx)
}

func renderApexDefine(node *MarkupNode, ctx *RenderContext) (string, error) {
	region := strings.TrimSpace(node.Attribute("name"))
	if region != "" && ctx.Defines != nil {
		ctx.Defines[region] = node
	}
	return "", nil
}

func renderApexInsert(node *MarkupNode, ctx *RenderContext) (string, error) {
	region := strings.TrimSpace(node.Attribute("name"))
	if region == "" {
		return renderChildren(node, ctx)
	}
	if ctx.ComponentFacets != nil {
		if facet, ok := ctx.ComponentFacets[region]; ok {
			renderCtx := ctx
			if ctx.ComponentParent != nil {
				renderCtx = ctx.ComponentParent
			}
			return renderChildren(facet, renderCtx)
		}
	}
	if ctx.Defines != nil {
		if defined, ok := ctx.Defines[region]; ok {
			return renderChildren(defined, ctx)
		}
	}
	return "", nil
}

func renderApexInclude(node *MarkupNode, ctx *RenderContext) (string, error) {
	pageName := strings.TrimSpace(node.Attribute("pagename"))
	if pageName == "" {
		pageName = strings.TrimSpace(node.Attribute("pageName"))
	}
	if pageName == "" || ctx.VFIndex == nil {
		return "", nil
	}
	included, ok := ctx.VFIndex.Page(pageName)
	if !ok {
		return "", nil
	}
	markup, err := os.ReadFile(included.File)
	if err != nil {
		return "", err
	}
	tree, err := ParseMarkupTree(string(markup))
	if err != nil {
		return "", err
	}
	childCtx := *ctx
	childCtx.PageName = included.Name
	if err := applyIncludedPageController(included, &childCtx); err != nil {
		return "", err
	}
	return RenderMarkupTree(tree, &childCtx)
}

func renderApexDynamicComponent(node *MarkupNode, ctx *RenderContext) (string, error) {
	target := strings.TrimSpace(node.Attribute("componentvalue"))
	if target == "" {
		target = strings.TrimSpace(node.Attribute("value"))
	}
	return `<div class="dynamicComponentFallback">dynamic component unavailable: ` + html.EscapeString(target) + `</div>`, nil
}

func renderApexComponentBody(_ *MarkupNode, ctx *RenderContext) (string, error) {
	if ctx == nil || len(ctx.ComponentBody) == 0 {
		return "", nil
	}
	renderCtx := ctx
	if ctx.ComponentParent != nil {
		renderCtx = ctx.ComponentParent
	}
	builder := strings.Builder{}
	for _, child := range ctx.ComponentBody {
		var rendered string
		var err error
		if child.Type == MarkupNodeText {
			// Component body text has already been entity-decoded by the HTML
			// parser. Re-escape literal text before rendering its expressions.
			// Skip the shared renderer's literal pass after escaping it here;
			// expression output still uses its normal escaping.
			// Element children keep their own markup on the normal path.
			rendered, err = renderVisualforceText(escapeComponentBodyLiterals(child.Text), renderCtx.Expression, true, false)
		} else {
			rendered, err = renderMarkupNode(child, renderCtx)
		}
		if err != nil {
			return "", err
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func renderCustomComponent(node *MarkupNode, ctx *RenderContext) (string, error) {
	if ctx.VFIndex == nil {
		return renderChildren(node, ctx)
	}
	componentName := node.Name
	if node.Namespace != "" && !strings.EqualFold(node.Namespace, "c") {
		componentName = node.Namespace + "__" + node.Name
	}
	component, ok := ctx.VFIndex.Component(componentName)
	if !ok {
		component, ok = ctx.VFIndex.Component(node.Name)
	}
	if !ok {
		return `<div class="customComponentMissing">` + html.EscapeString(componentName) + `</div>`, nil
	}
	prepared, initialized := takePreparedCustomComponent(node)
	tree := prepared.tree
	if !initialized {
		markup, err := os.ReadFile(component.File)
		if err != nil {
			return "", err
		}
		tree, err = ParseMarkupTree(string(markup))
		if err != nil {
			return "", err
		}
	}
	childCtx := *ctx
	childCtx.ComponentAttrs = make(map[string]string)
	childCtx.ComponentFacets = componentFacets(node)
	childCtx.ComponentBody = componentBodyNodes(node)
	childCtx.ComponentParent = ctx
	componentValues, err := evaluatedTypedComponentAttributes(node, ctx, component.Attributes)
	if err != nil {
		return "", err
	}
	childCtx.componentValues = componentValues
	childCtx.componentTypes = make(map[string]string, len(component.Attributes))
	for _, attr := range component.Attributes {
		childCtx.componentTypes[strings.ToLower(attr.Name)] = attr.Type
	}
	for key, value := range componentValues {
		childCtx.ComponentAttrs[key] = visualforceFormulaText(value)
	}
	if err := validateRequiredComponentAttributes(component.Attributes, node.Attributes); err != nil {
		return "", err
	}
	variables := childCtx.ComponentAttrsToVariables()
	if len(variables) > 0 {
		expr := ExpressionContext{}
		if childCtx.Expression != nil {
			expr = *childCtx.Expression
		}
		expr.Variables = variables
		childCtx.Expression = &expr
	}
	if component.Controller != "" && ctx.VM != nil {
		controller, constructErr := prepared.controller, prepared.constructErr
		if !initialized {
			controller, constructErr = ctx.VM.ConstructController(component.Controller)
		}
		if constructErr != nil {
			if componentHasAssignedValue(component.Attributes, childCtx.ComponentAttrs) {
				return "", constructErr
			}
		} else {
			if err := applyComponentAssignTo(ctx.VM, &controller, component.Attributes, componentValues); err != nil {
				return "", contextualComponentSetterError(err, component.Name)
			}
			childCtx.Expression = &ExpressionContext{VM: ctx.VM, Controller: controller, Variables: variables}
		}
	} else if componentHasAssignedValue(component.Attributes, childCtx.ComponentAttrs) {
		return "", fmt.Errorf("component with assignTo attributes requires a controller and VM")
	}
	rendered, err := RenderMarkupTree(tree, &childCtx)
	if err != nil {
		return "", err
	}
	return "<span>" + rendered + "</span>", nil
}

func applyComponentAssignTo(machine *vm.VM, controller *vm.Value, attrs []Attribute, values map[string]vm.Value) error {
	if controller == nil || controller.Kind != vm.ValueObject {
		return fmt.Errorf("component assignTo requires an object controller")
	}
	for _, attr := range attrs {
		if strings.TrimSpace(attr.AssignTo) == "" {
			continue
		}
		value, supplied := typedComponentAttributeValue(values, attr.Name)
		if !supplied {
			continue
		}
		target, err := visualforceAssignmentTarget(attr.AssignTo)
		if err != nil {
			return fmt.Errorf("component attribute %s assignTo: %w", attr.Name, err)
		}
		targetType, ok, err := machine.InstancePropertyType(*controller, target)
		if err != nil {
			return fmt.Errorf("component attribute %s assignTo %s: %w", attr.Name, target, err)
		}
		if !ok {
			return fmt.Errorf("component attribute %s assignTo %s does not name a readable and writable controller property", attr.Name, target)
		}
		if strings.TrimSpace(attr.Type) != "" && !componentBindingTypesEqual(attr.Type, targetType) {
			return fmt.Errorf("component attribute %s type %s does not match controller property %s type %s", attr.Name, attr.Type, target, targetType)
		}
		*controller, err = machine.AssignInstanceProperty(*controller, target, value)
		if err != nil {
			return &componentSetterError{expression: attr.AssignTo, cause: err}
		}
	}
	return nil
}

func validateRequiredComponentAttributes(attrs []Attribute, values map[string]string) error {
	for _, attr := range attrs {
		if strings.EqualFold(strings.TrimSpace(attr.Required), "true") {
			if _, ok := componentAttributeValue(values, attr.Name); !ok {
				return fmt.Errorf("required component attribute %s was not supplied", attr.Name)
			}
		}
	}
	return nil
}

func componentHasAssignedValue(attrs []Attribute, values map[string]string) bool {
	for _, attr := range attrs {
		if strings.TrimSpace(attr.AssignTo) != "" {
			if _, ok := componentAttributeValue(values, attr.Name); ok {
				return true
			}
		}
	}
	return false
}

func componentAttributeValue(values map[string]string, name string) (string, bool) {
	for key, value := range values {
		if strings.EqualFold(strings.TrimSpace(key), strings.TrimSpace(name)) {
			return value, true
		}
	}
	return "", false
}

func expressionFieldName(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{!") && strings.HasSuffix(raw, "}") {
		raw = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "{!"), "}"))
	}
	if raw == "" {
		return ""
	}
	if strings.Contains(raw, ".") {
		parts := strings.Split(raw, ".")
		raw = strings.TrimSpace(parts[len(parts)-1])
	}
	return raw
}

func componentFacets(node *MarkupNode) map[string]*MarkupNode {
	out := make(map[string]*MarkupNode)
	if node == nil {
		return out
	}
	for _, child := range node.Children {
		if child.Type != MarkupNodeElement || child.Namespace != "apex" || child.Name != "facet" {
			continue
		}
		name := strings.TrimSpace(child.Attribute("name"))
		if name != "" {
			out[name] = child
		}
	}
	return out
}

func componentBodyNodes(node *MarkupNode) []*MarkupNode {
	if node == nil {
		return nil
	}
	out := make([]*MarkupNode, 0, len(node.Children))
	for _, child := range node.Children {
		if child.Type == MarkupNodeElement && child.Namespace == "apex" && child.Name == "facet" {
			continue
		}
		out = append(out, child)
	}
	return out
}

func (ctx *RenderContext) ComponentAttrsToVariables() map[string]vm.Value {
	out := make(map[string]vm.Value)
	if ctx.componentValues != nil {
		for key, value := range ctx.componentValues {
			out[key] = value
		}
		return out
	}
	for key, raw := range ctx.ComponentAttrs {
		out[key] = vm.String(raw)
	}
	return out
}

func columnNodes(node *MarkupNode) []*MarkupNode {
	out := make([]*MarkupNode, 0)
	for _, child := range node.Children {
		if child.Type == MarkupNodeElement && child.Name == "column" {
			out = append(out, child)
		}
	}
	return out
}

type dataTableBinding struct {
	name  string
	value vm.Value
}

type dataTableColumn struct {
	node     *MarkupNode
	bindings []dataTableBinding
}

func (col dataTableColumn) bind(scope *ScopeStack) {
	for _, binding := range col.bindings {
		scope.Set(binding.name, binding.value)
	}
}

func dataTableColumns(node *MarkupNode, ctx *RenderContext, allowRepeat bool) ([]dataTableColumn, error) {
	if !allowRepeat {
		direct := columnNodes(node)
		columns := make([]dataTableColumn, 0, len(direct))
		for _, col := range direct {
			columns = append(columns, dataTableColumn{node: col})
		}
		return columns, nil
	}
	var columns []dataTableColumn
	var visit func(*MarkupNode, []dataTableBinding) error
	visit = func(parent *MarkupNode, bindings []dataTableBinding) error {
		for _, child := range parent.Children {
			if child.Type != MarkupNodeElement {
				continue
			}
			switch {
			case child.Name == "column":
				columns = append(columns, dataTableColumn{node: child, bindings: append([]dataTableBinding(nil), bindings...)})
			case child.Namespace == "apex" && child.Name == "repeat":
				shouldRender, err := visualforceComponentShouldRender(child, ctx)
				if err != nil {
					return err
				}
				if !shouldRender {
					// Native direct table generators are constructed before
					// rendered is applied (r_generated_*_hidden_exception and
					// hidden_missing). Retain the existing nested-repeat guard
					// and lenient malformed-fragment behavior; those routes are
					// not represented by the captured direct-generator controls.
					if len(bindings) != 0 || !repetitionExpressionValid(child.Attribute("value")) {
						continue
					}
				}
				items, err := repetitionItems(child, ctx)
				if err != nil {
					return err
				}
				varName := strings.TrimSpace(child.Attribute("var"))
				indexName := strings.TrimSpace(child.Attribute("indexvar"))
				for i, item := range items {
					if item.Kind == vm.ValueNull {
						continue
					}
					ctx.Scope.PushFrame()
					next := append([]dataTableBinding(nil), bindings...)
					if varName != "" {
						ctx.Scope.Set(varName, item)
						next = append(next, dataTableBinding{name: varName, value: item})
					}
					if indexName != "" {
						index := vm.Int(int64(i))
						ctx.Scope.Set(indexName, index)
						next = append(next, dataTableBinding{name: indexName, value: index})
					}
					err := visit(child, next)
					ctx.Scope.PopFrame()
					if err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	if err := visit(node, nil); err != nil {
		return nil, err
	}
	return columns, nil
}

func dataTableColumnHeader(col, table *MarkupNode, rows []vm.Value, ctx *RenderContext, deriveDefault, generated bool) string {
	headerValue := col.Attribute("headerValue")
	if generated {
		headerValue = repetitionAttributeTemplate(col, "headerValue")
	}
	header := firstNonEmpty(headerValue, col.Attribute("header"), col.Attribute("title"))
	if header != "" {
		rendered, err := RenderExpressionTemplate(header, ctx.Expression)
		if err == nil {
			return rendered
		}
		return header
	}
	if !deriveDefault {
		return ""
	}
	return fieldLabelFromColumnValue(col.Attribute("value"), table.Attribute("var"), rows)
}

func fieldLabelFromColumnValue(raw, varName string, rows []vm.Value) string {
	root, field, ok := splitFieldExpression(raw)
	if !ok {
		return ""
	}
	objectType := ""
	if strings.EqualFold(root, strings.TrimSpace(varName)) && len(rows) > 0 && rows[0].Kind == vm.ValueObject {
		objectType = rows[0].Type
	}
	return displayFieldLabel(objectType, field)
}

func splitFieldExpression(raw string) (string, string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "{!") && strings.HasSuffix(raw, "}") {
		raw = strings.TrimSpace(raw[2 : len(raw)-1])
	}
	parts := strings.Split(raw, ".")
	if len(parts) < 2 {
		return "", "", false
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[len(parts)-1]), true
}

func displayFieldLabel(objectType, field string) string {
	fieldLabel := humanizeIdentifier(field)
	if strings.EqualFold(field, "Name") {
		if objectLabel := humanizeIdentifier(objectType); objectLabel != "" {
			return objectLabel + " " + fieldLabel
		}
	}
	return fieldLabel
}

func humanizeIdentifier(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimSuffix(raw, "__c")
	raw = strings.TrimSuffix(raw, "__r")
	raw = strings.ReplaceAll(raw, "_", " ")
	if raw == "" {
		return ""
	}
	builder := strings.Builder{}
	var prev byte
	for i := 0; i < len(raw); i++ {
		ch := raw[i]
		if i > 0 && isUpperASCII(ch) && (isLowerASCII(prev) || isDigit(prev)) {
			builder.WriteByte(' ')
		}
		builder.WriteByte(ch)
		prev = ch
	}
	return strings.TrimSpace(builder.String())
}

func isUpperASCII(ch byte) bool {
	return ch >= 'A' && ch <= 'Z'
}

func isLowerASCII(ch byte) bool {
	return ch >= 'a' && ch <= 'z'
}

func evaluateListExpression(raw string, ctx *RenderContext) ([]vm.Value, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	exprText := raw
	if strings.HasPrefix(exprText, "{!") && strings.HasSuffix(exprText, "}") {
		exprText = strings.TrimSpace(exprText[2 : len(exprText)-1])
	}
	expr, err := parseExpression(exprText)
	if err != nil {
		return nil, err
	}
	value, err := evaluateExpressionNode(expr, ctx.Expression)
	if err != nil {
		return nil, err
	}
	if value.Kind == vm.ValueNull {
		return nil, nil
	}
	if value.Kind == vm.ValueList {
		return value.List, nil
	}
	return []vm.Value{value}, nil
}

type selectOptionRender struct {
	value       string
	label       string
	disabled    bool
	escapeLabel bool
}

func renderApexSelectInputs(node *MarkupNode, ctx *RenderContext, inputType, className string) (string, error) {
	name := fieldName(node)
	selected, err := selectSelectedValues(node, ctx)
	if err != nil {
		return "", err
	}
	options, err := selectOptionNodes(node, ctx)
	if err != nil {
		return "", err
	}
	builder := strings.Builder{}
	builder.WriteString(`<span`)
	builder.WriteString(componentIDAttr(node, ctx))
	builder.WriteString(` class="`)
	builder.WriteString(className)
	builder.WriteString(`">`)
	for _, option := range options {
		checked := ""
		if selected[option.value] {
			checked = ` checked="checked"`
		}
		builder.WriteString(`<input type="`)
		builder.WriteString(inputType)
		builder.WriteString(`" name="`)
		builder.WriteString(html.EscapeString(name))
		builder.WriteString(`" value="`)
		builder.WriteString(html.EscapeString(option.value))
		builder.WriteString(`"`)
		builder.WriteString(checked)
		if option.disabled || isTruthyExpression(node.Attribute("disabled"), ctx) {
			builder.WriteString(` disabled="disabled"`)
		}
		builder.WriteString(` />`)
		builder.WriteString(`<label> `)
		builder.WriteString(selectOptionLabelContent(option))
		builder.WriteString(`</label>`)
	}
	builder.WriteString(`</span>`)
	return builder.String(), nil
}

func selectOptionNodes(node *MarkupNode, ctx *RenderContext) ([]selectOptionRender, error) {
	options := make([]selectOptionRender, 0)
	for _, child := range node.Children {
		if child.Type != MarkupNodeElement || !strings.EqualFold(child.Namespace, "apex") {
			continue
		}
		switch {
		case strings.EqualFold(child.Name, "selectOption"):
			value, err := RenderExpressionTemplate(firstNonEmpty(child.Attribute("itemValue"), child.Attribute("value")), ctx.Expression)
			if err != nil {
				return nil, err
			}
			label, err := RenderExpressionTemplate(firstNonEmpty(child.Attribute("itemLabel"), child.Attribute("label"), value), ctx.Expression)
			if err != nil {
				return nil, err
			}
			options = append(options, selectOptionRender{value: value, label: label, escapeLabel: true})
		case strings.EqualFold(child.Name, "selectOptions"):
			value, ok, err := evaluateRenderExpressionValue(child.Attribute("value"), ctx)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			if value.Kind == vm.ValueNull {
				options = append(options, selectOptionRender{escapeLabel: true})
			} else {
				options = append(options, selectOptionsFromValue(value)...)
			}
		}
	}
	return options, nil
}

func evaluateRenderExpressionValue(raw string, ctx *RenderContext) (vm.Value, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return vm.Null, false, nil
	}
	if strings.HasPrefix(raw, "{!") && strings.HasSuffix(raw, "}") {
		raw = strings.TrimSpace(raw[2 : len(raw)-1])
	}
	expr, err := parseExpression(raw)
	if err != nil {
		return vm.Null, false, err
	}
	value, err := evaluateExpressionNode(expr, ctx.Expression)
	if err != nil {
		return vm.Null, true, err
	}
	return value, true, nil
}

func selectOptionsFromValue(value vm.Value) []selectOptionRender {
	switch value.Kind {
	case vm.ValueList:
		options := make([]selectOptionRender, 0, len(value.List))
		for _, item := range value.List {
			options = append(options, selectOptionsFromValue(item)...)
		}
		return options
	case vm.ValueSet:
		options := make([]selectOptionRender, 0, len(value.Set))
		for _, item := range value.Set {
			options = append(options, selectOptionsFromValue(item)...)
		}
		return options
	case vm.ValueObject:
		if strings.EqualFold(value.Type, "SelectOption") {
			optionValue, _ := selectOptionField(value, "value")
			label, ok := selectOptionField(value, "label")
			if !ok {
				label = optionValue
			}
			return []selectOptionRender{{
				value:       optionValue,
				label:       label,
				disabled:    selectOptionBooleanField(value, "disabled", false),
				escapeLabel: selectOptionBooleanField(value, "escapeItem", true),
			}}
		}
	}
	text := value.String()
	if text == "" || value.Kind == vm.ValueNull {
		return nil
	}
	return []selectOptionRender{{value: text, label: text, escapeLabel: true}}
}

func selectOptionField(option vm.Value, field string) (string, bool) {
	value, ok := objectFieldIgnoreCase(option, field)
	if !ok || value.Kind == vm.ValueNull {
		return "", false
	}
	return value.String(), true
}

func selectOptionBooleanField(option vm.Value, field string, fallback bool) bool {
	value, ok := objectFieldIgnoreCase(option, field)
	if !ok || value.Kind != vm.ValueBool {
		return fallback
	}
	return value.Bool
}

func selectOptionLabelContent(option selectOptionRender) string {
	if option.escapeLabel {
		return html.EscapeString(option.label)
	}
	return option.label
}

func applyIncludedPageController(page Page, ctx *RenderContext) error {
	if ctx == nil || ctx.VM == nil || strings.TrimSpace(page.Controller) == "" {
		return nil
	}
	controller, err := ctx.VM.ConstructController(page.Controller)
	if err != nil {
		return err
	}
	expr := ExpressionContext{}
	if ctx.Expression != nil {
		expr = *ctx.Expression
	}
	expr.VM = ctx.VM
	expr.Controller = controller
	expr.Scope = ctx.Scope
	ctx.Expression = &expr
	return nil
}

func selectedValueSet(raw string, ctx *RenderContext) (map[string]bool, error) {
	selected := make(map[string]bool)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return selected, nil
	}
	exprText := raw
	if strings.HasPrefix(exprText, "{!") && strings.HasSuffix(exprText, "}") {
		exprText = strings.TrimSpace(exprText[2 : len(exprText)-1])
	}
	expr, err := parseExpression(exprText)
	if err == nil {
		if global := unsupportedVisualforceGlobal(expr); global != "" {
			return nil, vm.NewUnsupportedFeatureError(fmt.Sprintf("%s: unsupported Visualforce global", global))
		}
		value, evalErr := evaluateExpressionNode(expr, ctx.Expression)
		if evalErr != nil {
			return nil, evalErr
		}
		addSelectedValue(selected, value)
		return selected, nil
	}
	rendered, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err != nil {
		return nil, err
	}
	if rendered != "" {
		selected[rendered] = true
	}
	return selected, nil
}

func addSelectedValue(selected map[string]bool, value vm.Value) {
	switch value.Kind {
	case vm.ValueList:
		for _, item := range value.List {
			addSelectedValue(selected, item)
		}
	case vm.ValueSet:
		for _, item := range value.Set {
			addSelectedValue(selected, item)
		}
	case vm.ValueNull:
		return
	default:
		selected[value.String()] = true
	}
}

func renderNamedFacet(node *MarkupNode, ctx *RenderContext, name string) (string, error) {
	for _, child := range node.Children {
		if child.Type != MarkupNodeElement || !strings.EqualFold(child.Namespace, "apex") || !strings.EqualFold(child.Name, "facet") {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(child.Attribute("name")), name) {
			return renderChildren(child, ctx)
		}
	}
	return "", nil
}

func normalizePollerInterval(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "60"
	}
	var interval int
	if _, err := fmt.Sscanf(raw, "%d", &interval); err != nil {
		return raw
	}
	if interval < 5 {
		interval = 5
	}
	return fmt.Sprintf("%d", interval)
}

func isTruthyExpression(raw string, ctx *RenderContext) bool {
	value, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err != nil {
		return false
	}
	return truthyExpressionValue(value)
}

func truthyExpressionValue(value string) bool {
	value = strings.TrimSpace(value)
	return value == "true" || value == "1" || strings.EqualFold(value, "on")
}

func fieldName(node *MarkupNode) string {
	if node == nil {
		return ""
	}
	if id := strings.TrimSpace(node.Attribute("id")); id != "" {
		return id
	}
	return strings.TrimSpace(node.Attribute("value"))
}

func componentIDAttr(node *MarkupNode, ctx *RenderContext) string {
	id := visualforceExplicitComponentClientID(node, ctx)
	if id == "" {
		return ""
	}
	return ` id="` + html.EscapeString(id) + `" data-rerender="` + html.EscapeString(id) + `"`
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// ReadStaticResource reads a project static resource without exposing a path
// that callers could read outside the project root.
func ReadStaticResource(projectRoot, resourceName, subpath string) ([]byte, string, error) {
	if err := ValidateStaticResourceName(resourceName); err != nil {
		return nil, "", err
	}
	normalizedSubpath, err := NormalizeStaticResourceSubpath(subpath)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrStaticResourceNotFound, err)
	}
	root, err := os.OpenRoot(projectRoot)
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	for _, directory := range []string{
		"force-app/main/default/staticresources",
		"force-app/main/staticresources",
	} {
		resourceRoot, err := root.OpenRoot(filepath.Join(directory, resourceName))
		if err != nil {
			if staticResourceCandidateAbsent(err) {
				continue
			}
			return nil, "", err
		}
		content, filename, readErr := readStaticResourceBundle(resourceRoot, normalizedSubpath)
		resourceRoot.Close()
		return content, filename, readErr
	}
	singleCandidates := []string{
		filepath.Join("force-app/main/default/staticresources", resourceName+".resource"),
		filepath.Join("force-app/main/staticresources", resourceName+".resource"),
	}
	var missingZipEntry bool
	for _, candidate := range singleCandidates {
		content, filename, err := readStaticResourceContent(root, candidate, normalizedSubpath)
		if err == nil {
			return content, filename, nil
		}
		if errors.Is(err, errStaticResourceZipEntryMissing) {
			missingZipEntry = true
		}
		if !staticResourceCandidateAbsent(err) {
			return nil, "", err
		}
	}
	if missingZipEntry {
		return nil, "", fmt.Errorf("%w: entry %q not found in %s.resource", ErrStaticResourceNotFound, normalizedSubpath, resourceName)
	}
	return nil, "", ErrStaticResourceNotFound
}

// ReadStaticResourceContentPath reads an absolute metadata content path using
// its parent as a confined root.
func ReadStaticResourceContentPath(contentPath, resourceName, subpath string) ([]byte, string, error) {
	if err := ValidateStaticResourceName(resourceName); err != nil {
		return nil, "", err
	}
	normalizedSubpath, err := NormalizeStaticResourceSubpath(subpath)
	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", ErrStaticResourceNotFound, err)
	}
	root, err := os.OpenRoot(filepath.Dir(contentPath))
	if err != nil {
		return nil, "", err
	}
	defer root.Close()
	return readStaticResourceContent(root, filepath.Base(contentPath), normalizedSubpath)
}

var (
	// ErrStaticResourceNotFound reports a missing resource or bundle entry.
	ErrStaticResourceNotFound        = errors.New("static resource not found")
	errStaticResourceZipEntryMissing = errors.New("static resource zip entry missing")
)

// ValidateStaticResourceName accepts one non-empty path-safe name segment.
func ValidateStaticResourceName(resourceName string) error {
	if resourceName == "" || resourceName == "." || resourceName == ".." || strings.ContainsAny(resourceName, "/\\\x00") {
		return fmt.Errorf("invalid static resource name")
	}
	return nil
}

func staticResourceCandidateAbsent(err error) bool {
	return errors.Is(err, ErrStaticResourceNotFound) || errors.Is(err, os.ErrNotExist) || errors.Is(err, syscall.ENOTDIR)
}

// NormalizeStaticResourceSubpath validates a relative resource path and
// returns its slash-normalized form.
func NormalizeStaticResourceSubpath(subpath string) (string, error) {
	subpath = strings.ReplaceAll(filepath.ToSlash(subpath), "\\", "/")
	subpath = strings.TrimPrefix(subpath, "/")
	if subpath == "" {
		return "", nil
	}
	if strings.Contains(subpath, "\x00") || strings.Contains(subpath, "..") {
		return "", fmt.Errorf("invalid static resource path")
	}
	for _, part := range strings.Split(subpath, "/") {
		if part == "" || part == "." || part == ".." {
			return "", fmt.Errorf("invalid static resource path")
		}
	}
	clean := strings.TrimPrefix(path.Clean("/"+subpath), "/")
	if clean == "." {
		return "", nil
	}
	return clean, nil
}

func readStaticResourceBundle(root *os.Root, subpath string) ([]byte, string, error) {
	if subpath == "" {
		return nil, "", fmt.Errorf("static resource bundle requires subpath")
	}
	content, err := root.ReadFile(filepath.FromSlash(subpath))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("%w: %v", ErrStaticResourceNotFound, err)
		}
		return nil, "", err
	}
	return content, subpath, nil
}

func readStaticResourceContent(root *os.Root, contentName, subpath string) ([]byte, string, error) {
	info, err := root.Stat(contentName)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, "", fmt.Errorf("%w: %v", ErrStaticResourceNotFound, err)
		}
		return nil, "", err
	}
	if info.IsDir() {
		bundle, err := root.OpenRoot(contentName)
		if err != nil {
			return nil, "", err
		}
		defer bundle.Close()
		return readStaticResourceBundle(bundle, subpath)
	}
	if subpath == "" {
		content, err := root.ReadFile(contentName)
		if err != nil {
			return nil, "", err
		}
		return content, filepath.Base(contentName), nil
	}
	return readZippedStaticResource(root, contentName, subpath)
}

func readZippedStaticResource(root *os.Root, zipName, subpath string) ([]byte, string, error) {
	file, err := root.Open(zipName)
	if err != nil {
		return nil, "", err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, "", err
	}
	reader, err := zip.NewReader(file, info.Size())
	if err != nil {
		return nil, "", err
	}
	for _, entry := range reader.File {
		entryName, err := NormalizeStaticResourceSubpath(entry.Name)
		if err != nil || entryName != subpath || entry.FileInfo().IsDir() {
			continue
		}
		content, err := readStaticResourceZipEntry(entry)
		if err != nil {
			return nil, "", err
		}
		return content, entryName, nil
	}
	return nil, "", fmt.Errorf("%w: %w", ErrStaticResourceNotFound, errStaticResourceZipEntryMissing)
}

func readStaticResourceZipEntry(entry *zip.File) ([]byte, error) {
	source, err := entry.Open()
	if err != nil {
		return nil, err
	}
	defer source.Close()
	return io.ReadAll(source)
}
