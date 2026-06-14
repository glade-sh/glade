package lwcshell

import (
	"errors"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/project"
)

func ResolvePageTarget(p project.Project, ctx PageContext) (ShellPage, []Diagnostic, error) {
	switch ctx.Kind {
	case RenderTargetRecordPage, RenderTargetAppPage, RenderTargetHomePage:
		page, ok, err := findFlexiPageForContext(p, ctx)
		if err != nil {
			return ShellPage{}, nil, err
		}
		if !ok {
			diag := Diagnostic{Code: "GLADELWC006", Message: fmt.Sprintf("page metadata %q not found", ctx.PageName)}
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		if diag, ok := validatePageKind(page, ctx.Kind); !ok {
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		if ctx.Kind == RenderTargetRecordPage {
			if ctx.ObjectAPIName == "" {
				ctx.ObjectAPIName = page.ObjectAPIName
			}
			if page.ObjectAPIName != "" && ctx.ObjectAPIName != "" && !strings.EqualFold(page.ObjectAPIName, ctx.ObjectAPIName) {
				diag := Diagnostic{Code: "GLADELWC004", Message: fmt.Sprintf("record page object %q does not match %q", page.ObjectAPIName, ctx.ObjectAPIName)}
				return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
			}
			ctx.ObjectAPIName = page.ObjectAPIName
		}
		regions := qualifyPageRegions(page.Regions, p.Namespace)
		return ShellPage{Context: ctx, Page: page, Regions: regions}, nil, nil
	case RenderTargetTab:
		tab, ok, err := findCustomTab(p, ctx.TabName)
		if err != nil {
			return ShellPage{}, nil, err
		}
		if !ok {
			diag := Diagnostic{Code: "GLADELWC006", Message: fmt.Sprintf("tab metadata %q not found", ctx.TabName)}
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		if diag := tab.UnsupportedDiagnostic(); diag.Code != "" {
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		return ShellPage{Context: ctx, Tab: tab}, nil, nil
	default:
		diag := Diagnostic{Code: "GLADELWC012", Message: "choose one render target"}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
}

func qualifyPageRegions(regions []PageRegion, namespace string) []PageRegion {
	if namespace == "" {
		namespace = "c"
	}
	out := make([]PageRegion, len(regions))
	for i, region := range regions {
		out[i] = region
		out[i].Components = make([]PageComponent, len(region.Components))
		for j, component := range region.Components {
			out[i].Components[j] = component
			name := strings.TrimSpace(component.ComponentName)
			if name != "" && !strings.Contains(name, ":") {
				out[i].Components[j].ComponentName = namespace + ":" + name
			}
		}
	}
	return out
}

func validatePageKind(page FlexiPage, kind RenderTargetKind) (Diagnostic, bool) {
	want := map[RenderTargetKind]string{
		RenderTargetRecordPage: "RecordPage",
		RenderTargetAppPage:    "AppPage",
		RenderTargetHomePage:   "HomePage",
	}[kind]
	if want == "" || strings.EqualFold(page.Type, want) {
		return Diagnostic{}, true
	}
	return Diagnostic{Code: "GLADELWC009", Message: fmt.Sprintf("page %q has type %q, want %q", page.Name, page.Type, want)}, false
}

func findFlexiPage(p project.Project, name string) (FlexiPage, bool, error) {
	name = strings.TrimSpace(name)
	for _, path := range p.FlexiPageFiles {
		page, err := LoadFlexiPage(path)
		if err != nil {
			return FlexiPage{}, false, err
		}
		if strings.EqualFold(page.Name, name) {
			return page, true, nil
		}
	}
	return FlexiPage{}, false, nil
}

func findFlexiPageForContext(p project.Project, ctx PageContext) (FlexiPage, bool, error) {
	if strings.TrimSpace(ctx.PageName) != "" {
		return findFlexiPage(p, ctx.PageName)
	}
	for _, path := range p.FlexiPageFiles {
		page, err := LoadFlexiPage(path)
		if err != nil {
			return FlexiPage{}, false, err
		}
		if diag, ok := validatePageKind(page, ctx.Kind); !ok || diag.Code != "" {
			continue
		}
		if ctx.Kind == RenderTargetRecordPage && ctx.ObjectAPIName != "" && page.ObjectAPIName != "" && !strings.EqualFold(page.ObjectAPIName, ctx.ObjectAPIName) {
			continue
		}
		return page, true, nil
	}
	return FlexiPage{}, false, nil
}

func findCustomTab(p project.Project, name string) (CustomTab, bool, error) {
	name = normalizeTabName(name)
	for _, path := range p.TabFiles {
		tab, err := LoadCustomTab(path)
		if err != nil {
			return CustomTab{}, false, err
		}
		if strings.EqualFold(normalizeTabName(tab.Name), name) || strings.EqualFold(normalizeTabName(tab.Label), name) {
			return tab, true, nil
		}
	}
	return CustomTab{}, false, nil
}

func normalizeTabName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "_")
	return value
}
