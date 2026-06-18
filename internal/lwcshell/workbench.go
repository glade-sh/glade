package lwcshell

import (
	"fmt"
	"net/url"
	"sort"
	"strings"

	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/project"
)

type WorkbenchModel struct {
	Title       string           `json:"title"`
	Mode        string           `json:"mode"`
	Apps        []ShellApp       `json:"apps"`
	Routes      []ShellRoute     `json:"routes"`
	Components  []ShellComponent `json:"components"`
	ActiveRoute string           `json:"activeRoute"`
	Active      ShellPage        `json:"active"`
	Diagnostics []Diagnostic     `json:"diagnostics,omitempty"`
}

type ShellApp struct {
	Name       string   `json:"name"`
	Label      string   `json:"label"`
	Mode       string   `json:"mode"`
	NavItems   []string `json:"navItems"`
	DefaultURL string   `json:"defaultUrl"`
}

type ShellRoute struct {
	Label       string            `json:"label"`
	URL         string            `json:"url"`
	Kind        RenderTargetKind  `json:"kind"`
	PageName    string            `json:"pageName,omitempty"`
	Component   string            `json:"component,omitempty"`
	ObjectName  string            `json:"objectApiName,omitempty"`
	RecordID    string            `json:"recordId,omitempty"`
	TabName     string            `json:"tabName,omitempty"`
	Diagnostics []Diagnostic      `json:"diagnostics,omitempty"`
	State       map[string]string `json:"state,omitempty"`
}

type ShellComponent struct {
	Name          string                 `json:"name"`
	Namespace     string                 `json:"namespace"`
	QualifiedName string                 `json:"qualifiedName"`
	Label         string                 `json:"label"`
	Exposed       bool                   `json:"exposed"`
	Targets       []string               `json:"targets,omitempty"`
	TargetSupport []ShellComponentTarget `json:"targetSupport,omitempty"`
}

type ShellComponentTarget struct {
	Target               string            `json:"target"`
	Properties           map[string]string `json:"properties,omitempty"`
	SupportedObjects     []string          `json:"supportedObjects,omitempty"`
	SupportedFormFactors []string          `json:"supportedFormFactors,omitempty"`
}

func BuildWorkbenchModel(p project.Project, active ShellPage, activeRoute string) WorkbenchModel {
	routes := DiscoverShellRoutes(p)
	appName := strings.TrimSpace(active.Context.AppName)
	if appName == "" {
		appName = "Local"
	}
	app := buildWorkbenchApp(p, appName, routes)
	return WorkbenchModel{
		Title:       "Glade Lightning Shell",
		Mode:        app.Mode,
		Apps:        []ShellApp{app},
		Routes:      routes,
		Components:  DiscoverWorkbenchComponents(p),
		ActiveRoute: activeRoute,
		Active:      active,
		Diagnostics: append([]Diagnostic(nil), active.Diagnostics...),
	}
}

func buildWorkbenchApp(p project.Project, appName string, routes []ShellRoute) ShellApp {
	app := ShellApp{
		Name:       appName,
		Label:      appName,
		Mode:       "standard",
		DefaultURL: "/lwc",
	}
	if customApp, ok := findWorkbenchApplication(p, appName); ok {
		app.Name = customApp.Name
		if customApp.Label != "" {
			app.Label = customApp.Label
		} else {
			app.Label = customApp.Name
		}
		app.NavItems = append([]string(nil), customApp.NavItems...)
		if customApp.Console {
			app.Mode = "console"
		}
		if customApp.DefaultLandingTab != "" {
			app.DefaultURL = routeURLForNavItem(routes, customApp.DefaultLandingTab)
		}
		if app.DefaultURL == "" {
			app.DefaultURL = "/lwc"
		}
		return app
	}
	for _, route := range routes {
		if route.Kind == RenderTargetTab {
			app.NavItems = append(app.NavItems, route.TabName)
		}
	}
	return app
}

func findWorkbenchApplication(p project.Project, name string) (CustomApplication, bool) {
	name = normalizeTabName(name)
	for _, path := range p.ApplicationFiles {
		app, err := LoadCustomApplication(path)
		if err != nil {
			continue
		}
		if strings.EqualFold(normalizeTabName(app.Name), name) || strings.EqualFold(normalizeTabName(app.Label), name) {
			return app, true
		}
	}
	return CustomApplication{}, false
}

func routeURLForNavItem(routes []ShellRoute, item string) string {
	item = normalizeTabName(item)
	for _, route := range routes {
		if route.Kind == RenderTargetTab && strings.EqualFold(normalizeTabName(route.TabName), item) {
			return route.URL
		}
	}
	return ""
}

func DiscoverShellRoutes(p project.Project) []ShellRoute {
	namespace := strings.TrimSpace(p.Namespace)
	if namespace == "" {
		namespace = "c"
	}
	routes := []ShellRoute{}
	if idx, err := lwc.BuildIndex(p); err == nil {
		for _, name := range idx.Names() {
			bundle, ok := idx.Bundle(name)
			if !ok || bundle.MetaFile == "" {
				continue
			}
			meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
			if err != nil || !meta.IsExposed {
				continue
			}
			component := namespace + ":" + bundle.Name
			routes = append(routes, ShellRoute{
				Label:     component,
				URL:       "/lwc/preview/component/" + namespace + "/" + bundle.Name,
				Kind:      RenderTargetComponent,
				Component: component,
			})
		}
	}
	for _, path := range p.FlexiPageFiles {
		page, err := LoadFlexiPage(path)
		if err != nil {
			continue
		}
		switch strings.ToLower(page.Type) {
		case "recordpage":
			objectAPIName := page.ObjectAPIName
			if objectAPIName == "" {
				objectAPIName = "<objectApiName>"
			}
			recordID := "<recordId>"
			routes = append(routes, ShellRoute{
				Label:      pageLabel(page),
				URL:        fmt.Sprintf("/lwc/preview/record/%s/%s?page=%s", url.PathEscape(objectAPIName), recordID, url.QueryEscape(page.Name)),
				Kind:       RenderTargetRecordPage,
				PageName:   page.Name,
				ObjectName: objectAPIName,
				RecordID:   recordID,
			})
		case "apppage":
			routes = append(routes, ShellRoute{
				Label:    pageLabel(page),
				URL:      "/lwc/preview/app/" + url.PathEscape(page.Name),
				Kind:     RenderTargetAppPage,
				PageName: page.Name,
			})
		case "homepage":
			routes = append(routes, ShellRoute{
				Label:    pageLabel(page),
				URL:      "/lwc/preview/home/" + url.PathEscape(page.Name),
				Kind:     RenderTargetHomePage,
				PageName: page.Name,
			})
		}
	}
	for _, path := range p.TabFiles {
		tab, err := LoadCustomTab(path)
		if err != nil {
			continue
		}
		route := ShellRoute{
			Label:   tabLabel(tab),
			URL:     "/lwc/preview/tab/" + url.PathEscape(tab.Name),
			Kind:    RenderTargetTab,
			TabName: tab.Name,
		}
		if diag := tab.UnsupportedDiagnostic(); diag.Code != "" {
			route.Diagnostics = []Diagnostic{diag}
		}
		routes = append(routes, route)
	}
	routes = appendCommunityPresetRoutes(p, routes)
	return routes
}

func DiscoverWorkbenchComponents(p project.Project) []ShellComponent {
	namespace := strings.TrimSpace(p.Namespace)
	if namespace == "" {
		namespace = "c"
	}
	idx, err := lwc.BuildIndex(p)
	if err != nil {
		return nil
	}
	names := idx.Names()
	out := make([]ShellComponent, 0, len(names))
	for _, name := range names {
		bundle, ok := idx.Bundle(name)
		if !ok || strings.TrimSpace(bundle.MetaFile) == "" {
			continue
		}
		meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
		if err != nil {
			continue
		}
		label := strings.TrimSpace(meta.MasterLabel)
		if label == "" {
			label = bundle.Name
		}
		out = append(out, ShellComponent{
			Name:          bundle.Name,
			Namespace:     namespace,
			QualifiedName: namespace + ":" + bundle.Name,
			Label:         label,
			Exposed:       meta.IsExposed,
			Targets:       workbenchComponentTargets(meta),
			TargetSupport: workbenchComponentTargetSupport(meta),
		})
	}
	return out
}

func workbenchComponentTargets(meta lwc.ComponentMeta) []string {
	seen := map[string]bool{}
	var targets []string
	for _, target := range meta.Targets {
		target = strings.TrimSpace(target)
		if target == "" || seen[strings.ToLower(target)] {
			continue
		}
		seen[strings.ToLower(target)] = true
		targets = append(targets, target)
	}
	for _, cfg := range meta.TargetConfigs {
		for _, target := range cfg.Targets {
			target = strings.TrimSpace(target)
			if target == "" || seen[strings.ToLower(target)] {
				continue
			}
			seen[strings.ToLower(target)] = true
			targets = append(targets, target)
		}
	}
	sort.Strings(targets)
	return targets
}

func workbenchComponentTargetSupport(meta lwc.ComponentMeta) []ShellComponentTarget {
	byTarget := map[string]*ShellComponentTarget{}
	for _, target := range workbenchComponentTargets(meta) {
		copyTarget := target
		byTarget[strings.ToLower(target)] = &ShellComponentTarget{Target: copyTarget}
	}
	for _, cfg := range meta.TargetConfigs {
		properties := workbenchComponentDefaultProperties(cfg)
		for _, target := range cfg.Targets {
			target = strings.TrimSpace(target)
			if target == "" {
				continue
			}
			key := strings.ToLower(target)
			row := byTarget[key]
			if row == nil {
				row = &ShellComponentTarget{Target: target}
				byTarget[key] = row
			}
			if len(properties) > 0 {
				row.Properties = properties
			}
			row.SupportedObjects = append([]string(nil), cfg.SupportedObjects...)
			row.SupportedFormFactors = append([]string(nil), cfg.SupportedFormFactors...)
		}
	}
	keys := make([]string, 0, len(byTarget))
	for key := range byTarget {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ShellComponentTarget, 0, len(keys))
	for _, key := range keys {
		out = append(out, *byTarget[key])
	}
	return out
}

func workbenchComponentDefaultProperties(cfg lwc.TargetConfig) map[string]string {
	properties := map[string]string{}
	for _, prop := range cfg.Properties {
		name := strings.TrimSpace(prop.Name)
		if name == "" || strings.TrimSpace(prop.Default) == "" {
			continue
		}
		properties[name] = strings.TrimSpace(prop.Default)
	}
	if len(properties) == 0 {
		return nil
	}
	return properties
}

func appendCommunityPresetRoutes(p project.Project, routes []ShellRoute) []ShellRoute {
	file, err := LoadContextPresets(p.Root)
	if err != nil || len(file.Contexts) == 0 {
		return routes
	}
	names := make([]string, 0, len(file.Contexts))
	for name := range file.Contexts {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		ctx, err := file.Contexts[name].ToPageContext()
		if err != nil || ctx.Kind != RenderTargetCommunityPage {
			continue
		}
		routeURL := communityPresetRouteURL(ctx)
		if routeURL == "" {
			continue
		}
		routes = append(routes, ShellRoute{
			Label:     communityPresetRouteLabel(ctx),
			URL:       routeURL,
			Kind:      RenderTargetCommunityPage,
			PageName:  ctx.PageName,
			Component: ctx.ComponentName,
			State:     ctx.State,
		})
	}
	return routes
}

func communityPresetRouteURL(ctx PageContext) string {
	if strings.TrimSpace(ctx.Community.Site) == "" || strings.TrimSpace(ctx.PageName) == "" {
		return ""
	}
	values := url.Values{}
	for key, value := range ctx.State {
		if strings.TrimSpace(key) != "" && strings.TrimSpace(value) != "" {
			values.Set("state."+key, value)
		}
	}
	path := "/lwc/preview/community/" + url.PathEscape(ctx.Community.Site) + "/" + url.PathEscape(ctx.PageName)
	if encoded := values.Encode(); encoded != "" {
		return path + "?" + encoded
	}
	return path
}

func communityPresetRouteLabel(ctx PageContext) string {
	if strings.TrimSpace(ctx.Community.Site) == "" {
		return ctx.PageName
	}
	if strings.TrimSpace(ctx.PageName) == "" {
		return ctx.Community.Site
	}
	return ctx.Community.Site + " / " + ctx.PageName
}

func pageLabel(page FlexiPage) string {
	if strings.TrimSpace(page.Label) != "" {
		return page.Label
	}
	return page.Name
}

func tabLabel(tab CustomTab) string {
	if strings.TrimSpace(tab.Label) != "" {
		return tab.Label
	}
	return tab.Name
}
