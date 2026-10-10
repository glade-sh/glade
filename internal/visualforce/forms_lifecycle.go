package visualforce

import (
	"errors"
	"fmt"
	"html"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// formLifecycle keeps component-local submitted values separate from the Apex
// model. API59/67 form captures distinguish decoding, conversion, setter calls,
// immediate actions and rendering; an invalid value must not reach the setter.
type formLifecycle struct {
	allowAction     bool
	properties      map[string]string
	values          map[string]string
	submitted       map[*MarkupNode]string
	componentValues map[string]string
	preserveInputs  map[*MarkupNode]bool
	selectionErrors map[*MarkupNode]string
}

type formInput struct {
	node       *MarkupNode
	form       *MarkupNode
	region     *MarkupNode
	property   string
	name       string
	typeName   string
	disabled   bool
	required   bool
	repeated   bool
	repeatPath []string
}

// Command identity distinguishes repeated instances and ordinary commands that
// share an action but have different submission flags.
func formCommandKey(node *MarkupNode, ctx *RenderContext) string {
	return visualforceComponentClientID(node, ctx)
}

func ordinaryCommandSharesAction(node *MarkupNode, ctx *RenderContext) bool {
	if ctx.ComponentReferences == nil {
		return false
	}
	scope := ctx.ComponentReferences.scopes[node]
	if scope == nil || scope.container == ctx.ComponentReferences.root {
		return false
	}
	action := strings.TrimSpace(node.Attribute("action"))
	if action == "" {
		return false
	}
	for other, otherScope := range ctx.ComponentReferences.scopes {
		if other != node && otherScope.container == scope.container &&
			(isApexStructureTag(other, "commandButton") || isApexStructureTag(other, "commandLink")) &&
			strings.TrimSpace(other.Attribute("action")) == action {
			return true
		}
	}
	return false
}

func repeatedCommandSubmission(node *MarkupNode, ctx *RenderContext) (script, marker, hook string) {
	if len(ctx.repeatPath) == 0 && !ordinaryCommandSharesAction(node, ctx) {
		return "", "", ""
	}
	marker = ` data-vf-command="` + html.EscapeString(formCommandKey(node, ctx)) + `"`
	hook = `GLADEVF.selectCommand(this);`
	if ctx.repeatCommandScripts == nil {
		ctx.repeatCommandScripts = map[string]bool{}
	}
	scriptKey := node.Name
	ajax := strings.TrimSpace(node.Attribute("rerender")) != ""
	if ajax {
		scriptKey += ":ajax"
	}
	if !ctx.repeatCommandScripts[scriptKey] {
		ctx.repeatCommandScripts[scriptKey] = true
		script = `<script`
		if node.Name == "commandlink" {
			script += ` type="text/javascript"`
		}
		script += `>window.GLADEVF=window.GLADEVF||{};GLADEVF.selectCommand=function(e){var f=e.closest('form');if(!f)return;var c=f.elements['__vf_command'];if(!c){c=document.createElement('input');c.type='hidden';c.name='__vf_command';f.appendChild(c);}c.value=e.getAttribute('data-vf-command');};</script>`
		// Native repeated AJAX links are anchors from the first instance;
		// only full-postback links include an inline setup script there.
		if ajax && ctx.collectPresentationHead {
			ctx.presentationHead = append(ctx.presentationHead, script)
			script = ""
		}
	}
	return script, marker, hook
}

func formInputProperty(node *MarkupNode) string {
	if node == nil || node.Namespace != "apex" {
		return ""
	}
	switch node.Name {
	case "inputtext", "inputsecret", "inputtextarea", "inputhidden", "inputcheckbox":
	default:
		return ""
	}
	return formBoundProperty(node)
}

func formLifecycleProperty(node *MarkupNode) string {
	if formSelectionControl(node) {
		return formBoundProperty(node)
	}
	return formInputProperty(node)
}

func formBoundProperty(node *MarkupNode) string {
	raw := strings.TrimSpace(node.Attribute("value"))
	if !strings.HasPrefix(raw, "{!") || !strings.HasSuffix(raw, "}") {
		return ""
	}
	name := strings.TrimSpace(raw[2 : len(raw)-1])
	if !visualforceControllerMethodName.MatchString(name) {
		return ""
	}
	return name
}

// prepareFormLifecycle is the single RenderPage hook. Controllers are still
// constructed/restored and actions invoked by their existing shared paths.
// Source-free field fixtures and standard-controller records keep their
// existing binding path; accessor-backed inputs use the VM's typed setter API.
func prepareFormLifecycle(tree *MarkupNode, req *PageRenderRequest, machine *vm.VM, controller *vm.Value, expression *ExpressionContext) (*formLifecycle, error) {
	// Submission visibility uses the renderer's controller, extension and
	// standard-controller bindings, including the same iteration scope.
	refs := newComponentReferenceResolver(tree, nil)
	ctx := &RenderContext{VM: machine, Expression: expression, Scope: expression.Scope, ComponentReferences: refs}
	immediate := false
	var inputs []formInput
	var activeForm, activeRegion, actionNode *MarkupNode
	var activeRegionID string
	hasPartialCommand := false
	var walk func(*MarkupNode, *MarkupNode, *MarkupNode) error
	walk = func(node, form, region *MarkupNode) error {
		if node == nil {
			return nil
		}
		if node.Type == MarkupNodeElement && node.Namespace != "" && (req.ViewState != nil || req.FormValues != nil || !strings.Contains(node.Attribute("rendered"), "{!")) {
			visible, err := visualforceComponentShouldRender(node, ctx)
			if err != nil {
				return err
			}
			if !visible {
				return nil
			}
		}
		if isApexStructureTag(node, "form") {
			form, region = node, nil
		}
		if isApexStructureTag(node, "actionRegion") {
			region = node
		}
		if form != nil && (isApexStructureTag(node, "commandButton") || isApexStructureTag(node, "commandLink")) && strings.TrimSpace(node.Attribute("rerender")) != "" {
			hasPartialCommand = true
		}
		if form != nil && (isApexStructureTag(node, "commandButton") || isApexStructureTag(node, "commandLink")) &&
			strings.TrimSpace(node.Attribute("action")) == strings.TrimSpace(req.Action) && req.Action != "" {
			formID := req.FormValues["__vf_form"]
			commandID := req.FormValues["__vf_command"]
			if actionNode == nil && (formID == "" || refs.clientIDs[form] == formID) &&
				(commandID == "" || commandID == formCommandKey(node, ctx)) &&
				!isTruthyExpression(node.Attribute("disabled"), ctx) {
				actionNode, activeForm, activeRegion = node, form, region
				activeRegionID = visualforceComponentClientID(region, ctx)
				immediate = isTruthyExpression(node.Attribute("immediate"), ctx)
			}
		}
		if property := formLifecycleProperty(node); property != "" {
			typeName, writable, err := machine.InstancePropertyType(*controller, property)
			if err != nil {
				return err
			}
			selection := formSelectionControl(node)
			if writable && ((!selection && formLifecycleType(typeName)) || (selection && formSelectionType(typeName))) {
				name := visualforceComponentClientID(node, ctx)
				if selection {
					name = fieldName(node)
				}
				inputs = append(inputs, formInput{node: node, form: form, region: region, property: property, name: name, typeName: typeName, disabled: isTruthyExpression(node.Attribute("disabled"), ctx), required: isTruthyExpression(node.Attribute("required"), ctx), repeated: len(ctx.repeatPath) != 0, repeatPath: append([]string(nil), ctx.repeatPath...)})
			}
		}
		if isApexStructureTag(node, "repeat") && (req.ViewState != nil || req.FormValues != nil) {
			items, err := evaluateListExpression(node.Attribute("value"), ctx)
			if err != nil {
				return err
			}
			for i, item := range items {
				ctx.repeatPath = append(ctx.repeatPath, refs.clientIDs[node]+":"+strconv.Itoa(i))
				ctx.Scope.PushFrame()
				if name := strings.TrimSpace(node.Attribute("var")); name != "" {
					ctx.Scope.Set(name, item)
				}
				if name := strings.TrimSpace(node.Attribute("indexvar")); name != "" {
					ctx.Scope.Set(name, vm.Int(int64(i)))
				}
				for _, child := range node.Children {
					if err = walk(child, form, region); err != nil {
						break
					}
				}
				ctx.Scope.PopFrame()
				ctx.repeatPath = ctx.repeatPath[:len(ctx.repeatPath)-1]
				if err != nil {
					return err
				}
			}
			return nil
		}
		for _, child := range node.Children {
			if err := walk(child, form, region); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(tree, nil, nil); err != nil {
		return nil, err
	}
	if len(inputs) == 0 {
		return nil, nil
	}
	state := &formLifecycle{allowAction: true, properties: map[string]string{}, values: map[string]string{}, submitted: map[*MarkupNode]string{}, preserveInputs: map[*MarkupNode]bool{}}
	if hasPartialCommand {
		state.componentValues = map[string]string{}
	}
	if state.componentValues != nil && req.ViewState != nil {
		for key, value := range req.ViewState.ComponentState {
			if strings.HasPrefix(key, formComponentStatePrefix) {
				state.componentValues[strings.TrimPrefix(key, formComponentStatePrefix)] = value
			}
		}
	}
	for _, input := range inputs {
		state.properties[strings.ToLower(input.property)] = input.typeName
	}
	if req.ViewState == nil && req.FormValues == nil {
		return state, nil
	}
	if actionNode == nil {
		return nil, nil
	}
	selected := make([]formInput, 0, len(inputs))
	targets := formRerenderNodes(tree, ParseRerenderTargets(actionNode.Attribute("rerender")), refs)
	for _, input := range inputs {
		if len(targets) != 0 {
			state.preserveInputs[input.node] = true
			for _, target := range targets {
				if formRegionContains(target, input.node) {
					state.preserveInputs[input.node] = false
					break
				}
			}
		}
		if input.form == activeForm && (activeRegion == nil || formRegionContains(activeRegion, input.node)) {
			// A repeated region shares its markup node with every instance.
			// Match the actual client ID to exclude other rows, including their
			// validation getter reads, while retaining nested-region ancestry.
			inputCtx := &RenderContext{ComponentReferences: refs, repeatPath: input.repeatPath}
			if activeRegion != nil && visualforceComponentClientID(activeRegion, inputCtx) != activeRegionID {
				continue
			}
			selected = append(selected, input)
		}
	}
	values := req.FormValues
	// Typed inputs are assigned only by the selected shared phases. Remove
	// all their names from the legacy binder so a supplied field outside the
	// active form/region cannot bypass that boundary. Params and unrelated
	// bindings remain on the legacy path.
	remaining := make(map[string]string, len(values))
	for name, value := range values {
		remaining[name] = value
	}
	for _, input := range inputs {
		for name := range remaining {
			if strings.EqualFold(name, input.name) || strings.EqualFold(formFieldBindingName(name), inputFieldName(input.node)) {
				delete(remaining, name)
			}
		}
	}
	req.FormValues = remaining
	if immediate {
		for _, input := range selected {
			if raw, ok := formInputSubmittedValue(values, input); ok && !input.disabled && !input.repeated {
				state.submitted[input.node] = raw
			}
		}
		return state, nil
	}
	// Native setter_ab/ba/duplicate_*: read each distinct property once during
	// validation, then invoke every submitted component's setter in tree order.
	read := map[string]bool{}
	for _, input := range selected {
		key := strings.ToLower(input.property)
		if read[key] {
			continue
		}
		if _, _, err := machine.ReadInstanceProperty(*controller, input.property); err != nil {
			return nil, err
		}
		read[key] = true
	}
	for _, input := range selected {
		raw, supplied := formInputSubmittedValue(values, input)
		selection := formSelectionControl(input.node)
		if (!supplied && !selection) || input.disabled {
			continue
		}
		var converted vm.Value
		var message string
		if selection {
			var err error
			converted, message, err = formSelectionValue(raw, input.typeName, supplied, input.node, ctx, refs.clientIDs[input.node])
			if err != nil {
				return nil, err
			}
			if message != "" {
				if state.selectionErrors == nil {
					state.selectionErrors = map[*MarkupNode]string{}
				}
				state.selectionErrors[input.node] = message
			}
		} else {
			converted, message = formConvertedValue(raw, input.typeName, input.node.Name)
		}
		if message == "" && raw == "" && input.required {
			message = input.name + ": Validation Error: Value is required."
		}
		if message != "" {
			state.allowAction = false
			state.submitted[input.node] = raw
			formAddError(machine, message)
			continue
		}
		updated, err := machine.AssignInstanceProperty(*controller, input.property, converted)
		*controller = updated
		ctx.Expression.Controller = updated
		if err != nil {
			state.allowAction = false
			state.submitted[input.node] = raw
			message := err.Error()
			var runtimeErr *vm.RuntimeError
			if errors.As(err, &runtimeErr) {
				message = runtimeErr.Message
			}
			formAddError(machine, "common.apex.runtime.impl.ExecutionException: "+message)
		}
	}
	return state, nil
}

const formComponentStatePrefix = "form-input:"

func formComponentState(state *formLifecycle, previous *ViewStatePayload) map[string]string {
	values := map[string]string{}
	if previous != nil {
		for key, value := range previous.ComponentState {
			values[key] = value
		}
	}
	if state != nil {
		for key, value := range state.componentValues {
			values[formComponentStatePrefix+key] = value
		}
	}
	return values
}

func formRerenderNodes(tree *MarkupNode, targets []string, refs *componentReferenceResolver) []*MarkupNode {
	var nodes []*MarkupNode
	var walk func(*MarkupNode)
	walk = func(node *MarkupNode) {
		if node == nil {
			return
		}
		for _, target := range targets {
			id := refs.clientIDs[node]
			if node.Attribute("id") == target || id == target || strings.HasSuffix(id, ":"+target) {
				nodes = append(nodes, node)
				break
			}
		}
		for _, child := range node.Children {
			walk(child)
		}
	}
	walk(tree)
	return nodes
}

func formRegionContains(region, target *MarkupNode) bool {
	if region == target {
		return true
	}
	for _, child := range region.Children {
		if formRegionContains(child, target) {
			return true
		}
	}
	return false
}

func formLifecycleType(typeName string) bool {
	switch strings.ToLower(typeName) {
	case "string", "integer", "decimal", "date", "id", "boolean":
		return true
	default:
		return false
	}
}

// Native DOM uses client IDs for accessor-backed input names. Retain the
// legacy explicit/property name path for callers with an existing form payload.
func formInputSubmittedValue(values map[string]string, input formInput) (string, bool) {
	if value, ok := values[input.name]; ok {
		return value, true
	}
	return formSubmittedValue(values, inputFieldName(input.node))
}

func formRenderedInputName(node *MarkupNode, ctx *RenderContext) string {
	if ctx != nil && ctx.formLifecycle != nil && ctx.formLifecycle.properties[strings.ToLower(formInputProperty(node))] != "" {
		if name := visualforceComponentClientID(node, ctx); name != "" {
			return name
		}
	}
	return inputFieldName(node)
}

func formSubmittedValue(values map[string]string, name string) (string, bool) {
	if value, ok := values[name]; ok {
		return value, true
	}
	for key, value := range values {
		if strings.EqualFold(formFieldBindingName(key), name) {
			return value, true
		}
	}
	return "", false
}

func formConvertedValue(raw, typeName, component string) (vm.Value, string) {
	text := strings.TrimSpace(raw)
	conversionError := func(target string) (vm.Value, string) {
		return vm.Null, fmt.Sprintf("Value '%s' cannot be converted from Text to %s", raw, target)
	}
	switch strings.ToLower(typeName) {
	case "integer":
		if text == "" {
			return vm.Int(0), ""
		}
		value, err := strconv.ParseInt(text, 10, 32)
		if err != nil {
			return conversionError("Number")
		}
		return vm.Int(value), ""
	case "decimal":
		if text == "" {
			return vm.Decimal(0), ""
		}
		value, ok := parseVisualforceFormDecimal(text)
		if !ok || math.IsNaN(value.Decimal) || math.IsInf(value.Decimal, 0) {
			return conversionError("Number")
		}
		// The native form converter retains decimal scale and canonicalizes
		// exponent notation before invoking the typed Apex setter.
		if i := strings.IndexAny(text, "eE"); i >= 0 {
			if exponent, err := strconv.Atoi(text[i+1:]); err == nil {
				value.Text = fmt.Sprintf("%sE%+d", text[:i], exponent)
			}
		}
		return value, ""
	case "date":
		if text == "" {
			return vm.Null, ""
		}
		// Native inputText rejects even a valid ISO leap date before calling
		// the Date setter. Other components retain their existing converter.
		if component == "inputtext" {
			return conversionError("com.force.swag.soap.DateOnlyWrapper")
		}
		value, ok := parseVisualforceFormDate(text)
		if !ok {
			return conversionError("com.force.swag.soap.DateOnlyWrapper")
		}
		return value, ""
	case "id":
		if text != "" {
			if len(text) != 15 && len(text) != 18 {
				return vm.Null, fmt.Sprintf("id %s must be 15 characters", text)
			}
			if err := storage.ValidateID(storage.ID(text)); err != nil {
				return vm.Null, err.Error()
			}
		}
		// The form converter supplies a typed Id, including the native empty
		// Id value, rather than asking Apex to cast the submitted String.
		// Reuse the VM scalar display path for 15-to-18-character conversion.
		value := vmPlatformScalar("Id", text)
		value.Fields["value"] = vm.String(value.String())
		return value, ""
	case "boolean":
		value, ok := parseVisualforceFormBool(text)
		if !ok {
			return conversionError("Boolean")
		}
		return value, ""
	case "string":
		return vm.String(raw), ""
	default:
		return vm.String(raw), ""
	}
}

func formAddError(machine *vm.VM, summary string) {
	message := vm.Object("ApexPages.Message")
	message.Fields["severity"] = vm.String("ERROR")
	message.Fields["summary"], message.Fields["detail"] = vm.String(summary), vm.String(summary)
	context := machine.SnapshotVisualforcePageContext()
	context.PageMessages = append(context.PageMessages, message)
	machine.RestoreVisualforcePageContext(context)
}

func renderFormExpression(raw string, ctx *RenderContext) (string, error) {
	state := ctx.formLifecycle
	key := strings.TrimSpace(raw)
	if strings.HasPrefix(key, "{!") && strings.HasSuffix(key, "}") {
		key = strings.TrimSpace(key[2 : len(key)-1])
	}
	key = strings.ToLower(key)
	if state == nil || state.properties[key] == "" {
		return RenderExpressionTemplate(raw, ctx.Expression)
	}
	if value, ok := state.values[key]; ok {
		return value, nil
	}
	value, err := RenderExpressionTemplate(raw, ctx.Expression)
	if err == nil {
		state.values[key] = value
	}
	return value, err
}

func renderFormInputValue(node *MarkupNode, ctx *RenderContext) (value string, err error) {
	if state := ctx.formLifecycle; state != nil && state.componentValues != nil {
		id := visualforceComponentClientID(node, ctx)
		// Partial responses do not render input components outside the target.
		// Retain their component value without invoking their model getters.
		if state.preserveInputs[node] {
			if saved, ok := state.componentValues[id]; ok {
				return saved, nil
			}
		}
		defer func() {
			if err == nil {
				state.componentValues[id] = value
			}
		}()
	}
	if ctx.formLifecycle != nil {
		if value, ok := ctx.formLifecycle.submitted[node]; ok {
			return value, nil
		}
	}
	value, err = renderFormExpression(node.Attribute("value"), ctx)
	if err != nil {
		return "", err
	}
	if state := ctx.formLifecycle; state != nil && node.Name == "inputtext" &&
		strings.EqualFold(state.properties[strings.ToLower(formInputProperty(node))], "Date") {
		// Format the input only: model JSON and outputText retain the Date's
		// ISO representation, and submitted values above remain verbatim.
		if date, err := time.Parse("2006-01-02", value); err == nil {
			return date.Format("Mon Jan 02 15:04:05 GMT 2006"), nil
		}
	}
	return value, nil
}

func formBooleanAttrs(node *MarkupNode, ctx *RenderContext) string {
	var out strings.Builder
	for _, attribute := range []string{"disabled", "required"} {
		if isTruthyExpression(node.Attribute(attribute), ctx) {
			fmt.Fprintf(&out, ` %s="%s"`, attribute, attribute)
		}
	}
	return out.String()
}

func renderFormPageMessage(message vm.Value) string {
	rawSeverity := message.Fields["severity"]
	severity := rawSeverity.String()
	if rawSeverity.Kind == vm.ValueObject && strings.EqualFold(rawSeverity.Type, "ApexPages.Severity") && rawSeverity.Text != "" {
		severity = rawSeverity.Text
	}
	if severity == "" {
		severity = "INFO"
	}
	label := strings.ToUpper(severity[:1]) + strings.ToLower(severity[1:])
	return `<div class="message ` + html.EscapeString(strings.ToLower(severity)) + `"><span class="messageText">` + html.EscapeString(label+":"+message.Fields["summary"].String()) + `</span></div>`
}
