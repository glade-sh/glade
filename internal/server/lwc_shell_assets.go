package server

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/gladehome"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
)

type lwcLocalContextPayload struct {
	ActiveRoute     string                  `json:"activeRoute"`
	PageReference   map[string]any          `json:"pageReference"`
	Context         lwcshell.PageContext    `json:"context"`
	Mounts          []lwcShellMount         `json:"mounts"`
	Apps            []lwcshell.ShellApp     `json:"apps"`
	Routes          []lwcshell.ShellRoute   `json:"routes"`
	DefaultContext  string                  `json:"defaultContext,omitempty"`
	SelectedContext string                  `json:"selectedContext,omitempty"`
	Contexts        []lwcLocalContextPreset `json:"contexts,omitempty"`
	Services        map[string]string       `json:"services"`
	Diagnostics     []lwcshell.Diagnostic   `json:"diagnostics,omitempty"`
}

type lwcLocalContextPreset struct {
	Name        string                `json:"name"`
	SelectedURL string                `json:"selectedUrl,omitempty"`
	Context     lwcshell.PageContext  `json:"context,omitempty"`
	Diagnostics []lwcshell.Diagnostic `json:"diagnostics,omitempty"`
}

func renderLWCShellDocument(p project.Project, cfg lwcbrowser.PageConfig, shell lwcshell.ShellPage, activeRoute string) string {
	model := lwcshell.BuildWorkbenchModel(p, shell, activeRoute)
	mounts := lwcShellMounts(shell)
	cfg.PageReference = lwcShellPageReference(shell)
	modelJSON := mustScriptJSON(model)
	contextJSON := mustScriptJSON(shell.Context)
	mountsJSON := mustScriptJSON(mounts)
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>Glade LWC Shell</title>`)
	b.WriteString(`<link rel="stylesheet" href="/lightning/runtime/shell/glade-shell.css">`)
	b.WriteString(lwcbrowser.BootstrapHTML(cfg))
	b.WriteString(`</head><body class="glade-shell" data-glade-shell="workbench" data-glade-app-mode="`)
	b.WriteString(html.EscapeString(model.Mode))
	b.WriteString(`">`)
	b.WriteString(`<header class="glade-global-header"><button class="glade-app-launcher" type="button" aria-label="App launcher">`)
	b.WriteString(`&#9638;</button><strong>`)
	b.WriteString(html.EscapeString(workbenchAppLabel(model)))
	b.WriteString(`</strong><span class="glade-shell-subtitle">Local Lightning Workbench</span></header>`)
	b.WriteString(lwcShellWorkbenchNavHTML(model))
	b.WriteString(`<div class="glade-workbench">`)
	if model.Mode == "console" {
		b.WriteString(lwcShellConsoleRailHTML(model))
	}
	b.WriteString(`<main class="glade-stage">`)
	b.WriteString(lwcShellDiagnosticsHTML(shell.Diagnostics))
	b.WriteString(lwcShellRoutePickerHTML(model))
	b.WriteString(lwcShellRegionsHTML(shell))
	b.WriteString(`</main><aside class="glade-context-panel" data-glade-context-panel>`)
	b.WriteString(`<h2>Context</h2><dl>`)
	contextRows := []struct{ name, value string }{
		{"Target", string(shell.Context.Kind)},
		{"Page", shell.Context.PageName},
		{"Component", shell.Context.ComponentName},
		{"Record", shell.Context.RecordID},
		{"Object", shell.Context.ObjectAPIName},
		{"App", shell.Context.AppName},
		{"Tab", shell.Context.TabName},
		{"Form factor", shell.Context.FormFactor},
	}
	for _, row := range contextRows {
		if strings.TrimSpace(row.value) == "" {
			continue
		}
		b.WriteString(`<dt>`)
		b.WriteString(html.EscapeString(row.name))
		b.WriteString(`</dt><dd>`)
		b.WriteString(html.EscapeString(row.value))
		b.WriteString(`</dd>`)
	}
	b.WriteString(`</dl></aside></div>`)
	b.WriteString(`<script type="application/json" id="glade-lwc-workbench">`)
	b.WriteString(modelJSON)
	b.WriteString(`</script><script type="application/json" id="glade-lwc-context">`)
	b.WriteString(contextJSON)
	b.WriteString(`</script><script>`)
	b.WriteString(`(function(){var mounts=`)
	b.WriteString(mountsJSON)
	b.WriteString(`;function go(){for(var i=0;i<mounts.length;i++){var m=mounts[i];window.$Lightning.createComponent(m.qualified,m.attrs,m.hostId,function(){});}}if(window.$Lightning){go();}else{window.addEventListener("load",go);}})();`)
	b.WriteString(`</script><script type="module" src="/lightning/runtime/shell/app.js"></script></body></html>`)
	return b.String()
}

func lwcShellRoutePickerHTML(model lwcshell.WorkbenchModel) string {
	if len(model.Routes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="glade-route-picker" aria-label="Preview routes"><h2>Preview routes</h2><div>`)
	for _, route := range model.Routes {
		b.WriteString(`<a data-glade-route data-glade-route-kind="`)
		b.WriteString(html.EscapeString(string(route.Kind)))
		b.WriteString(`" href="`)
		b.WriteString(html.EscapeString(route.URL))
		b.WriteString(`"><span>`)
		b.WriteString(html.EscapeString(route.Label))
		b.WriteString(`</span><code>`)
		b.WriteString(html.EscapeString(route.URL))
		b.WriteString(`</code></a>`)
	}
	b.WriteString(`</div></section>`)
	return b.String()
}

func workbenchAppLabel(model lwcshell.WorkbenchModel) string {
	if len(model.Apps) > 0 && strings.TrimSpace(model.Apps[0].Label) != "" {
		return model.Apps[0].Label
	}
	return "Glade LWC Shell"
}

func lwcShellWorkbenchNavHTML(model lwcshell.WorkbenchModel) string {
	if model.Mode == "console" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<nav class="glade-app-tabs" aria-label="Local routes">`)
	for _, link := range lwcShellWorkbenchNavLinks(model) {
		b.WriteString(`<a data-glade-route href="`)
		b.WriteString(html.EscapeString(link.URL))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(link.Label))
		b.WriteString(`</a>`)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

func lwcShellConsoleRailHTML(model lwcshell.WorkbenchModel) string {
	var b strings.Builder
	b.WriteString(`<nav class="glade-console-rail" aria-label="Console navigation">`)
	for _, link := range lwcShellWorkbenchNavLinks(model) {
		b.WriteString(`<a data-glade-route href="`)
		b.WriteString(html.EscapeString(link.URL))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(link.Label))
		b.WriteString(`</a>`)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

type lwcShellNavLink struct {
	Label string
	URL   string
}

func lwcShellWorkbenchNavLinks(model lwcshell.WorkbenchModel) []lwcShellNavLink {
	if len(model.Apps) == 0 || len(model.Apps[0].NavItems) == 0 {
		links := make([]lwcShellNavLink, 0, len(model.Routes))
		for _, route := range model.Routes {
			links = append(links, lwcShellNavLink{Label: route.Label, URL: route.URL})
		}
		return links
	}
	routeByTab := map[string]lwcshell.ShellRoute{}
	for _, route := range model.Routes {
		if route.Kind == lwcshell.RenderTargetTab {
			routeByTab[strings.ToLower(route.TabName)] = route
		}
	}
	links := make([]lwcShellNavLink, 0, len(model.Apps[0].NavItems))
	for _, item := range model.Apps[0].NavItems {
		if route, ok := routeByTab[strings.ToLower(item)]; ok {
			links = append(links, lwcShellNavLink{Label: route.Label, URL: route.URL})
			continue
		}
		links = append(links, lwcShellNavLink{Label: item, URL: "/lwc"})
	}
	return links
}

func (s *Server) serveLightningShellRuntime(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet || len(parts) != 2 {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning runtime asset")
		return
	}
	contentType, content, ok := lightningRuntimeAsset(parts[0], parts[1])
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning runtime asset")
		return
	}
	w.Header().Set("Content-Type", contentType)
	setDevNoStore(w)
	_, _ = w.Write(content)
}

func lightningRuntimeAsset(kind, name string) (string, []byte, bool) {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" || strings.Contains(name, "/") || strings.Contains(name, "..") {
		return "", nil, false
	}
	dir, err := gladehome.RuntimeAssetDir(kind)
	if err != nil {
		return "", nil, false
	}
	candidates := []string{name}
	if strings.HasSuffix(name, ".js") {
		candidates = append(candidates, strings.TrimSuffix(name, ".js")+".mjs")
	}
	for _, candidate := range candidates {
		content, err := os.ReadFile(filepath.Join(dir, candidate))
		if err != nil {
			continue
		}
		return lightningRuntimeContentType(candidate), content, true
	}
	return "", nil, false
}

func lightningRuntimeContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	default:
		return "application/javascript; charset=utf-8"
	}
}

func (s *Server) handleLightningLocal(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet || len(parts) != 1 || parts[0] != "context.json" {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning local endpoint")
		return
	}
	activeRoute := strings.TrimSpace(r.URL.Query().Get("url"))
	if activeRoute == "" {
		activeRoute = "/lwc"
	}
	shell, diagnostics, err := s.resolveLWCShellRoute(activeRoute)
	if err != nil && len(diagnostics) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"error": err.Error()})
		return
	}
	model := lwcshell.BuildWorkbenchModel(s.Source.Project, shell, activeRoute)
	defaultContext, selectedContext, contexts, presetDiagnostics := lwcLocalContextPresets(s.Source.Project, activeRoute)
	payload := lwcLocalContextPayload{
		ActiveRoute:     activeRoute,
		PageReference:   lwcShellPageReference(shell),
		Context:         shell.Context,
		Mounts:          lwcShellMounts(shell),
		Apps:            model.Apps,
		Routes:          model.Routes,
		DefaultContext:  defaultContext,
		SelectedContext: selectedContext,
		Contexts:        contexts,
		Services:        lwcShellServiceStatus(),
		Diagnostics:     append(append(shell.Diagnostics, diagnostics...), presetDiagnostics...),
	}
	writeJSON(w, http.StatusOK, payload)
}

func lwcLocalContextPresets(p project.Project, activeRoute string) (string, string, []lwcLocalContextPreset, []lwcshell.Diagnostic) {
	file, err := lwcshell.LoadContextPresets(p.Root)
	if err != nil {
		if presetErr, ok := err.(*lwcshell.ContextPresetError); ok && presetErr.Diagnostic.Code != "" {
			return "", "", nil, []lwcshell.Diagnostic{presetErr.Diagnostic}
		}
		return "", "", nil, []lwcshell.Diagnostic{{Code: "GLADELWC020", Message: err.Error()}}
	}
	names := make([]string, 0, len(file.Contexts))
	for name := range file.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	active := normalizeLWCContextRoute(activeRoute)
	contexts := make([]lwcLocalContextPreset, 0, len(names))
	selected := ""
	for _, name := range names {
		ctx, err := file.Contexts[name].ToPageContext()
		row := lwcLocalContextPreset{Name: name}
		if err != nil {
			if presetErr, ok := err.(*lwcshell.ContextPresetError); ok && presetErr.Diagnostic.Code != "" {
				row.Diagnostics = []lwcshell.Diagnostic{presetErr.Diagnostic}
			} else {
				row.Diagnostics = []lwcshell.Diagnostic{{Code: "GLADELWC020", Message: err.Error()}}
			}
			contexts = append(contexts, row)
			continue
		}
		row.Context = ctx
		row.SelectedURL = localLWCSelectedRoute(ctx)
		if selected == "" && row.SelectedURL != "" && normalizeLWCContextRoute(row.SelectedURL) == active {
			selected = name
		}
		contexts = append(contexts, row)
	}
	return file.DefaultContext, selected, contexts, nil
}

func localLWCSelectedRoute(ctx lwcshell.PageContext) string {
	selected := lwcshell.SelectedURL("http://glade.local", ctx)
	if selected == "" {
		return ""
	}
	parsed, err := url.Parse(selected)
	if err != nil {
		return ""
	}
	return parsed.RequestURI()
}

func normalizeLWCContextRoute(raw string) string {
	if raw == "" {
		return ""
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	if parsed.Scheme != "" || parsed.Host != "" {
		return parsed.RequestURI()
	}
	return parsed.RequestURI()
}

func (s *Server) resolveLWCShellRoute(rawURL string) (lwcshell.ShellPage, []lwcshell.Diagnostic, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return lwcshell.ShellPage{}, nil, err
	}
	parts := splitPath(parsed.EscapedPath())
	if len(parts) == 0 || parts[0] != "lwc" {
		return lwcshell.ShellPage{}, nil, nil
	}
	if len(parts) == 1 {
		return lwcshell.ShellPage{}, nil, nil
	}
	if parts[1] != "preview" {
		return lwcshell.ShellPage{}, nil, fmt.Errorf("unknown LWC shell route %q", rawURL)
	}
	req := lwcShellRequestFromURL(parsed)
	shell, _, diagnostics, err := s.resolveLWCShellRequest(req, parts[2:])
	return shell, diagnostics, err
}

func lwcShellRequestFromURL(u *url.URL) *http.Request {
	req, _ := http.NewRequest(http.MethodGet, u.String(), nil)
	return req
}

func lwcShellServiceStatus() map[string]string {
	return map[string]string{
		"apex":                   "supported",
		"apexWire":               "supported",
		"imperativeApex":         "supported",
		"lds":                    "supported",
		"uiRecordApi":            "supported",
		"uiObjectInfoApi":        "supported",
		"uiLayoutApi":            "supported",
		"uiRelatedListApi":       "supported",
		"navigation":             "supported",
		"labels":                 "supported",
		"resources":              "supported",
		"schema":                 "supported",
		"user":                   "supported",
		"i18n":                   "supported",
		"toast":                  "supported",
		"lms":                    "supported",
		"platformResourceLoader": "supported",
		"urlAddressable":         "supported-local",
		"quickActions":           "supported-local",
		"baseComponents":         "supported-local",
		"visualforceHost":        "supported-local",
		"platformWorkspaceApi":   "partial",
		"slds":                   "partial",
	}
}
