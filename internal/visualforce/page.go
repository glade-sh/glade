package visualforce

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

type RenderMetrics struct {
	ComponentCounts     map[string]int
	ExpressionEvals     int
	ExpressionCacheHits int
}

type PageRenderRequest struct {
	Project            project.Project
	VFIndex            Index
	Org                *storage.OrgState
	Machine            *vm.VM
	PageName           string
	PageURL            string
	ViewState          *ViewStatePayload
	FormValues         map[string]string
	Action             string
	Debug              bool
	LightningBootstrap *lwcbrowser.PageConfig
	ViewStateSecret    []byte
}

type PageRenderResult struct {
	HTML        string
	ViewState   string
	RenderAs    string
	RedirectURL string
	Redirect    bool
	Metrics     RenderMetrics
	Error       *RenderError
}

type RenderError struct {
	Message string
	File    string
	Line    int
	Column  int
	Expr    string
}

func (e *RenderError) Error() string {
	if e == nil {
		return ""
	}
	if e.File != "" {
		return fmt.Sprintf("%s (%s:%d)", e.Message, e.File, e.Line)
	}
	return e.Message
}

func RenderPage(req PageRenderRequest) (PageRenderResult, error) {
	pageMeta, ok := req.VFIndex.Page(req.PageName)
	if !ok {
		return PageRenderResult{}, fmt.Errorf("unknown Visualforce page %q", req.PageName)
	}
	markup, err := os.ReadFile(pageMeta.File)
	if err != nil {
		return PageRenderResult{}, fmt.Errorf("read page markup: %w", err)
	}
	tree, err := ParseMarkupTree(string(markup))
	if err != nil {
		return PageRenderResult{}, fmt.Errorf("parse markup: %w", err)
	}
	renderAs := pageRenderAs(tree)
	machine := req.Machine
	if machine == nil {
		machine = vm.New(nil)
	}
	if req.Org != nil {
		machine.Org = req.Org
	}
	namespace := strings.TrimSpace(req.Project.Namespace)
	if namespace == "" && req.Org != nil {
		namespace = strings.TrimSpace(req.Org.Namespace)
	}
	if namespace != "" {
		machine.SetCurrentNamespace(namespace)
	}
	pageURL := strings.TrimSpace(req.PageURL)
	if pageURL == "" {
		pageURL = "/apex/" + req.PageName
	}
	machine.SetCurrentPageURL(pageURL)
	if err := validateViewStateForPage(pageMeta, req.PageName, req.ViewState); err != nil {
		return PageRenderResult{}, err
	}

	savedPageMessages := viewStatePageMessages(req.ViewState)
	controller, extensions, stdController, err := bootstrapControllers(machine, pageMeta, req.ViewState)
	if err != nil {
		return PageRenderResult{}, err
	}
	allowedFormFields := VisualforceFormFieldNames(tree)
	if len(req.FormValues) > 0 {
		savedPageMessages = mergePageMessages(savedPageMessages, applyFormValues(controller, req.FormValues, allowedFormFields))
		savedPageMessages = mergePageMessages(savedPageMessages, applyStandardControllerFormValues(&stdController, req.FormValues, allowedFormFields, machine))
	}
	actionParams := visualforceActionParams(req.FormValues, allowedFormFields)
	machine.SetVisualforceActionInvoker(func(actionExpr string, actionPageURL string) (vm.Value, error) {
		if strings.TrimSpace(actionPageURL) == "" {
			actionPageURL = pageURL
		}
		actionName := actionMethodName(actionExpr)
		value, result, err := invokeVisualforceAction(machine, controller, extensions, stdController, pageMeta, actionName, actionPageURL, actionParams)
		if err != nil {
			return vm.Null, err
		}
		if result.Error != nil {
			return vm.Null, vm.UnsupportedFeature(result.Error.Message)
		}
		return value, nil
	})
	defer machine.ClearVisualforceActionInvoker()
	var redirectURL string
	var redirect bool
	if strings.TrimSpace(req.Action) == "" && strings.TrimSpace(pageMeta.Action) != "" {
		value, result, err := invokeVisualforceAction(machine, controller, extensions, stdController, pageMeta, actionMethodName(pageMeta.Action), pageURL, actionParams)
		if err != nil {
			return PageRenderResult{}, err
		}
		if result.Error != nil {
			return PageRenderResult{}, vm.UnsupportedFeature(result.Error.Message)
		}
		if navURL, shouldRedirect, ok := pageReferenceNavigation(value); ok {
			if shouldRedirect {
				redirectURL = navURL
				redirect = true
			} else if targetPage := apexPageNameFromURL(navURL); targetPage != "" && !strings.EqualFold(targetPage, req.PageName) {
				nextReq := req
				nextReq.PageName = targetPage
				nextReq.PageURL = navURL
				nextReq.Action = ""
				nextReq.FormValues = nil
				nextReq.ViewState = nil
				return RenderPage(nextReq)
			}
		}
	} else if strings.TrimSpace(req.Action) != "" {
		value, result, err := invokeVisualforceAction(machine, controller, extensions, stdController, pageMeta, actionMethodName(req.Action), pageURL, actionParams)
		if err != nil {
			return PageRenderResult{}, err
		}
		if result.Error != nil {
			return PageRenderResult{}, vm.UnsupportedFeature(result.Error.Message)
		}
		if navURL, shouldRedirect, ok := pageReferenceNavigation(value); ok {
			if shouldRedirect {
				redirectURL = navURL
				redirect = true
			} else if targetPage := apexPageNameFromURL(navURL); targetPage != "" && !strings.EqualFold(targetPage, req.PageName) {
				nextReq := req
				nextReq.PageName = targetPage
				nextReq.PageURL = navURL
				nextReq.Action = ""
				nextReq.FormValues = nil
				nextReq.ViewState = nil
				return RenderPage(nextReq)
			}
		}
	}
	refreshStandardSetControllerExposure(&stdController, pageMeta.RecordSetVar)

	exprCtx := &ExpressionContext{
		VM:                 machine,
		Controller:         controller,
		Extensions:         extensions,
		StandardController: stdController,
		CurrentPage:        machine.CurrentPage(),
		ProjectNamespace:   namespace,
	}
	metrics := RenderMetrics{ComponentCounts: make(map[string]int)}
	renderCtx := &RenderContext{
		VM:                 machine,
		PageName:           req.PageName,
		PageURL:            pageURL,
		PageMeta:           pageMeta,
		VFIndex:            &req.VFIndex,
		Project:            req.Project,
		Expression:         exprCtx,
		Scope:              NewScopeStack(),
		Defines:            make(map[string]*MarkupNode),
		Metrics:            &metrics,
		Debug:              req.Debug,
		LightningBootstrap: req.LightningBootstrap,
	}
	rendered, err := RenderMarkupTree(tree, renderCtx)
	if err != nil {
		out := PageRenderResult{Metrics: metrics, Error: &RenderError{Message: err.Error(), File: pageMeta.File}}
		if req.Debug {
			out.HTML = renderErrorOverlay(out.Error)
		}
		return out, err
	}
	currentPageMessages := pageMessagesToStrings(machine)
	rendered = injectViewStatePageMessages(rendered, missingPageMessages(savedPageMessages, currentPageMessages))
	payload := ViewStatePayload{
		Version:          CurrentViewStateVersion,
		PageName:         req.PageName,
		ControllerType:   pageMeta.Controller,
		ControllerValues: valueFieldsToViewStateValues(machine, pageMeta.Controller, controller),
		ControllerFields: valueFieldsToStrings(machine, pageMeta.Controller, controller),
		ExtensionValues:  extensionFieldsToViewStateValues(machine, pageMeta.Extensions, extensions),
		ExtensionFields:  extensionFieldsToStrings(machine, pageMeta.Extensions, extensions),
		ComponentState:   map[string]string{},
		PageMessages:     mergePageMessages(savedPageMessages, currentPageMessages),
	}
	if req.ViewState != nil {
		payload.CSRF = req.ViewState.CSRF
	}
	if err := ensureViewStateCSRF(&payload); err != nil {
		return PageRenderResult{}, err
	}
	encoded, err := EncodeViewState(payload, req.ViewStateSecret)
	if err != nil {
		return PageRenderResult{}, err
	}
	if err := CheckVisualforceViewStateSize(len(encoded)); err != nil {
		return PageRenderResult{Metrics: metrics, Error: &RenderError{Message: err.Error(), File: pageMeta.File}}, err
	}
	finalHTML := InjectCSRF(InjectViewState(rendered, encoded), payload.CSRF)
	return PageRenderResult{HTML: finalHTML, ViewState: encoded, RenderAs: renderAs, RedirectURL: redirectURL, Redirect: redirect, Metrics: metrics}, nil
}

func validateViewStateForPage(page Page, pageName string, payload *ViewStatePayload) error {
	if payload == nil {
		return nil
	}
	if strings.TrimSpace(payload.PageName) != "" && !strings.EqualFold(strings.TrimSpace(payload.PageName), strings.TrimSpace(pageName)) {
		return fmt.Errorf("view state page mismatch")
	}
	if strings.TrimSpace(payload.ControllerType) != "" && !strings.EqualFold(strings.TrimSpace(payload.ControllerType), strings.TrimSpace(page.Controller)) {
		return fmt.Errorf("view state controller mismatch")
	}
	return nil
}

func invokeVisualforceAction(machine *vm.VM, controller vm.Value, extensions []vm.Value, stdController vm.Value, page Page, actionName string, pageURL string, params map[string]string) (vm.Value, vm.UIInvocationResult, error) {
	if strings.TrimSpace(actionName) == "" {
		return vm.Null, vm.UIInvocationResult{Success: true}, nil
	}
	type actionCandidate struct {
		className  string
		controller vm.Value
		apply      func(vm.Value)
	}
	candidates := []actionCandidate{
		{className: page.Controller, controller: controller, apply: func(updated vm.Value) { controller = updated }},
	}
	for i, extName := range page.Extensions {
		if i < len(extensions) {
			idx := i
			candidates = append(candidates, actionCandidate{
				className:  extName,
				controller: extensions[i],
				apply: func(updated vm.Value) {
					extensions[idx] = updated
				},
			})
		}
	}
	if page.StandardController != "" {
		className := "ApexPages.StandardController"
		if strings.TrimSpace(stdController.Type) != "" {
			className = stdController.Type
		}
		candidates = append(candidates, actionCandidate{
			className:  className,
			controller: stdController,
			apply: func(updated vm.Value) {
				stdController = updated
			},
		})
	}
	var lastValue vm.Value
	var lastResult vm.UIInvocationResult
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.className) == "" || candidate.controller.Kind != vm.ValueObject {
			continue
		}
		value, updated, result, err := machine.InvokeVisualforceActionOnController(candidate.controller, candidate.className, actionName, pageURL, params)
		if err != nil {
			return value, result, err
		}
		if candidate.apply != nil {
			candidate.apply(updated)
		}
		lastValue = value
		lastResult = result
		if result.Error == nil {
			return value, result, nil
		}
		if !visualforceActionCandidateMissing(result) {
			return value, result, nil
		}
	}
	if lastResult.Error != nil {
		return lastValue, lastResult, nil
	}
	return vm.Null, vm.UIInvocationResult{
		Framework:  "visualforce",
		MethodName: actionName,
		Success:    false,
		Error:      &vm.UIActionError{Type: "UnsupportedFeature", Message: "Visualforce action requires controller, extension, or standard controller method " + actionName},
	}, nil
}

func visualforceActionCandidateMissing(result vm.UIInvocationResult) bool {
	if result.Error == nil {
		return false
	}
	if result.Error.Type != "" && !strings.EqualFold(result.Error.Type, "UnsupportedFeature") {
		return false
	}
	message := strings.TrimSpace(result.Error.Message)
	return strings.HasPrefix(message, "no instance Visualforce action ") ||
		strings.HasPrefix(message, "no standard Visualforce action ") ||
		strings.HasPrefix(message, "Visualforce action requires ")
}

func pageReferenceNavigation(value vm.Value) (string, bool, bool) {
	if value.Kind != vm.ValueObject || !strings.EqualFold(value.Type, "PageReference") {
		return "", false, false
	}
	urlValue, ok := value.Fields["url"]
	if !ok || urlValue.Kind != vm.ValueString || strings.TrimSpace(urlValue.Text) == "" {
		return "", false, false
	}
	redirect := false
	if redirectValue, ok := value.Fields["redirect"]; ok && redirectValue.Kind == vm.ValueBool {
		redirect = redirectValue.Bool
	}
	return urlValue.Text, redirect, true
}

func RenderPageURL(machine *vm.VM, pageURL string, asPDF bool) (vm.Value, error) {
	ctx := context.Background()
	if machine == nil {
		return vm.Null, fmt.Errorf("Visualforce page render requires VM")
	}
	savedPageContext := machine.SnapshotVisualforcePageContext()
	defer machine.RestoreVisualforcePageContext(savedPageContext)
	if asPDF && renderEnvironmentFromVM(machine).Project.Root == "" {
		pdfBytes, err := renderPDF(ctx, "", pageURL)
		if err != nil {
			return vm.Null, err
		}
		if err := CheckVisualforcePDFSize(len(pdfBytes)); err != nil {
			return vm.Null, err
		}
		return vm.NewBlobValue(string(pdfBytes)), nil
	}
	pageName := pageNameFromURL(pageURL)
	if pageName == "" {
		return vm.Null, fmt.Errorf("invalid Visualforce page URL %q", pageURL)
	}
	env := renderEnvironmentFromVM(machine)
	if env.Project.Root == "" {
		return vm.Null, vm.UnsupportedFeature("PageReference.getContent local Visualforce page rendering surface")
	}
	idx, err := LoadProject(env.Project)
	if err != nil {
		return vm.Null, err
	}
	result, err := RenderPage(PageRenderRequest{
		Project:  env.Project,
		VFIndex:  idx,
		Org:      machine.Org,
		Machine:  machine,
		PageName: pageName,
		PageURL:  pageURL,
	})
	if err != nil {
		return vm.Null, err
	}
	if asPDF || strings.EqualFold(strings.TrimSpace(result.RenderAs), "pdf") {
		if err := CheckVisualforcePDFHTMLResponseSize(len(result.HTML)); err != nil {
			return vm.Null, err
		}
		pdfBytes, err := renderPDF(ctx, result.HTML, pageURL)
		if err != nil {
			return vm.Null, err
		}
		if err := CheckVisualforcePDFSize(len(pdfBytes)); err != nil {
			return vm.Null, err
		}
		return vm.NewBlobValue(string(pdfBytes)), nil
	}
	return vm.NewBlobValue(result.HTML), nil
}

func RenderPageForTest(machine *vm.VM, projectRoot, pageName string) (string, error) {
	p, err := project.Load(projectRoot)
	if err != nil {
		return "", err
	}
	idx, err := LoadProject(p)
	if err != nil {
		return "", err
	}
	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      machine.Org,
		Machine:  machine,
		PageName: pageName,
	})
	if err != nil {
		return "", err
	}
	return result.HTML, nil
}

func pageNameFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Path != "" {
		rawURL = parsed.Path
	}
	rawURL = strings.TrimPrefix(rawURL, "/")
	rawURL = strings.TrimPrefix(rawURL, "apex/")
	return strings.Trim(rawURL, "/")
}

func apexPageNameFromURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err == nil && parsed.Path != "" {
		rawURL = parsed.Path
	}
	rawURL = strings.Trim(rawURL, "/")
	if !strings.HasPrefix(strings.ToLower(rawURL), "apex/") {
		return ""
	}
	return strings.Trim(rawURL[len("apex/"):], "/")
}

func pageRenderAs(root *MarkupNode) string {
	if root == nil {
		return ""
	}
	if root.Type == MarkupNodeElement && strings.EqualFold(root.Namespace, "apex") && strings.EqualFold(root.Name, "page") {
		return strings.TrimSpace(root.Attribute("renderAs"))
	}
	for _, child := range root.Children {
		if renderAs := pageRenderAs(child); renderAs != "" {
			return renderAs
		}
	}
	return ""
}

func bootstrapControllers(machine *vm.VM, page Page, saved *ViewStatePayload) (vm.Value, []vm.Value, vm.Value, error) {
	var controller vm.Value
	var extensions []vm.Value
	var stdController vm.Value
	if page.Controller != "" {
		constructed, err := machine.ConstructController(page.Controller)
		if err != nil {
			return vm.Null, nil, vm.Null, err
		}
		controller = constructed
		if saved != nil && saved.ControllerValues != nil {
			applyValueFields(&controller, saved.ControllerValues)
		} else if saved != nil && saved.ControllerFields != nil {
			applyStringFields(&controller, saved.ControllerFields)
		}
	}
	if page.StandardController != "" {
		if strings.TrimSpace(page.RecordSetVar) != "" {
			stdController = standardSetController(machine, page.StandardController, page.RecordSetVar)
		} else {
			record := standardControllerRecord(machine, page.StandardController)
			stdController = vm.Object("ApexPages.StandardController")
			stdController.Fields["record"] = record
			stdController.Fields["__glade_visualforce_context"] = vm.Bool(true)
		}
	}
	for i, extName := range page.Extensions {
		ext, err := constructVisualforceExtension(machine, extName, controller, stdController)
		if err != nil {
			return vm.Null, nil, vm.Null, err
		}
		bindStandardSetControllerExtensionFields(machine, &ext, extName, stdController)
		if saved != nil && i < len(saved.ExtensionValues) && saved.ExtensionValues[i] != nil {
			applyValueFields(&ext, saved.ExtensionValues[i])
		} else if saved != nil && i < len(saved.ExtensionFields) && saved.ExtensionFields[i] != nil {
			applyStringFields(&ext, saved.ExtensionFields[i])
		}
		extensions = append(extensions, ext)
	}
	return controller, extensions, stdController, nil
}

func constructVisualforceExtension(machine *vm.VM, extName string, controller vm.Value, stdController vm.Value) (vm.Value, error) {
	if arg, ok := visualforceExtensionConstructorArg(machine, extName, controller, stdController); ok {
		return machine.ConstructControllerWithArgs(extName, []vm.Value{arg})
	}
	return machine.ConstructController(extName)
}

func visualforceExtensionConstructorArg(machine *vm.VM, extName string, controller vm.Value, stdController vm.Value) (vm.Value, bool) {
	class, ok := visualforceVMClass(machine, extName)
	if !ok {
		return vm.Null, false
	}
	for _, constructor := range class.Constructors {
		if len(constructor.Params) != 1 {
			continue
		}
		paramType := strings.TrimSpace(constructor.Params[0].Type)
		if visualforceConstructorParamMatches(paramType, stdController) {
			return stdController, true
		}
		if visualforceConstructorParamMatches(paramType, controller) {
			return controller, true
		}
	}
	return vm.Null, false
}

func visualforceConstructorParamMatches(paramType string, value vm.Value) bool {
	if strings.TrimSpace(paramType) == "" || value.Kind != vm.ValueObject || strings.TrimSpace(value.Type) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(paramType), strings.TrimSpace(value.Type))
}

func standardSetController(machine *vm.VM, objectName, recordSetVar string) vm.Value {
	records := standardSetControllerRecords(machine, objectName)
	controller := vm.Object("ApexPages.StandardSetController")
	controller.Fields["records"] = records
	controller.Fields["selected"] = vm.List()
	controller.Fields["pageSize"] = vm.Int(20)
	controller.Fields["pageNumber"] = vm.Int(1)
	exposeStandardSetControllerFields(&controller, recordSetVar)
	return controller
}

func standardSetControllerRecords(machine *vm.VM, objectName string) vm.Value {
	records := vm.List()
	objectKey := strings.TrimSpace(objectName)
	if machine == nil || machine.Org == nil || objectKey == "" {
		return records
	}
	resolvedObject, ok := storage.ResolveObjectName(*machine.Org, objectKey)
	if !ok {
		return records
	}
	object := machine.Org.Objects[resolvedObject]
	ids := make([]string, 0, len(object.Records))
	for id := range object.Records {
		ids = append(ids, string(id))
	}
	sort.Strings(ids)
	values := make([]vm.Value, 0, len(ids))
	for _, id := range ids {
		values = append(values, vmValueFromStorageRecord(object.Records[storage.ID(id)]))
	}
	return vm.List(values...)
}

func bindStandardSetControllerExtensionFields(machine *vm.VM, extension *vm.Value, extensionName string, stdController vm.Value) {
	if machine == nil || extension == nil || extension.Kind != vm.ValueObject || !strings.EqualFold(stdController.Type, "ApexPages.StandardSetController") {
		return
	}
	class, ok := visualforceVMClass(machine, extensionName, extension.Type)
	if !ok {
		return
	}
	for name, field := range class.Fields {
		if !strings.EqualFold(field.Type, "ApexPages.StandardSetController") {
			continue
		}
		fieldName := strings.TrimSpace(field.Name)
		if fieldName == "" {
			fieldName = name
		}
		extension.Fields[fieldName] = stdController
	}
}

func visualforceVMClass(machine *vm.VM, names ...string) (vm.Class, bool) {
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		for key, class := range machine.Classes {
			if strings.EqualFold(key, name) || strings.EqualFold(class.Name, name) {
				return class, true
			}
		}
	}
	return vm.Class{}, false
}

func exposeStandardSetControllerFields(controller *vm.Value, recordSetVar string) {
	if controller == nil || !strings.EqualFold(controller.Type, "ApexPages.StandardSetController") {
		return
	}
	records := controller.Fields["records"]
	if records.Kind != vm.ValueList {
		records = vm.List()
	}
	currentPage := standardSetCurrentPageRecords(*controller, records)
	if name := strings.TrimSpace(recordSetVar); name != "" {
		controller.Fields[name] = currentPage
	}
	controller.Fields["resultSize"] = vm.Int(int64(len(records.List)))
	controller.Fields["hasNext"] = vm.Bool(standardSetPageNumber(*controller) < standardSetPageCount(*controller, records))
	controller.Fields["hasPrevious"] = vm.Bool(standardSetPageNumber(*controller) > 1)
}

func refreshStandardSetControllerExposure(controller *vm.Value, recordSetVar string) {
	if controller == nil || !strings.EqualFold(controller.Type, "ApexPages.StandardSetController") {
		return
	}
	exposeStandardSetControllerFields(controller, recordSetVar)
}

func standardSetCurrentPageRecords(controller, records vm.Value) vm.Value {
	if records.Kind != vm.ValueList {
		return vm.List()
	}
	pageSize := standardSetPageSize(controller)
	pageNumber := standardSetPageNumber(controller)
	start := (pageNumber - 1) * pageSize
	if start >= len(records.List) {
		return vm.List()
	}
	end := start + pageSize
	if end > len(records.List) {
		end = len(records.List)
	}
	return vm.List(records.List[start:end]...)
}

func standardSetPageSize(controller vm.Value) int {
	if value := controller.Fields["pageSize"]; value.Kind == vm.ValueInt && value.Int > 0 {
		return int(value.Int)
	}
	return 20
}

func standardSetPageNumber(controller vm.Value) int {
	if value := controller.Fields["pageNumber"]; value.Kind == vm.ValueInt && value.Int > 0 {
		return int(value.Int)
	}
	return 1
}

func standardSetPageCount(controller, records vm.Value) int {
	if records.Kind != vm.ValueList || len(records.List) == 0 {
		return 1
	}
	pages := (len(records.List) + standardSetPageSize(controller) - 1) / standardSetPageSize(controller)
	if pages < 1 {
		return 1
	}
	return pages
}

func standardControllerRecord(machine *vm.VM, objectName string) vm.Value {
	objectKey := strings.TrimSpace(objectName)
	record := vm.Object(objectKey)
	if machine == nil || machine.Org == nil {
		return record
	}
	resolvedObject, ok := storage.ResolveObjectName(*machine.Org, objectKey)
	if !ok {
		return record
	}
	record = vm.Object(resolvedObject)
	object := machine.Org.Objects[resolvedObject]
	if recordID, ok := pageParameterString(machine.CurrentPage(), "id"); ok {
		if _, stored, found := storage.LookupRecordByID(object.Records, storage.ID(recordID)); found {
			return vmValueFromStorageRecord(stored)
		}
	}
	return record
}

func applyFormValues(controller vm.Value, values map[string]string, allowedFields map[string]bool) []string {
	if controller.Kind != vm.ValueObject {
		return nil
	}
	var diagnostics []string
	for _, binding := range VisualforceFormBindingsForFields(values, allowedFields) {
		value, diagnostic := visualforceTypedFormValueWithDiagnostic(binding.Value, controller.Fields[binding.FieldName], nil, binding.FieldName)
		controller.Fields[binding.FieldName] = value
		if diagnostic != nil {
			diagnostics = append(diagnostics, diagnostic.Message)
		}
	}
	return diagnostics
}

func applyStandardControllerFormValues(controller *vm.Value, values map[string]string, allowedFields map[string]bool, machine *vm.VM) []string {
	if controller == nil || controller.Kind != vm.ValueObject || len(values) == 0 {
		return nil
	}
	record, ok := controller.Fields["record"]
	if !ok || record.Kind != vm.ValueObject {
		return nil
	}
	if record.Fields == nil {
		record.Fields = make(map[string]vm.Value)
	}
	var diagnostics []string
	for _, binding := range VisualforceFormBindingsForFields(values, allowedFields) {
		field := visualforceFormFieldSchema(machine, record, binding.FieldName)
		value, diagnostic := visualforceTypedFormValueWithDiagnostic(binding.Value, record.Fields[binding.FieldName], field, binding.FieldName)
		record.Fields[binding.FieldName] = value
		if diagnostic != nil {
			diagnostics = append(diagnostics, diagnostic.Message)
		}
	}
	controller.Fields["record"] = record
	return diagnostics
}

func visualforceFormFieldSchema(machine *vm.VM, record vm.Value, fieldName string) *storage.Field {
	if machine == nil || machine.Org == nil || record.Kind != vm.ValueObject || strings.TrimSpace(record.Type) == "" {
		return nil
	}
	objectName, ok := storage.ResolveObjectName(*machine.Org, record.Type)
	if !ok {
		return nil
	}
	state := machine.Org.Objects[objectName]
	resolvedField, ok := storage.ResolveFieldName(state.Definition, machine.Org.Namespace, fieldName)
	if !ok {
		return nil
	}
	field := state.Definition.Fields[resolvedField]
	return &field
}

func vmValueFromStorageRecord(record storage.Record) vm.Value {
	value := vm.Object(record.Object)
	if record.ID != "" {
		value.Fields["Id"] = vmPlatformScalar("Id", string(record.ID))
	}
	for fieldName, fieldValue := range record.Fields {
		putVMFieldPath(value, fieldName, vmValueFromStorageValue(fieldValue))
	}
	return value
}

func vmValueFromStorageValue(value storage.Value) vm.Value {
	switch value.Kind {
	case storage.ValueNull:
		return vm.Null
	case storage.ValueString:
		return vm.String(value.String)
	case storage.ValueDate:
		return vmPlatformScalar("Date", value.String)
	case storage.ValueDateTime:
		return vmPlatformScalar("Datetime", value.String)
	case storage.ValueBlob:
		return vmPlatformScalar("Blob", value.String)
	case storage.ValueID:
		return vmPlatformScalar("Id", string(value.ID))
	case storage.ValueInteger:
		return vm.Int(value.Integer)
	case storage.ValueBoolean:
		return vm.Bool(value.Boolean)
	case storage.ValueDecimal:
		parsed, err := strconv.ParseFloat(value.Decimal, 64)
		if err != nil {
			return vm.String(value.Decimal)
		}
		out := vm.Decimal(parsed)
		out.Text = value.Decimal
		return out
	case storage.ValueList:
		values := make([]vm.Value, 0, len(value.List))
		for _, item := range value.List {
			values = append(values, vmValueFromStorageValue(item))
		}
		return vm.List(values...)
	default:
		return vm.Null
	}
}

func vmPlatformScalar(typeName, text string) vm.Value {
	value := vm.Object(typeName)
	value.Fields["value"] = vm.String(text)
	return value
}

func putVMFieldPath(root vm.Value, field string, fieldValue vm.Value) {
	if !strings.Contains(field, ".") {
		root.Fields[field] = fieldValue
		return
	}
	parts := strings.Split(field, ".")
	current := root
	for _, part := range parts[:len(parts)-1] {
		next, ok := current.Fields[part]
		if !ok || next.Kind != vm.ValueObject {
			next = vm.Object(part)
			current.Fields[part] = next
		}
		current = next
	}
	current.Fields[parts[len(parts)-1]] = fieldValue
}

func formFieldBindingName(key string) string {
	key = strings.TrimSpace(key)
	if key == "" {
		return ""
	}
	if strings.Contains(key, ":") {
		parts := strings.Split(key, ":")
		key = strings.TrimSpace(parts[len(parts)-1])
	}
	if strings.HasPrefix(key, "{!") && strings.HasSuffix(key, "}") {
		key = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(key, "{!"), "}"))
	}
	if strings.Contains(key, ".") {
		parts := strings.Split(key, ".")
		key = strings.TrimSpace(parts[len(parts)-1])
	}
	return key
}

func actionMethodName(action string) string {
	action = strings.TrimSpace(action)
	if strings.HasPrefix(action, "{!") && strings.HasSuffix(action, "}") {
		action = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(action, "{!"), "}"))
	}
	if strings.HasPrefix(action, "$Action.") {
		parts := strings.Split(action, ".")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	return strings.TrimSpace(action)
}

func visualforceActionParams(values map[string]string, allowedFields map[string]bool) map[string]string {
	if len(values) == 0 {
		return nil
	}
	bindings := VisualforceFormBindingsForFields(values, allowedFields)
	out := make(map[string]string, len(bindings))
	for _, binding := range bindings {
		out[binding.FieldName] = binding.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func applyStringFields(target *vm.Value, fields map[string]string) {
	if target == nil || target.Kind != vm.ValueObject {
		return
	}
	for key, raw := range fields {
		target.Fields[key] = vm.String(raw)
	}
}

func applyValueFields(target *vm.Value, fields map[string]vm.Value) {
	if target == nil || target.Kind != vm.ValueObject {
		return
	}
	if target.Fields == nil {
		target.Fields = make(map[string]vm.Value)
	}
	for key, value := range fields {
		target.Fields[key] = restoreViewStateValue(value)
	}
}

func restoreViewStateValue(value vm.Value) vm.Value {
	switch value.Kind {
	case vm.ValueNull, vm.ValueInt, vm.ValueDecimal, vm.ValueBool, vm.ValueString, vm.ValueList, vm.ValueSet, vm.ValueMap, vm.ValueObject:
		return value
	default:
		return vm.String(value.String())
	}
}

func valueFieldsToStrings(machine *vm.VM, className string, value vm.Value) map[string]string {
	out := make(map[string]string)
	if value.Kind != vm.ValueObject {
		return out
	}
	for key, field := range value.Fields {
		if visualforceFieldIsTransient(machine, className, key) {
			continue
		}
		if field.Kind == vm.ValueString {
			out[key] = field.Text
		} else if field.Kind != vm.ValueNull {
			out[key] = field.String()
		}
	}
	return out
}

func valueFieldsToViewStateValues(machine *vm.VM, className string, value vm.Value) map[string]vm.Value {
	out := make(map[string]vm.Value)
	if value.Kind != vm.ValueObject {
		return out
	}
	for key, field := range value.Fields {
		if visualforceFieldIsTransient(machine, className, key) {
			continue
		}
		if field.Kind == vm.ValueNull {
			continue
		}
		out[key] = field
	}
	return out
}

func extensionFieldsToStrings(machine *vm.VM, classNames []string, values []vm.Value) []map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make([]map[string]string, len(values))
	for i, value := range values {
		out[i] = valueFieldsToStrings(machine, viewStateClassName(classNames, i, value), value)
	}
	return out
}

func extensionFieldsToViewStateValues(machine *vm.VM, classNames []string, values []vm.Value) []map[string]vm.Value {
	if len(values) == 0 {
		return nil
	}
	out := make([]map[string]vm.Value, 0, len(values))
	for i, value := range values {
		out = append(out, valueFieldsToViewStateValues(machine, viewStateClassName(classNames, i, value), value))
	}
	return out
}

func viewStateClassName(classNames []string, index int, value vm.Value) string {
	if index >= 0 && index < len(classNames) && strings.TrimSpace(classNames[index]) != "" {
		return classNames[index]
	}
	return value.Type
}

func visualforceFieldIsTransient(machine *vm.VM, className string, fieldName string) bool {
	class, ok := visualforceVMClass(machine, className)
	if !ok {
		return false
	}
	for key, field := range class.Fields {
		if !strings.EqualFold(key, fieldName) && !strings.EqualFold(field.Name, fieldName) {
			continue
		}
		for _, modifier := range field.Modifiers {
			if strings.EqualFold(modifier, "transient") {
				return true
			}
		}
		return false
	}
	return false
}

func pageMessagesToStrings(machine *vm.VM) []string {
	if machine == nil {
		return nil
	}
	messages := machine.PageMessages()
	out := make([]string, 0, len(messages))
	for _, message := range messages {
		if message.Kind == vm.ValueObject {
			if summary, ok := message.Fields["summary"]; ok {
				out = append(out, summary.String())
				continue
			}
		}
		out = append(out, message.String())
	}
	return out
}

func viewStatePageMessages(payload *ViewStatePayload) []string {
	if payload == nil || len(payload.PageMessages) == 0 {
		return nil
	}
	return append([]string(nil), payload.PageMessages...)
}

func mergePageMessages(saved []string, current []string) []string {
	if len(saved) == 0 && len(current) == 0 {
		return nil
	}
	out := make([]string, 0, len(saved)+len(current))
	seen := make(map[string]bool, len(saved)+len(current))
	for _, message := range append(append([]string(nil), saved...), current...) {
		key := strings.TrimSpace(message)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, message)
	}
	return out
}

func missingPageMessages(saved []string, current []string) []string {
	if len(saved) == 0 {
		return nil
	}
	currentSet := make(map[string]bool, len(current))
	for _, message := range current {
		key := strings.TrimSpace(message)
		if key != "" {
			currentSet[key] = true
		}
	}
	missing := make([]string, 0, len(saved))
	for _, message := range saved {
		key := strings.TrimSpace(message)
		if key == "" || currentSet[key] {
			continue
		}
		missing = append(missing, message)
	}
	return missing
}

func injectViewStatePageMessages(rendered string, messages []string) string {
	if len(messages) == 0 {
		return rendered
	}
	marker := `<div class="pageMessages">`
	index := strings.Index(rendered, marker)
	if index < 0 {
		return rendered
	}
	builder := strings.Builder{}
	for _, message := range messages {
		if strings.TrimSpace(message) == "" {
			continue
		}
		builder.WriteString(renderPageMessage(vm.String(message)))
	}
	if builder.Len() == 0 {
		return rendered
	}
	insertAt := index + len(marker)
	return rendered[:insertAt] + builder.String() + rendered[insertAt:]
}

func renderErrorOverlay(err *RenderError) string {
	if err == nil {
		return ""
	}
	msg := htmlEscape(err.Message)
	file := htmlEscape(err.File)
	expr := htmlEscape(err.Expr)
	return `<!DOCTYPE html><html><head><title>Visualforce Error</title><style>body{font-family:system-ui;background:#1e1e1e;color:#eee;margin:0;padding:1rem}.overlay{border:1px solid #c00;background:#2a1212;padding:1rem;border-radius:4px}code{color:#ffb4b4}</style></head><body><div class="overlay"><h1>Visualforce render error</h1><p>` + msg + `</p>` +
		func() string {
			if file == "" {
				return ""
			}
			return `<p><code>` + file + `</code></p>`
		}() +
		func() string {
			if expr == "" {
				return ""
			}
			return `<p>Expression: <code>` + expr + `</code></p>`
		}() +
		`</div></body></html>`
}

func htmlEscape(raw string) string {
	raw = strings.ReplaceAll(raw, "&", "&amp;")
	raw = strings.ReplaceAll(raw, "<", "&lt;")
	raw = strings.ReplaceAll(raw, ">", "&gt;")
	return raw
}

type renderEnvironment struct {
	Project project.Project
}

var (
	vmRenderEnvironmentMu sync.RWMutex
	vmRenderEnvironments  = map[*vm.VM]renderEnvironment{}
)

func SetVMRenderEnvironment(machine *vm.VM, p project.Project) {
	if machine == nil {
		return
	}
	vmRenderEnvironmentMu.Lock()
	defer vmRenderEnvironmentMu.Unlock()
	vmRenderEnvironments[machine] = renderEnvironment{Project: p}
}

func renderEnvironmentFromVM(machine *vm.VM) renderEnvironment {
	if machine == nil {
		return renderEnvironment{}
	}
	vmRenderEnvironmentMu.RLock()
	defer vmRenderEnvironmentMu.RUnlock()
	if env, ok := vmRenderEnvironments[machine]; ok {
		return env
	}
	return renderEnvironment{}
}
