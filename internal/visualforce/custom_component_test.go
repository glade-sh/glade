package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderCustomComponentAssignToFacetAndBody(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/Card.component"), `<apex:component controller="CardController">
  <apex:attribute name="title" type="String" assignTo="{!heading}" required="true" description="Card heading"/>
  <apex:facet name="actions"/>
  <section class="card">
    <h2>{!heading}</h2>
    <div class="actions"><apex:insert name="actions"/></div>
    <apex:componentBody/>
  </section>
</apex:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/CardHost.page"), `<apex:page controller="CardHostController">
  <c:Card title="{!title}">
    <apex:facet name="actions">
      <apex:commandLink value="Edit" action="{!edit}"/>
    </apex:facet>
    <apex:outputText value="{!body}"/>
  </c:Card>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	if err := machine.RegisterClass(vm.Class{Name: "CardController"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(vm.Class{
		Name: "CardHostController",
		Fields: map[string]vm.Field{
			"title": {Name: "title", Type: "String", InitialValue: vm.String("Deal Summary")},
			"body":  {Name: "body", Type: "String", InitialValue: vm.String("Quarterly renewal")},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "CardHost",
		PageURL:  "/apex/CardHost",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`<h2>Deal Summary</h2>`, `Edit`, `Quarterly renewal`} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestRenderControllerlessCustomComponentUsesAttributesAsVariables(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/components/Badge.component"), `<apex:component>
  <apex:attribute name="title" type="String" description="Badge title"/>
  <span class="badge">{!title}</span>
</apex:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/BadgeHost.page"), `<apex:page>
  <c:Badge title="Ready"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  vm.New(nil),
		PageName: "BadgeHost",
		PageURL:  "/apex/BadgeHost",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, `<span class="badge">Ready</span>`) {
		t.Fatalf("html = %s", result.HTML)
	}
}

func TestRenderIncludeUsesIncludedPageControllerInsideTemplate(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Template.page"), `<apex:page><html><body><header><apex:insert name="title"/></header><main><apex:insert name="body"/></main></body></html></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Included.page"), `<apex:page controller="IncludedController"><h1>Lifecycle Basic</h1><apex:outputText value="{!message}"/></apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Host.page"), `<apex:page>
  <apex:composition template="Template">
    <apex:define name="title">Nested Template</apex:define>
    <apex:define name="body"><apex:include pageName="Included"/></apex:define>
  </apex:composition>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	if err := machine.RegisterClass(vm.Class{
		Name: "IncludedController",
		Fields: map[string]vm.Field{
			"message": {Name: "message", Type: "String", InitialValue: vm.String("probe ready")},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "Host",
		PageURL:  "/apex/Host",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Nested Template", "Lifecycle Basic", "probe ready"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}
