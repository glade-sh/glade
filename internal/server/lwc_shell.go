package server

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/lwcshell"
	"github.com/glade-sh/glade/internal/project"
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
	if len(parts) == 0 || (len(parts) == 1 && parts[0] == "preview") {
		cfg, ok, err := s.lightningBootstrapConfigLocked()
		if err != nil {
			writeSalesforceError(w, errUnsupportedFeature, err.Error())
			return
		}
		if !ok {
			cfg = &lwcbrowser.PageConfig{}
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		setDevNoStore(w)
		_, _ = w.Write([]byte(renderLWCShellDocument(s.Source.Project, *cfg, lwcshell.ShellPage{}, "/lwc")))
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
		if len(lwcShellMounts(shell)) == 0 && lwcShellDiagnosticsBlockEmptyMounts(diagnostics) {
			writeJSON(w, http.StatusBadRequest, map[string]any{"error": diagnostics[0].Message, "diagnostics": diagnostics})
			return
		}
		shell.Diagnostics = appendLWCShellDiagnosticsUnique(shell.Diagnostics, diagnostics...)
	}
	cfg, ok, err := s.lightningBootstrapConfigLocked()
	if err != nil {
		writeSalesforceError(w, errUnsupportedFeature, err.Error())
		return
	}
	if !ok {
		if len(lwcShellMounts(shell)) == 0 && len(shell.Diagnostics) > 0 {
			cfg = &lwcbrowser.PageConfig{}
		} else {
			writeSalesforceError(w, errUnsupportedFeature, "no LWC modules found in project")
			return
		}
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	setDevNoStore(w)
	_, _ = w.Write([]byte(renderLWCShellDocument(s.Source.Project, *cfg, shell, r.URL.RequestURI())))
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
	case "cmp":
		if len(parts) != 3 {
			return lwcshell.ShellPage{}, "", nil, fmt.Errorf("URL-addressable preview requires namespace and component")
		}
		if diag, ok := lwcShellInvalidURLAddressableStateDiagnostic(r); ok {
			return lwcshell.ShellPage{}, "", []lwcshell.Diagnostic{diag}, fmt.Errorf("%s", diag.Message)
		}
		ctx.Kind = lwcshell.RenderTargetComponent
		ctx.ComponentName = parts[1] + ":" + parts[2]
		shell, diagnostics, err := s.validateLWCShellPageWithTarget(lwcshell.ShellPage{Context: ctx}, nil, nil, "lightning__UrlAddressable")
		return shell, "", diagnostics, err
	case "community":
		shell, diagnostics, err := s.resolveLWCShellCommunityRequest(r, parts[1:])
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
	case "action":
		switch {
		case len(parts) == 3 && strings.EqualFold(parts[1], "global"):
			ctx.Kind = lwcshell.RenderTargetQuickAction
			ctx.ActionName = parts[2]
		case len(parts) == 4:
			ctx.Kind = lwcshell.RenderTargetQuickAction
			ctx.ObjectAPIName = parts[1]
			ctx.RecordID = parts[2]
			ctx.ActionName = parts[3]
		default:
			return lwcshell.ShellPage{}, "", nil, fmt.Errorf("quick action preview requires object, record ID, and action name or global action name")
		}
		shell, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, ctx)
		shell, diagnostics, err = s.validateLWCShellPageWithTarget(shell, diagnostics, err, "lightning__RecordAction")
		return shell, "", diagnostics, err
	case "app":
		ctx.Kind = lwcshell.RenderTargetAppPage
		ctx.PageName = firstPathOrQueryValue(parts[1:], r, "page")
		ctx.AppName = r.URL.Query().Get("app")
		if ctx.AppName == "" {
			ctx.AppName = ctx.PageName
		}
		shell, diagnostics, err := lwcshell.ResolvePageTarget(s.Source.Project, ctx)
		if err != nil && len(diagnostics) > 0 && diagnostics[0].Code == "GLADELWC006" && ctx.AppName != "" && ctx.PageName == ctx.AppName {
			fallback := ctx
			fallback.Kind = lwcshell.RenderTargetTab
			fallback.PageName = ""
			fallback.TabName = ""
			shell, diagnostics, err = lwcshell.ResolvePageTarget(s.Source.Project, fallback)
			if err == nil {
				var redirect string
				shell, redirect, diagnostics, err = s.resolveLWCShellTabTarget(shell, diagnostics)
				if redirect != "" {
					return shell, redirect, diagnostics, err
				}
			}
		}
		shell, diagnostics, err = s.validateLWCShellPage(shell, diagnostics, err)
		shell, diagnostics = s.addApplicationModeDiagnostics(shell, diagnostics)
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
		return s.resolveLWCShellTabTarget(shell, diagnostics)
	default:
		return lwcshell.ShellPage{}, "", nil, fmt.Errorf("unknown LWC preview target %q", parts[0])
	}
}

func (s *Server) resolveLWCShellTabTarget(shell lwcshell.ShellPage, diagnostics []lwcshell.Diagnostic) (lwcshell.ShellPage, string, []lwcshell.Diagnostic, error) {
	switch shell.Tab.Type {
	case lwcshell.TabTypeVisualforce:
		return shell, "/apex/" + shell.Tab.Target, nil, nil
	case lwcshell.TabTypeLWC:
		shell.Context.ComponentName = shell.Tab.Target
		var err error
		shell, diagnostics, err = s.validateLWCShellPageWithTarget(shell, diagnostics, nil, "lightning__Tab")
		return shell, "", diagnostics, err
	case lwcshell.TabTypeFlexiPage:
		var err error
		shell, diagnostics, err = s.resolveLWCShellFlexiPageTab(shell.Context, shell.Tab)
		return shell, "", diagnostics, err
	default:
		return shell, "", diagnostics, nil
	}
}

func (s *Server) resolveLWCShellCommunityRequest(r *http.Request, parts []string) (lwcshell.ShellPage, []lwcshell.Diagnostic, error) {
	if len(parts) < 2 {
		return lwcshell.ShellPage{}, nil, fmt.Errorf("community preview requires site and page or component")
	}
	site := strings.TrimSpace(parts[0])
	if site == "" {
		diag := lwcshell.Diagnostic{Code: "GLADELWC100", Message: "community context required"}
		return lwcshell.ShellPage{}, []lwcshell.Diagnostic{diag}, fmt.Errorf("%s", diag.Message)
	}
	ctx := lwcShellContextFromQuery(r)
	ctx.Kind = lwcshell.RenderTargetCommunityPage
	ctx.Community.Site = site
	if strings.EqualFold(parts[1], "cmp") {
		if len(parts) != 4 {
			return lwcshell.ShellPage{}, nil, fmt.Errorf("community component preview requires site, namespace, and component")
		}
		ctx.ComponentName = parts[2] + ":" + parts[3]
		shell, diagnostics, err := s.validateLWCShellPageWithTarget(lwcshell.ShellPage{Context: ctx}, nil, nil, "lightningCommunity__Default")
		return s.finalizeCommunityShell(shell, diagnostics, err)
	}
	if len(parts) != 2 {
		diag := lwcshell.Diagnostic{Code: "GLADELWC101", Message: fmt.Sprintf("unsupported Experience Builder route %q", strings.Join(parts[1:], "/"))}
		return lwcshell.ShellPage{}, []lwcshell.Diagnostic{diag}, fmt.Errorf("%s", diag.Message)
	}
	ctx.PageName = strings.TrimSpace(parts[1])
	presetCtx, presetDiagnostics, ok := s.communityContextPreset(site, ctx.PageName)
	if !ok {
		diag := lwcshell.Diagnostic{Code: "GLADELWC100", Message: fmt.Sprintf("community context for site %q page %q not found", site, ctx.PageName)}
		return lwcshell.ShellPage{}, append(presetDiagnostics, diag), fmt.Errorf("%s", diag.Message)
	}
	ctx = mergeCommunityRouteContext(presetCtx, ctx)
	shell, diagnostics, err := s.validateLWCShellPageWithTarget(lwcshell.ShellPage{Context: ctx}, presetDiagnostics, nil, "lightningCommunity__Page")
	return s.finalizeCommunityShell(shell, diagnostics, err)
}

func (s *Server) communityContextPreset(site, page string) (lwcshell.PageContext, []lwcshell.Diagnostic, bool) {
	file, err := lwcshell.LoadContextPresets(s.Source.Project.Root)
	if err != nil {
		if presetErr, ok := err.(*lwcshell.ContextPresetError); ok && presetErr.Diagnostic.Code != "" {
			return lwcshell.PageContext{}, []lwcshell.Diagnostic{presetErr.Diagnostic}, false
		}
		return lwcshell.PageContext{}, []lwcshell.Diagnostic{{Code: "GLADELWC020", Message: err.Error()}}, false
	}
	var diagnostics []lwcshell.Diagnostic
	for _, preset := range file.Contexts {
		ctx, err := preset.ToPageContext()
		if err != nil {
			if presetErr, ok := err.(*lwcshell.ContextPresetError); ok && presetErr.Diagnostic.Code != "" {
				diagnostics = append(diagnostics, presetErr.Diagnostic)
			}
			continue
		}
		if ctx.Kind != lwcshell.RenderTargetCommunityPage {
			continue
		}
		if strings.EqualFold(ctx.Community.Site, site) && strings.EqualFold(ctx.PageName, page) {
			return ctx, diagnostics, true
		}
	}
	return lwcshell.PageContext{}, diagnostics, false
}

func mergeCommunityRouteContext(preset, route lwcshell.PageContext) lwcshell.PageContext {
	out := preset
	out.Kind = lwcshell.RenderTargetCommunityPage
	if route.PageName != "" {
		out.PageName = route.PageName
	}
	if route.Community.Site != "" {
		out.Community.Site = route.Community.Site
	}
	if route.Community.BasePath != "" {
		out.Community.BasePath = route.Community.BasePath
	}
	if route.Community.SiteID != "" {
		out.Community.SiteID = route.Community.SiteID
	}
	if route.Community.NetworkID != "" {
		out.Community.NetworkID = route.Community.NetworkID
	}
	if route.Community.Language != "" {
		out.Community.Language = route.Community.Language
	}
	if route.Community.Guest {
		out.Community.Guest = true
	}
	if route.AppName != "" {
		out.AppName = route.AppName
	}
	if route.FormFactor != "" {
		out.FormFactor = route.FormFactor
	}
	if len(route.State) > 0 {
		if out.State == nil {
			out.State = map[string]string{}
		}
		for key, value := range route.State {
			out.State[key] = value
		}
	}
	return out
}

func (s *Server) finalizeCommunityShell(shell lwcshell.ShellPage, diagnostics []lwcshell.Diagnostic, err error) (lwcshell.ShellPage, []lwcshell.Diagnostic, error) {
	if err != nil {
		return shell, diagnostics, err
	}
	shell.Context = normalizeCommunityPageContext(shell.Context)
	diagnostics = appendLWCShellDiagnosticsUnique(diagnostics, communityContextDiagnostics(shell.Context)...)
	shell.Diagnostics = appendLWCShellDiagnosticsUnique(shell.Diagnostics, diagnostics...)
	shell = s.attachCommunityThemeLayout(shell)
	return shell, diagnostics, nil
}

func normalizeCommunityPageContext(ctx lwcshell.PageContext) lwcshell.PageContext {
	ctx.Community.Site = strings.TrimSpace(ctx.Community.Site)
	ctx.Community.BasePath = strings.TrimSpace(ctx.Community.BasePath)
	if ctx.Community.BasePath == "" {
		ctx.Community.BasePath = "/s"
	}
	ctx.Community.SiteID = strings.TrimSpace(ctx.Community.SiteID)
	ctx.Community.NetworkID = strings.TrimSpace(ctx.Community.NetworkID)
	ctx.Community.Language = strings.TrimSpace(ctx.Community.Language)
	if ctx.PageReference == nil && ctx.PageName != "" {
		ctx.PageReference = map[string]any{
			"type":       "comm__namedPage",
			"attributes": map[string]any{"name": ctx.PageName},
		}
	}
	return ctx
}

func communityContextDiagnostics(ctx lwcshell.PageContext) []lwcshell.Diagnostic {
	if ctx.Kind != lwcshell.RenderTargetCommunityPage {
		return nil
	}
	var diagnostics []lwcshell.Diagnostic
	if strings.TrimSpace(ctx.Community.Site) == "" {
		diagnostics = append(diagnostics, lwcshell.Diagnostic{Code: "GLADELWC100", Message: "community context required"})
	}
	if strings.TrimSpace(ctx.Community.SiteID) == "" || strings.TrimSpace(ctx.Community.NetworkID) == "" {
		diagnostics = append(diagnostics, lwcshell.Diagnostic{Code: "GLADELWC102", Message: "community siteId or networkId is missing; local shims export empty IDs"})
	}
	return diagnostics
}

func (s *Server) attachCommunityThemeLayout(shell lwcshell.ShellPage) lwcshell.ShellPage {
	if shell.Context.Kind != lwcshell.RenderTargetCommunityPage || shell.ThemeLayout != nil {
		return shell
	}
	idx, err := lwc.BuildIndex(s.Source.Project)
	if err != nil {
		return shell
	}
	for _, name := range idx.Names() {
		bundle, ok := idx.Bundle(name)
		if !ok || bundle.MetaFile == "" {
			continue
		}
		meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
		if err != nil || !lwcMetaSupportsTarget(meta, "lightningCommunity__Theme_Layout") {
			continue
		}
		component := lwcshell.PageComponent{ComponentName: shellNamespace(s.Source.Project.Namespace) + ":" + bundle.Name}
		updated, diag := s.applyLWCMetadataToComponent(idx, component, shell.Context, "lightningCommunity__Theme_Layout")
		if diag.Code == "" {
			shell.ThemeLayout = &updated
		}
		return shell
	}
	return shell
}

func shellNamespace(namespace string) string {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return "c"
	}
	return namespace
}

func lwcShellDiagnosticsBlockEmptyMounts(diagnostics []lwcshell.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Code != "GLADELWC007" && diagnostic.Code != "GLADELWC072" && diagnostic.Code != "GLADELWC102" {
			return true
		}
	}
	return false
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
	auraComponents := lwcShellAuraComponentNames(s.Source.Project.AuraFiles)
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
			if placeholder, ok := lwcShellPlatformPlaceholder(component, s.Source.Project.Namespace); ok {
				components = append(components, placeholder)
				continue
			}
			updated, diag := s.applyLWCMetadataToComponent(idx, component, shell.Context, targetOverride)
			if diag.Code != "" {
				if diag.Code == "GLADELWC005" {
					if placeholder, ok := lwcShellAuraPlaceholder(component, s.Source.Project.Namespace, auraComponents); ok {
						components = append(components, placeholder)
						continue
					}
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

func lwcShellAuraComponentNames(paths []string) map[string]bool {
	out := map[string]bool{}
	for _, path := range paths {
		if !strings.HasSuffix(strings.ToLower(path), ".cmp") || strings.HasSuffix(strings.ToLower(path), "-meta.xml") {
			continue
		}
		name := strings.TrimSpace(filepath.Base(filepath.Dir(path)))
		if name != "" {
			out[strings.ToLower(name)] = true
		}
	}
	return out
}

func lwcShellPlatformPlaceholder(component lwcshell.PageComponent, projectNamespace string) (lwcshell.PageComponent, bool) {
	raw := strings.TrimSpace(component.ComponentName)
	if raw == "" || !strings.Contains(raw, ":") {
		return lwcshell.PageComponent{}, false
	}
	namespace, _ := splitQualifiedLWCName(raw, projectNamespace)
	if isLocalLWCNamespace(namespace, projectNamespace) {
		return lwcshell.PageComponent{}, false
	}
	component.Kind = "platform"
	component.UnsupportedReason = "Salesforce platform component is shown as a local placeholder in LWC preview."
	return component, true
}

func lwcShellAuraPlaceholder(component lwcshell.PageComponent, projectNamespace string, auraComponents map[string]bool) (lwcshell.PageComponent, bool) {
	namespace, name := splitQualifiedLWCName(component.ComponentName, projectNamespace)
	if !isLocalLWCNamespace(namespace, projectNamespace) || !auraComponents[strings.ToLower(name)] {
		return lwcshell.PageComponent{}, false
	}
	component.Kind = "aura"
	component.UnsupportedReason = "Aura component is shown as a local placeholder in LWC preview."
	return component, true
}

func isLocalLWCNamespace(namespace, projectNamespace string) bool {
	namespace = strings.TrimSpace(namespace)
	projectNamespace = strings.TrimSpace(projectNamespace)
	if namespace == "" || strings.EqualFold(namespace, "c") {
		return true
	}
	return projectNamespace != "" && strings.EqualFold(namespace, projectNamespace)
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
	case lwcshell.RenderTargetQuickAction:
		return "lightning__RecordAction"
	case lwcshell.RenderTargetCommunityPage:
		return "lightningCommunity__Page"
	case lwcshell.RenderTargetTab, lwcshell.RenderTargetComponent, lwcshell.RenderTargetURLAddressable:
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
		AppName:       q.Get("app"),
		FormFactor:    q.Get("formFactor"),
		State:         map[string]string{},
		Community: lwcshell.CommunityContext{
			Site:      q.Get("site"),
			BasePath:  q.Get("basePath"),
			SiteID:    q.Get("siteId"),
			NetworkID: q.Get("networkId"),
			Guest:     queryBool(q.Get("guest")),
			Language:  q.Get("language"),
		},
	}
	for key, values := range q {
		if len(values) == 0 {
			continue
		}
		stateKey := ""
		switch {
		case strings.HasPrefix(key, "state."):
			stateKey = strings.TrimPrefix(key, "state.")
		case strings.Contains(key, "__"):
			stateKey = key
		default:
			continue
		}
		if stateKey != "" {
			ctx.State[stateKey] = values[len(values)-1]
		}
	}
	if len(ctx.State) == 0 {
		ctx.State = nil
	}
	return ctx
}

func lwcShellInvalidURLAddressableStateDiagnostic(r *http.Request) (lwcshell.Diagnostic, bool) {
	for key := range r.URL.Query() {
		if strings.HasPrefix(key, "state.") || strings.Contains(key, "__") || lwcShellReservedQueryKey(key) {
			continue
		}
		return lwcshell.Diagnostic{
			Code:    "GLADELWC071",
			Message: fmt.Sprintf("URL-addressable state key %q must use a namespace prefix such as c__", key),
		}, true
	}
	return lwcshell.Diagnostic{}, false
}

func lwcShellReservedQueryKey(key string) bool {
	switch key {
	case "app", "recordId", "objectApiName", "formFactor", "site", "basePath", "siteId", "networkId", "guest", "language":
		return true
	default:
		return false
	}
}

func queryBool(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes":
		return true
	default:
		return false
	}
}

func (s *Server) addApplicationModeDiagnostics(shell lwcshell.ShellPage, diagnostics []lwcshell.Diagnostic) (lwcshell.ShellPage, []lwcshell.Diagnostic) {
	appName := strings.TrimSpace(shell.Context.AppName)
	if appName == "" {
		return shell, diagnostics
	}
	for _, path := range s.Source.Project.ApplicationFiles {
		app, err := lwcshell.LoadCustomApplication(path)
		if err != nil {
			continue
		}
		if !strings.EqualFold(app.Name, appName) && !strings.EqualFold(app.Label, appName) {
			continue
		}
		if app.Console {
			diag := lwcshell.Diagnostic{Code: "GLADELWC072", Message: fmt.Sprintf("console application %q uses local workspace tab approximations", app.Name)}
			diagnostics = append(diagnostics, diag)
			shell.Diagnostics = append(shell.Diagnostics, diag)
		}
		return shell, diagnostics
	}
	return shell, diagnostics
}

func appendLWCShellDiagnosticsUnique(existing []lwcshell.Diagnostic, incoming ...lwcshell.Diagnostic) []lwcshell.Diagnostic {
	for _, diagnostic := range incoming {
		duplicate := false
		for _, current := range existing {
			if current.Code == diagnostic.Code && current.Message == diagnostic.Message {
				duplicate = true
				break
			}
		}
		if !duplicate {
			existing = append(existing, diagnostic)
		}
	}
	return existing
}

func firstPathOrQueryValue(parts []string, r *http.Request, key string) string {
	if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
		return parts[0]
	}
	return r.URL.Query().Get(key)
}

func renderLWCShellHTML(cfg lwcbrowser.PageConfig, shell lwcshell.ShellPage) string {
	return renderLWCShellDocument(project.Project{}, cfg, shell, "")
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
	if shell.Context.PageReference != nil {
		return lwcShellPageReferenceWithState(shell.Context.PageReference, state)
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
	case lwcshell.RenderTargetQuickAction:
		return map[string]any{
			"type": "standard__quickAction",
			"attributes": map[string]any{
				"apiName":       shell.Context.ActionName,
				"recordId":      shell.Context.RecordID,
				"objectApiName": shell.Context.ObjectAPIName,
			},
			"state": state,
		}
	case lwcshell.RenderTargetCommunityPage:
		if shell.Context.PageName != "" {
			return map[string]any{
				"type":       "comm__namedPage",
				"attributes": map[string]any{"name": shell.Context.PageName},
				"state":      state,
			}
		}
		attrs := map[string]any{}
		if shell.Context.ComponentName != "" {
			attrs["componentName"] = shell.Context.ComponentName
		}
		return map[string]any{
			"type":       "standard__component",
			"attributes": attrs,
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

func lwcShellPageReferenceWithState(in map[string]any, state map[string]string) map[string]any {
	out := make(map[string]any, len(in)+1)
	for key, value := range in {
		out[key] = value
	}
	if len(state) == 0 {
		if _, ok := out["state"]; !ok {
			out["state"] = map[string]string{}
		}
		return out
	}
	merged := map[string]any{}
	if existing, ok := out["state"].(map[string]any); ok {
		for key, value := range existing {
			merged[key] = value
		}
	}
	if existing, ok := out["state"].(map[string]string); ok {
		for key, value := range existing {
			merged[key] = value
		}
	}
	for key, value := range state {
		merged[key] = value
	}
	out["state"] = merged
	return out
}

func lwcShellMounts(shell lwcshell.ShellPage) []lwcShellMount {
	base := lwcShellContextAttrs(shell.Context)
	mounts := []lwcShellMount{}
	if shell.ThemeLayout != nil && shell.ThemeLayout.ComponentName != "" {
		attrs := copyAttrs(base)
		for key, value := range shell.ThemeLayout.Properties {
			attrs[key] = value
		}
		mounts = append(mounts, lwcShellMount{
			Qualified: shell.ThemeLayout.ComponentName,
			HostID:    "glade-lwc-theme-layout",
			Attrs:     attrs,
			Region:    "theme",
		})
	}
	if shell.Context.ComponentName != "" {
		attrs := copyAttrs(base)
		if len(shell.Regions) > 0 && len(shell.Regions[0].Components) > 0 {
			for key, value := range shell.Regions[0].Components[0].Properties {
				attrs[key] = value
			}
		}
		mounts = append(mounts, lwcShellMount{
			Qualified: shell.Context.ComponentName,
			HostID:    "glade-lwc-main-0",
			Attrs:     attrs,
			Region:    "main",
		})
		return mounts
	}
	next := 0
	for _, region := range shell.Regions {
		regionName := strings.TrimSpace(region.Name)
		if regionName == "" {
			regionName = "main"
		}
		for _, component := range region.Components {
			if lwcShellComponentIsPlaceholder(component) {
				continue
			}
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

func lwcShellComponentIsPlaceholder(component lwcshell.PageComponent) bool {
	return strings.TrimSpace(component.Kind) != "" || strings.TrimSpace(component.UnsupportedReason) != ""
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
	if ctx.ActionName != "" {
		attrs["actionName"] = ctx.ActionName
	}
	if ctx.ActionType != "" {
		attrs["actionType"] = ctx.ActionType
	}
	if len(ctx.State) > 0 {
		attrs["state"] = ctx.State
		attrs["pageReference"] = map[string]any{"state": ctx.State}
	}
	if strings.TrimSpace(ctx.Community.Site) != "" {
		attrs["community"] = ctx.Community
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

func lwcShellRegionsHTML(shell lwcshell.ShellPage) string {
	if shell.Context.ComponentName != "" || len(shell.Regions) == 0 {
		return lwcShellHostsHTML(lwcShellMounts(shell))
	}
	var b strings.Builder
	currentRegion := ""
	nextMount := 0
	renderedAny := false
	for _, region := range shell.Regions {
		regionName := strings.TrimSpace(region.Name)
		if regionName == "" {
			regionName = "main"
		}
		for _, component := range region.Components {
			if regionName != currentRegion {
				if currentRegion != "" {
					b.WriteString(`</section>`)
				}
				b.WriteString(`<section class="glade-region" data-glade-region="`)
				b.WriteString(html.EscapeString(regionName))
				b.WriteString(`">`)
				currentRegion = regionName
			}
			if lwcShellComponentIsPlaceholder(component) {
				b.WriteString(lwcShellPlaceholderHTML(component))
			} else {
				b.WriteString(`<div class="glade-host" id="`)
				b.WriteString(html.EscapeString(fmt.Sprintf("glade-lwc-%d", nextMount)))
				b.WriteString(`"></div>`)
				nextMount++
			}
			renderedAny = true
		}
	}
	if currentRegion != "" {
		b.WriteString(`</section>`)
	}
	if !renderedAny {
		return `<section class="glade-region" data-glade-region="main"><div class="glade-host">No components on this page.</div></section>`
	}
	return b.String()
}

func lwcShellPlaceholderHTML(component lwcshell.PageComponent) string {
	reason := strings.TrimSpace(component.UnsupportedReason)
	if reason == "" {
		reason = "Component is shown as a local placeholder in LWC preview."
	}
	return `<div class="glade-host glade-placeholder"><div class="glade-placeholder-title">` +
		html.EscapeString(component.ComponentName) +
		`</div><div class="glade-placeholder-note">` +
		html.EscapeString(reason) +
		`</div></div>`
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
