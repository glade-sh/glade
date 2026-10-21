package visualforce

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"math/big"
	"net/url"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
	nethtml "golang.org/x/net/html"
)

// A field component binds to the SObject returned by its controller expression,
// not to the name of that expression. field and input rendering filled/blank and relationship controls
// establish this distinction. Standard-controller roots stay on their authorized
// projection path in resolveFieldBinding.
func resolveControllerFieldBinding(ctx *RenderContext, parts []string) (FieldBinding, bool) {
	recordExpr := strings.Join(parts[:len(parts)-1], ".")
	record, _, err := evaluateRenderExpressionValue(recordExpr, ctx)
	if err != nil || record.Kind != vm.ValueObject {
		return FieldBinding{}, false
	}
	objectName, ok := storage.ResolveObjectName(*ctx.VM.Org, record.Type)
	if !ok {
		return FieldBinding{}, false
	}
	definition := ctx.VM.Org.Objects[objectName].Definition
	fieldName, ok := storage.ResolveFieldName(definition, ctx.Project.Namespace, parts[len(parts)-1])
	if !ok {
		return FieldBinding{}, false
	}
	field := definition.Fields[fieldName]
	binding := FieldBinding{ObjectName: objectName, FieldName: fieldName, Field: field,
		Kind: fieldRenderKind(field), Value: storage.NullValue(), ControllerRecord: true}
	// Supplied values do not bypass the bound user's object/field permissions.
	// The native denied controls omit the whole component, including lookup
	// hidden state. Keep only metadata when authorization fails.
	if _, err := ctx.VM.AuthorizeVisualforceRecordFields(objectName, []string{fieldName}); err != nil {
		return binding, true
	}
	// Read the field from that same SObject, rather than invoking its getter a
	// second time to evaluate the whole path. Absent fields on a new record are
	// raw null, with metadata defaults applied only by the appropriate widget.
	value, _ := readObjectMember(ctx.Expression, record, fieldName)
	stored, ok := visualforceFieldStorageValue(value)
	if !ok {
		return FieldBinding{}, false
	}
	binding.Value, binding.Authorized = stored, true
	return binding, true
}

func controllerFieldRequired(node *MarkupNode, ctx *RenderContext, binding FieldBinding) bool {
	if node.HasAttribute("required") {
		value, _, err := evaluateRenderExpressionValue(node.Attribute("required"), ctx)
		if err == nil && value.Kind != vm.ValueNull {
			if value.Kind == vm.ValueBool {
				return value.Bool
			}
			return truthyExpressionValue(value.String())
		}
	}
	// required=false overrides metadata; raw null restores the metadata default.
	return binding.Kind != FieldCheckbox && fieldIsRequired(binding.Field)
}

func renderControllerFieldInput(ctx *RenderContext, binding FieldBinding, id string, required bool, presentation ...string) string {
	if fieldIsReadonly(binding.Field) || binding.Field.Formula != "" {
		return `<span` + fieldInputIDAttr(id) + `>` + renderControllerFieldOutput(ctx, binding) + `</span>`
	}
	name := fieldInputName(binding, "")
	if binding.Kind == FieldDate || binding.Kind == FieldDatetime || fieldIsReference(binding.Field) || fieldIsMultiSelect(binding.Field) {
		// These captured compound widgets bind submission and JavaScript by
		// client ID. Preserve the existing names of other scalar field widgets.
		name = id
	}
	attrs := ` name="` + html.EscapeString(name) + `"` + fieldInputIDAttr(id)
	// The lookup's required_ true/false/null controls mark the label/wrapper,
	// without setting HTML required on its visible search input.
	if !fieldIsReference(binding.Field) {
		attrs += fieldInputRequiredAttr(required)
	}
	// Forwarded presentation attributes are captured for text inputs only
	// (override_style_runtime). Choices and rich text hide a backing element
	// with their own style, so nothing is forwarded onto it until styled rows
	// for those widgets exist.
	if !fieldIsMultiSelect(binding.Field) && !(binding.Kind == FieldTextarea && fieldIsRichText(binding.Field)) {
		attrs += strings.Join(presentation, "")
	}
	text := controllerFieldText(ctx, binding, false)
	if name, _, found := controllerReferenceName(ctx, binding); found {
		text = name
	}
	value := html.EscapeString(text)
	if fieldIsReference(binding.Field) {
		return renderControllerLookup(ctx, binding, id, attrs, text)
	}
	switch binding.Kind {
	case FieldCheckbox:
		checked := ""
		if binding.Value.Kind == storage.ValueBoolean && binding.Value.Boolean {
			checked = ` checked="checked"`
		}
		return `<input type="hidden" name="` + html.EscapeString(fieldInputName(binding, "")) + `" value="0" /><input type="checkbox"` + attrs + ` value="1"` + checked + ` />`
	case FieldTextarea:
		if fieldIsRichText(binding.Field) {
			attrs += ` style="visibility: hidden;"`
		}
		return `<textarea` + attrs + `>` + html.EscapeString(storageValueText(binding.Value)) + `</textarea>`
	case FieldSelect:
		// The single-select native blank record takes the metadata default. The
		// --None-- option is present only when this component is not required.
		if fieldIsMultiSelect(binding.Field) {
			return renderControllerMultiSelect(binding, id, attrs)
		}
		var out strings.Builder
		out.WriteString(`<select` + attrs + `>`)
		selected := controllerPicklistValue(binding)
		if !required {
			mark := ""
			if selected == "" {
				mark = ` selected="selected"`
			}
			out.WriteString(`<option value=""` + mark + `>--None--</option>`)
		}
		for _, option := range binding.Field.PicklistValues {
			if !option.Active && option.Value != selected {
				continue
			}
			mark := ""
			if option.Value == selected {
				mark = ` selected="selected"`
			}
			out.WriteString(`<option value="` + html.EscapeString(option.Value) + `"` + mark + `>` + html.EscapeString(firstNonEmpty(option.Label, option.Value)) + `</option>`)
		}
		out.WriteString(`</select>`)
		return out.String()
	case FieldDate, FieldDatetime:
		if controllerFieldLocale(ctx) != "en_US" {
			return `<input type="text"` + attrs + ` value="` + value + `" />`
		}
		// Native API59/67 clicked Date/DateTime controls retain size and omit
		// the value attribute when null; clicks change only the DOM property.
		size := "12"
		if binding.Kind == FieldDatetime {
			size = "20"
		}
		attrs += ` size="` + size + `"`
		valueAttr := ` value="` + value + `"`
		if binding.Value.Kind == storage.ValueNull {
			valueAttr = ""
		}
		current := time.Now().In(controllerFieldLocation(ctx))
		layout := "1/2/2006"
		if binding.Kind == FieldDatetime {
			layout += ", 3:04 PM"
		}
		shortcut := current.Format(layout)
		href := "DatePicker.insertDate(" + controllerJavascriptString(shortcut) + ", " + controllerJavascriptString(id) + ", true)"
		if binding.Kind == FieldDatetime {
			href = "void(" + href + ")"
		}
		return `<span class="dateInput"><input type="text"` + attrs + valueAttr + ` /><span class="dateFormat">[&#160;<a href="` + html.EscapeString("javascript:"+href+";") + `">` + html.EscapeString(shortcut) + `</a>&#160;]</span></span>`
	default:
		return `<input type="text"` + attrs + ` value="` + value + `" />`
	}
}

func renderControllerMultiSelect(binding FieldBinding, id, attrs string) string {
	var out strings.Builder
	out.WriteString(`<select multiple="multiple"` + attrs + ` style="display:none;">`)
	selected := fieldSelectedValues(binding.Value)
	var available, chosen strings.Builder
	for i, option := range binding.Field.PicklistValues {
		if !option.Active && !selected[option.Value] {
			continue
		}
		mark := ""
		if selected[option.Value] {
			mark = ` selected="selected"`
		}
		out.WriteString(`<option value="` + html.EscapeString(option.Value) + `"` + mark + `>` + html.EscapeString(firstNonEmpty(option.Label, option.Value)) + `</option>`)
		list := &available
		if selected[option.Value] {
			list = &chosen
		}
		fmt.Fprintf(list, `<option value="%d">%s</option>`, i, html.EscapeString(firstNonEmpty(option.Label, option.Value)))
	}
	out.WriteString(`</select>`)
	// Filled, blank and single-choice native controls have unselected visible
	// lists; only the hidden backing select carries submission selection.
	link := func(action, direction string) string {
		href := "javascript:MultiSelectPicklist." + action + url.QueryEscape("('"+id+"');")
		label := "Add"
		if direction == "left" {
			label = "Remove"
		}
		return `<a href="` + html.EscapeString(href) + `"><img` + fieldInputIDAttr(id+"_"+direction+"_arrow") + ` class="picklistArrow` + strings.ToUpper(direction[:1]) + direction[1:] + `" alt="` + label + `" title="` + label + `" style="cursor:pointer;" /></a>`
	}
	out.WriteString("<table class=\"multiSelectPicklistTable\"><tbody><tr><td></td><td></td></tr>\n<tr class=\"multiSelectPicklistRow\">\n<td><select multiple=\"multiple\"" + fieldInputIDAttr(id+"_unselected") + "><optgroup style=\"text-decoration:none;\">" + available.String() + "</optgroup></select></td>\n<td class=\"multiSelectPicklistCell\">\n" + link("handleMSPSelect", "right") + "<br /><br />\n" + link("handleMSPUnSelect", "left") + "</td>\n<td><select multiple=\"multiple\"" + fieldInputIDAttr(id+"_selected") + "><optgroup style=\"text-decoration:none;\">" + chosen.String() + "</optgroup></select></td>\n</tr></tbody></table>")
	return out.String()
}

func controllerFieldLocation(ctx *RenderContext) *time.Location {
	if ctx != nil && ctx.VM != nil {
		if user, err := ctx.VM.VisualforceExecutionUserRecord(); err == nil {
			if zone, ok := user.GetField("TimeZoneSidKey"); ok {
				if location, err := time.LoadLocation(zone.String); err == nil {
					return location
				}
			}
		}
	}
	return time.UTC
}

func controllerJavascriptString(value string) string {
	return "'" + strings.NewReplacer("\\", "\\\\", "'", "\\'", "\r", "\\r", "\n", "\\n", "<", "\\x3c", ">", "\\x3e").Replace(value) + "'"
}

func controllerFieldLocale(ctx *RenderContext) string {
	if ctx != nil && ctx.VM != nil {
		if user, err := ctx.VM.VisualforceExecutionUserRecord(); err == nil {
			if current, ok := user.GetField("LocaleSidKey"); ok && current.String != "" {
				return current.String
			}
		}
	}
	return "en_US"
}

func renderControllerLookup(ctx *RenderContext, binding FieldBinding, id, attrs, text string) string {
	formID := ""
	if ctx.ComponentReferences != nil {
		for node, clientID := range ctx.ComponentReferences.clientIDs {
			if isApexStructureTag(node, "form") && strings.HasPrefix(id, clientID+":") && len(clientID) > len(formID) {
				formID = clientID
			}
		}
	}
	prefix := ""
	for _, object := range binding.Field.ReferenceTo {
		if objectName, ok := storage.ResolveObjectName(*ctx.VM.Org, object); ok {
			prefix = ctx.VM.Org.Objects[objectName].Definition.KeyPrefix
			break
		}
	}
	var hidden strings.Builder
	lookupID, previousText := storageValueText(binding.Value), text
	valueAttr := ` value="` + html.EscapeString(text) + `"`
	if binding.Value.Kind == storage.ValueNull {
		// Both an unset lookup and an explicit null use these native hidden
		// sentinels. The visible input has no value attribute (not value="").
		lookupID, previousText, valueAttr = "000000000000000", "null", ""
	}
	for _, field := range [][2]string{{"lkid", lookupID}, {"lkold", previousText}, {"lktp", prefix}, {"lspf", "0"}, {"lspfsub", "0"}, {"mod", "0"}} {
		name := id + "_" + field[0]
		hidden.WriteString(`<input type="hidden"` + fieldInputIDAttr(name) + ` name="` + html.EscapeString(name) + `" value="` + html.EscapeString(field[1]) + `" />`)
	}
	encoded := func(value string) string { return strings.ReplaceAll(url.QueryEscape(value), "+", "%20") }
	lookup := "/_ui/common/data/LookupPage?lkfm=" + encoded(formID) + "&lknm=" + encoded(id) + "&lktp="
	code := " openLookup(" + controllerJavascriptString(lookup) + " + getElementByIdCS(" + controllerJavascriptString(id+"_lktp") + ").value,670,'1','&lksrch=' + escapeUTF(getElementByIdCS(" + controllerJavascriptString(id) + ").value.substring(0, 80)))"
	return `<span class="lookupInput">` + hidden.String() + `<input type="text"` + attrs + valueAttr + ` size="20" maxlength="255" /><a` + fieldInputIDAttr(id+"_lkwgt") + ` href="javascript:` + html.EscapeString(encoded(code)) + `"><img src="/s.gif" class="lookupIcon" /></a></span>`
}

func controllerPicklistValue(binding FieldBinding) string {
	value := storageValueText(binding.Value)
	if value == "" && binding.Value.Kind == storage.ValueNull {
		for _, option := range binding.Field.PicklistValues {
			if option.Default && option.Active {
				return option.Value
			}
		}
	}
	return value
}

func controllerFieldText(ctx *RenderContext, binding FieldBinding, output bool) string {
	value := storageValueText(binding.Value)
	if binding.Kind == FieldSelect {
		value = controllerPicklistValue(binding)
		if output {
			parts := strings.Split(value, ";")
			for i, part := range parts {
				for _, option := range binding.Field.PicklistValues {
					if option.Value == part {
						parts[i] = firstNonEmpty(option.Label, part)
						break
					}
				}
			}
			value = strings.Join(parts, "; ")
		}
	}
	// This native fixture measures en_US. Keep other locale profiles on their
	// original scalar representation until their field-format rows are captured.
	locale := controllerFieldLocale(ctx)
	if binding.Kind == FieldDate && value != "" && locale == "en_US" {
		if date, err := time.Parse("2006-01-02", value); err == nil {
			value = date.Format("1/2/2006")
		}
	}
	if binding.Kind == FieldDatetime && value != "" && locale == "en_US" {
		if instant, err := time.Parse(time.RFC3339Nano, value); err == nil {
			value = instant.In(controllerFieldLocation(ctx)).Format("1/2/2006, 3:04 PM")
		}
	}
	if binding.Kind == FieldNumber && value != "" && locale == "en_US" {
		if number, ok := new(big.Rat).SetString(value); ok {
			value = number.FloatString(binding.Field.Scale)
			parts := strings.SplitN(value, ".", 2)
			negative := strings.HasPrefix(parts[0], "-")
			integer := strings.TrimPrefix(parts[0], "-")
			for pos := len(integer) - 3; pos > 0; pos -= 3 {
				integer = integer[:pos] + "," + integer[pos:]
			}
			if negative {
				integer = "-" + integer
			}
			parts[0] = integer
			value = strings.Join(parts, ".")
			if output {
				switch strings.ToUpper(binding.Field.DisplayType) {
				case "CURRENCY":
					value = "$" + strings.TrimPrefix(value, "-")
					if negative {
						value = "-" + value
					}
				case "PERCENT":
					value += "%"
				}
			}
		}
	}
	if output && value == "" {
		switch binding.Kind {
		case FieldNumber, FieldDate, FieldDatetime, FieldEmail, FieldURL, FieldPhone:
			return "\u00a0"
		case FieldSelect:
			if fieldIsMultiSelect(binding.Field) {
				return "\u00a0"
			}
		}
		if fieldIsReference(binding.Field) {
			return "\u00a0"
		}
	}
	return value
}

func renderControllerFieldOutput(ctx *RenderContext, binding FieldBinding) string {
	if binding.Kind == FieldCheckbox {
		checked := binding.Value.Kind == storage.ValueBoolean && binding.Value.Boolean
		return fmt.Sprintf(`<img src="/img/checkbox_%t.gif" alt="%t" />`, checked, checked)
	}
	text := controllerFieldText(ctx, binding, true)
	if name, id, found := controllerReferenceName(ctx, binding); found {
		return `<a href="/` + html.EscapeString(string(id)) + `">` + html.EscapeString(name) + `</a>`
	}
	if binding.Kind == FieldEmail && storageValueText(binding.Value) != "" {
		return `<a href="mailto:` + html.EscapeString(storageValueText(binding.Value)) + `">` + html.EscapeString(text) + `</a>`
	}
	if binding.Kind == FieldURL && storageValueText(binding.Value) != "" {
		// Captured URL controls prepend http:// to values without an http(s)
		// scheme and route through the shield, including active-looking text.
		// Never make the raw value an executable href.
		if !strings.HasPrefix(strings.ToLower(text), "http://") && !strings.HasPrefix(strings.ToLower(text), "https://") {
			text = "http://" + text
		}
		var nonce [8]byte
		if _, err := rand.Read(nonce[:]); err != nil {
			return html.EscapeString(text)
		}
		id := "vf_url_" + hex.EncodeToString(nonce[:])
		href := "/servlet/servlet.ExternalRedirect?url=" + url.QueryEscape(text)
		return `<a href="` + html.EscapeString(href) + `" target="_blank"><span` + fieldInputIDAttr(id) + `>` + html.EscapeString(text) + `</span><script>Sfdc.onload(function() {    blankshield(document.getElementById('` + id + `').parentElement);});</script></a>`
	}
	if binding.Kind == FieldTextarea {
		if fieldIsRichText(binding.Field) {
			return `<table><tbody><tr><td><div>` + controllerRichText(text) + "\n</div></td></tr></tbody></table>"
		}
		return strings.ReplaceAll(html.EscapeString(text), "\n", "<br />")
	}
	return html.EscapeString(text)
}

// The field and input rendering lookup input and output controls use the target's Name. Keep that
// single-field read on authorization and request context's bound-user API; a missing or denied target never
// supplies a label, and never triggers an unrestricted record query.
func controllerReferenceName(ctx *RenderContext, binding FieldBinding) (string, storage.ID, bool) {
	if !fieldIsReference(binding.Field) || !binding.Authorized || storageValueText(binding.Value) == "" {
		return "", "", false
	}
	id := storage.ID(storageValueText(binding.Value))
	for _, candidate := range binding.Field.ReferenceTo {
		objectName, known := storage.ResolveObjectName(*ctx.VM.Org, candidate)
		if !known {
			continue
		}
		prefix := ctx.VM.Org.Objects[objectName].Definition.KeyPrefix
		if prefix == "" || !strings.HasPrefix(string(id), prefix) {
			continue
		}
		record, found, err := ctx.VM.ReadVisualforceRecordFields(objectName, id, []string{"Name"})
		if err == nil && found {
			if name, present := record.GetField("Name"); present && name.Kind == storage.ValueString {
				return name.String, id, true
			}
		}
	}
	return "", "", false
}

// field and input rendering Rich filled/blank and boundary_rich_script establish markup preservation
// and removal of script contents. Allow only inert text formatting; do not turn
// previously escaped event handlers, scripts or active embedded content into
// executable output while adding rich-field rendering.
func controllerRichText(source string) string {
	doc, err := nethtml.Parse(strings.NewReader(source))
	if err != nil {
		return html.EscapeString(source)
	}
	var out strings.Builder
	var walk func(*nethtml.Node)
	walk = func(node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			out.WriteString(html.EscapeString(node.Data))
			return
		}
		if node.Type == nethtml.ElementNode && (node.Data == "script" || node.Data == "style") {
			return
		}
		format := node.Type == nethtml.ElementNode && (node.Data == "b" || node.Data == "i" || node.Data == "strong" || node.Data == "em")
		if format {
			out.WriteString("<" + node.Data + ">")
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if format {
			out.WriteString("</" + node.Data + ">")
		}
	}
	walk(doc)
	return out.String()
}

func visualforceFormContainsField(node *MarkupNode) bool {
	if isApexStructureTag(node, "inputField") || isApexStructureTag(node, "outputField") {
		return true
	}
	for _, child := range node.Children {
		if visualforceFormContainsField(child) {
			return true
		}
	}
	return false
}

func controllerFieldNode(node *MarkupNode, ctx *RenderContext) bool {
	if !isApexStructureTag(node, "inputField") && !isApexStructureTag(node, "outputField") {
		return false
	}
	binding, ok := resolveFieldBinding(ctx, node.Attribute("value"))
	return ok && binding.ControllerRecord
}

func sectionContainsControllerField(node *MarkupNode, ctx *RenderContext) bool {
	children := panelGridCellNodes(node)
	if len(children) == 0 {
		return false
	}
	for _, child := range children {
		if controllerFieldNode(child, ctx) {
			continue
		}
		if isApexStructureTag(child, "pageBlockSectionItem") {
			values := panelGridCellNodes(child)
			if len(values) == 2 && simplePresentationValue(values[0], "outputText") && controllerFieldNode(values[1], ctx) {
				continue
			}
		}
		return false
	}
	return true
}

func renderControllerFieldSection(node *MarkupNode, ctx *RenderContext) (string, error) {
	columns := 2
	if raw := strings.TrimSpace(node.Attribute("columns")); raw != "" {
		columns = panelGridColumns(raw)
	}
	if columns < 1 {
		return renderApexContainer(node, "div", "pbSubsection", ctx)
	}
	var rows, row strings.Builder
	cells := 0
	for _, child := range panelGridCellNodes(node) {
		show, err := visualforceComponentShouldRender(child, ctx)
		if err != nil {
			return "", err
		}
		if !show {
			continue
		}
		field := child
		item := isApexStructureTag(child, "pageBlockSectionItem")
		var label string
		if item {
			values := panelGridCellNodes(child)
			label, err = renderMarkupNode(values[0], ctx)
			if err != nil {
				return "", err
			}
			field = values[1]
		} else {
			binding, _ := resolveFieldBinding(ctx, child.Attribute("value"))
			label = binding.Field.Label
			if child.HasAttribute("label") {
				label, err = renderInputFieldLabel(child.Attribute("label"), ctx.Expression)
				if err != nil {
					return "", err
				}
			}
			label = html.EscapeString(label)
			if isApexStructureTag(child, "inputField") && label != "" {
				if controllerFieldRequired(child, ctx, binding) {
					label = `<span>*</span>` + label
				}
				forAttr := ""
				if !fieldIsReadonly(binding.Field) && binding.Field.Formula == "" && !fieldIsRichText(binding.Field) {
					forAttr = ` for="` + html.EscapeString(visualforceComponentClientID(child, ctx)) + `"`
				}
				label = `<label` + forAttr + `>` + label + `</label>`
			}
		}
		previous := ctx.inputFieldSectionLabel
		ctx.inputFieldSectionLabel = false // The section owns the label cell.
		value, err := renderMarkupNode(field, ctx)
		ctx.inputFieldSectionLabel = previous
		if err != nil {
			return "", err
		}
		if label != "" {
			row.WriteString(`<th scope="row">` + label + `</th>`)
		}
		row.WriteString(`<td>` + value + `</td>`)
		cells++
		if cells%columns == 0 {
			rows.WriteString(`<tr>` + row.String() + `</tr>`)
			row.Reset()
		}
	}
	if row.Len() != 0 {
		for cells%columns != 0 {
			row.WriteString(presentationEmptyCells())
			cells++
		}
		rows.WriteString(`<tr>` + row.String() + `</tr>`)
	}
	body := ""
	if rows.Len() != 0 {
		body = `<tbody>` + rows.String() + `</tbody>`
	}
	// Native field sections use a distinct label/value table even when their
	// only field is hidden. Preserve its component identity for label association.
	return `<div` + fieldInputIDAttr(visualforceComponentClientID(node, ctx)) + `><div class="pbSubsection"><div><div><table class="detailList">` + body + `</table></div></div></div></div>`, nil
}
