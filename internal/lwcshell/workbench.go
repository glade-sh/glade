package lwcshell

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/project"
)

type WorkbenchModel struct {
	Title       string       `json:"title"`
	Mode        string       `json:"mode"`
	Apps        []ShellApp   `json:"apps"`
	Routes      []ShellRoute `json:"routes"`
	ActiveRoute string       `json:"activeRoute"`
	Active      ShellPage    `json:"active"`
	Diagnostics []Diagnostic `json:"diagnostics,omitempty"`
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
	return routes
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
