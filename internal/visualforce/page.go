package visualforce

import (
	"context"
	"errors"
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
	HTML          string
	ViewState     string
	RenderAs      string
	HeaderOptions *VisualforcePageHeaderOptions
	RedirectURL   string
	Redirect      bool
	Metrics       RenderMetrics
	Error         *RenderError
}

type RenderError struct {
	Message string
	File    string
	Line    int
	Column  int
	Expr    string
}

// PageRenderingServiceError is a rendering-profile rejection, rather than an
// Apex controller exception or a component diagnostic.
type PageRenderingServiceError struct {
	Service string
}

func (e *PageRenderingServiceError) Error() string {
	return fmt.Sprintf("Unsupported value %s for <apex:page renderAs> encountered.", e.Service)
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
	machine := req.Machine
	if machine == nil {
		machine = vm.New(nil)
	}
	if req.Org != nil {
		machine.Org = req.Org
	}
	if err := machine.LoadStandardSetListViews(req.Project.ListViewFiles); err != nil {
		return PageRenderResult{}, err
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
	defer machine.BindVisualforceStandardControllerReset(standardControllerResetter(machine, pageMeta, tree, namespace))()
	if err := validateViewStateForPage(pageMeta, req.PageName, req.ViewState); err != nil {
		return PageRenderResult{}, err
	}
	if req.ViewState != nil {
		for _, name := range []string{viewStateVersionFieldName, viewStateMACFieldName, viewStateCSRFFieldName} {
			if _, present := req.FormValues[name]; present {
				if err := VerifyViewStateFormCSRF(*req.ViewState, req.FormValues); err != nil {
					return PageRenderResult{}, err
				}
				break
			}
		}
	}

	savedPageMessages := viewStatePageMessages(req.ViewState)
	controller, extensions, stdController, releaseComponents, err := bootstrapCustomComponentPageControllers(machine, pageMeta, tree, req)
	defer releaseComponents()
	if err != nil {
		return PageRenderResult{}, err
	}
	authorizedStandardFields := extendStandardControllerFields(machine, pageMeta, tree, namespace, &stdController)
	exprCtx := &ExpressionContext{
		VM:                       machine,
		Controller:               controller,
		Extensions:               extensions,
		StandardController:       stdController,
		AuthorizedStandardFields: authorizedStandardFields,
		CurrentPage:              machine.CurrentPage(),
		ProjectNamespace:         namespace,
		Scope:                    NewScopeStack(),
	}
	formState, err := prepareFormLifecycle(tree, &req, machine, &controller, exprCtx)
	if err != nil {
		return PageRenderResult{}, err
	}
	allowedFormFields := VisualforceFormFieldNames(tree)
	paramAction := req.Action
	if strings.TrimSpace(paramAction) == "" {
		paramAction = pageMeta.Action
	}
	paramAssignments, err := visualforceParamAssignments(tree, paramAction)
	if err != nil {
		return PageRenderResult{}, err
	}
	ordinaryFormValues := visualforceFormValuesWithoutParamAssignments(req.FormValues, paramAssignments)
	if len(req.FormValues) > 0 {
		savedPageMessages = mergePageMessages(savedPageMessages, applyFormValues(controller, ordinaryFormValues, allowedFormFields))
		savedPageMessages = mergePageMessages(savedPageMessages, applyStandardControllerFormValues(&stdController, ordinaryFormValues, allowedFormFields, machine))
		controller, err = applyVisualforceParamAssignments(machine, controller, paramAssignments, req.FormValues)
		if err != nil {
			return PageRenderResult{}, err
		}
	}
	actionParams := visualforceActionParams(req.FormValues, allowedFormFields)
	exprCtx.Controller = controller
	exprCtx.StandardController = stdController
	machine.SetVisualforceActionInvoker(func(actionExpr string, actionPageURL string) (vm.Value, error) {
		if strings.TrimSpace(actionPageURL) == "" {
			actionPageURL = pageURL
		}
		actionName := actionMethodName(actionExpr)
		value, result, err := invokeVisualforceAction(machine, controller, extensions, stdController, pageMeta, actionName, actionPageURL, actionParams, false)
		if err != nil {
			return vm.Null, err
		}
		if result.Error != nil {
			return vm.Null, visualforceCommandActionError(result.Error)
		}
		return value, nil
	})
	defer machine.ClearVisualforceActionInvoker()
	var redirectURL string
	var redirect bool
	if (formState == nil || formState.allowAction) && strings.TrimSpace(req.Action) == "" && strings.TrimSpace(pageMeta.Action) != "" {
		value, formulaAction, err := evaluateVisualforcePageAction(pageMeta.Action, exprCtx)
		if err != nil {
			return PageRenderResult{}, err
		}
		if !formulaAction {
			var result vm.UIInvocationResult
			value, result, err = invokeVisualforceAction(machine, controller, extensions, stdController, pageMeta, actionMethodName(pageMeta.Action), pageURL, actionParams, true)
			if err != nil {
				return PageRenderResult{}, err
			}
			if result.Error != nil {
				return PageRenderResult{}, visualforcePageActionError(pageMeta.Name, pageMeta.Action, result.Error)
			}
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
			} else if targetPage := apexPageNameFromURL(navURL); strings.EqualFold(targetPage, req.PageName) {
				machine.SetCurrentPageURL(navURL)
			}
		} else if formulaAction && value.Kind == vm.ValueString && strings.TrimSpace(value.Text) != "" {
			redirectURL = value.Text
			redirect = true
		}
	} else if (formState == nil || formState.allowAction) && strings.TrimSpace(req.Action) != "" {
		value, result, err := invokeVisualforceAction(machine, controller, extensions, stdController, pageMeta, actionMethodName(req.Action), pageURL, actionParams, false)
		if err != nil {
			return PageRenderResult{}, err
		}
		if result.Error != nil {
			if node := visualforceActionComponent(tree, req.Action); node != nil {
				return PageRenderResult{}, visualforceComponentActionError(pageMeta.Name, node.Attribute("action"), node.RawName, result.Error)
			}
			return PageRenderResult{}, visualforceCommandActionError(result.Error)
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
			} else if targetPage := apexPageNameFromURL(navURL); strings.EqualFold(targetPage, req.PageName) {
				machine.SetCurrentPageURL(navURL)
			}
		}
	}
	refreshStandardSetControllerExposure(&stdController, pageMeta.RecordSetVar)
	exprCtx.Controller = controller
	exprCtx.Extensions = extensions
	exprCtx.StandardController = stdController
	exprCtx.CurrentPage = machine.CurrentPage()
	renderAs, err := pageRenderAs(tree, exprCtx)
	if err != nil {
		return PageRenderResult{}, err
	}
	headerOptions, err := EvaluateVisualforcePageHeaderOptions(tree, exprCtx)
	if err != nil {
		return PageRenderResult{}, err
	}
	exprCtx.escapeLiteralAmpersands = pageEscapesLiteralAmpersands(headerOptions, renderAs)
	if err := validateLiteralFlowInputs(tree, req.Project); err != nil {
		return PageRenderResult{}, err
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
		Scope:              exprCtx.Scope,
		Defines:            make(map[string]*MarkupNode),
		Metrics:            &metrics,
		Debug:              req.Debug,
		LightningBootstrap: req.LightningBootstrap,
		formLifecycle:      formState,
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
	controllerValues := valueFieldsToViewStateValues(machine, pageMeta.Controller, controller)
	controllerNullFields := omitViewStateNullFields(controllerValues)
	extensionValues := extensionFieldsToViewStateValues(machine, pageMeta.Extensions, extensions)
	var extensionNullFields [][]string
	for i, fields := range extensionValues {
		if names := omitViewStateNullFields(fields); len(names) != 0 {
			if extensionNullFields == nil {
				extensionNullFields = make([][]string, len(extensionValues))
			}
			extensionNullFields[i] = names
		}
	}
	payload := ViewStatePayload{
		Version:                 CurrentViewStateVersion,
		PageName:                req.PageName,
		ControllerType:          pageMeta.Controller,
		ControllerValues:        controllerValues,
		ControllerFields:        valueFieldsToStrings(machine, pageMeta.Controller, controller),
		ControllerNullFields:    controllerNullFields,
		ExtensionValues:         extensionValues,
		ExtensionNullFields:     extensionNullFields,
		ExtensionControllerRefs: extensionControllerReferences(controller, extensionValues),
		ExtensionFields:         extensionFieldsToStrings(machine, pageMeta.Extensions, extensions),
		ComponentState:          formComponentState(formState, req.ViewState),
		PageMessages:            mergePageMessages(savedPageMessages, currentPageMessages),
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
	stateSize, err := encodedViewStateSize(encoded)
	if err != nil {
		return PageRenderResult{}, err
	}
	if err := CheckVisualforceViewStateSize(stateSize); err != nil {
		return PageRenderResult{Metrics: metrics, Error: &RenderError{Message: err.Error(), File: pageMeta.File}}, err
	}
	finalHTML := InjectCSRF(InjectViewState(rendered, encoded), payload.CSRF)
	return PageRenderResult{HTML: finalHTML, ViewState: encoded, RenderAs: renderAs, HeaderOptions: &headerOptions, RedirectURL: redirectURL, Redirect: redirect, Metrics: metrics}, nil
}

func evaluateVisualforcePageAction(action string, ctx *ExpressionContext) (vm.Value, bool, error) {
	expr, err := parseExpression(actionMethodName(action))
	if err != nil {
		return vm.Null, false, nil
	}
	function, ok := expr.(visualforceFunctionExpr)
	if !ok || !strings.EqualFold(function.name, "URLFOR") {
		return vm.Null, false, nil
	}
	value, err := evaluateExpressionNode(expr, ctx)
	return value, true, err
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

func invokeVisualforceAction(machine *vm.VM, controller vm.Value, extensions []vm.Value, stdController vm.Value, page Page, actionName string, pageURL string, params map[string]string, pageAction bool) (vm.Value, vm.UIInvocationResult, error) {
	if strings.TrimSpace(actionName) == "" {
		return vm.Null, vm.UIInvocationResult{Success: true}, nil
	}
	type actionCandidate struct {
		className  string
		controller vm.Value
		apply      func(vm.Value)
	}
	var candidates []actionCandidate
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
	candidates = append(candidates, actionCandidate{className: page.Controller, controller: controller, apply: func(updated vm.Value) { controller = updated }})
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
		methodName := actionName
		if method, ok := visualforceControllerAction(machine, candidate.className, actionName); ok {
			methodName = method.Name[strings.LastIndex(method.Name, ".")+1:]
			// The native return-type errors were captured on <apex:page action>.
			if pageAction {
				if err := visualforcePageActionReturnType(method.ReturnType); err != nil {
					return vm.Null, vm.UIInvocationResult{}, err
				}
			}
		}
		value, updated, result, err := machine.InvokeVisualforceActionOnController(candidate.controller, candidate.className, methodName, pageURL, params)
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

// Native action_casefold and action_ext_* cases resolve the first extension's
// zero-argument method, using its declared spelling for VM invocation.
func visualforceControllerAction(machine *vm.VM, className, name string) (vm.Method, bool) {
	seen := map[string]bool{}
	for className != "" && !seen[strings.ToLower(className)] {
		seen[strings.ToLower(className)] = true
		class, ok := visualforceVMClass(machine, className)
		if !ok {
			break
		}
		for _, method := range class.Methods {
			shortName := method.Name[strings.LastIndex(method.Name, ".")+1:]
			if strings.EqualFold(shortName, name) && !method.IsStatic && len(method.Params) == 0 {
				return method, true
			}
		}
		className = class.SuperClass
	}
	return vm.Method{}, false
}

func visualforcePageActionReturnType(returnType string) error {
	switch strings.ToLower(returnType) {
	case "string":
		return fmt.Errorf("Formula Expression is required on the action attributes.")
	case "integer":
		return fmt.Errorf("Return type of an Apex action method must be a PageReference. Found: java.lang.Integer")
	case "boolean":
		return fmt.Errorf("Return type of an Apex action method must be a PageReference. Found: java.lang.Boolean")
	}
	return nil
}

type visualforceActionError struct {
	page       string
	expression string
	component  string
	cause      *vm.UIActionError
}

func visualforceActionComponent(node *MarkupNode, action string) *MarkupNode {
	var found *MarkupNode
	ambiguous := false
	var visit func(*MarkupNode)
	visit = func(node *MarkupNode) {
		if node == nil {
			return
		}
		// A page action runs on initial load, not as the submitted component.
		// Method-only action requests cannot distinguish two matching buttons.
		if expression := node.Attribute("action"); expression != "" &&
			!(strings.EqualFold(node.Namespace, "apex") && strings.EqualFold(node.Name, "page")) &&
			strings.EqualFold(actionMethodName(expression), actionMethodName(action)) {
			if found != nil {
				ambiguous = true
			}
			found = node
		}
		for _, child := range node.Children {
			visit(child)
		}
	}
	visit(node)
	if ambiguous {
		return nil
	}
	return found
}

func visualforcePageActionError(page, expression string, cause *vm.UIActionError) error {
	return visualforceComponentActionError(page, expression, "apex:page", cause)
}

func visualforceComponentActionError(page, expression, component string, cause *vm.UIActionError) error {
	if cause.Type == "UnsupportedFeature" {
		return vm.UnsupportedFeature(cause.Message)
	}
	return &visualforceActionError{page: page, expression: expression, component: component, cause: cause}
}

// ActionDiagnostic retains the originating Apex location and the component's
// actual expression. The cause separately preserves the message and full stack.
func (err *visualforceActionError) ActionDiagnostic() string {
	location, _, _ := strings.Cut(err.cause.StackTraceString(), "\n")
	if location == "" {
		return ""
	}
	return fmt.Sprintf("Error is in expression '%s' in component <%s> in page %s: %s",
		err.expression, err.component, strings.ToLower(err.page), location)
}

func visualforceCommandActionError(cause *vm.UIActionError) error {
	if cause.Type == "UnsupportedFeature" {
		return vm.UnsupportedFeature(cause.Message)
	}
	return cause
}

func (err *visualforceActionError) Error() string {
	if diagnostic := err.ActionDiagnostic(); diagnostic != "" {
		return diagnostic + "\n" + err.cause.Message
	}
	return err.cause.Message
}

func (err *visualforceActionError) Unwrap() error {
	return err.cause
}

func pageReferenceNavigation(value vm.Value) (string, bool, bool) {
	if value.Kind != vm.ValueObject || !strings.EqualFold(value.Type, "PageReference") {
		return "", false, false
	}
	urlValue := vm.PageReferenceURL(value)
	if urlValue.Kind != vm.ValueString || strings.TrimSpace(urlValue.Text) == "" {
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
		return vm.Null, vm.NewVisualforceException(fmt.Sprintf("invalid Visualforce page URL %q", pageURL))
	}
	env := renderEnvironmentFromVM(machine)
	if env.Project.Root == "" {
		return vm.Null, vm.UnsupportedFeature("PageReference.getContent local Visualforce page rendering surface")
	}
	idx, err := LoadProjectForRender(env.Project)
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
		return vm.Null, pageReferenceRenderError(err)
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

func pageReferenceRenderError(err error) error {
	if err == nil {
		return nil
	}
	var runtimeErr *vm.RuntimeError
	if errors.As(err, &runtimeErr) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return vm.NewExecutionException(err.Error())
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

func pageRenderAs(root *MarkupNode, ctx *ExpressionContext) (string, error) {
	page := firstVisualforcePageNode(root)
	if page == nil {
		return "", nil
	}
	raw := page.Attribute("renderAs")
	if strings.HasPrefix(raw, "{!") && findExpressionTemplateEnd(raw, 2) == len(raw)-1 {
		expression, err := parseExpression(strings.TrimSpace(raw[2 : len(raw)-1]))
		if err != nil {
			return "", err
		}
		if global := unsupportedVisualforceGlobal(expression); global != "" {
			return "", vm.NewUnsupportedFeatureError(fmt.Sprintf("%s: unsupported Visualforce global", global))
		}
		value, err := evaluateExpressionNode(expression, ctx)
		if err != nil {
			return "", err
		}
		switch value.Kind {
		case vm.ValueBool:
			return "", &PageRenderingServiceError{Service: value.String()}
		case vm.ValueNull:
			return "", nil
		case vm.ValueString:
			return strings.TrimSpace(value.Text), nil
		default:
			return strings.TrimSpace(value.String()), nil
		}
	}
	value, err := RenderVisualforceRawText(raw, ctx)
	return strings.TrimSpace(value), err
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
		if saved != nil {
			restoreViewStateNullFields(&controller, saved.ControllerNullFields)
		}
	}
	if page.StandardController != "" {
		if strings.TrimSpace(page.RecordSetVar) != "" {
			var err error
			stdController, err = standardSetController(machine, page)
			if err != nil {
				return vm.Null, nil, vm.Null, err
			}
		} else {
			record, err := standardControllerRecord(machine, page.StandardController)
			if err != nil {
				return vm.Null, nil, vm.Null, err
			}
			stdController = vm.Object("ApexPages.StandardController")
			stdController.Fields["record"] = record
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
			if i < len(saved.ExtensionControllerRefs) {
				restoreExtensionControllerReferences(&ext, saved.ExtensionControllerRefs[i], controller)
			}
		} else if saved != nil && i < len(saved.ExtensionFields) && saved.ExtensionFields[i] != nil {
			applyStringFields(&ext, saved.ExtensionFields[i])
		}
		if saved != nil && i < len(saved.ExtensionNullFields) {
			restoreViewStateNullFields(&ext, saved.ExtensionNullFields[i])
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

func standardSetController(machine *vm.VM, page Page) (vm.Value, error) {
	records, err := standardSetControllerRecords(machine, page.StandardController)
	if err != nil {
		return vm.Null, err
	}
	filterFields, err := standardSetControllerPageFields(page)
	if err != nil {
		return vm.Null, err
	}
	controller := vm.NewVisualforceStandardSetController(records, page.StandardController, filterFields)
	controller.Fields["__glade_record_set_var"] = vm.String(page.RecordSetVar)
	exposeStandardSetControllerFields(&controller, page.RecordSetVar)
	return controller, nil
}

// standardSetControllerPageFields retains the page's direct row bindings.
// List-view display columns do not add queried fields to a native controller.
func standardSetControllerPageFields(page Page) ([]string, error) {
	markup, err := os.ReadFile(page.File)
	if err != nil {
		return nil, err
	}
	tree, err := ParseMarkupTree(string(markup))
	if err != nil {
		return nil, err
	}
	fields := []string{"Id"}
	seen := map[string]bool{"id": true}
	var visitExpr func(Expression, map[string]bool)
	visitExpr = func(expr Expression, aliases map[string]bool) {
		if parts, static := standardStaticFieldPath(expr); static {
			if len(parts) == 2 && aliases[strings.ToLower(parts[0])] && !seen[strings.ToLower(parts[1])] {
				fields = append(fields, parts[1])
				seen[strings.ToLower(parts[1])] = true
			}
			return
		}
		switch value := expr.(type) {
		case functionExpr:
			for _, arg := range value.args {
				visitExpr(arg, aliases)
			}
		case visualforceFunctionExpr:
			for _, arg := range value.args {
				visitExpr(arg, aliases)
			}
		case binaryExpr:
			visitExpr(value.left, aliases)
			visitExpr(value.right, aliases)
		case unaryExpr:
			visitExpr(value.value, aliases)
		case indexExpr:
			visitExpr(value.target, aliases)
			visitExpr(value.key, aliases)
		case memberExpr:
			visitExpr(value.target, aliases)
		case methodCallExpr:
			visitExpr(value.target, aliases)
			for _, arg := range value.args {
				visitExpr(arg, aliases)
			}
		}
	}
	scan := func(raw string, aliases map[string]bool) {
		for _, ref := range ExtractMergeReferences(raw) {
			if expr, err := parseExpression(ref.Expression); err == nil {
				visitExpr(expr, aliases)
			}
		}
	}
	var walk func(*MarkupNode, map[string]bool)
	walk = func(node *MarkupNode, aliases map[string]bool) {
		if node == nil {
			return
		}
		// A nested iterator can shadow its parent's row variable. Keep the
		// alias local to this subtree, including non-controller collections.
		if variable := strings.ToLower(strings.TrimSpace(node.Attribute("var"))); variable != "" {
			scope := make(map[string]bool, len(aliases)+1)
			for name, bound := range aliases {
				scope[name] = bound
			}
			scope[variable] = false
			if strings.EqualFold(node.Namespace, "apex") && (strings.EqualFold(node.Name, "repeat") || strings.EqualFold(node.Name, "dataTable") || strings.EqualFold(node.Name, "pageBlockTable")) {
				refs := ExtractMergeReferences(node.Attribute("value"))
				if len(refs) == 1 {
					if expr, err := parseExpression(refs[0].Expression); err == nil {
						parts, static := standardStaticFieldPath(expr)
						scope[variable] = static && len(parts) == 1 && strings.EqualFold(parts[0], page.RecordSetVar)
					}
				}
			}
			aliases = scope
		}
		scan(node.Text, aliases)
		for _, raw := range node.Attributes {
			scan(raw, aliases)
		}
		for _, child := range node.Children {
			walk(child, aliases)
		}
	}
	walk(tree, nil)
	return fields, nil
}

func standardSetControllerRecords(machine *vm.VM, objectName string) (vm.Value, error) {
	if machine == nil {
		return vm.Null, fmt.Errorf("Visualforce set record read requires a VM")
	}
	records, err := machine.ReadVisualforceRecords(objectName)
	if err != nil {
		return vm.Null, err
	}
	values := make([]vm.Value, 0, len(records))
	for _, record := range records {
		values = append(values, vmValueFromStorageRecord(record))
	}
	return vm.List(values...), nil
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

func standardControllerRecord(machine *vm.VM, objectName string) (vm.Value, error) {
	objectKey := strings.TrimSpace(objectName)
	record := vm.Object(objectKey)
	if machine == nil || machine.Org == nil {
		return record, nil
	}
	resolvedObject, ok := storage.ResolveObjectName(*machine.Org, objectKey)
	if ok {
		objectKey = resolvedObject
		record = vm.Object(objectKey)
	}
	if recordID, ok := pageParameterString(machine.CurrentPage(), "id"); ok {
		stored, found, err := machine.ReadVisualforceRecord(objectKey, storage.ID(recordID))
		if err != nil {
			return vm.Null, fmt.Errorf("read Visualforce standard controller record: %w", err)
		}
		if found {
			fields := []string{"Id"}
			for field := range stored.Fields {
				fields = append(fields, field)
			}
			return machine.MarkVisualforceRecordFields(vmValueFromStorageRecord(stored), fields), nil
		}
	}
	return record, nil
}

func standardControllerResetter(machine *vm.VM, page Page, tree *MarkupNode, namespace string) func(string) (vm.Value, error) {
	if machine == nil || machine.Org == nil || page.StandardController == "" || strings.TrimSpace(page.RecordSetVar) != "" {
		return nil
	}
	if _, hasID := pageParameterString(machine.CurrentPage(), "id"); !hasID {
		return nil
	}
	objectName, ok := storage.ResolveObjectName(*machine.Org, page.StandardController)
	if !ok {
		return nil
	}
	return func(recordType string) (vm.Value, error) {
		if !strings.EqualFold(recordType, objectName) {
			return vm.Null, nil
		}
		record, err := standardControllerRecord(machine, objectName)
		if err != nil {
			return vm.Null, err
		}
		controller := vm.Object("ApexPages.StandardController")
		controller.Fields["record"] = record
		extendStandardControllerFields(machine, page, tree, namespace, &controller)
		return controller.Fields["record"], nil
	}
}

// extendStandardControllerFields exposes only direct page bindings authorized
// by USER_MODE. Row reads remain separate, so absent and unshared rows provide
// no field values; other readable fields on the page can still render.
func extendStandardControllerFields(machine *vm.VM, page Page, tree *MarkupNode, namespace string, controller *vm.Value) map[string]storage.Value {
	allowed := make(map[string]storage.Value)
	if machine == nil || machine.Org == nil || controller == nil || controller.Kind != vm.ValueObject ||
		strings.TrimSpace(page.RecordSetVar) != "" || page.StandardController == "" {
		return allowed
	}
	record := controller.Fields["record"]
	if record.Kind != vm.ValueObject {
		return allowed
	}
	objectKey, ok := storage.ResolveObjectName(*machine.Org, page.StandardController)
	if !ok {
		return allowed
	}
	object := machine.Org.Objects[objectKey]
	checked := make(map[string]bool)
	var requested []string
	for _, fieldName := range standardControllerDirectBindings(tree, page.StandardController, machine.Org) {
		resolved, ok := storage.ResolveFieldName(object.Definition, namespace, fieldName)
		if !ok || strings.EqualFold(resolved, "Name") || strings.EqualFold(resolved, "Id") {
			continue
		}
		key := strings.ToLower(resolved)
		if checked[key] {
			continue
		}
		checked[key] = true
		requested = append(requested, resolved)
	}
	if len(requested) == 0 {
		return allowed
	}
	// Authorization is independent of row visibility. A missing or unshared
	// row may render an empty input only for fields admitted by USER_MODE.
	granted, err := machine.AuthorizeVisualforceRecordFields(objectKey, requested)
	if err != nil {
		granted = nil
		for _, field := range requested {
			if _, fieldErr := machine.AuthorizeVisualforceRecordFields(objectKey, []string{field}); fieldErr == nil {
				granted = append(granted, field)
			}
		}
	}
	if len(granted) == 0 {
		return allowed
	}
	for _, field := range granted {
		allowed[strings.ToLower(field)] = storage.NullValue()
	}
	recordID, hasID := pageParameterString(machine.CurrentPage(), "id")
	if !hasID {
		return allowed
	}
	merge := func(projected storage.Record, fields []string) {
		for _, field := range fields {
			value, present := projected.GetField(field)
			if !present {
				continue
			}
			allowed[strings.ToLower(field)] = value
			record.Fields[field] = vmValueFromStorageValue(value)
			record = machine.MarkVisualforceRecordFields(record, []string{field})
		}
	}
	projected, found, err := machine.ReadVisualforceRecordFields(objectKey, storage.ID(recordID), granted)
	if err == nil && found {
		merge(projected, granted)
	} else if err != nil {
		// A read failure must not admit the field without a successful
		// projection; independently readable siblings can still render.
		for _, field := range granted {
			delete(allowed, strings.ToLower(field))
			projected, found, fieldErr := machine.ReadVisualforceRecordFields(objectKey, storage.ID(recordID), []string{field})
			if fieldErr == nil {
				allowed[strings.ToLower(field)] = storage.NullValue()
				if found {
					merge(projected, []string{field})
				}
			}
		}
	}
	controller.Fields["record"] = record
	return allowed
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

func applyVisualforceParamAssignments(machine *vm.VM, controller vm.Value, assignments []visualforceParamAssignment, values map[string]string) (vm.Value, error) {
	for _, assignment := range assignments {
		raw, supplied, err := visualforceParamSubmittedValue(values, assignment.SubmittedName)
		if err != nil {
			return controller, err
		}
		if !supplied {
			continue
		}
		typeName, ok, err := machine.InstancePropertyType(controller, assignment.TargetName)
		if err != nil {
			return controller, fmt.Errorf("apex:param %s assignTo %s: %w", assignment.SubmittedName, assignment.TargetName, err)
		}
		if !ok {
			return controller, fmt.Errorf("apex:param %s assignTo %s does not name a readable and writable controller property", assignment.SubmittedName, assignment.TargetName)
		}
		value, err := visualforceAssignmentValue(raw, typeName, assignment.SubmittedName)
		if err != nil {
			return controller, err
		}
		controller, err = machine.AssignInstanceProperty(controller, assignment.TargetName, value)
		if err != nil {
			return controller, fmt.Errorf("apex:param %s assignTo %s: %w", assignment.SubmittedName, assignment.TargetName, err)
		}
	}
	return controller, nil
}

func applyStandardControllerFormValues(controller *vm.Value, values map[string]string, allowedFields map[string]bool, machine *vm.VM) []string {
	if controller == nil || controller.Kind != vm.ValueObject || len(values) == 0 {
		return nil
	}
	record, ok := controller.Fields["record"]
	if !ok || record.Kind != vm.ValueObject {
		return nil
	}
	if pageID, hasID := pageParameterString(machine.CurrentPage(), "id"); hasID {
		storedID := record.Fields["Id"].Fields["value"]
		if storedID.Kind != vm.ValueString || storedID.Text != pageID {
			return nil
		}
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

// Root nulls are omitted from the projected value maps, but their names must
// survive so restoration can clear values supplied by a new constructor.
// Nested nulls and collection metadata remain in the graph unchanged.
func omitViewStateNullFields(fields map[string]vm.Value) []string {
	var names []string
	for name, value := range fields {
		if value.Kind == vm.ValueNull {
			names = append(names, name)
			delete(fields, name)
		}
	}
	sort.Strings(names)
	return names
}

func restoreViewStateNullFields(target *vm.Value, names []string) {
	if target == nil || target.Kind != vm.ValueObject || len(names) == 0 {
		return
	}
	if target.Fields == nil {
		target.Fields = make(map[string]vm.Value, len(names))
	}
	for _, name := range names {
		target.Fields[name] = vm.Null
	}
}

// Record only direct references to the actual primary controller. Type or
// field equality cannot distinguish an alias from an independent instance.
func extensionControllerReferences(controller vm.Value, extensions []map[string]vm.Value) [][]string {
	if controller.Kind != vm.ValueObject || controller.Ref == 0 {
		return nil
	}
	var references [][]string
	for i, fields := range extensions {
		for name, value := range fields {
			if value.Kind != vm.ValueObject || value.Ref != controller.Ref {
				continue
			}
			if references == nil {
				references = make([][]string, len(extensions))
			}
			references[i] = append(references[i], name)
		}
		if references != nil {
			sort.Strings(references[i])
		}
	}
	return references
}

func restoreExtensionControllerReferences(extension *vm.Value, references []string, controller vm.Value) {
	if controller.Kind != vm.ValueObject || extension == nil || extension.Kind != vm.ValueObject {
		return
	}
	for _, name := range references {
		if value, ok := extension.Fields[name]; ok && value.Kind == vm.ValueObject {
			extension.Fields[name] = controller
		}
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
	seen := make(map[uint64]vm.Value)
	if value.Kind != vm.ValueObject {
		return out
	}
	for key, field := range value.Fields {
		if visualforceFieldIsTransient(machine, className, key) {
			continue
		}
		out[key] = filterViewStateValue(machine, field, seen)
	}
	return out
}

// Keep explicit nulls and collection identity while removing transient members
// throughout the saved graph. Copy containers so rendering the response never
// changes the live controller or its aliases.
func filterViewStateValue(machine *vm.VM, value vm.Value, seen map[uint64]vm.Value) vm.Value {
	if value.Ref != 0 {
		if saved, ok := seen[value.Ref]; ok {
			return saved
		}
	}
	out := value
	if value.Fields != nil {
		out.Fields = make(map[string]vm.Value, len(value.Fields))
	}
	if value.List != nil {
		out.List = make([]vm.Value, len(value.List))
	}
	if value.Set != nil {
		out.Set = make([]vm.Value, len(value.Set))
	}
	if value.Map != nil {
		out.Map = make(map[string]vm.Value, len(value.Map))
	}
	if value.MapKeys != nil {
		out.MapKeys = make(map[string]vm.Value, len(value.MapKeys))
	}
	if value.Ref != 0 {
		seen[value.Ref] = out
	}
	for key, field := range value.Fields {
		if !visualforceFieldIsTransient(machine, value.Type, key) {
			out.Fields[key] = filterViewStateValue(machine, field, seen)
		}
	}
	for i, field := range value.List {
		out.List[i] = filterViewStateValue(machine, field, seen)
	}
	for i, field := range value.Set {
		out.Set[i] = filterViewStateValue(machine, field, seen)
	}
	for key, field := range value.Map {
		out.Map[key] = filterViewStateValue(machine, field, seen)
	}
	for key, field := range value.MapKeys {
		out.MapKeys[key] = filterViewStateValue(machine, field, seen)
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

// ClearVMRenderEnvironment removes the render environment associated with a VM.
func ClearVMRenderEnvironment(machine *vm.VM) {
	if machine == nil {
		return
	}
	vmRenderEnvironmentMu.Lock()
	defer vmRenderEnvironmentMu.Unlock()
	delete(vmRenderEnvironments, machine)
}

// VMRenderEnvironmentCountForTest reports the number of retained VM environments.
func VMRenderEnvironmentCountForTest() int {
	vmRenderEnvironmentMu.RLock()
	defer vmRenderEnvironmentMu.RUnlock()
	return len(vmRenderEnvironments)
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
