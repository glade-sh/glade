package visualforce

import (
	"archive/zip"
	"crypto/sha256"
	"errors"
	"fmt"
	"html"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

type RenderContext struct {
	VM                 *vm.VM
	PageName           string
	PageURL            string
	PageMeta           Page
	VFIndex            *Index
	Project            project.Project
	Expression         *ExpressionContext
	Scope              *ScopeStack
	Defines            map[string]*MarkupNode
	ComponentAttrs     map[string]string
	Metrics            *RenderMetrics
	Debug              bool
	LightningOut       bool
	LightningBootstrap *lwcbrowser.PageConfig
	ComponentBody      []*MarkupNode
	ComponentFacets    map[string]*MarkupNode
	ComponentParent    *RenderContext
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
	switch node.Type {
	case MarkupNodeText:
		if ctx.Metrics != nil {
			ctx.Metrics.ExpressionEvals++
		}
		return RenderVisualforceText(node.Text, ctx.Expression)
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
	if namespace != "" {
		if spec, ok := StandardComponentSpec(namespace, component); ok {
			if spec.Render == nil {
				return renderUnsupportedComponent(node, spec)
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
		return "", fmt.Errorf("unsupported Visualforce component %s", name)
	}
	return "", fmt.Errorf("unsupported Visualforce component %s: %s", name, reason)
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
	head += VisualforceAjaxScript()
	return "<!DOCTYPE html><html><head>" + head + "</head><body>" + body + "</body></html>", nil
}

func renderApexOutput(node *MarkupNode, ctx *RenderContext, outputField bool) (string, error) {
	raw, hasValue := node.Attributes["value"]
	if outputField && !hasValue {
		raw = ""
	}
	if outputField {
		if value, ok := renderFieldOutput(ctx, raw); ok {
			return "<span>" + value + "</span>", nil
		}
	}
	literalValue := hasValue && !strings.Contains(raw, "{!")
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
	escape := !strings.EqualFold(strings.TrimSpace(node.Attribute("escape")), "false")
	renderedValue := EscapeVisualforceOutput(value, escape)
	if literalValue && !escape {
		renderedValue = html.EscapeString(value)
	}
	if attrs := outputTextSpanAttrs(node); outputField || attrs != "" {
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
	return "<span>" + EscapeVisualforceOutput(format, escape) + "</span>", nil
}

func outputTextSpanAttrs(node *MarkupNode) string {
	var attrs []string
	if id := strings.TrimSpace(node.Attribute("id")); id != "" {
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
	if ctx != nil {
		if strings.TrimSpace(ctx.PageURL) != "" {
			action = ctx.PageURL
		} else if strings.TrimSpace(ctx.PageName) != "" {
			action += ctx.PageName
		}
	}
	attrs := strings.Builder{}
	attrs.WriteString(` method="post" action="`)
	attrs.WriteString(html.EscapeString(action))
	attrs.WriteString(`"`)
	if id := strings.TrimSpace(node.Attribute("id")); id != "" {
		attrs.WriteString(` id="`)
		attrs.WriteString(html.EscapeString(id))
		attrs.WriteString(`"`)
	}
	enctype := strings.TrimSpace(node.Attribute("enctype"))
	if enctype == "" && visualforceFormContainsInputFile(node) {
		enctype = "multipart/form-data"
	}
	if enctype != "" {
		attrs.WriteString(` enctype="`)
		attrs.WriteString(html.EscapeString(enctype))
		attrs.WriteString(`"`)
	}
	return "<form" + attrs.String() + `><input type="hidden" name="` + ViewStateActionFieldName() + `" value="" />` + children + "</form>", nil
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
	name := inputFieldName(node)
	value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	return `<input type="` + html.EscapeString(inputType) + `" name="` + html.EscapeString(name) + `" value="` + html.EscapeString(value) + `" />`, nil
}

func renderApexInputTextarea(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := inputFieldName(node)
	value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	attrs := strings.Builder{}
	attrs.WriteString(` name="`)
	attrs.WriteString(html.EscapeString(name))
	attrs.WriteString(`"`)
	for _, attr := range []string{"rows", "cols"} {
		if raw := strings.TrimSpace(node.Attribute(attr)); raw != "" {
			attrs.WriteString(` `)
			attrs.WriteString(attr)
			attrs.WriteString(`="`)
			attrs.WriteString(html.EscapeString(raw))
			attrs.WriteString(`"`)
		}
	}
	return `<textarea` + attrs.String() + `>` + html.EscapeString(value) + `</textarea>`, nil
}

func renderApexInputCheckbox(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := inputFieldName(node)
	checked := ""
	if isTruthyExpression(node.Attribute("selected"), ctx) {
		checked = ` checked="checked"`
	}
	escapedName := html.EscapeString(name)
	return `<input type="hidden" name="` + escapedName + `" value="false" />` +
		`<input type="checkbox" name="` + escapedName + `" value="true"` + checked + " />", nil
}

func renderApexInputField(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := inputFieldName(node)
	if rendered, ok := renderFieldInput(ctx, node.Attribute("value"), name); ok {
		return rendered, nil
	}
	value, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	return `<input type="text" class="inputField" name="` + html.EscapeString(name) + `" value="` + html.EscapeString(value) + `" />`, nil
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
	attrs := ` type="submit" value="` + html.EscapeString(label) + `" data-action="` + html.EscapeString(action) + `"`
	if rerender := strings.TrimSpace(node.Attribute("rerender")); rerender != "" {
		hook := VisualforceAjaxSubmitHookWithStatus(action, rerender, strings.TrimSpace(node.Attribute("status")))
		return `<input` + attrs + ` onclick="` + html.EscapeString(hook) + `" />`, nil
	}
	hook := `if(this.form&&this.form.elements['` + ViewStateActionFieldName() + `']){this.form.elements['` + ViewStateActionFieldName() + `'].value=` + jsStringLiteral(action) + `;}`
	return `<input` + attrs + ` onclick="` + html.EscapeString(hook) + `" />`, nil
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
	if rerender := strings.TrimSpace(node.Attribute("rerender")); rerender != "" {
		hook := VisualforceAjaxLinkHookWithStatus(action, rerender, strings.TrimSpace(node.Attribute("status")))
		return `<a href="#" onclick="` + html.EscapeString(hook) + `">` + html.EscapeString(label) + `</a>`, nil
	}
	hook := `var f=this.closest('form')||document.forms[0];if(f&&f.elements['` + ViewStateActionFieldName() + `']){f.elements['` + ViewStateActionFieldName() + `'].value=` + jsStringLiteral(action) + `;f.submit();}return false;`
	return `<a href="#" onclick="` + html.EscapeString(hook) + `">` + html.EscapeString(label) + `</a>`, nil
}

func renderApexSelectList(node *MarkupNode, ctx *RenderContext) (string, error) {
	name := fieldName(node)
	selected, err := RenderExpressionTemplate(node.Attribute("value"), ctx.Expression)
	if err != nil {
		return "", err
	}
	options, err := selectOptionNodes(node, ctx)
	if err != nil {
		return "", err
	}
	builder := strings.Builder{}
	builder.WriteString(`<select name="`)
	builder.WriteString(html.EscapeString(name))
	builder.WriteString(`">`)
	for _, option := range options {
		selectedAttr := ""
		if selected != "" && selected == option.value {
			selectedAttr = ` selected="selected"`
		}
		builder.WriteString(`<option value="`)
		builder.WriteString(html.EscapeString(option.value))
		builder.WriteString(`"`)
		builder.WriteString(selectedAttr)
		builder.WriteString(`>`)
		builder.WriteString(html.EscapeString(option.label))
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
		header := dataTableColumnHeader(col, node, rows, ctx, pageBlockStyle)
		builder.WriteString(`<th>`)
		builder.WriteString(html.EscapeString(header))
		builder.WriteString(`</th>`)
	}
	builder.WriteString(`</tr></thead><tbody>`)
	for _, row := range rows {
		ctx.Scope.PushFrame()
		varName := strings.TrimSpace(node.Attribute("var"))
		if varName != "" {
			ctx.Scope.Set(varName, row)
		}
		builder.WriteString(`<tr>`)
		for _, col := range columns {
			cell, err := RenderExpressionTemplate(col.Attribute("value"), ctx.Expression)
			if err != nil {
				ctx.Scope.PopFrame()
				return "", err
			}
			builder.WriteString(`<td>`)
			builder.WriteString(html.EscapeString(cell))
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
	builder.WriteString(`<table`)
	if id := strings.TrimSpace(node.Attribute("id")); id != "" {
		builder.WriteString(` id="`)
		builder.WriteString(html.EscapeString(id))
		builder.WriteString(`"`)
	}
	if className := strings.TrimSpace(firstNonEmpty(node.Attribute("styleClass"), node.Attribute("class"))); className != "" {
		builder.WriteString(` class="`)
		builder.WriteString(html.EscapeString(className))
		builder.WriteString(`"`)
	}
	builder.WriteString(`>`)
	if err := renderPanelGridFacet(&builder, node, ctx, "caption", "caption", node.Attribute("captionClass"), columns); err != nil {
		return "", err
	}
	if err := renderPanelGridFacet(&builder, node, ctx, "header", "thead", node.Attribute("headerClass"), columns); err != nil {
		return "", err
	}
	builder.WriteString(`<tbody>`)
	for i, child := range cells {
		if i%columns == 0 {
			builder.WriteString(`<tr>`)
		}
		rendered, err := renderMarkupNode(child, ctx)
		if err != nil {
			return "", err
		}
		builder.WriteString(`<td>`)
		builder.WriteString(rendered)
		builder.WriteString(`</td>`)
		if i%columns == columns-1 {
			builder.WriteString(`</tr>`)
		}
	}
	if len(cells) > 0 && len(cells)%columns != 0 {
		builder.WriteString(`</tr>`)
	}
	builder.WriteString(`</tbody>`)
	if err := renderPanelGridFacet(&builder, node, ctx, "footer", "tfoot", node.Attribute("footerClass"), columns); err != nil {
		return "", err
	}
	builder.WriteString(`</table>`)
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
		builder.WriteString(`<thead><tr><th`)
		builder.WriteString(classAttr)
		builder.WriteString(` colspan="`)
		builder.WriteString(fmt.Sprintf("%d", columns))
		builder.WriteString(`">`)
		builder.WriteString(rendered)
		builder.WriteString(`</th></tr></thead>`)
	case "tfoot":
		builder.WriteString(`<tfoot><tr><td`)
		builder.WriteString(classAttr)
		builder.WriteString(` colspan="`)
		builder.WriteString(fmt.Sprintf("%d", columns))
		builder.WriteString(`">`)
		builder.WriteString(rendered)
		builder.WriteString(`</td></tr></tfoot>`)
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
	return `<span class="actionSupport" data-event="` + html.EscapeString(event) + `" data-rerender="` + html.EscapeString(target) + `" on` + html.EscapeString(event) + `="` + html.EscapeString(hook) + `">` + children + `</span>`, nil
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
	attrs := renderHTMLAttributes(node)
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
	return `<img src="` + html.EscapeString(src) + `" alt="` + html.EscapeString(alt) + `" />`, nil
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
		rendered, err := renderMarkupNode(child, renderCtx)
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
	childCtx.ComponentFacets = componentFacets(node)
	childCtx.ComponentBody = componentBodyNodes(node)
	childCtx.ComponentParent = ctx
	for key, value := range evaluatedComponentAttributes(node, ctx) {
		childCtx.ComponentAttrs[key] = value
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
		controller, constructErr := ctx.VM.ConstructController(component.Controller)
		if constructErr == nil {
			applyComponentAssignTo(&controller, component.Attributes, childCtx.ComponentAttrs)
			childCtx.Expression = &ExpressionContext{
				VM:         ctx.VM,
				Controller: controller,
				Variables:  variables,
			}
		}
	}
	return RenderMarkupTree(tree, &childCtx)
}

func evaluatedComponentAttributes(node *MarkupNode, ctx *RenderContext) map[string]string {
	out := make(map[string]string)
	if node == nil {
		return out
	}
	for key, value := range node.Attributes {
		rendered, err := RenderExpressionTemplate(value, ctx.Expression)
		if err != nil {
			rendered = value
		}
		out[key] = rendered
	}
	return out
}

func applyComponentAssignTo(controller *vm.Value, attrs []Attribute, values map[string]string) {
	if controller == nil || controller.Kind != vm.ValueObject {
		return
	}
	for _, attr := range attrs {
		target := expressionFieldName(attr.AssignTo)
		if target == "" {
			continue
		}
		value, ok := values[strings.ToLower(strings.TrimSpace(attr.Name))]
		if !ok {
			value, ok = values[strings.TrimSpace(attr.Name)]
		}
		if !ok {
			continue
		}
		controller.Fields[target] = vm.String(value)
	}
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

func dataTableColumnHeader(col, table *MarkupNode, rows []vm.Value, ctx *RenderContext, deriveDefault bool) string {
	header := firstNonEmpty(col.Attribute("header"), col.Attribute("title"))
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
	value := expr.Eval(ctx.Expression)
	if value == nil || value.Kind == vm.ValueNull {
		return nil, nil
	}
	if value.Kind == vm.ValueList {
		return value.List, nil
	}
	return []vm.Value{*value}, nil
}

type selectOptionRender struct {
	value string
	label string
}

func renderApexSelectInputs(node *MarkupNode, ctx *RenderContext, inputType, className string) (string, error) {
	name := fieldName(node)
	selected := selectedValueSet(node.Attribute("value"), ctx)
	options, err := selectOptionNodes(node, ctx)
	if err != nil {
		return "", err
	}
	builder := strings.Builder{}
	builder.WriteString(`<span class="`)
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
		builder.WriteString(` />`)
		builder.WriteString(`<label>`)
		builder.WriteString(html.EscapeString(option.label))
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
			options = append(options, selectOptionRender{value: value, label: label})
		case strings.EqualFold(child.Name, "selectOptions"):
			value, ok, err := evaluateRenderExpressionValue(child.Attribute("value"), ctx)
			if err != nil {
				return nil, err
			}
			if !ok {
				continue
			}
			options = append(options, selectOptionsFromValue(value)...)
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
	value := expr.Eval(ctx.Expression)
	if value == nil {
		return vm.Null, true, nil
	}
	return *value, true, nil
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
			return []selectOptionRender{{value: optionValue, label: label}}
		}
	}
	text := value.String()
	if text == "" || value.Kind == vm.ValueNull {
		return nil
	}
	return []selectOptionRender{{value: text, label: text}}
}

func selectOptionField(option vm.Value, field string) (string, bool) {
	value, ok := objectFieldIgnoreCase(option, field)
	if !ok || value.Kind == vm.ValueNull {
		return "", false
	}
	return value.String(), true
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

func selectedValueSet(raw string, ctx *RenderContext) map[string]bool {
	selected := make(map[string]bool)
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return selected
	}
	exprText := raw
	if strings.HasPrefix(exprText, "{!") && strings.HasSuffix(exprText, "}") {
		exprText = strings.TrimSpace(exprText[2 : len(exprText)-1])
	}
	expr, err := parseExpression(exprText)
	if err == nil {
		if value := expr.Eval(ctx.Expression); value != nil {
			addSelectedValue(selected, *value)
			return selected
		}
	}
	rendered, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err == nil && rendered != "" {
		selected[rendered] = true
	}
	return selected
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
	normalizedSubpath, err := normalizeStaticResourceSubpath(subpath)
	if err != nil {
		return "", err
	}
	if bundle := staticResourceBundleDir(projectRoot, resourceName); bundle != "" {
		if normalizedSubpath == "" {
			return "", fmt.Errorf("static resource bundle requires subpath")
		}
		full := filepath.Join(bundle, filepath.FromSlash(normalizedSubpath))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, nil
		}
	}
	singleCandidates := []string{
		filepath.Join(projectRoot, "force-app/main/default/staticresources", resourceName+".resource"),
		filepath.Join(projectRoot, "force-app/main/staticresources", resourceName+".resource"),
	}
	var missingZipEntry bool
	for _, candidate := range singleCandidates {
		resolved, err := ResolveStaticResourceContentPath(candidate, resourceName, normalizedSubpath)
		if err == nil {
			return resolved, nil
		}
		if errors.Is(err, errStaticResourceZipEntryMissing) {
			missingZipEntry = true
		}
	}
	if missingZipEntry {
		return "", fmt.Errorf("static resource entry %q not found in %s.resource", normalizedSubpath, resourceName)
	}
	return "", fmt.Errorf("static resource not found")
}

func ResolveStaticResourceContentPath(contentPath, resourceName, subpath string) (string, error) {
	normalizedSubpath, err := normalizeStaticResourceSubpath(subpath)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(contentPath)
	if err != nil {
		return "", err
	}
	if info.IsDir() {
		if normalizedSubpath == "" {
			return "", fmt.Errorf("static resource bundle requires subpath")
		}
		full := filepath.Join(contentPath, filepath.FromSlash(normalizedSubpath))
		if info, err := os.Stat(full); err == nil && !info.IsDir() {
			return full, nil
		}
		return "", fmt.Errorf("static resource not found")
	}
	if normalizedSubpath == "" {
		return contentPath, nil
	}
	return resolveZippedStaticResourceFile(contentPath, resourceName, normalizedSubpath)
}

var errStaticResourceZipEntryMissing = errors.New("static resource zip entry missing")

func normalizeStaticResourceSubpath(subpath string) (string, error) {
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

func resolveZippedStaticResourceFile(zipPath, resourceName, subpath string) (string, error) {
	reader, err := zip.OpenReader(zipPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()
	for _, entry := range reader.File {
		entryName, err := normalizeStaticResourceSubpath(entry.Name)
		if err != nil || entryName != subpath || entry.FileInfo().IsDir() {
			continue
		}
		return extractStaticResourceZipEntry(zipPath, resourceName, subpath, entry)
	}
	return "", errStaticResourceZipEntryMissing
}

func extractStaticResourceZipEntry(zipPath, resourceName, subpath string, entry *zip.File) (string, error) {
	source, err := entry.Open()
	if err != nil {
		return "", err
	}
	defer source.Close()
	target := staticResourceZipCachePath(zipPath, resourceName, subpath)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}
	output, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(output, source)
	closeErr := output.Close()
	if copyErr != nil {
		return "", copyErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return target, nil
}

func staticResourceZipCachePath(zipPath, resourceName, subpath string) string {
	info, _ := os.Stat(zipPath)
	keyInput := zipPath
	if info != nil {
		keyInput = fmt.Sprintf("%s:%d:%d", zipPath, info.ModTime().UnixNano(), info.Size())
	}
	sum := sha256.Sum256([]byte(keyInput))
	return filepath.Join(os.TempDir(), "glade-static-resource", "zip", resourceName, fmt.Sprintf("%x", sum[:8]), filepath.FromSlash(subpath))
}
