package visualforce

import (
	"archive/zip"
	"html"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderMarkupTreeRendersOutputTextAndOutputField(t *testing.T) {
	markup := `<apex:page><apex:outputText value="{!greeting}" /><apex:outputText value="{!$Label.EditTitle}" /><apex:outputField value="{!account.Name}" /></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Metadata.Labels = append(org.Metadata.Labels, storage.LabelMetadata{Name: "EditTitle", Value: "Hello"})
	runner := vm.New(nil)
	runner.Org = &org
	controller := vm.Object("Controller")
	controller.Fields["greeting"] = vm.String("Greetings")
	account := vm.Object("Account")
	account.Fields["Name"] = vm.String("Acme")
	controller.Fields["account"] = account
	ctx := RenderContext{
		VM:         runner,
		PageName:   "Test",
		Expression: &ExpressionContext{VM: runner, Controller: controller},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "Greetings") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "Hello") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "Acme") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderMarkupTreeRendersPagePrimitives(t *testing.T) {
	markup := `<apex:page title="Demo"><apex:form><apex:outputPanel><apex:outputLink value="/next">Next</apex:outputLink></apex:outputPanel></apex:form><apex:pageBlock><apex:pageBlockSection><apex:pageBlockSectionItem><apex:outputText value="{!status}" /></apex:pageBlockSectionItem></apex:pageBlockSection></apex:pageBlock></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	runner := vm.New(nil)
	runner.Org = &org
	controller := vm.Object("Controller")
	controller.Fields["status"] = vm.String("ok")
	ctx := RenderContext{
		VM:         runner,
		PageName:   "Demo",
		Expression: &ExpressionContext{VM: runner, Controller: controller},
	}
	rendered, err := RenderMarkupTree(tree, &ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, "<form") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "<a href=\"/next\">Next</a>") {
		t.Fatalf("rendered = %q", rendered)
	}
	if !strings.Contains(rendered, "bPageBlock") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderCommandControlsPostActionExpression(t *testing.T) {
	markup := `<apex:page><apex:form><apex:commandButton value="Save" action="{!save}"/><apex:commandLink value="Cancel" action="{!cancel}"/></apex:form></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{PageName: "Edit", Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatal(err)
	}
	rendered = html.UnescapeString(rendered)
	for _, want := range []string{
		`type="hidden" name="__vf_action" value=""`,
		`value="Save"`,
		`elements['__vf_action'].value='{!save}'`,
		`elements['__vf_action'].value='{!cancel}'`,
	} {
		if !strings.Contains(rendered, want) {
			t.Fatalf("rendered missing %q: %s", want, rendered)
		}
	}
	if strings.Contains(rendered, `name="__vf_action" value="Save"`) {
		t.Fatalf("command button posts label as action: %s", rendered)
	}
}

func TestRenderMarkupTreeUsesComponentRegistry(t *testing.T) {
	markup := `<apex:page><apex:facet name="body"><span>facet body</span></apex:facet></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(rendered), "<facet") {
		t.Fatalf("rendered leaked unsupported facet element = %q", rendered)
	}
	if !strings.Contains(rendered, "facet body") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderMarkupTreeUsesRegistryForDocumentedNonApexComponents(t *testing.T) {
	markup := `<flow:interview name="demo"/>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	_, err = RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{}})
	if err == nil || !strings.Contains(err.Error(), "flow:interview") {
		t.Fatalf("err = %v, want flow:interview unsupported diagnostic", err)
	}
}

func TestRenderMarkupTreeHonorsOutputTextEscapeFalse(t *testing.T) {
	markup := `<apex:page><apex:outputText escape="false" value="{!payload}"/></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	controller := vm.Object("Controller")
	controller.Fields["payload"] = vm.String(`<strong>raw</strong>`)
	rendered, err := RenderMarkupTree(tree, &RenderContext{
		Expression: &ExpressionContext{Controller: controller},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered, `<strong>raw</strong>`) {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestRenderIncludeLightningWithoutBootstrapDoesNotEmitNotice(t *testing.T) {
	markup := `<apex:page><apex:includeLightning/><div id="probeLightning">Lightning Out include</div></apex:page>`
	tree, err := ParseMarkupTree(markup)
	if err != nil {
		t.Fatal(err)
	}
	rendered, err := RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rendered, "Lightning Out is not available") {
		t.Fatalf("includeLightning emitted local fallback notice: %s", rendered)
	}
	if !strings.Contains(rendered, "Lightning Out include") {
		t.Fatalf("rendered = %q", rendered)
	}
}

func TestResolveStaticResourceFileReadsNestedZipResourceEntry(t *testing.T) {
	root := t.TempDir()
	resourcePath := filepath.Join(root, "force-app/main/default/staticresources", "Bundle.resource")
	writeStaticResourceZip(t, resourcePath, map[string]string{
		"css/site.css": "body { color: steelblue; }",
		"img/logo.svg": "<svg></svg>",
	})

	resolved, err := ResolveStaticResourceFile(root, "Bundle", "css/site.css")
	if err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(resolved)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "body { color: steelblue; }" {
		t.Fatalf("content = %q", content)
	}
}

func TestResolveStaticResourceFileKeepsDirectoryBundleBehavior(t *testing.T) {
	root := t.TempDir()
	resourcePath := filepath.Join(root, "force-app/main/default/staticresources", "Bundle", "css", "site.css")
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resourcePath, []byte("body { color: green; }"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveStaticResourceFile(root, "Bundle", "css/site.css")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != resourcePath {
		t.Fatalf("resolved = %q, want %q", resolved, resourcePath)
	}
}

func TestResolveStaticResourceFileKeepsFlatResourceBehavior(t *testing.T) {
	root := t.TempDir()
	resourcePath := filepath.Join(root, "force-app/main/default/staticresources", "Logo.resource")
	if err := os.MkdirAll(filepath.Dir(resourcePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(resourcePath, []byte("plain bytes"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := ResolveStaticResourceFile(root, "Logo", "")
	if err != nil {
		t.Fatal(err)
	}
	if resolved != resourcePath {
		t.Fatalf("resolved = %q, want %q", resolved, resourcePath)
	}
}

func TestResolveStaticResourceFileReportsMissingZipResourceEntry(t *testing.T) {
	root := t.TempDir()
	resourcePath := filepath.Join(root, "force-app/main/default/staticresources", "Bundle.resource")
	writeStaticResourceZip(t, resourcePath, map[string]string{
		"css/site.css": "body {}",
	})

	_, err := ResolveStaticResourceFile(root, "Bundle", "css/missing.css")
	if err == nil || !strings.Contains(err.Error(), "css/missing.css") {
		t.Fatalf("err = %v, want missing zip entry name", err)
	}
}

func TestResolveStaticResourceFileRejectsUnsafeZipResourcePath(t *testing.T) {
	root := t.TempDir()
	resourcePath := filepath.Join(root, "force-app/main/default/staticresources", "Bundle.resource")
	writeStaticResourceZip(t, resourcePath, map[string]string{
		"css/site.css": "body {}",
	})

	_, err := ResolveStaticResourceFile(root, "Bundle", "../outside.css")
	if err == nil || !strings.Contains(err.Error(), "invalid static resource path") {
		t.Fatalf("err = %v, want invalid static resource path", err)
	}
}

func TestResolveStaticResourceFileIgnoresUnsafeZipResourceEntry(t *testing.T) {
	root := t.TempDir()
	resourcePath := filepath.Join(root, "force-app/main/default/staticresources", "Bundle.resource")
	writeStaticResourceZip(t, resourcePath, map[string]string{
		"css/../site.css": "body {}",
	})

	_, err := ResolveStaticResourceFile(root, "Bundle", "site.css")
	if err == nil || !strings.Contains(err.Error(), "site.css") {
		t.Fatalf("err = %v, want missing zip entry name", err)
	}
}

func writeStaticResourceZip(t *testing.T, path string, entries map[string]string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	writer := zip.NewWriter(file)
	for name, content := range entries {
		entry, err := writer.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
}
