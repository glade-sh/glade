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
		{namespace: "apex", component: "chart", want: "charting runtime"},
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
