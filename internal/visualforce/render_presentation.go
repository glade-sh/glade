package visualforce

import (
	"html"
	"sort"
	"strings"
)

// Presentation components expose client IDs without adding local transport
// attributes. Rerender lookup already accepts the client's id itself.
// Native dom_wrap_{default,inline,block}_* and dom_layout_panelGroup_* establish
// this wrapper contract, including class/style and evaluated html-* attributes.
func presentationAttributes(node *MarkupNode, ctx *RenderContext, generatedID, passthrough bool, native ...string) (string, error) {
	attrs := map[string]string{}
	id := visualforceExplicitComponentClientID(node, ctx)
	if generatedID {
		id = visualforceComponentClientID(node, ctx)
	}
	if id != "" {
		attrs["id"] = id
	}
	for _, name := range native {
		raw, exists := node.Attributes[strings.ToLower(name)]
		if !exists {
			continue
		}
		value, err := RenderExpressionTemplate(raw, ctx.Expression)
		if err != nil {
			return "", err
		}
		key := strings.ToLower(name)
		if key == "styleclass" {
			key = "class"
		}
		if value != "" {
			attrs[key] = value
		}
	}
	if passthrough {
		for name, raw := range node.Attributes {
			if !strings.HasPrefix(name, "html-") || len(name) == len("html-") {
				continue
			}
			value, err := RenderExpressionTemplate(raw, ctx.Expression)
			if err != nil {
				return "", err
			}
			// dom_attr_{outputPanel,outputText,panelGrid}_null preserves the
			// empty attribute produced by a raw-null property.
			attrs[strings.TrimPrefix(name, "html-")] = value
		}
	}
	return presentationAttributeText(attrs), nil
}

func presentationAttributeText(attrs map[string]string) string {
	keys := make([]string, 0, len(attrs))
	for key := range attrs {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, key := range keys {
		out.WriteString(" " + key + `="` + html.EscapeString(attrs[key]) + `"`)
	}
	return out.String()
}

// dom_layout_pageBlock_{normal,empty,pass} and dom_attr_pageBlock_* show the
// outer pass-through wrapper and the inner title/body/footer structure. Page
// sections and record tables continue to use their own rendering callbacks.
func renderApexPageBlock(node *MarkupNode, ctx *RenderContext) (string, error) {
	previousBlockID, previousSingleSection := ctx.presentationBlockID, ctx.presentationSingleSection
	ctx.presentationBlockID = visualforceComponentClientID(node, ctx)
	blockChildren := panelGridCellNodes(node)
	ctx.presentationSingleSection = len(blockChildren) == 1 && isApexStructureTag(blockChildren[0], "pageBlockSection")
	defer func() {
		ctx.presentationBlockID, ctx.presentationSingleSection = previousBlockID, previousSingleSection
	}()
	children, eventAttrs, err := renderApexContainerChildren(node, ctx)
	if err != nil {
		return "", err
	}
	title, err := RenderExpressionTemplate(node.Attribute("title"), ctx.Expression)
	if err != nil {
		return "", err
	}
	inner, err := presentationAttributes(node, ctx, true, false, "onclick")
	if err != nil {
		return "", err
	}
	outer := map[string]string{"class": "apexp"}
	for key, raw := range node.Attributes {
		if !strings.HasPrefix(key, "html-") || len(key) == len("html-") {
			continue
		}
		value, err := RenderExpressionTemplate(raw, ctx.Expression)
		if err != nil {
			return "", err
		}
		outer[strings.TrimPrefix(key, "html-")] = value
	}
	var out strings.Builder
	out.WriteString("<div" + presentationAttributeText(outer) + `><div` + inner + eventAttrs + ` class="bPageBlock brandSecondaryBrd apexDefaultPageBlock secondaryPalette">`)
	if title != "" {
		out.WriteString(`<div class="pbHeader"><table border="0" cellpadding="0" cellspacing="0" role="presentation"><tbody><tr><td class="pbTitle"><h2 class="mainTitle">` + html.EscapeString(title) + `</h2></td><td>&nbsp;</td></tr></tbody></table></div>`)
	}
	out.WriteString(`<div class="pbBody">` + children + `</div><div class="pbFooter secondaryPalette"><div class="bg"></div></div></div></div>`)
	return out.String(), nil
}

// The native single-row Section/SectionItem controls establish section IDs,
// title chrome, data cells and label/value pairs. Other child components and
// multi-row layouts keep their existing paths until rendered controls cover
// them; in particular this path does not change record/input-field projection.
func renderApexPageBlockSection(node *MarkupNode, ctx *RenderContext) (string, error) {
	if !ctx.presentationSingleSection {
		return renderApexContainer(node, "div", "pbSubsection", ctx)
	}
	columns := 2
	if raw := strings.TrimSpace(node.Attribute("columns")); raw != "" {
		columns = panelGridColumns(raw)
	}
	children := panelGridCellNodes(node)
	if len(children) > columns {
		return renderApexContainer(node, "div", "pbSubsection", ctx)
	}
	for _, child := range children {
		if !simplePresentationValue(child, "outputText") && !simplePresentationSectionItem(child) {
			return renderApexContainer(node, "div", "pbSubsection", ctx)
		}
	}
	if len(children) > 1 {
		for _, child := range children {
			if simplePresentationSectionItem(child) {
				return renderApexContainer(node, "div", "pbSubsection", ctx)
			}
		}
	}
	id := visualforceComponentClientID(node, ctx)
	if ctx.presentationBlockID != "" && id != "" {
		id = ctx.presentationBlockID + ":" + id[strings.LastIndex(id, ":")+1:]
	}
	attrs, err := presentationPassthrough(node, ctx)
	if err != nil {
		return "", err
	}
	if id != "" {
		attrs["id"] = id
	}
	title, err := RenderExpressionTemplate(node.Attribute("title"), ctx.Expression)
	if err != nil {
		return "", err
	}
	var out strings.Builder
	out.WriteString("<div" + presentationAttributeText(attrs) + ">")
	if title != "" {
		icon := map[string]string{
			"alt": "Hide Section - " + title, "class": "hideListButton", "id": "img_" + id,
			"name": title, "onclick": "twistSection(this);", "onkeypress": "if (event.keyCode=='13')twistSection(this);",
			"role": "button", "src": "/img/s.gif", "style": "cursor:pointer;", "tabindex": "0", "title": "Hide Section - " + title,
		}
		out.WriteString(`<div class="pbSubheader brandTertiaryBgr first tertiaryPalette"><img` + presentationAttributeText(icon) + ` /><h3>` + html.EscapeString(title) + `</h3></div>`)
	}
	out.WriteString(`<div class="pbSubsection"><table class="detailList" border="0" cellpadding="0" cellspacing="0">`)
	dataClass := "dataCol"
	if columns == 1 {
		// Captured page-block sections use this single-column cell class.
		dataClass = "data2Col"
	}
	var row strings.Builder
	visibleCells := 0
	rowAttrs := map[string]string{}
	for _, child := range children {
		show, err := visualforceComponentShouldRender(child, ctx)
		if err != nil {
			return "", err
		}
		if !show {
			continue
		}
		if simplePresentationSectionItem(child) {
			ctx.countComponent("apex:pageBlockSectionItem")
			rowAttrs, err = presentationPassthrough(child, ctx)
			if err != nil {
				return "", err
			}
			values := panelGridCellNodes(child)
			if len(values) == 0 {
				row.WriteString(presentationEmptyCells())
			} else {
				var label string
				if isApexStructureTag(values[0], "outputLabel") {
					value, err := RenderExpressionTemplate(values[0].Attribute("value"), ctx.Expression)
					if err != nil {
						return "", err
					}
					ctx.countComponent("apex:outputLabel")
					label = "<label>\n" + html.EscapeString(value) + "</label>"
				} else {
					label, err = renderMarkupNode(values[0], ctx)
					if err != nil {
						return "", err
					}
				}
				value, err := renderMarkupNode(values[1], ctx)
				if err != nil {
					return "", err
				}
				row.WriteString(`<th class="labelCol vfLabelColTextWrap  first  last " scope="row">` + label + `</th><td class="` + dataClass + `  first  last ">` + value + `</td>`)
			}
		} else {
			value, err := renderMarkupNode(child, ctx)
			if err != nil {
				return "", err
			}
			row.WriteString(`<td class="` + dataClass + `  first  last " colspan="2">` + value + `</td>`)
		}
		visibleCells++
	}
	if visibleCells != 0 {
		for i := visibleCells; i < columns; i++ {
			row.WriteString(presentationEmptyCells())
		}
		out.WriteString("<tbody><tr" + presentationAttributeText(rowAttrs) + ">" + row.String() + "</tr></tbody>")
	}
	out.WriteString("</table></div></div>")
	return out.String(), nil
}

func presentationEmptyCells() string {
	return `<td class="labelCol empty">&nbsp;</td><td class="dataCol empty">&nbsp;</td>`
}

func simplePresentationValue(node *MarkupNode, tag string) bool {
	if !isApexStructureTag(node, tag) || len(node.Children) != 0 {
		return false
	}
	for attr := range node.Attributes {
		if attr != "value" && attr != "escape" {
			return false
		}
	}
	return true
}

func simplePresentationSectionItem(node *MarkupNode) bool {
	if !isApexStructureTag(node, "pageBlockSectionItem") {
		return false
	}
	values := panelGridCellNodes(node)
	return len(values) == 0 || (len(values) == 2 && (simplePresentationValue(values[0], "outputLabel") || simplePresentationValue(values[0], "outputText")) && simplePresentationValue(values[1], "outputText"))
}

func presentationPassthrough(node *MarkupNode, ctx *RenderContext) (map[string]string, error) {
	attrs := map[string]string{}
	for key, raw := range node.Attributes {
		if !strings.HasPrefix(key, "html-") || len(key) == len("html-") {
			continue
		}
		value, err := RenderExpressionTemplate(raw, ctx.Expression)
		if err != nil {
			return nil, err
		}
		attrs[strings.TrimPrefix(key, "html-")] = value
	}
	return attrs, nil
}
