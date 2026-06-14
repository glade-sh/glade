package visualforce

import (
	"strings"
	"testing"
)

func TestStandardComponentSupportRowsExposeProductTableFields(t *testing.T) {
	rows := StandardComponentSupport()
	specs := StandardComponentSpecs()
	if len(rows) != len(specs) {
		t.Fatalf("support row count = %d, want %d component specs", len(rows), len(specs))
	}

	seen := map[string]bool{}
	for _, row := range rows {
		seen[row.Name] = true
		if strings.TrimSpace(row.Name) == "" {
			t.Fatalf("support row has empty name: %#v", row)
		}
		switch row.Status {
		case ComponentSupported, ComponentPartial, ComponentUnsupported:
		default:
			t.Fatalf("%s status = %q, want a known support status", row.Name, row.Status)
		}
		if strings.TrimSpace(row.Reason) == "" {
			t.Fatalf("%s has empty support reason", row.Name)
		}
		if strings.TrimSpace(row.DocSource) == "" {
			t.Fatalf("%s has empty doc source", row.Name)
		}
		if row.Attributes == nil {
			t.Fatalf("%s has nil attributes in support row", row.Name)
		}
		if strings.TrimSpace(row.LocalEvidence) == "" {
			t.Fatalf("%s has empty local evidence", row.Name)
		}
		if row.Status == ComponentUnsupported && !validHostedBoundary(row.HostedBoundary) {
			t.Fatalf("%s hosted boundary = %q, want stable unsupported boundary", row.Name, row.HostedBoundary)
		}
	}
	for name := range specs {
		if !seen[name] {
			t.Fatalf("support rows missing %s", name)
		}
	}
}

func TestPromotedUIOnlyComponentsHaveRenderers(t *testing.T) {
	for _, component := range []string{
		"panelGrid",
		"panelGroup",
		"sectionHeader",
		"toolbar",
		"toolbarGroup",
		"tabPanel",
		"tab",
		"panelBar",
		"panelBarItem",
	} {
		spec, ok := StandardComponentSpec("apex", component)
		if !ok {
			t.Fatalf("missing apex:%s spec", component)
		}
		if spec.Render == nil {
			t.Fatalf("apex:%s has nil renderer after promotion", component)
		}
		row := findSupportRow(t, "apex:"+strings.ToLower(component))
		if !strings.Contains(row.LocalEvidence, "renderer") {
			t.Fatalf("apex:%s local evidence = %q, want renderer evidence", component, row.LocalEvidence)
		}
	}
}

func TestInteractiveUIOnlyComponentsStayPartialUntilLifecycleIsModeled(t *testing.T) {
	for _, component := range []string{"tabPanel", "tab", "panelBar", "panelBarItem"} {
		spec, ok := StandardComponentSpec("apex", component)
		if !ok {
			t.Fatalf("missing apex:%s spec", component)
		}
		if spec.Status != ComponentPartial {
			t.Fatalf("apex:%s status = %s, want %s", component, spec.Status, ComponentPartial)
		}
		row := findSupportRow(t, "apex:"+strings.ToLower(component))
		if !strings.Contains(strings.ToLower(row.Reason), "lifecycle") {
			t.Fatalf("apex:%s reason = %q, want lifecycle boundary", component, row.Reason)
		}
	}
}

func TestUnsupportedHostedFamiliesHaveDiagnostics(t *testing.T) {
	cases := []struct {
		name     string
		boundary string
		reason   string
	}{
		{name: "analytics:reportchart", boundary: "hosted-service", reason: "Analytics"},
		{name: "chatter:feed", boundary: "hosted-service", reason: "Chatter"},
		{name: "chatteranswers:login", boundary: "hosted-service", reason: "Chatter Answers"},
		{name: "ideas:detailoutputlink", boundary: "hosted-service", reason: "Ideas"},
		{name: "knowledge:articlelist", boundary: "hosted-service", reason: "Knowledge"},
		{name: "liveagent:clientchat", boundary: "hosted-service", reason: "Live Agent"},
		{name: "site:previewasadmin", boundary: "hosted-service", reason: "Sites"},
		{name: "social:profileviewer", boundary: "hosted-service", reason: "social profile"},
		{name: "support:casearticles", boundary: "hosted-service", reason: "Service Cloud"},
		{name: "topics:widget", boundary: "hosted-service", reason: "Topics"},
		{name: "wave:dashboard", boundary: "hosted-service", reason: "CRM Analytics"},
		{name: "apex:canvasapp", boundary: "hosted-service", reason: "Canvas"},
		{name: "apex:vote", boundary: "hosted-service", reason: "vote service"},
		{name: "apex:milestonetracker", boundary: "hosted-service", reason: "Entitlement milestone"},
		{name: "apex:emailpublisher", boundary: "hosted-service", reason: "publisher action"},
		{name: "apex:logcallpublisher", boundary: "hosted-service", reason: "publisher action"},
		{name: "apex:flash", boundary: "obsolete-runtime", reason: "Flash"},
		{name: "apex:scontrol", boundary: "obsolete-runtime", reason: "s-control"},
	}
	for _, tc := range cases {
		row := findSupportRow(t, tc.name)
		if row.Status != ComponentUnsupported {
			t.Fatalf("%s status = %s, want unsupported", tc.name, row.Status)
		}
		if row.HostedBoundary != tc.boundary {
			t.Fatalf("%s hosted boundary = %q, want %q", tc.name, row.HostedBoundary, tc.boundary)
		}
		if !strings.Contains(row.Reason, tc.reason) {
			t.Fatalf("%s reason = %q, want it to mention %q", tc.name, row.Reason, tc.reason)
		}
	}
}

func TestRenderPromotedUIOnlyComponents(t *testing.T) {
	cases := []struct {
		name   string
		markup string
		wants  []string
	}{
		{
			name:   "panelGrid",
			markup: `<apex:page><apex:panelGrid columns="2" id="grid"><apex:outputText value="A"/><apex:outputText value="B"/></apex:panelGrid></apex:page>`,
			wants:  []string{`<table id="grid">`, `<tbody><tr><td>A</td><td>B</td></tr></tbody>`},
		},
		{
			name:   "panelGroup",
			markup: `<apex:page><apex:panelGroup id="group" layout="block" styleClass="box"><apex:outputText value="Grouped"/></apex:panelGroup></apex:page>`,
			wants:  []string{`<div id="group" data-rerender="group" class="box">Grouped</div>`},
		},
		{
			name:   "sectionHeader",
			markup: `<apex:page><apex:sectionHeader title="Accounts" subtitle="Recent" description="Local list"/></apex:page>`,
			wants:  []string{`<div class="sectionHeader">`, `<h1>Accounts</h1>`, `<h2>Recent</h2>`, `<p>Local list</p>`},
		},
		{
			name:   "toolbar",
			markup: `<apex:page><apex:toolbar id="tools"><apex:commandButton value="Save"/><apex:commandButton value="Cancel"/></apex:toolbar></apex:page>`,
			wants:  []string{`<div id="tools" data-rerender="tools" class="toolbar">`, `value="Save"`, `value="Cancel"`},
		},
		{
			name:   "toolbarGroup",
			markup: `<apex:page><apex:toolbarGroup id="left" location="left"><apex:outputText value="Left"/></apex:toolbarGroup></apex:page>`,
			wants:  []string{`<span id="left" data-rerender="left" class="toolbarGroup" data-location="left">Left</span>`},
		},
		{
			name:   "tabPanel",
			markup: `<apex:page><apex:tabPanel selectedTab="details"><apex:tab label="Details" name="details">Body</apex:tab><apex:tab label="Other" name="other">Other body</apex:tab></apex:tabPanel></apex:page>`,
			wants:  []string{`<div class="tabPanel">`, `<button type="button" class="tab active" data-tab="details">Details</button>`, `<div class="tabContent active" data-tab="details">Body</div>`, `<div class="tabContent" data-tab="other">Other body</div>`},
		},
		{
			name:   "tab",
			markup: `<apex:page><apex:tab label="Loose">Loose body</apex:tab></apex:page>`,
			wants:  []string{`<div class="tabContent" data-tab="Loose">Loose body</div>`},
		},
		{
			name:   "panelBar",
			markup: `<apex:page><apex:panelBar id="bar"><apex:panelBarItem label="One" expanded="true">First</apex:panelBarItem><apex:panelBarItem label="Two">Second</apex:panelBarItem></apex:panelBar></apex:page>`,
			wants:  []string{`<div id="bar" data-rerender="bar" class="panelBar">`, `<section class="panelBarItem active">`, `<h3>One</h3>`, `<div class="panelBarContent">First</div>`},
		},
		{
			name:   "panelBarItem",
			markup: `<apex:page><apex:panelBarItem label="Solo">Alone</apex:panelBarItem></apex:page>`,
			wants:  []string{`<section class="panelBarItem">`, `<h3>Solo</h3>`, `<div class="panelBarContent">Alone</div>`},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rendered := renderSupportMarkup(t, tc.markup)
			for _, want := range tc.wants {
				if !strings.Contains(rendered, want) {
					t.Fatalf("%s rendered = %s, want %s", tc.name, rendered, want)
				}
			}
		})
	}
}

func findSupportRow(t *testing.T, name string) ComponentSupportRow {
	t.Helper()
	for _, row := range StandardComponentSupport() {
		if row.Name == name {
			return row
		}
	}
	t.Fatalf("missing support row %s", name)
	return ComponentSupportRow{}
}

func validHostedBoundary(boundary string) bool {
	switch boundary {
	case "hosted-service", "obsolete-runtime", "missing-local-subsystem", "not-a-standalone-component":
		return true
	default:
		return false
	}
}

func renderSupportMarkup(t *testing.T, markup string) string {
	t.Helper()
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatalf("RenderMarkupTree err = %v", err)
	}
	return rendered
}
