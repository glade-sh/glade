package visualforce

import (
	"strings"
	"testing"
)

func TestStandardComponentSpecsCoverReferenceCatalog(t *testing.T) {
	missing := []string{}
	for _, entry := range StandardComponentCatalog() {
		name := strings.TrimSpace(entry.Name)
		if name == "" {
			continue
		}
		if strings.Contains(entry.SourceFile, "additional_") {
			continue
		}
		if _, ok := StandardComponentSpec(componentNamespace(name), strings.TrimPrefix(name, componentNamespace(name)+":")); !ok {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("missing Visualforce component specs: %v", missing)
	}
}

func TestStandardComponentSpecsOnlyExposeDocsBackedComponents(t *testing.T) {
	for key, spec := range StandardComponentSpecs() {
		if !strings.Contains(spec.Name, ":") {
			t.Fatalf("component spec %q exposes non-component docs page %q", key, spec.Name)
		}
	}
}

func TestStandardComponentSpecsHaveExplicitStatus(t *testing.T) {
	for name, spec := range StandardComponentSpecs() {
		switch spec.Status {
		case ComponentSupported, ComponentPartial, ComponentUnsupported:
		default:
			t.Fatalf("component %q status = %q, want supported, partial, or unsupported", name, spec.Status)
		}
	}
}

func TestPartialComponentSpecsDescribeMissingBehavior(t *testing.T) {
	missing := []string{}
	for name, spec := range StandardComponentSpecs() {
		if spec.Status != ComponentPartial {
			continue
		}
		reason := strings.TrimSpace(spec.Reason)
		if reason == "" || reason == "current local renderer covers a subset of documented behavior" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("partial components missing stable gap reasons: %v", missing)
	}

	cases := []struct {
		namespace string
		component string
		want      string
	}{
		{namespace: "apex", component: "page", want: "standard controller"},
		{namespace: "apex", component: "commandButton", want: "AJAX"},
		{namespace: "apex", component: "component", want: "typed attribute"},
		{namespace: "apex", component: "relatedList", want: "related list data"},
		{namespace: "apex", component: "remoteObjects", want: "client model scaffold"},
		// V15 dom_widget_chart_{pie,bar,line,legend} promotes these bounded
		// profiles without claiming other chart configurations or interaction.
		{namespace: "apex", component: "chart", want: "other chart configurations"},
	}
	for _, tc := range cases {
		spec, ok := StandardComponentSpec(tc.namespace, tc.component)
		if !ok {
			t.Fatalf("missing component spec for %s:%s", tc.namespace, tc.component)
		}
		if !strings.Contains(spec.Reason, tc.want) {
			t.Fatalf("%s:%s reason = %q, want it to mention %q", tc.namespace, tc.component, spec.Reason, tc.want)
		}
	}
}

func TestPresentationChartUnsupportedNeighbours(t *testing.T) {
	pie := `<apex:chart id="target" data="{!points}" height="240" width="320"><apex:pieSeries dataField="amount" labelField="name"/></apex:chart>`
	bar := `<apex:chart id="target" data="{!points}" height="240" width="320"><apex:axis type="Category" position="bottom" fields="name"/><apex:axis type="Numeric" position="left" fields="amount"/><apex:barSeries orientation="vertical" axis="left" xField="name" yField="amount"/></apex:chart>`
	// These configurations were unsupported before V15 and have no captured
	// presentation contract. The bounded renderer must retain that boundary.
	cases := map[string]string{
		"horizontal bar":     strings.Replace(bar, `orientation="vertical"`, `orientation="horizontal"`, 1),
		"right numeric axis": strings.Replace(bar, `position="left"`, `position="right"`, 1),
		"other fields":       strings.Replace(bar, `fields="amount"`, `fields="other"`, 1),
		"custom palette":     strings.Replace(pie, `dataField="amount"`, `dataField="amount" colorSet="red,blue"`, 1),
		"other legend":       strings.Replace(pie, `<apex:pieSeries`, `<apex:legend position="left"/><apex:pieSeries`, 1),
		"multiple series":    strings.Replace(pie, `</apex:chart>`, `<apex:pieSeries dataField="amount" labelField="name"/></apex:chart>`, 1),
		"non-finite width":   strings.Replace(pie, `width="320"`, `width="NaN"`, 1),
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			root, err := ParseMarkupTree(source)
			if err != nil {
				t.Fatal(err)
			}
			var chart *MarkupNode
			var find func(*MarkupNode)
			find = func(n *MarkupNode) {
				if n.Namespace == "apex" && strings.EqualFold(n.Name, "chart") {
					chart = n
				}
				for _, child := range n.Children {
					find(child)
				}
			}
			find(root)
			if chart == nil {
				t.Fatal("missing chart")
			}
			if _, _, _, ok := presentationChartProfile(chart); ok {
				t.Fatal("uncaptured configuration must retain unsupported boundary")
			}
		})
	}
}

func TestUnsupportedComponentSpecsUseStableFamilyReasons(t *testing.T) {
	missing := []string{}
	for name, spec := range StandardComponentSpecs() {
		if spec.Status != ComponentUnsupported {
			continue
		}
		reason := strings.TrimSpace(spec.Reason)
		if reason == "" ||
			reason == "renderer not implemented in local Visualforce renderer" ||
			reason == "requires explicit local Visualforce renderer classification before support can be claimed" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("unsupported components missing stable family reasons: %v", missing)
	}

	cases := []struct {
		namespace string
		component string
		want      string
	}{
		// V15's captured root chart profiles now have a local renderer. A
		// series still needs that supported parent; standalone series retain
		// the charting-runtime boundary.
		{namespace: "apex", component: "pieSeries", want: "charting runtime"},
		{namespace: "apex", component: "map", want: "map widget runtime"},
		{namespace: "apex", component: "canvasApp", want: "Canvas signed request"},
		{namespace: "knowledge", component: "articleList", want: "Knowledge service"},
		{namespace: "liveAgent", component: "clientChat", want: "Live Agent chat service"},
		{namespace: "messaging", component: "emailTemplate", want: "email template render pipeline"},
		{namespace: "support", component: "caseArticles", want: "Service Cloud support runtime"},
		{namespace: "wave", component: "dashboard", want: "CRM Analytics runtime"},
	}
	for _, tc := range cases {
		spec, ok := StandardComponentSpec(tc.namespace, tc.component)
		if !ok {
			t.Fatalf("missing component spec for %s:%s", tc.namespace, tc.component)
		}
		if spec.Status != ComponentUnsupported {
			t.Fatalf("%s:%s status = %s, want unsupported", tc.namespace, tc.component, spec.Status)
		}
		if !strings.Contains(spec.Reason, tc.want) {
			t.Fatalf("%s:%s reason = %q, want it to mention %q", tc.namespace, tc.component, spec.Reason, tc.want)
		}
	}
}

func TestStandardComponentSpecsCannotPoisonCachedRegistry(t *testing.T) {
	specs := StandardComponentSpecs()
	delete(specs, "apex:page")
	if _, ok := StandardComponentSpec("apex", "page"); !ok {
		t.Fatal("mutating returned specs map poisoned cached registry")
	}
}
