package visualforce

import (
	"fmt"
	"net/url"
	"os"
	"strings"

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
	Project    project.Project
	VFIndex    Index
	Org        *storage.OrgState
	Machine    *vm.VM
	PageName   string
	PageURL    string
	ViewState  *ViewStatePayload
	FormValues map[string]string
	Action     string
	Debug              bool
	LightningBootstrap *lwcbrowser.PageConfig
}

type PageRenderResult struct {
	HTML      string
	ViewState string
	Metrics   RenderMetrics
	Error     *RenderError
}

type RenderError struct {
	Message  string
	File     string
	Line     int
	Column   int
	Expr     string
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

	controller, extensions, stdController, err := bootstrapControllers(machine, pageMeta, req.ViewState)
	if err != nil {
		return PageRenderResult{}, err
	}
	if req.ViewState != nil && len(req.FormValues) > 0 {
		applyFormValues(controller, req.FormValues)
	}
	if strings.TrimSpace(req.Action) != "" && pageMeta.Controller != "" {
		_, _ = machine.InvokeVisualforceAction(pageMeta.Controller, req.Action, pageURL, nil)
		controller, extensions, stdController, _ = bootstrapControllers(machine, pageMeta, nil)
		if len(req.FormValues) > 0 {
			applyFormValues(controller, req.FormValues)
		}
	}

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
		VM:         machine,
		PageName:   req.PageName,
		PageMeta:   pageMeta,
		VFIndex:    &req.VFIndex,
		Project:    req.Project,
		Expression: exprCtx,
		Scope:      NewScopeStack(),
		Defines:    make(map[string]*MarkupNode),
		Metrics:    &metrics,
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
	payload := ViewStatePayload{
		PageName:         req.PageName,
		ControllerType:   pageMeta.Controller,
		ControllerFields: valueFieldsToStrings(controller),
		ComponentState:   map[string]string{},
		PageMessages:     pageMessagesToStrings(machine),
	}
	if req.ViewState != nil {
		payload.CSRF = req.ViewState.CSRF
	}
	encoded, err := EncodeViewState(payload, nil)
	if err != nil {
		return PageRenderResult{}, err
	}
	finalHTML := InjectViewState(rendered, encoded)
	return PageRenderResult{HTML: finalHTML, ViewState: encoded, Metrics: metrics}, nil
}

func RenderPageURL(machine *vm.VM, pageURL string, asPDF bool) (vm.Value, error) {
	if asPDF {
		return vm.Null, vm.UnsupportedFeature("PageReference.getContentAsPDF local Visualforce page rendering surface")
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
		if saved != nil && saved.ControllerFields != nil {
			applyStringFields(&controller, saved.ControllerFields)
		}
	}
	for _, extName := range page.Extensions {
		ext, err := machine.ConstructController(extName)
		if err != nil {
			return vm.Null, nil, vm.Null, err
		}
		extensions = append(extensions, ext)
	}
	if page.StandardController != "" {
		record := vm.Object(page.StandardController)
		stdController = vm.Object("ApexPages.StandardController")
		stdController.Fields["record"] = record
	}
	return controller, extensions, stdController, nil
}

func applyFormValues(controller vm.Value, values map[string]string) {
	if controller.Kind != vm.ValueObject {
		return
	}
	for key, raw := range values {
		if strings.HasPrefix(key, "__") || key == viewStateFieldName {
			continue
		}
		controller.Fields[key] = vm.String(raw)
	}
}

func applyStringFields(target *vm.Value, fields map[string]string) {
	if target == nil || target.Kind != vm.ValueObject {
		return
	}
	for key, raw := range fields {
		target.Fields[key] = vm.String(raw)
	}
}

func valueFieldsToStrings(value vm.Value) map[string]string {
	out := make(map[string]string)
	if value.Kind != vm.ValueObject {
		return out
	}
	for key, field := range value.Fields {
		if field.Kind == vm.ValueString {
			out[key] = field.Text
		} else if field.Kind != vm.ValueNull {
			out[key] = field.String()
		}
	}
	return out
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

var vmRenderEnvironments = map[*vm.VM]renderEnvironment{}

func SetVMRenderEnvironment(machine *vm.VM, p project.Project) {
	if machine == nil {
		return
	}
	vmRenderEnvironments[machine] = renderEnvironment{Project: p}
}

func renderEnvironmentFromVM(machine *vm.VM) renderEnvironment {
	if machine == nil {
		return renderEnvironment{}
	}
	if env, ok := vmRenderEnvironments[machine]; ok {
		return env
	}
	return renderEnvironment{}
}
