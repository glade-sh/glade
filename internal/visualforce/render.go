package visualforce

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

type RenderContext struct {
	VM         *vm.VM
	PageName   string
	PageMeta   Page
	VFIndex    *Index
	Project    project.Project
	Expression *ExpressionContext
	Scope      *ScopeStack
	Defines    map[string]*MarkupNode
	ComponentAttrs map[string]string
	Metrics    *RenderMetrics
	Debug              bool
	LightningOut       bool
	LightningBootstrap *lwcbrowser.PageConfig
}

func RenderMarkupTree(node *MarkupNode, ctx *RenderContext) (string, error) {
	if node == nil {
		return "", nil
	}
	ctx.ensureExpression()
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
	if ctx.Expression.Scope == nil {
		ctx.Expression.Scope = ctx.Scope
	}
	if ctx.Scope == nil {
		ctx.Scope = NewScopeStack()
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
	switch node.Type {
	case MarkupNodeText:
		if ctx.Metrics != nil {
			ctx.Metrics.ExpressionEvals++
		}
		return RenderExpressionTemplate(node.Text, ctx.Expression)
	case MarkupNodeElement:
		return renderElement(node, ctx)
	default:
		return "", nil
	}
}

func renderElement(node *MarkupNode, ctx *RenderContext) (string, error) {
	if node == nil {
		return "", nil
	}
	component := strings.TrimSpace(node.Name)
	namespace := strings.ToLower(strings.TrimSpace(node.Namespace))
	ctx.countComponent(namespace + ":" + component)
	if namespace == "apex" {
		switch component {
		case "page":
			return renderApexPage(node, ctx)
		case "outputtext", "outputfield":
			return renderApexOutput(node, ctx, component == "outputfield")
		case "outputpanel":
			return renderApexContainer(node, "div", "outputPanel", ctx)
		case "outputlabel":
			return renderApexOutputLabel(node, ctx)
		case "pageblock":
			return renderApexContainer(node, "div", "bPageBlock", ctx)
		case "pageblocksection":
			return renderApexContainer(node, "div", "pbSubsection", ctx)
		case "pageblocksectionitem":
			return renderApexContainer(node, "div", "pbSectionItem", ctx)
		case "pageblockbuttons":
			return renderApexContainer(node, "div", "pbButtonbar", ctx)
		case "pageblocktable":
			return renderApexPageBlockTable(node, ctx)
		case "pagemessages":
			return renderApexPageMessages(node, ctx)
		case "pagemessage":
			return renderApexPageMessage(node, ctx)
		case "outputlink":
			return renderApexLink(node, ctx)
		case "form":
			return renderApexForm(node, ctx)
		case "inputtext":
			return renderApexInputText(node, ctx, "text")
		case "inputhidden":
			return renderApexInputText(node, ctx, "hidden")
		case "inputcheckbox":
			return renderApexInputCheckbox(node, ctx)
		case "inputfield":
			return renderApexInputField(node, ctx)
		case "commandbutton":
			return renderApexCommandButton(node, ctx)
		case "commandlink":
			return renderApexCommandLink(node, ctx)
		case "selectlist":
			return renderApexSelectList(node, ctx)
		case "selectoption", "selectoptions":
			return renderChildren(node, ctx)
		case "repeat":
			return renderApexRepeat(node, ctx)
		case "datatable":
			return renderApexDataTable(node, ctx, false)
		case "datalist":
			return renderApexDataList(node, ctx)
		case "column":
			return renderChildren(node, ctx)
		case "detail":
			return renderApexDetail(node, ctx)
		case "relatedlist", "enhancedlist":
			return renderApexContainer(node, "div", component, ctx)
		case "actionsupport":
			return renderApexActionSupport(node, ctx)
		case "variable":
			return renderApexVariable(node, ctx)
		case "attribute":
			return "", nil
		case "stylesheet":
			return renderApexStylesheet(node, ctx)
		case "includescript":
			return renderApexIncludeScript(node, ctx)
		case "includelightning":
			return renderApexIncludeLightning(node, ctx)
		case "slds":
			return renderApexSLDS(node, ctx)
		case "image":
			return renderApexImage(node, ctx)
		case "composition":
			return renderApexComposition(node, ctx)
		case "define":
			return renderApexDefine(node, ctx)
		case "insert":
			return renderApexInsert(node, ctx)
		case "include":
			return renderApexInclude(node, ctx)
		case "component":
			return renderChildren(node, ctx)
		case "dynamiccomponent":
			return renderApexDynamicComponent(node, ctx)
		case "param":
			return "", nil
		}
	}
	if namespace == "c" || (namespace != "" && namespace != "apex") {
		return renderCustomComponent(node, ctx)
	}
	return renderHTMLPassthrough(node, ctx)
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
	return "<!DOCTYPE html><html><head>" + head + "</head><body>" + body + "</body></html>", nil
}

func renderApexOutput(node *MarkupNode, ctx *RenderContext, outputField bool) (string, error) {
	raw, hasValue := node.Attributes["value"]
	if outputField && !hasValue {
		raw = ""
	}
	value := raw
	if hasValue {
		rendered, err := RenderExpressionTemplate(raw, ctx.Expression)
		if err != nil {
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
	return "<span>" + html.EscapeString(value) + "</span>", nil
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
	return "<label class=\"vfLabel\">" + html.EscapeString(value) + "</label>", nil
}

func renderApexContainer(node *MarkupNode, tag string, className string, ctx *RenderContext) (string, error) {
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	idAttr := componentIDAttr(node)
	if className == "" {
		return "<" + tag + idAttr + ">" + children + "</" + tag + ">", nil
	}
	return "<" + tag + idAttr + " class=\"" + className + "\">" + children + "</" + tag + ">", nil
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
	return "<a href=\"" + html.EscapeString(renderedHref) + "\">" + child + "</a>", nil
}

func renderApexForm(node *MarkupNode, ctx *RenderContext) (string, error) {
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	action := "/apex/"
	if ctx != nil && strings.TrimSpace(ctx.PageName) != "" {
		action += ctx.PageName
	}
	return "<form method=\"post\" action=\"" + html.EscapeString(action) + "\">" + children + "</form>", nil
}

func renderApexInputText(node *MarkupNode, ctx *RenderContext, inputType string) (string, error) {
	name := fieldName(node)
	value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	return `<input type="` + html.EscapeString(inputType) + `" name="` + html.EscapeString(name) + `" value="` + html.EscapeString(value) + `" />`, nil
}

func renderApexInputCheckbox(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := fieldName(node)
	checked := ""
	if isTruthyExpression(node.Attribute("selected"), ctx) {
		checked = ` checked="checked"`
	}
	return `<input type="checkbox" name="` + html.EscapeString(name) + `" value="true"` + checked + " />", nil
}

func renderApexInputField(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := fieldName(node)
	value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	return `<input type="text" class="inputField" name="` + html.EscapeString(name) + `" value="` + html.EscapeString(value) + `" />`, nil
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
	return `<button type="submit" name="` + ViewStateActionFieldName() + `" value="` + html.EscapeString(action) + `">` + html.EscapeString(label) + `</button>`, nil
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
	return `<a href="#" onclick="document.forms[0].elements['` + ViewStateActionFieldName() + `'].value='` + html.EscapeString(action) + `';document.forms[0].submit();return false;">` + html.EscapeString(label) + `</a>`, nil
}

func renderApexSelectList(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := fieldName(node)
	selected, _ := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	builder := strings.Builder{}
	builder.WriteString(`<select name="`)
	builder.WriteString(html.EscapeString(name))
	builder.WriteString(`">`)
	for _, child := range node.Children {
		if child.Type != MarkupNodeElement {
			continue
		}
		optValue := child.Attribute("value")
		optLabel := child.Attribute("label")
		if optLabel == "" {
			optLabel = optValue
		}
		optValue, _ = RenderExpressionTemplate(optValue, ctx.Expression)
		optLabel, _ = RenderExpressionTemplate(optLabel, ctx.Expression)
		selectedAttr := ""
		if selected != "" && selected == optValue {
			selectedAttr = ` selected="selected"`
		}
		builder.WriteString(`<option value="`)
		builder.WriteString(html.EscapeString(optValue))
		builder.WriteString(`"`)
		builder.WriteString(selectedAttr)
		builder.WriteString(`>`)
		builder.WriteString(html.EscapeString(optLabel))
		builder.WriteString(`</option>`)
	}
	builder.WriteString(`</select>`)
	return builder.String(), nil
}

func renderApexRepeat(node *MarkupNode, ctx *RenderContext) (string, error) {
	items, err := evaluateListExpression(node.Attribute("value"), ctx)
	if err != nil {
		return "", err
	}
	varName := strings.TrimSpace(node.Attribute("var"))
	indexName := strings.TrimSpace(node.Attribute("indexvar"))
	builder := strings.Builder{}
	for i, item := range items {
		ctx.Scope.PushFrame()
		if varName != "" {
			ctx.Scope.Set(varName, item)
		}
		if indexName != "" {
			ctx.Scope.Set(indexName, vm.Int(int64(i)))
		}
		rendered, renderErr := renderChildren(node, ctx)
		ctx.Scope.PopFrame()
		if renderErr != nil {
			return "", renderErr
		}
		builder.WriteString(rendered)
	}
	return builder.String(), nil
}

func renderApexDataTable(node *MarkupNode, ctx *RenderContext, pageBlockStyle bool) (string, error) {
	rows, err := evaluateListExpression(node.Attribute("value"), ctx)
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
	columns := columnNodes(node)
	for _, col := range columns {
		header := col.Attribute("header")
		if header == "" {
			header = col.Attribute("value")
		}
		header, _ = RenderExpressionTemplate(header, ctx.Expression)
		builder.WriteString(`<th>`)
		builder.WriteString(html.EscapeString(header))
		builder.WriteString(`</th>`)
	}
	builder.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		ctx.Scope.WithFrame(func() {
			varName := strings.TrimSpace(node.Attribute("var"))
			if varName != "" {
				ctx.Scope.Set(varName, row)
			}
			builder.WriteString(`<tr>`)
			for _, col := range columns {
				cell, _ := RenderExpressionTemplate(col.Attribute("value"), ctx.Expression)
				builder.WriteString(`<td>`)
				builder.WriteString(html.EscapeString(cell))
				builder.WriteString(`</td>`)
			}
			builder.WriteString(`</tr>`)
		})
	}
	builder.WriteString(`</tbody></table>`)
	return builder.String(), nil
}

func renderApexPageBlockTable(node *MarkupNode, ctx *RenderContext) (string, error) {
	table, err := renderApexDataTable(node, ctx, true)
	if err != nil {
		return "", err
	}
	return `<div class="pbBody">` + table + `</div>`, nil
}

func renderApexDataList(node *MarkupNode, ctx *RenderContext) (string, error) {
	rows, err := evaluateListExpression(node.Attribute("value"), ctx)
	if err != nil {
		return "", err
	}
	builder := strings.Builder{}
	builder.WriteString(`<dl class="dataList">`)
	varName := strings.TrimSpace(node.Attribute("var"))
	for _, row := range rows {
		ctx.Scope.WithFrame(func() {
			if varName != "" {
				ctx.Scope.Set(varName, row)
			}
			item, renderErr := renderChildren(node, ctx)
			if renderErr != nil {
				err = renderErr
				return
			}
			builder.WriteString(`<div class="dataListItem">`)
			builder.WriteString(item)
			builder.WriteString(`</div>`)
		})
		if err != nil {
			return "", err
		}
	}
	builder.WriteString(`</dl>`)
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
		builder.WriteString(renderPageMessage(message))
	}
	builder.WriteString(`</div>`)
	return builder.String(), nil
}

func renderApexPageMessage(node *MarkupNode, ctx *RenderContext) (string, error) {
	severity := strings.ToLower(strings.TrimSpace(node.Attribute("severity")))
	summary, _ := RenderExpressionTemplate(node.Attribute("summary"), ctx.Expression)
	if summary == "" {
		summary, _ = renderChildren(node, ctx)
	}
	return `<div class="message ` + html.EscapeString(severity) + `">` + html.EscapeString(summary) + `</div>`, nil
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
	return `<span class="actionSupport" data-rerender="` + html.EscapeString(target) + `">` + children + `</span>`, nil
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
	return `<link rel="stylesheet" type="text/css" href="` + html.EscapeString(href) + `" />`, nil
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
	}
	return `<div class="glade-vf-lightning-notice" style="margin:1rem;padding:0.75rem 1rem;border:1px solid #c9c9c9;background:#fff8e6;font:14px/1.4 system-ui,sans-serif;">` +
		"Lightning Out is not available in local Visualforce preview. The page markup renders, but $Lightning components will not boot." +
		`</div>`, nil
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
	attrs := renderHTMLAttributes(node)
	children, err := renderChildren(node, ctx)
	if err != nil {
		return "", err
	}
	if isVoidHTMLElement(tag) {
		return "<" + tag + attrs + " />", nil
	}
	return "<" + tag + attrs + ">" + children + "</" + tag + ">", nil
}

func renderHTMLAttributes(node *MarkupNode) string {
	if node == nil || len(node.Attributes) == 0 {
		return ""
	}
	builder := strings.Builder{}
	for key, value := range node.Attributes {
		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			continue
		}
		builder.WriteString(` `)
		builder.WriteString(key)
		if value != "" {
			builder.WriteString(`="`)
			builder.WriteString(html.EscapeString(value))
			builder.WriteString(`"`)
		}
	}
	return builder.String()
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
	alt, _ := RenderExpressionTemplate(node.Attribute("alt"), ctx.Expression)
	return `<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) + `" />`, nil
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
	return RenderMarkupTree(tree, ctx)
}

func renderApexDynamicComponent(node *MarkupNode, ctx *RenderContext) (string, error) {
	target := strings.TrimSpace(node.Attribute("componentvalue"))
	if target == "" {
		target = strings.TrimSpace(node.Attribute("value"))
	}
	return `<div class="dynamicComponentFallback">dynamic component unavailable: ` + html.EscapeString(target) + `</div>`, nil
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
	markup, err := os.ReadFile(component.File)
	if err != nil {
		return "", err
	}
	tree, err := ParseMarkupTree(string(markup))
	if err != nil {
		return "", err
	}
	childCtx := *ctx
	childCtx.ComponentAttrs = make(map[string]string)
	for key, value := range node.Attributes {
		childCtx.ComponentAttrs[key] = value
	}
	if component.Controller != "" && ctx.VM != nil {
		controller, constructErr := ctx.VM.ConstructController(component.Controller)
		if constructErr == nil {
			childCtx.Expression = &ExpressionContext{
				VM:         ctx.VM,
				Controller: controller,
				Variables:  childCtx.ComponentAttrsToVariables(),
			}
		}
	}
	return RenderMarkupTree(tree, &childCtx)
}

func (ctx *RenderContext) ComponentAttrsToVariables() map[string]vm.Value {
	out := make(map[string]vm.Value)
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
	value := expr.Eval(ctx.Expression)
	if value == nil || value.Kind == vm.ValueNull {
		return nil, nil
	}
	if value.Kind == vm.ValueList {
		return value.List, nil
	}
	return []vm.Value{*value}, nil
}

func isTruthyExpression(raw string, ctx *RenderContext) bool {
	value, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err != nil {
		return false
	}
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

func componentIDAttr(node *MarkupNode) string {
	id := strings.TrimSpace(node.Attribute("id"))
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

func staticResourceBundleDir(projectRoot, resourceName string) string {
	candidates := []string{
		filepath.Join(projectRoot, "force-app/main/default/staticresources", resourceName),
		filepath.Join(projectRoot, "force-app/main/staticresources", resourceName),
	}
	for _, candidate := range candidates {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return ""
}

func ResolveStaticResourceFile(projectRoot, resourceName, subpath string) (string, error) {
	subpath = strings.TrimPrefix(filepath.ToSlash(subpath), "/")
	if strings.Contains(subpath, "..") {
		return "", fmt.Errorf("invalid static resource path")
	}
	if bundle := staticResourceBundleDir(projectRoot, resourceName); bundle != "" {
		if subpath == "" {
			return "", fmt.Errorf("static resource bundle requires subpath")
		}
		full := filepath.Join(bundle, filepath.FromSlash(subpath))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, nil
		}
	}
	singleCandidates := []string{
		filepath.Join(projectRoot, "force-app/main/default/staticresources", resourceName+".resource"),
		filepath.Join(projectRoot, "force-app/main/staticresources", resourceName+".resource"),
	}
	for _, candidate := range singleCandidates {
		if _, err := os.Stat(candidate); err == nil && subpath == "" {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("static resource not found")
}
