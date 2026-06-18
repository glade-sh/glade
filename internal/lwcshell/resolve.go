package lwcshell

import (
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/glade-sh/glade/internal/lwc"
	"github.com/glade-sh/glade/internal/project"
)

func ResolvePageTarget(p project.Project, ctx PageContext) (ShellPage, []Diagnostic, error) {
	if ctx.AppName != "" && ctx.Kind == RenderTargetTab && strings.TrimSpace(ctx.TabName) == "" {
		app, ok, err := findCustomApplication(p, ctx.AppName)
		if err != nil {
			return ShellPage{}, nil, err
		}
		if !ok {
			diag := Diagnostic{Code: "GLADELWC035", Message: fmt.Sprintf("application metadata %q not found", ctx.AppName)}
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		if app.DefaultLandingTab == "" {
			diag := Diagnostic{Code: "GLADELWC035", Message: fmt.Sprintf("application %q has no navigation tabs", app.Name)}
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		ctx.TabName = app.DefaultLandingTab
	}
	switch ctx.Kind {
	case RenderTargetComponent, RenderTargetURLAddressable:
		return resolveComponentTarget(p, ctx)
	case RenderTargetRecordPage, RenderTargetAppPage, RenderTargetHomePage, RenderTargetUtilityBar:
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
			if page.ObjectAPIName != "" {
				ctx.ObjectAPIName = page.ObjectAPIName
			}
		}
		regions := qualifyPageRegions(page.Regions, p.Namespace)
		if diag, ok := validateResolvedComponents(p, regions, ctx); !ok {
			return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
		}
		diagnostics := visibilityRuleApproximationDiagnostics(regions)
		return ShellPage{Context: ctx, Page: page, Regions: regions, Diagnostics: diagnostics}, diagnostics, nil
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
			shell := ShellPage{Context: ctx, Tab: tab, Diagnostics: []Diagnostic{diag}}
			return shell, []Diagnostic{diag}, nil
		}
		return ShellPage{Context: ctx, Tab: tab}, nil, nil
	case RenderTargetQuickAction:
		return resolveQuickActionTarget(p, ctx)
	case RenderTargetFlowScreen:
		return resolveFlowScreenTarget(p, ctx)
	default:
		diag := Diagnostic{Code: "GLADELWC012", Message: "choose one render target"}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
}

func resolveFlowScreenTarget(p project.Project, ctx PageContext) (ShellPage, []Diagnostic, error) {
	if strings.TrimSpace(ctx.ComponentName) == "" {
		diag := Diagnostic{Code: "GLADELWC012", Message: "component render target requires a component name"}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	ctx.Kind = RenderTargetFlowScreen
	component := PageComponent{ComponentName: qualifyComponentName(ctx.ComponentName, p.Namespace)}
	ctx.ComponentName = component.ComponentName
	return ShellPage{
		Context: ctx,
		Regions: []PageRegion{{
			Name:       "main",
			Components: []PageComponent{component},
		}},
	}, nil, nil
}

func resolveComponentTarget(p project.Project, ctx PageContext) (ShellPage, []Diagnostic, error) {
	componentName := strings.TrimSpace(ctx.ComponentName)
	if componentName == "" {
		diag := Diagnostic{Code: "GLADELWC012", Message: "component render target requires a component name"}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	component := PageComponent{ComponentName: qualifyComponentName(componentName, p.Namespace)}
	diag, ok := validateComponentMetadata(p, component, ctx, "lightning__UrlAddressable")
	if !ok {
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	ctx.ComponentName = component.ComponentName
	return ShellPage{
		Context: ctx,
		Regions: []PageRegion{{
			Name:       "main",
			Components: []PageComponent{component},
		}},
	}, nil, nil
}

func resolveQuickActionTarget(p project.Project, ctx PageContext) (ShellPage, []Diagnostic, error) {
	action, ok, err := findQuickAction(p, ctx)
	if err != nil {
		return ShellPage{}, nil, err
	}
	if !ok {
		diag := Diagnostic{Code: "GLADELWC070", Message: fmt.Sprintf("quick action %q not found", quickActionLookupName(ctx))}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	componentName := strings.TrimSpace(action.ComponentName)
	if componentName == "" {
		diag := Diagnostic{Code: "GLADELWC070", Message: fmt.Sprintf("quick action %q is not backed by an LWC component", action.Name)}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	if action.TargetObject != "" && ctx.ObjectAPIName != "" && !strings.EqualFold(action.TargetObject, ctx.ObjectAPIName) {
		diag := Diagnostic{Code: "GLADELWC070", Message: fmt.Sprintf("quick action %q targets %q, not %q", action.Name, action.TargetObject, ctx.ObjectAPIName)}
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	ctx.Kind = RenderTargetQuickAction
	ctx.ComponentName = qualifyComponentName(componentName, p.Namespace)
	ctx.ActionName = action.Name
	if ctx.ObjectAPIName == "" {
		ctx.ObjectAPIName = action.TargetObject
	}
	component := PageComponent{ComponentName: ctx.ComponentName}
	actionType, diag, ok := quickActionComponentActionType(p, component, action.ActionType)
	if !ok {
		return ShellPage{}, []Diagnostic{diag}, errors.New(diag.Message)
	}
	ctx.ActionType = actionType
	return ShellPage{
		Context: ctx,
		Regions: []PageRegion{{
			Name:       "main",
			Components: []PageComponent{component},
		}},
	}, nil, nil
}

func quickActionComponentActionType(p project.Project, component PageComponent, fallback string) (string, Diagnostic, bool) {
	_, name := splitComponentName(component.ComponentName, p.Namespace)
	bundle, ok, err := findLWCBundleByName(p, name)
	if err != nil {
		return "", Diagnostic{Code: "GLADELWC070", Message: err.Error()}, false
	}
	if !ok || bundle.MetaFile == "" {
		return "", Diagnostic{Code: "GLADELWC070", Message: fmt.Sprintf("LWC component %q not found for quick action", component.ComponentName)}, false
	}
	meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
	if err != nil {
		return "", Diagnostic{Code: "GLADELWC070", Message: fmt.Sprintf("read LWC metadata for quick action component %q: %v", component.ComponentName, err)}, false
	}
	if !meta.SupportsTarget("lightning__RecordAction") {
		return "", Diagnostic{Code: "GLADELWC070", Message: fmt.Sprintf("LWC component %q does not support lightning__RecordAction", component.ComponentName)}, false
	}
	actionType := strings.TrimSpace(meta.TargetConfigFor("lightning__RecordAction").ActionType)
	if actionType == "" {
		actionType = strings.TrimSpace(fallback)
	}
	if actionType == "" {
		actionType = "ScreenAction"
	}
	if !quickActionTypeSupported(actionType) {
		return "", Diagnostic{Code: "GLADELWC015", Message: fmt.Sprintf("quick action action type %q is not supported locally", actionType)}, false
	}
	return actionType, Diagnostic{}, true
}

func quickActionTypeSupported(actionType string) bool {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "screenaction", "action":
		return true
	default:
		return false
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
		RenderTargetUtilityBar: "UtilityBar",
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

func findCustomApplication(p project.Project, name string) (CustomApplication, bool, error) {
	name = normalizeTabName(name)
	for _, path := range p.ApplicationFiles {
		app, err := LoadCustomApplication(path)
		if err != nil {
			return CustomApplication{}, false, err
		}
		if strings.EqualFold(normalizeTabName(app.Name), name) || strings.EqualFold(normalizeTabName(app.Label), name) {
			return app, true, nil
		}
	}
	return CustomApplication{}, false, nil
}

type quickActionXML struct {
	Label                 string `xml:"label"`
	Type                  string `xml:"type"`
	TargetObject          string `xml:"targetObject"`
	LightningComponent    string `xml:"lightningComponent"`
	LightningWebComponent string `xml:"lightningWebComponent"`
	ActionType            string `xml:"actionType"`
	ActionSubtype         string `xml:"actionSubtype"`
}

func LoadQuickAction(path string) (QuickAction, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return QuickAction{}, err
	}
	var raw quickActionXML
	if len(strings.TrimSpace(string(data))) > 0 {
		if err := xml.Unmarshal(data, &raw); err != nil {
			return QuickAction{}, err
		}
	}
	name := metadataName(path, ".quickAction-meta.xml", ".quickaction-meta.xml", ".quickAction", ".quickaction")
	targetObject := strings.TrimSpace(raw.TargetObject)
	if targetObject == "" {
		if before, _, ok := strings.Cut(name, "."); ok {
			targetObject = before
		}
	}
	label := strings.TrimSpace(raw.Label)
	if label == "" {
		label = name
	}
	return QuickAction{
		Name:          name,
		Label:         label,
		Type:          strings.TrimSpace(raw.Type),
		TargetObject:  targetObject,
		ComponentName: firstNonBlank(raw.LightningComponent, raw.LightningWebComponent),
		ActionType:    firstNonBlank(raw.ActionType, raw.ActionSubtype),
		File:          path,
	}, nil
}

func firstNonBlank(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func findQuickAction(p project.Project, ctx PageContext) (QuickAction, bool, error) {
	want := normalizeQuickActionName(ctx.ActionName)
	if ctx.ObjectAPIName != "" && !strings.Contains(want, ".") {
		want = normalizeQuickActionName(ctx.ObjectAPIName + "." + ctx.ActionName)
	}
	for _, path := range p.QuickActionFiles {
		action, err := LoadQuickAction(path)
		if err != nil {
			return QuickAction{}, false, err
		}
		if !quickActionMatches(action, ctx, want) {
			continue
		}
		return action, true, nil
	}
	return QuickAction{}, false, nil
}

func quickActionMatches(action QuickAction, ctx PageContext, want string) bool {
	if !strings.EqualFold(normalizeQuickActionName(action.Name), want) {
		return false
	}
	if ctx.ObjectAPIName == "" {
		return action.TargetObject == "" || !strings.Contains(action.Name, ".")
	}
	return strings.EqualFold(action.TargetObject, ctx.ObjectAPIName)
}

func normalizeQuickActionName(value string) string {
	return strings.TrimSpace(value)
}

func quickActionLookupName(ctx PageContext) string {
	if ctx.ObjectAPIName != "" && ctx.ActionName != "" && !strings.Contains(ctx.ActionName, ".") {
		return ctx.ObjectAPIName + "." + ctx.ActionName
	}
	return ctx.ActionName
}

func normalizeTabName(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, " ", "_")
	return value
}

func validateResolvedComponents(p project.Project, regions []PageRegion, ctx PageContext) (Diagnostic, bool) {
	target := shellTargetName(ctx.Kind)
	for _, region := range regions {
		for _, component := range region.Components {
			if diag, ok := validateComponentMetadata(p, component, ctx, target); !ok {
				return diag, false
			}
		}
	}
	return Diagnostic{}, true
}

func visibilityRuleApproximationDiagnostics(regions []PageRegion) []Diagnostic {
	var diagnostics []Diagnostic
	for _, region := range regions {
		for _, component := range region.Components {
			rule, ok := ComponentVisibilityRule(component)
			if !ok {
				continue
			}
			message := fmt.Sprintf("visibility rule on %s is approximated locally", component.ComponentName)
			if rule.BooleanFilter != "" {
				message += fmt.Sprintf("; boolean filter %q is not evaluated", rule.BooleanFilter)
			}
			diagnostics = append(diagnostics, Diagnostic{Code: "GLADELWC034", Message: message})
		}
	}
	return diagnostics
}

func validateComponentMetadata(p project.Project, component PageComponent, ctx PageContext, target string) (Diagnostic, bool) {
	_, name := splitComponentName(component.ComponentName, p.Namespace)
	bundle, ok, err := findLWCBundleByName(p, name)
	if err != nil {
		return Diagnostic{Code: "GLADELWC030", Message: err.Error()}, false
	}
	if !ok || bundle.MetaFile == "" {
		return Diagnostic{}, true
	}
	meta, err := lwc.ParseComponentMeta(bundle.MetaFile)
	if err != nil {
		return Diagnostic{Code: "GLADELWC030", Message: fmt.Sprintf("read LWC metadata for %q: %v", component.ComponentName, err)}, false
	}
	if target != "" && !meta.SupportsTarget(target) {
		return Diagnostic{Code: "GLADELWC031", Message: fmt.Sprintf("LWC component %q does not support %s", component.ComponentName, target)}, false
	}
	config := meta.TargetConfigFor(target)
	if ctx.FormFactor != "" && len(config.SupportedFormFactors) > 0 && !containsEqualFold(config.SupportedFormFactors, ctx.FormFactor) {
		return Diagnostic{Code: "GLADELWC032", Message: fmt.Sprintf("LWC component %q does not support form factor %q", component.ComponentName, ctx.FormFactor)}, false
	}
	for _, prop := range config.Properties {
		if !prop.Required || prop.Name == "" || prop.Default != "" {
			continue
		}
		if component.Properties == nil || strings.TrimSpace(component.Properties[prop.Name]) == "" {
			return Diagnostic{Code: "GLADELWC033", Message: fmt.Sprintf("LWC component %q requires property %q", component.ComponentName, prop.Name)}, false
		}
	}
	return Diagnostic{}, true
}

func findLWCBundleByName(p project.Project, name string) (lwc.Bundle, bool, error) {
	idx, err := lwc.BuildIndex(p)
	if err != nil {
		return lwc.Bundle{}, false, err
	}
	bundle, ok := idx.Bundle(name)
	return bundle, ok, nil
}

func shellTargetName(kind RenderTargetKind) string {
	switch kind {
	case RenderTargetRecordPage:
		return "lightning__RecordPage"
	case RenderTargetAppPage:
		return "lightning__AppPage"
	case RenderTargetHomePage:
		return "lightning__HomePage"
	default:
		return ""
	}
}

func qualifyComponentName(name, namespace string) string {
	name = strings.TrimSpace(name)
	if namespace == "" {
		namespace = "c"
	}
	if strings.Contains(name, ":") {
		return name
	}
	return namespace + ":" + name
}

func splitComponentName(qualified, defaultNamespace string) (namespace, name string) {
	if defaultNamespace == "" {
		defaultNamespace = "c"
	}
	qualified = strings.TrimSpace(qualified)
	if before, after, ok := strings.Cut(qualified, ":"); ok {
		return strings.TrimSpace(before), strings.TrimSpace(after)
	}
	return defaultNamespace, qualified
}

func containsEqualFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(want)) {
			return true
		}
	}
	return false
}

func trimStringList(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
