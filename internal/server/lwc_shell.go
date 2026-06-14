package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strings"

	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
)

type lwcShellMount struct {
	Qualified string         `json:"qualified"`
	HostID    string         `json:"hostId"`
	Attrs     map[string]any `json:"attrs"`
	Region    string         `json:"region,omitempty"`
}

func (s *Server) handleLWCShell(w http.ResponseWriter, r *http.Request, parts []string) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w, http.MethodGet)
		return
	}
	if len(parts) < 2 || parts[0] != "preview" {
		writeSalesforceError(w, errUnknownEndpoint, "unknown LWC shell endpoint")
		return
	}
	shell, redirect, diagnostics, err := s.resolveLWCShellRequest(r, parts[1:])
	if redirect != "" {
		http.Redirect(w, r, redirect, http.StatusFound)
		return
	}
	if err != nil {
		status := http.StatusBadRequest
		if len(diagnostics) > 0 && diagnostics[0].Code == "GLADELWC006" {
			status = http.StatusNotFound
		}
		writeJSON(w, status, map[string]any{"error": err.Error(), "diagnostics": diagnostics})
		return
	}
	if len(diagnostics) > 0 {
		if len(lwcShellMounts(shell)) == 0 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": diagnostics[0].Message, "diagnostics": diagnostics})
			return
		}
		shell.Diagnostics = append(shell.Diagnostics, diagnostics...)
	}
	cfg, ok, err := s.lightningBootstrapConfigLocked()
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	if !ok {
		writeSalesforceError(w, errUnsupportedFeature, "no LWC modules found in project")
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setDevNoStore(w)
	_, _ = w.Write([]byte(renderLWCShellHTML(*cfg, shell)))
}

func (s *Server) resolveLWCShellRequest(r *http.Request, parts []string) (lwcshell.ShellPage, string, []lwcshell.Diagnostic, error) {
	if len(parts) == 0 {
		return lwcshell.ShellPage{}, "", nil, fmt.Errorf("missing LWC preview target")
	}
	ctx := lwcShellContextFromQuery(r)
	switch parts[0] {
	case "component":
		if len(parts) != 3 {
			return lwcshell.ShellPage{}, "", nil, fmt.Errorf("component preview requires namespace and component")
		}
		ctx.Kind = lwcshell.RenderTargetComponent
		ctx.ComponentName = parts[1] + ":" + parts[2]
		shell, diagnostics, err := s.validateLWCShellPage(lwcshell.ShellPage{Context: ctx}, nil, nil)
		return shell, "", diagnostics, err
	case "record":
		if len(parts) < 3 {
			return lwcshell.ShellPage{}, "", nil, fmt.Errorf("record preview requires object API name and record ID")
		}
		ctx.Kind = lwcshell.RenderTargetRecordPage
		ctx.ObjectAPIName = parts[1]
		ctx.RecordID = parts[2]
		ctx.PageName = r.URL.Query().Get("page")
		shell, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, ctx)
		shell, diagnostics, err = s.validateLWCShellPage(shell, diagnostics, err)
		return shell, "", diagnostics, err
	case "app":
		ctx.Kind = lwcshell.RenderTargetAppPage
		ctx.PageName = firstPathOrQueryValue(parts[1:], r, "page")
		ctx.AppName = r.URL.Query().Get("app")
		shell, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, ctx)
		shell, diagnostics, err = s.validateLWCShellPage(shell, diagnostics, err)
		return shell, "", diagnostics, err
	case "home":
		ctx.Kind = lwcshell.RenderTargetHomePage
		ctx.PageName = firstPathOrQueryValue(parts[1:], r, "page")
		shell, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, ctx)
		shell, diagnostics, err = s.validateLWCShellPage(shell, diagnostics, err)
		return shell, "", diagnostics, err
	case "tab":
		if len(parts) != 2 {
			return lwcshell.ShellPage{}, "", nil, fmt.Errorf("tab preview requires tab name")
		}
		ctx.Kind = lwcshell.RenderTargetTab
		ctx.TabName = parts[1]
		shell, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, ctx)
		if err != nil {
			return shell, "", diagnostics, err
		}
		switch shell.Tab.Type {
		case lwcshell.TabTypeVisualforce:
			return shell, "/apex/" + shell.Tab.Target, nil, nil
		case lwcshell.TabTypeLWC:
			shell.Context.ComponentName = shell.Tab.Target
			shell, diagnostics, err = s.validateLWCShellPageWithTarget(shell, diagnostics, err, "lightning__Tab")
			return shell, "", diagnostics, err
		case lwcshell.TabTypeFlexiPage:
			shell, diagnostics, err = s.resolveLWCShellFlexiPageTab(ctx, shell.Tab)
			return shell, "", diagnostics, err
		default:
			return shell, "", nil, nil
		}
	default:
		return lwcshell.ShellPage{}, "", nil, fmt.Errorf("unknown LWC preview target %q", parts[0])
	}
}

func (s *Server) resolveLWCShellFlexiPageTab(ctx lwcshell.PageContext, tab lwcshell.CustomTab) (lwcshell.ShellPage, []lwcshell.Diagnostic, error) {
	var lastShell lwcshell.ShellPage
	var lastDiagnostics []lwcshell.Diagnostic
	var lastErr error
	for _, kind := range []lwcshell.RenderTargetKind{lwcshell.RenderTargetAppPage, lwcshell.RenderTargetHomePage, lwcshell.RenderTargetRecordPage} {
		pageCtx := ctx
		pageCtx.Kind = kind
		pageCtx.PageName = tab.Target
		resolved, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, pageCtx)
		if err != nil {
			lastShell, lastDiagnostics, lastErr = resolved, diagnostics, err
			if lwcShellPageKindMismatch(diagnostics) {
				continue
			}
			return resolved, diagnostics, err
		}
		resolved.Tab = tab
		resolved.Context.Kind = lwcshell.RenderTargetTab
		resolved.Context.TabName = ctx.TabName
		resolved.Context.PageName = tab.Target
		return s.validateLWCShellPageWithTarget(resolved, diagnostics, nil, lwcShellTargetName(kind))
	}
	return lastShell, lastDiagnostics, lastErr
}

func lwcShellPageKindMismatch(diagnostics []lwcshell.Diagnostic) bool {
	return len(diagnostics) > 0 && diagnostics[0].Code == "GLADELWC009"
}

func (s *Server) validateLWCShellPage(shell lwcshell.ShellPage, diagnostics []lwcshell.Diagnostic, err error) (lwcshell.ShellPage, []lwcshell.Diagnostic, error) {
	return s.validateLWCShellPageWithTarget(shell, diagnostics, err, "")
}

func (s *Server) validateLWCShellPageWithTarget(shell lwcshell.ShellPage, diagnostics []lwcshell.Diagnostic, err error, targetOverride string) (lwcshell.ShellPage, []lwcshell.Diagnostic, error) {
	if err != nil || s == nil {
		return shell, diagnostics, err
	}
	idx, indexErr := lwc.BuildIndex(s.Source.Project)
	if indexErr != nil {
		return shell, diagnostics, indexErr
	}
	if shell.Context.ComponentName != "" && len(shell.Regions) == 0 {
		component := lwcshell.PageComponent{ComponentName: shell.Context.ComponentName}
		updated, diag := s.applyLWCMetadataToComponent(idx, component, shell.Context, targetOverride)
		if diag.Code != "" {
			return shell, append(diagnostics, diag), fmt.Errorf("%s", diag.Message)
		}
		shell.Context.ComponentName = updated.ComponentName
		shell.Regions = []lwcshell.PageRegion{{
			Name:       "main",
			Components: []lwcshell.PageComponent{updated},
		}}
		return shell, diagnostics, nil
	}
	for i := range shell.Regions {
		var components []lwcshell.PageComponent
		for _, component := range shell.Regions[i].Components {
			updated, diag := s.applyLWCMetadataToComponent(idx, component, shell.Context, targetOverride)
			if diag.Code != "" {
				if diag.Code == "GLADELWC005" {
					diagnostics = append(diagnostics, diag)
					continue
				}
				return shell, append(diagnostics, diag), fmt.Errorf("%s", diag.Message)
			}
			components = append(components, updated)
		}
		shell.Regions[i].Components = components
	}
	return shell, diagnostics, nil
}

func (s *Server) applyLWCMetadataToComponent(idx lwc.Index, component lwcshell.PageComponent, ctx lwcshell.PageContext, targetOverride string) (lwcshell.PageComponent, lwcshell.Diagnostic) {
	namespace, name := splitQualifiedLWCName(component.ComponentName, s.Source.Project.Namespace)
	bundle, ok := idx.Bundle(name)
	if !ok || bundle.MetaFile == "" {
		return component, lwcshell.Diagnostic{Code: "GLADELWC005", Message: fmt.Sprintf("LWC component %q not found", component.ComponentName)}
	}
	meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
	if err != nil {
		return component, lwcshell.Diagnostic{Code: "GLADELWC005", Message: fmt.Sprintf("read LWC metadata for %q: %v", component.ComponentName, err)}
	}
	target := strings.TrimSpace(targetOverride)
	if target == "" {
		target = lwcShellTargetName(ctx.Kind)
	}
	if target != "" && !lwcMetaSupportsTarget(meta, target) {
		return component, lwcshell.Diagnostic{Code: "GLADELWC005", Message: fmt.Sprintf("LWC component %q does not support %s", component.ComponentName, target)}
	}
	targetConfig := lwcTargetConfigFor(meta, target)
	if ctx.ObjectAPIName != "" && len(targetConfig.SupportedObjects) > 0 && !containsEqualFold(targetConfig.SupportedObjects, ctx.ObjectAPIName) {
		return component, lwcshell.Diagnostic{Code: "GLADELWC004", Message: fmt.Sprintf("LWC component %q does not support object %q", component.ComponentName, ctx.ObjectAPIName)}
	}
	if component.Properties == nil {
		component.Properties = map[string]string{}
	}
	for _, prop := range targetConfig.Properties {
		if prop.Name == "" || prop.Default == "" {
			continue
		}
		if _, exists := component.Properties[prop.Name]; !exists {
			component.Properties[prop.Name] = prop.Default
		}
	}
	if len(component.Properties) == 0 {
		component.Properties = nil
	}
	component.ComponentName = namespace + ":" + bundle.Name
	return component, lwcshell.Diagnostic{}
}

func splitQualifiedLWCName(qualified, defaultNamespace string) (namespace, name string) {
	if defaultNamespace == "" {
		defaultNamespace = "c"
	}
	qualified = strings.TrimSpace(qualified)
	if before, after, ok := strings.Cut(qualified, ":"); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	return defaultNamespace, qualified
}

func lwcShellTargetName(kind lwcshell.RenderTargetKind) string {
	switch kind {
	case lwcshell.RenderTargetRecordPage:
		return "lightning__RecordPage"
	case lwcshell.RenderTargetAppPage:
		return "lightning__AppPage"
	case lwcshell.RenderTargetHomePage:
		return "lightning__HomePage"
	case lwcshell.RenderTargetTab, lwcshell.RenderTargetComponent:
		return ""
	default:
		return ""
	}
}

func lwcMetaSupportsTarget(meta lwc.ComponentMeta, target string) bool {
	if target == "" {
		return true
	}
	for _, value := range meta.Targets {
		if strings.EqualFold(strings.TrimSpace(value), target) {
			return true
		}
	}
	for _, cfg := range meta.TargetConfigs {
		if containsEqualFold(cfg.Targets, target) {
			return true
		}
	}
	return false
}

func lwcTargetConfigFor(meta lwc.ComponentMeta, target string) lwc.TargetConfig {
	for _, cfg := range meta.TargetConfigs {
		if target == "" || containsEqualFold(cfg.Targets, target) {
			return cfg
		}
	}
	return lwc.TargetConfig{}
}

func containsEqualFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func lwcShellContextFromQuery(r *http.Request) lwcshell.PageContext {
	q := r.URL.Query()
	ctx := lwcshell.PageContext{
		RecordID:      q.Get("recordId"),
		ObjectAPIName: q.Get("objectApiName"),
		FormFactor:    q.Get("formFactor"),
		State:         map[string]string{},
	}
	for key, values := range q {
		if !strings.HasPrefix(key, "state.") || len(values) == 0 {
			continue
		}
		stateKey := strings.TrimPrefix(key, "state.")
		if stateKey != "" {
			ctx.State[stateKey] = values[len(values)-1]
		}
	}
	if len(ctx.State) == 0 {
		ctx.State = nil
	}
	return ctx
}

func firstPathOrQueryValue(parts []string, r *http.Request, key string) string {
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		return parts[0]
	}
	return r.URL.Query().Get(key)
}

func renderLWCShellHTML(cfg lwcbrowser.PageConfig, shell lwcshell.ShellPage) string {
	mounts := lwcShellMounts(shell)
	cfg.PageReference = lwcShellPageReference(shell)
	contextJSON := mustScriptJSON(shell.Context)
	mountsJSON := mustScriptJSON(mounts)
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>Glade LWC Shell</title>`)
	b.WriteString(`<style>body{margin:0;font-family:system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",sans-serif;background:#f3f3f3;color:#181818}.glade-shell-bar{height:44px;display:flex;align-items:center;padding:0 16px;background:#fff;border-bottom:1px solid #d8dde6;font-size:14px}.glade-page{max-width:1200px;margin:0 auto;padding:16px}.glade-region{display:grid;gap:16px;margin-bottom:16px}.glade-host{display:block;min-height:48px;background:#fff;border:1px solid #d8dde6;border-radius:4px;padding:12px}</style>`)
	b.WriteString(lwcbrowser.BootstrapHTML(cfg))
	b.WriteString(`</head><body><div class="glade-shell-bar">Glade LWC Shell</div><main class="glade-page">`)
	b.WriteString(lwcShellDiagnosticsHTML(shell.Diagnostics))
	b.WriteString(lwcShellHostsHTML(mounts))
	b.WriteString(`</main><script type="application/json" id="glade-lwc-context">`)
	b.WriteString(contextJSON)
	b.WriteString(`</script><script>`)
	b.WriteString(`(function(){var mounts=`)
	b.WriteString(mountsJSON)
	b.WriteString(`;function go(){for(var i=0;i<mounts.length;i++){var m=mounts[i];window.$Lightning.createComponent(m.qualified,m.attrs,m.hostId,function(){});}}if(window.$Lightning){go();}else{window.addEventListener("load",go);}})();`)
	b.WriteString(`</script></body></html>`)
	return b.String()
}

func lwcShellDiagnosticsHTML(diagnostics []lwcshell.Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<section class="glade-region" data-glade-region="diagnostics"><div class="glade-host">`)
	for i, diagnostic := range diagnostics {
		if i > 0 {
			b.WriteString(`<br>`)
		}
		b.WriteString(html.EscapeString(diagnostic.Code))
		if diagnostic.Message != "" {
			b.WriteString(`: `)
			b.WriteString(html.EscapeString(diagnostic.Message))
		}
	}
	b.WriteString(`</div></section>`)
	return b.String()
}

func lwcShellPageReference(shell lwcshell.ShellPage) map[string]any {
	state := map[string]string{}
	for key, value := range shell.Context.State {
		state[key] = value
	}
	switch shell.Context.Kind {
	case lwcshell.RenderTargetRecordPage:
		return map[string]any{
			"type": "standard__recordPage",
			"attributes": map[string]any{
				"recordId":      shell.Context.RecordID,
				"objectApiName": shell.Context.ObjectAPIName,
				"actionName":    "view",
			},
			"state": state,
		}
	case lwcshell.RenderTargetAppPage:
		return map[string]any{
			"type": "standard__app",
			"attributes": map[string]any{
				"appTarget": shell.Context.AppName,
				"pageName":  shell.Context.PageName,
			},
			"state": state,
		}
	case lwcshell.RenderTargetHomePage:
		return map[string]any{
			"type":       "standard__namedPage",
			"attributes": map[string]any{"pageName": "home"},
			"state":      state,
		}
	case lwcshell.RenderTargetTab:
		return map[string]any{
			"type":       "standard__navItemPage",
			"attributes": map[string]any{"apiName": shell.Context.TabName},
			"state":      state,
		}
	default:
		attrs := map[string]any{}
		if shell.Context.ComponentName != "" {
			attrs["componentName"] = shell.Context.ComponentName
		}
		return map[string]any{
			"type":       "standard__component",
			"attributes": attrs,
			"state":      state,
		}
	}
}

func lwcShellMounts(shell lwcshell.ShellPage) []lwcShellMount {
	base := lwcShellContextAttrs(shell.Context)
	if shell.Context.ComponentName != "" {
		attrs := copyAttrs(base)
		if len(shell.Regions) > 0 && len(shell.Regions[0].Components) > 0 {
			for key, value := range shell.Regions[0].Components[0].Properties {
				attrs[key] = value
			}
		}
		return []lwcShellMount{{
			Qualified: shell.Context.ComponentName,
			HostID:    "glade-lwc-main-0",
			Attrs:     attrs,
			Region:    "main",
		}}
	}
	var mounts []lwcShellMount
	next := 0
	for _, region := range shell.Regions {
		regionName := strings.TrimSpace(region.Name)
		if regionName == "" {
			regionName = "main"
		}
		for _, component := range region.Components {
			attrs := copyAttrs(base)
			for key, value := range component.Properties {
				attrs[key] = value
			}
			mounts = append(mounts, lwcShellMount{
				Qualified: component.ComponentName,
				HostID:    fmt.Sprintf("glade-lwc-%d", next),
				Attrs:     attrs,
				Region:    regionName,
			})
			next++
		}
	}
	return mounts
}

func lwcShellContextAttrs(ctx lwcshell.PageContext) map[string]any {
	attrs := map[string]any{}
	if ctx.RecordID != "" {
		attrs["recordId"] = ctx.RecordID
	}
	if ctx.ObjectAPIName != "" {
		attrs["objectApiName"] = ctx.ObjectAPIName
	}
	if ctx.FormFactor != "" {
		attrs["formFactor"] = ctx.FormFactor
	}
	if len(ctx.State) > 0 {
		attrs["pageReference"] = map[string]any{"state": ctx.State}
	}
	return attrs
}

func copyAttrs(in map[string]any) map[string]any {
	out := make(map[string]any, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func lwcShellHostsHTML(mounts []lwcShellMount) string {
	if len(mounts) == 0 {
		return `<section class="glade-region" data-glade-region="main"><div class="glade-host">No components on this page.</div></section>`
	}
	var b strings.Builder
	currentRegion := ""
	for _, mount := range mounts {
		region := mount.Region
		if region == "" {
			region = "main"
		}
		if region != currentRegion {
			if currentRegion != "" {
				b.WriteString(`</section>`)
			}
			b.WriteString(`<section class="glade-region" data-glade-region="`)
			b.WriteString(html.EscapeString(region))
			b.WriteString(`">`)
			currentRegion = region
		}
		b.WriteString(`<div class="glade-host" id="`)
		b.WriteString(html.EscapeString(mount.HostID))
		b.WriteString(`"></div>`)
	}
	if currentRegion != "" {
		b.WriteString(`</section>`)
	}
	return b.String()
}

func mustScriptJSON(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = []byte("null")
	}
	out := string(raw)
	out = strings.ReplaceAll(out, "</", "<\\/")
	return out
}
