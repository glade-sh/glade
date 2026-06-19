package server

import (
	"fmt"
	"html"
	"net/http"
	"net/url"
	"os"
	"path"
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

func renderLWCShellDocument(p project.Project, cfg lwcbrowser.PageConfig, shell lwcshell.ShellPage, activeRoute string, sampleRecordID string) string {
	model := lwcshell.BuildWorkbenchModel(p, shell, activeRoute)
	if strings.TrimSpace(sampleRecordID) == "" {
		sampleRecordID = lwcShellDefaultSampleRecordID
	}
	model.SampleRecordID = sampleRecordID
	lwcShellApplySampleRecordID(&model, "Account", sampleRecordID)
	activeContext := model.Active.Context
	mounts := lwcShellMounts(shell)
	cfg.PageReference = lwcShellPageReference(shell)
	modelJSON := mustScriptJSON(model)
	contextJSON := mustScriptJSON(activeContext)
	mountsJSON := mustScriptJSON(mounts)
	builderActive := lwcShellWorkbenchBuilderActive(activeRoute)
	homeActive := lwcShellWorkbenchHomeActive(activeRoute)
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>Glade LWC Shell</title>`)
	b.WriteString(`<link rel="stylesheet" href="/lightning/runtime/shell/glade-shell.css">`)
	b.WriteString(lwcbrowser.BootstrapHTML(cfg))
	b.WriteString(`</head><body class="glade-shell" data-glade-shell="workbench" data-glade-app-mode="`)
	b.WriteString(html.EscapeString(model.Mode))
	if builderActive {
		b.WriteString(`" data-glade-builder-active="true`)
	}
	b.WriteString(`">`)
	if strings.TrimSpace(activeContext.Community.Site) != "" {
		b.WriteString(`<div hidden data-glade-community-shell data-glade-community-site="`)
		b.WriteString(html.EscapeString(activeContext.Community.Site))
		b.WriteString(`" data-glade-community-guest="`)
		b.WriteString(html.EscapeString(fmt.Sprintf("%t", activeContext.Community.Guest)))
		b.WriteString(`"></div>`)
	}
	b.WriteString(`<header class="glade-global-header"><button class="glade-app-launcher" type="button" aria-label="App launcher">`)
	b.WriteString(`&#9638;</button><strong>`)
	b.WriteString(html.EscapeString(workbenchAppLabel(model)))
	b.WriteString(`</strong><span class="glade-shell-subtitle">Local Lightning Workbench</span>`)
	if !builderActive {
		b.WriteString(lwcShellRouteMenuHTML(model))
	}
	b.WriteString(`</header>`)
	b.WriteString(lwcShellWorkbenchNavHTML(model))
	b.WriteString(`<div class="glade-workbench">`)
	if model.Mode == "console" {
		b.WriteString(lwcShellConsoleRailHTML(model))
	}
	b.WriteString(lwcShellCommunityChromeHTML(activeContext.Community))
	b.WriteString(`<main class="glade-stage">`)
	b.WriteString(lwcShellDiagnosticsHTML(shell.Diagnostics))
	b.WriteString(lwcShellFlowEventsHTML(activeContext.Flow))
	if !builderActive && !homeActive {
		b.WriteString(lwcShellUtilityBarHTML(activeContext.Workspace.Utilities))
	}
	if builderActive {
		b.WriteString(lwcShellWorkbenchBuilderHTML(model))
	}
	if homeActive {
		b.WriteString(lwcShellWorkbenchHomeHTML(model))
	}
	if !builderActive && !homeActive {
		b.WriteString(lwcShellRegionsHTML(shell))
	}
	b.WriteString(`</main><aside class="glade-context-panel" data-glade-context-panel>`)
	b.WriteString(`<h2>Context</h2><dl>`)
	communityGuest := ""
	if strings.TrimSpace(activeContext.Community.Site) != "" {
		communityGuest = fmt.Sprintf("%t", activeContext.Community.Guest)
	}
	contextRows := []struct{ name, value string }{
		{"Target", string(activeContext.Kind)},
		{"Page", activeContext.PageName},
		{"Component", activeContext.ComponentName},
		{"Record", activeContext.RecordID},
		{"Object", activeContext.ObjectAPIName},
		{"App", activeContext.AppName},
		{"Tab", activeContext.TabName},
		{"Form factor", activeContext.FormFactor},
		{"Community", activeContext.Community.Site},
		{"Community base path", activeContext.Community.BasePath},
		{"Community site ID", activeContext.Community.SiteID},
		{"Community network ID", activeContext.Community.NetworkID},
		{"Community guest", communityGuest},
		{"Community language", activeContext.Community.Language},
		{"Flow", activeContext.Flow.APIName},
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
	b.WriteString(`;function create(m,done){window.$Lightning.createComponent(m.qualified,m.attrs,m.hostId,function(cmp,status,msg){if(done){done(cmp,status,msg);}});}function go(){var theme=null;var rest=[];for(var i=0;i<mounts.length;i++){if(!theme&&mounts[i].region==="theme"){theme=mounts[i];}else{rest.push(mounts[i]);}}if(!theme){for(var j=0;j<mounts.length;j++){create(mounts[j]);}return;}create(theme,function(cmp,status){if(status!=="SUCCESS"||!cmp){for(var j=0;j<rest.length;j++){create(rest[j]);}return;}cmp.setAttribute("data-glade-theme-wrapper","true");for(var k=0;k<rest.length;k++){var host=document.getElementById(rest[k].hostId);if(host){cmp.appendChild(host);}create(rest[k]);}});}if(window.$Lightning){go();}else{window.addEventListener("load",go);}})();`)
	b.WriteString(`</script><script type="module" src="/lightning/runtime/shell/app.js"></script></body></html>`)
	return b.String()
}

func lwcShellWorkbenchBuilderActive(activeRoute string) bool {
	activeRoute = lwcShellRoutePath(activeRoute)
	return activeRoute == "/lwc/builder"
}

func lwcShellWorkbenchHomeActive(activeRoute string) bool {
	activeRoute = lwcShellRoutePath(activeRoute)
	return activeRoute == "/" || activeRoute == "/lwc"
}

func lwcShellRoutePath(activeRoute string) string {
	activeRoute = strings.TrimSpace(activeRoute)
	if activeRoute == "" {
		return ""
	}
	if parsed, err := url.Parse(activeRoute); err == nil && parsed.Path != "" {
		return parsed.Path
	}
	if before, _, ok := strings.Cut(activeRoute, "?"); ok {
		activeRoute = before
	}
	return activeRoute
}

func lwcShellApplySampleRecordID(model *lwcshell.WorkbenchModel, objectName, recordID string) {
	if model == nil || strings.TrimSpace(recordID) == "" {
		return
	}
	for i := range model.Routes {
		route := &model.Routes[i]
		if route.Kind != lwcshell.RenderTargetRecordPage || !strings.EqualFold(route.ObjectName, objectName) {
			continue
		}
		if route.RecordID != "" && !strings.Contains(route.RecordID, "<") && route.RecordID != lwcShellDefaultSampleRecordID {
			continue
		}
		route.RecordID = recordID
		path := "/lwc/preview/record/" + url.PathEscape(route.ObjectName) + "/" + url.PathEscape(recordID)
		if route.PageName != "" {
			path += "?page=" + url.QueryEscape(route.PageName)
		}
		route.URL = path
	}
}

func lwcShellWorkbenchBuilderHTML(model lwcshell.WorkbenchModel) string {
	if len(model.Components) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="glade-workbench-builder glade-app-builder-shell" data-glade-workbench-builder aria-label="Local page builder">`)
	b.WriteString(`<header class="glade-builder-app-header">`)
	b.WriteString(`<a class="glade-builder-home" data-glade-home-link href="/lwc">Home</a>`)
	b.WriteString(`<div class="glade-builder-brand"><span class="glade-builder-tile" aria-hidden="true">&#9638;</span><strong>Glade LWC Workbench</strong><span>Local shell</span></div>`)
	b.WriteString(lwcShellRouteMenuHTML(model))
	b.WriteString(`<div class="glade-builder-page-name" data-glade-draft-title>Draft App Page</div>`)
	b.WriteString(`</header>`)
	b.WriteString(`<div class="glade-builder-commandbar" aria-label="Builder controls">`)
	b.WriteString(`<label class="glade-form-factor-select">Form factor<select data-glade-form-factor><option value="Large">Large</option><option value="Medium">Medium</option><option value="Small">Small</option></select></label>`)
	b.WriteString(`<div class="glade-builder-command-group"><span class="glade-builder-command-label">Viewport</span>`)
	b.WriteString(`<div class="glade-segmented-control" role="group" aria-label="Form factor">`)
	for _, option := range []string{"Large", "Medium", "Small"} {
		b.WriteString(`<button class="glade-shell-button" type="button" data-glade-form-factor-option="`)
		b.WriteString(html.EscapeString(option))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(option))
		b.WriteString(`</button>`)
	}
	b.WriteString(`</div></div>`)
	b.WriteString(`<label class="glade-layout-select"><span>Layout</span><select data-glade-layout-picker><option value="mainSidebar">Main + sidebar</option><option value="single">Single column</option><option value="twoColumn">Two columns</option></select></label>`)
	b.WriteString(`<div class="glade-builder-status" data-glade-builder-status><span data-glade-draft-status></span></div>`)
	b.WriteString(`</div>`)
	b.WriteString(`<div class="glade-builder-layout">`)
	b.WriteString(`<section class="glade-component-catalog glade-builder-palette" data-glade-component-catalog aria-label="Available Lightning Web Components">`)
	b.WriteString(`<div class="glade-catalog-title"><h2>Components</h2><span data-glade-catalog-count>`)
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", len(model.Components))))
	b.WriteString(`</span></div>`)
	b.WriteString(`<div class="glade-catalog-header"><label><span>Search</span><input type="search" data-glade-component-search placeholder="Search components" autocomplete="off"></label></div><div class="glade-component-list">`)
	for _, component := range model.Components {
		b.WriteString(`<article class="glade-component-card" data-glade-component-card data-glade-component="`)
		b.WriteString(html.EscapeString(component.QualifiedName))
		b.WriteString(`" data-glade-drag-component="`)
		b.WriteString(html.EscapeString(component.QualifiedName))
		b.WriteString(`" data-glade-component-exposed="`)
		b.WriteString(html.EscapeString(fmt.Sprintf("%t", component.Exposed)))
		b.WriteString(`" draggable="true"><div><strong>`)
		b.WriteString(html.EscapeString(component.Label))
		b.WriteString(`</strong><code>`)
		b.WriteString(html.EscapeString(component.QualifiedName))
		b.WriteString(`</code></div>`)
		b.WriteString(`<div class="glade-component-actions">`)
		for _, region := range []struct{ name, label string }{
			{"main", "Main"},
			{"sidebar", "Sidebar"},
		} {
			b.WriteString(`<button class="glade-shell-button" type="button" data-glade-add-component="`)
			b.WriteString(html.EscapeString(component.QualifiedName))
			b.WriteString(`" data-glade-region="`)
			b.WriteString(html.EscapeString(region.name))
			b.WriteString(`">`)
			b.WriteString(html.EscapeString("+ " + region.label))
			b.WriteString(`</button>`)
		}
		b.WriteString(`</div></article>`)
	}
	b.WriteString(`</div></section>`)
	b.WriteString(`<section class="glade-builder-canvas-shell" aria-label="Page canvas">`)
	b.WriteString(`<div class="glade-canvas-toolbar"><span>Canvas</span><span>Desktop</span></div>`)
	b.WriteString(`<section class="glade-page-canvas" data-glade-page-canvas data-glade-page-layout data-glade-layout="mainSidebar" aria-label="Draft Lightning page">`)
	for _, region := range []struct{ name, label string }{
		{"main", "Main"},
		{"sidebar", "Sidebar"},
	} {
		b.WriteString(`<section class="glade-draft-region" data-glade-region-drop="`)
		b.WriteString(html.EscapeString(region.name))
		b.WriteString(`"><h3>`)
		b.WriteString(html.EscapeString(region.label))
		b.WriteString(`</h3><div data-glade-region-items="`)
		b.WriteString(html.EscapeString(region.name))
		b.WriteString(`"></div></section>`)
	}
	b.WriteString(`</section></section>`)
	b.WriteString(`<aside class="glade-builder-properties" aria-label="Page properties">`)
	b.WriteString(`<div class="glade-properties-header"><div><span>Context</span><h2>Page</h2></div><button class="glade-shell-button" type="button" data-glade-clear-draft>Clear</button></div>`)
	b.WriteString(`<label>Target<select data-glade-page-kind data-glade-target-picker>`)
	for _, option := range []struct{ value, label string }{
		{string(lwcshell.RenderTargetAppPage), "App page"},
		{string(lwcshell.RenderTargetRecordPage), "Record page"},
		{string(lwcshell.RenderTargetHomePage), "Home page"},
		{string(lwcshell.RenderTargetTab), "Tab"},
		{string(lwcshell.RenderTargetURLAddressable), "URL addressable"},
		{string(lwcshell.RenderTargetQuickAction), "Record action"},
		{string(lwcshell.RenderTargetCommunityPage), "Community page"},
		{string(lwcshell.RenderTargetUtilityBar), "Utility bar"},
		{string(lwcshell.RenderTargetFlowScreen), "Flow screen"},
		{string(lwcshell.RenderTargetFlowAction), "Flow action"},
	} {
		b.WriteString(`<option value="`)
		b.WriteString(html.EscapeString(option.value))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(option.label))
		b.WriteString(`</option>`)
	}
	b.WriteString(`</select></label>`)
	b.WriteString(`<label>Component<select data-glade-component-picker><option value="">Choose component</option>`)
	for _, component := range model.Components {
		b.WriteString(`<option value="`)
		b.WriteString(html.EscapeString(component.QualifiedName))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(component.Label))
		b.WriteString(`</option>`)
	}
	b.WriteString(`</select></label>`)
	b.WriteString(`<details class="glade-property-group" open><summary>Record context</summary>`)
	b.WriteString(`<label>Object<input data-glade-object-input data-glade-object-selector value="Account" autocomplete="off"></label>`)
	b.WriteString(`<label>Record<span class="glade-inline-control"><input data-glade-record-input value="`)
	b.WriteString(html.EscapeString(model.SampleRecordID))
	b.WriteString(`" autocomplete="off"><button class="glade-icon-button" type="button" data-glade-sample-record title="Use sample record ID" aria-label="Use sample record ID">#</button></span></label>`)
	b.WriteString(`<label>App<input data-glade-app-input data-glade-app-selector value="Local" autocomplete="off"></label>`)
	b.WriteString(`<label>Community<input data-glade-community-selector value="" autocomplete="off" placeholder="Site name"></label>`)
	b.WriteString(`<label class="glade-checkbox-control"><input type="checkbox" data-glade-console-mode> Console mode</label>`)
	b.WriteString(`</details>`)
	b.WriteString(`<details class="glade-property-group" open><summary>Navigation state</summary>`)
	b.WriteString(`<label>State key<input data-glade-state-key value="" autocomplete="off" placeholder="c__name"></label>`)
	b.WriteString(`<label>State value<input data-glade-state-value value="" autocomplete="off"></label>`)
	b.WriteString(`</details>`)
	b.WriteString(`<details class="glade-property-group" open><summary>Flow inputs</summary>`)
	b.WriteString(`<label class="glade-flow-inputs">Flow inputs<textarea data-glade-flow-inputs rows="2" placeholder="name=value"></textarea></label>`)
	b.WriteString(`</details>`)
	b.WriteString(`</aside></div></section>`)
	return b.String()
}

func lwcShellWorkbenchHomeHTML(model lwcshell.WorkbenchModel) string {
	var b strings.Builder
	b.WriteString(`<section class="glade-workbench-home" data-glade-workbench-home aria-label="LWC preview home">`)
	b.WriteString(`<header class="glade-home-header"><div><h1>LWC Preview</h1><p>`)
	b.WriteString(html.EscapeString(workbenchAppLabel(model)))
	b.WriteString(`</p></div><a class="glade-shell-button glade-home-builder" data-glade-builder-link href="/lwc/builder">Open builder</a></header>`)
	b.WriteString(lwcShellHomeRouteGroupHTML("Tabs", model.Routes, func(route lwcshell.ShellRoute) bool {
		return route.Kind == lwcshell.RenderTargetTab
	}, true))
	b.WriteString(`<div class="glade-home-grid">`)
	b.WriteString(lwcShellHomeRouteGroupHTML("App pages", model.Routes, func(route lwcshell.ShellRoute) bool {
		return route.Kind == lwcshell.RenderTargetAppPage || route.Kind == lwcshell.RenderTargetHomePage
	}, false))
	b.WriteString(lwcShellHomeRouteGroupHTML("Record pages", model.Routes, func(route lwcshell.ShellRoute) bool {
		return route.Kind == lwcshell.RenderTargetRecordPage
	}, false))
	b.WriteString(lwcShellHomeRouteGroupHTML("Components", model.Routes, func(route lwcshell.ShellRoute) bool {
		return route.Kind == lwcshell.RenderTargetComponent || route.Kind == lwcshell.RenderTargetURLAddressable
	}, false))
	b.WriteString(lwcShellHomeRouteGroupHTML("Utilities and flows", model.Routes, func(route lwcshell.ShellRoute) bool {
		return route.Kind == lwcshell.RenderTargetUtilityBar || route.Kind == lwcshell.RenderTargetFlowScreen || route.Kind == lwcshell.RenderTargetFlowAction || route.Kind == lwcshell.RenderTargetQuickAction || route.Kind == lwcshell.RenderTargetCommunityPage
	}, false))
	b.WriteString(`</div></section>`)
	return b.String()
}

func lwcShellHomeRouteGroupHTML(title string, routes []lwcshell.ShellRoute, include func(lwcshell.ShellRoute) bool, tabGroup bool) string {
	var matches []lwcshell.ShellRoute
	for _, route := range routes {
		if include(route) {
			matches = append(matches, route)
		}
	}
	if len(matches) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="glade-home-section`)
	if tabGroup {
		b.WriteString(` glade-home-tabs`)
	}
	b.WriteString(`"><header><h2>`)
	b.WriteString(html.EscapeString(title))
	b.WriteString(`</h2><span>`)
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", len(matches))))
	b.WriteString(`</span></header><div>`)
	for _, route := range matches {
		b.WriteString(`<a data-glade-home-route`)
		if tabGroup {
			b.WriteString(` data-glade-home-tab`)
		}
		b.WriteString(` data-glade-route-kind="`)
		b.WriteString(html.EscapeString(string(route.Kind)))
		b.WriteString(`" href="`)
		b.WriteString(html.EscapeString(route.URL))
		b.WriteString(`"><strong>`)
		b.WriteString(html.EscapeString(route.Label))
		b.WriteString(`</strong><code>`)
		b.WriteString(html.EscapeString(route.URL))
		b.WriteString(`</code></a>`)
	}
	b.WriteString(`</div></section>`)
	return b.String()
}

func lwcShellRouteMenuHTML(model lwcshell.WorkbenchModel) string {
	if len(model.Routes) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<details class="glade-route-menu" data-glade-route-menu aria-label="Preview routes"><summary>Routes <span>`)
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", len(model.Routes))))
	b.WriteString(`</span></summary><div>`)
	for _, route := range model.Routes {
		b.WriteString(`<a data-glade-route data-glade-route-link data-glade-route-kind="`)
		b.WriteString(html.EscapeString(string(route.Kind)))
		b.WriteString(`" href="`)
		b.WriteString(html.EscapeString(route.URL))
		b.WriteString(`"><span>`)
		b.WriteString(html.EscapeString(route.Label))
		b.WriteString(`</span><code>`)
		b.WriteString(html.EscapeString(route.URL))
		b.WriteString(`</code></a>`)
	}
	b.WriteString(`</div></details>`)
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
		b.WriteString(`<a data-glade-route`)
		if link.URL == "/lwc" && link.Label == "Home" {
			b.WriteString(` data-glade-home-link`)
		}
		if link.URL == "/lwc/builder" && link.Label == "Builder" {
			b.WriteString(` data-glade-builder-link`)
		}
		b.WriteString(` href="`)
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
		b.WriteString(`<a data-glade-route`)
		if link.URL == "/lwc" && link.Label == "Home" {
			b.WriteString(` data-glade-home-link`)
		}
		if link.URL == "/lwc/builder" && link.Label == "Builder" {
			b.WriteString(` data-glade-builder-link`)
		}
		b.WriteString(` href="`)
		b.WriteString(html.EscapeString(link.URL))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(link.Label))
		b.WriteString(`</a>`)
	}
	b.WriteString(`</nav>`)
	return b.String()
}

func lwcShellUtilityBarHTML(items []lwcshell.UtilityItem) string {
	if len(items) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="glade-utility-dock" data-glade-utility-bar aria-label="Workspace utilities"><header><strong>Utilities</strong><span>`)
	b.WriteString(html.EscapeString(fmt.Sprintf("%d", len(items))))
	b.WriteString(`</span></header><nav class="glade-utility-bar" aria-label="Utility bar">`)
	for _, item := range items {
		label := strings.TrimSpace(item.Label)
		if label == "" {
			label = item.ID
		}
		b.WriteString(`<a data-glade-utility-item="`)
		b.WriteString(html.EscapeString(item.ID))
		b.WriteString(`" href="`)
		b.WriteString(html.EscapeString(item.URL))
		b.WriteString(`">`)
		b.WriteString(html.EscapeString(label))
		b.WriteString(`</a>`)
	}
	b.WriteString(`</nav></section>`)
	return b.String()
}

func lwcShellCommunityChromeHTML(ctx lwcshell.CommunityContext) string {
	if strings.TrimSpace(ctx.Site) == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="glade-community-chrome" data-glade-community-chrome><strong>`)
	b.WriteString(html.EscapeString(ctx.Site))
	b.WriteString(`</strong>`)
	if len(ctx.Menus) > 0 {
		keys := make([]string, 0, len(ctx.Menus))
		for key := range ctx.Menus {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(`<nav data-glade-community-menu="`)
			b.WriteString(html.EscapeString(key))
			b.WriteString(`">`)
			for _, item := range ctx.Menus[key] {
				b.WriteString(`<a href="`)
				b.WriteString(html.EscapeString(communityMenuItemURL(ctx, item)))
				b.WriteString(`">`)
				b.WriteString(html.EscapeString(item.Label))
				b.WriteString(`</a>`)
			}
			b.WriteString(`</nav>`)
		}
	}
	if len(ctx.ManagedContent) > 0 {
		keys := make([]string, 0, len(ctx.ManagedContent))
		for key := range ctx.ManagedContent {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			item := ctx.ManagedContent[key]
			b.WriteString(`<article data-glade-managed-content="`)
			b.WriteString(html.EscapeString(key))
			b.WriteString(`"><h2>`)
			b.WriteString(html.EscapeString(item.Title))
			b.WriteString(`</h2><p>`)
			b.WriteString(html.EscapeString(item.Body))
			b.WriteString(`</p></article>`)
		}
	}
	b.WriteString(`</section>`)
	return b.String()
}

func communityMenuItemURL(ctx lwcshell.CommunityContext, item lwcshell.CommunityMenuItem) string {
	switch item.Type {
	case "comm__managedContentPage":
		if item.ContentKey != "" {
			return "/lwc/preview/community/" + url.PathEscape(ctx.Site) + "/" + url.PathEscape(item.Target) + "?contentKey=" + url.QueryEscape(item.ContentKey)
		}
	}
	if item.Target != "" {
		return "/lwc/preview/community/" + url.PathEscape(ctx.Site) + "/" + url.PathEscape(item.Target)
	}
	return "#"
}

func lwcShellFlowEventsHTML(ctx lwcshell.FlowContext) string {
	if strings.TrimSpace(ctx.APIName) == "" {
		return ""
	}
	return `<section class="glade-flow-events" data-glade-flow-events aria-label="Flow events"></section>`
}

type lwcShellNavLink struct {
	Label string
	URL   string
}

func lwcShellWorkbenchNavLinks(model lwcshell.WorkbenchModel) []lwcShellNavLink {
	home := lwcShellNavLink{Label: "Home", URL: "/lwc"}
	builder := lwcShellNavLink{Label: "Builder", URL: "/lwc/builder"}
	if len(model.Apps) == 0 || len(model.Apps[0].NavItems) == 0 {
		links := make([]lwcShellNavLink, 0, len(model.Routes)+2)
		links = append(links, home, builder)
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
	links := make([]lwcShellNavLink, 0, len(model.Apps[0].NavItems)+2)
	links = append(links, home, builder)
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
	if r.Method != http.MethodGet || len(parts) < 2 {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning runtime asset")
		return
	}
	contentType, content, ok := lightningRuntimeAsset(parts[0], strings.Join(parts[1:], "/"))
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning runtime asset")
		return
	}
	w.Header().Set("Content-Type", contentType)
	setDevNoStore(w)
	_, _ = w.Write(content)
}

func lightningRuntimeAsset(kind, name string) (string, []byte, bool) {
	kind, kindOK := cleanLightningRuntimeAssetKind(kind)
	name, nameOK := cleanLightningRuntimeAssetName(name)
	if !kindOK || !nameOK {
		return "", nil, false
	}
	candidates := []string{name}
	if strings.HasSuffix(name, ".js") {
		candidates = append(candidates, strings.TrimSuffix(name, ".js")+".mjs")
	}
	for _, dir := range lightningRuntimeAssetDirs(kind) {
		for _, candidate := range candidates {
			content, err := os.ReadFile(filepath.Join(dir, candidate))
			if err != nil {
				continue
			}
			return lightningRuntimeContentType(candidate), content, true
		}
	}
	return "", nil, false
}

func cleanLightningRuntimeAssetKind(kind string) (string, bool) {
	kind = strings.TrimSpace(strings.ReplaceAll(kind, "\\", "/"))
	if kind == "" || strings.Contains(kind, "\x00") || strings.Contains(kind, "/") || strings.Contains(kind, "..") {
		return "", false
	}
	return kind, true
}

func cleanLightningRuntimeAssetName(name string) (string, bool) {
	name = strings.TrimSpace(strings.ReplaceAll(name, "\\", "/"))
	if name == "" || strings.Contains(name, "\x00") || strings.Contains(name, "..") {
		return "", false
	}
	cleaned := strings.TrimPrefix(path.Clean("/"+name), "/")
	if cleaned == "" || cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", false
	}
	return cleaned, true
}

func lightningRuntimeAssetDirs(kind string) []string {
	var dirs []string
	if sourceRoot, err := gladehome.SourceRoot(); err == nil {
		dirs = append(dirs, filepath.Join(sourceRoot, "lwcruntime", "src", kind))
	}
	if dir, err := gladehome.RuntimeAssetDir(kind); err == nil {
		dirs = appendRuntimeAssetDir(dirs, dir)
	}
	return dirs
}

func appendRuntimeAssetDir(dirs []string, dir string) []string {
	for _, existing := range dirs {
		if existing == dir {
			return dirs
		}
	}
	return append(dirs, dir)
}

func (s *Server) handleLightningAssets(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet || len(parts) != 4 ||
		parts[0] != "icons" || parts[2] != "svg" || parts[3] != "symbols.svg" {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning asset")
		return
	}
	contentType, content, ok := lightningRuntimeAsset("slds", "design-system/assets/"+strings.Join(parts, "/"))
	if ok {
		w.Header().Set("Content-Type", contentType)
		setDevNoStore(w)
		_, _ = w.Write(content)
		return
	}
	spriteContent, ok := localSLDSSprite(parts[1])
	if !ok {
		writeSalesforceError(w, errUnknownEndpoint, "unknown lightning asset")
		return
	}
	w.Header().Set("Content-Type", "image/svg+xml; charset=utf-8")
	setDevNoStore(w)
	_, _ = w.Write([]byte(spriteContent))
}

func localSLDSSprite(sprite string) (string, bool) {
	switch strings.TrimSpace(sprite) {
	case "utility-sprite", "action-sprite", "standard-sprite", "doctype-sprite", "custom-sprite":
	default:
		return "", false
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" style="display:none">`)
	for _, id := range localSLDSIconIDs() {
		b.WriteString(`<symbol id="`)
		b.WriteString(html.EscapeString(id))
		b.WriteString(`" viewBox="0 0 52 52"><path d="M26 4 48 26 26 48 4 26z"/></symbol>`)
	}
	b.WriteString(`</svg>`)
	return b.String(), true
}

func localSLDSIconIDs() []string {
	return []string{
		"add",
		"apps",
		"arrowdown",
		"back",
		"check",
		"chevrondown",
		"chevronleft",
		"chevronright",
		"chevronup",
		"close",
		"delete",
		"down",
		"edit",
		"error",
		"event",
		"fallback",
		"filter",
		"help",
		"info",
		"new",
		"preview",
		"record",
		"refresh",
		"right",
		"search",
		"settings",
		"success",
		"up",
		"user",
		"warning",
	}
}

func lightningRuntimeContentType(name string) string {
	switch {
	case strings.HasSuffix(name, ".css"):
		return "text/css; charset=utf-8"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml; charset=utf-8"
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(name, ".gif"):
		return "image/gif"
	case strings.HasSuffix(name, ".json"):
		return "application/json; charset=utf-8"
	case strings.HasSuffix(name, ".woff"):
		return "font/woff"
	case strings.HasSuffix(name, ".woff2"):
		return "font/woff2"
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
	activeContext := model.Active.Context
	defaultContext, selectedContext, contexts, presetDiagnostics := lwcLocalContextPresets(s.Source.Project, activeRoute)
	payload := lwcLocalContextPayload{
		ActiveRoute:     activeRoute,
		PageReference:   lwcShellPageReference(shell),
		Context:         activeContext,
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
		"empApi":                 "supported-local",
		"platformResourceLoader": "supported",
		"urlAddressable":         "supported-local",
		"quickActions":           "supported-local",
		"baseComponents":         "supported-local",
		"community":              "supported-local",
		"flow":                   "supported-local",
		"visualforceHost":        "supported-local",
		"platformWorkspaceApi":   "partial",
		"slds":                   "supported-local",
	}
}
