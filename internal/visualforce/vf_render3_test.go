package visualforce

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/vm"
)

// vfRender3Page loads a one-page project and returns the page index and machine.
func vfRender3Project(t *testing.T, markup string) (project.Project, Index) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/P.page"), markup)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, idx
}

// Rows Unb* of testdata/vf_render_markup: an input control names itself by its
// client ID, bound or not, with or without an id attribute. A browser post of
// the rendered page must still reach the same bindings.
func TestVFRender3UnboundInputNamesAreClientIDs(t *testing.T) {
	p, idx := vfRender3Project(t, `<apex:page controller="PostController">
  <apex:form id="f1">
    <apex:inputText id="i1"/>
    <apex:inputText value="{!mode}"/>
    <apex:inputTextarea id="a1"/>
    <apex:inputSecret id="s1"/>
    <apex:inputCheckbox id="c1"/>
    <apex:selectList id="l1" size="1"><apex:selectOption itemValue="a" itemLabel="A"/></apex:selectList>
    <apex:commandButton value="Save" action="{!save}"/>
    <apex:pageMessages/>
  </apex:form>
</apex:page>`)
	saveProgram, err := vm.CompileAnonymous(`
String seen = '';
for (String key : ApexPages.currentPage().getParameters().keySet()) {
  if (!key.startsWith('com.') && !key.startsWith('__')) { seen += key + '=' + ApexPages.currentPage().getParameters().get(key) + ';'; }
}
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Seen ' + ApexPages.currentPage().getParameters().get('mode') + ' ' + ApexPages.currentPage().getParameters().get('i1')));
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "PostController",
		Methods: map[string]vm.Method{
			"save": {Name: "PostController.save", ClassName: "PostController", ReturnType: "PageReference", Program: saveProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}
	result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "P", PageURL: "/apex/P"})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		`id="j_id0:f1:i1"`, `name="j_id0:f1:i1"`,
		`name="j_id0:f1:j_id1"`,
		`id="j_id0:f1:a1"`, `name="j_id0:f1:a1"`,
		`name="j_id0:f1:s1"`, `name="j_id0:f1:c1"`,
		`name="j_id0:f1:l1"`,
	} {
		if !strings.Contains(result.HTML, want) {
			t.Errorf("missing %s in %s", want, result.HTML)
		}
	}
	if strings.Contains(result.HTML, `name="i1"`) || strings.Contains(result.HTML, `name="a1"`) || strings.Contains(result.HTML, `name="mode"`) {
		t.Errorf("a bare id is still a control name: %s", result.HTML)
	}

	// The browser posts the client IDs; the binding reached before is kept.
	posted, err := RenderPage(PageRenderRequest{
		Project: p, VFIndex: idx, Machine: machine, PageName: "P", PageURL: "/apex/P", Action: "{!save}",
		FormValues: map[string]string{"j_id0:f1:i1": "one", "j_id0:f1:j_id1": "quick"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(posted.HTML, "Seen quick one") {
		t.Fatalf("postback did not reach the action: %s", posted.HTML)
	}
}

// A client-ID post reaches the legacy binding for inputs without an id, hidden
// inputs, inputs in iteration and inputs whose bare id repeats in two forms
// (rows Hid*, Rep*, Pst* of testdata/vf_render_markup, and the review probes of
// vfrender3). The controller has no property named mode, so the value arrives
// through the action parameters, as it did when inputs posted bare names.
func TestVFRender3ClientIDPostsReachLegacyBindings(t *testing.T) {
	for _, tc := range []struct {
		name, form, post string
		param            string // the action parameter the bare name arrives under
		want             string
	}{
		{"hidden no id", `<apex:inputHidden value="{!mode}"/>`, "j_id0:f1:j_id1", "mode", "Seen posted"},
		{"hidden with id", `<apex:inputHidden value="{!mode}" id="h1"/>`, "j_id0:f1:h1", "h1", "Seen posted"},
		{"text no id", `<apex:inputText value="{!mode}"/>`, "j_id0:f1:j_id1", "mode", "Seen posted"},
		{"repeat no id", `<apex:repeat value="{!'a'}" var="x"><apex:inputText value="{!mode}"/></apex:repeat>`, "j_id0:f1:j_id1:0:j_id2", "mode", "Seen posted"},
		{"repeat second iteration only", `<apex:repeat value="{!'a'}" var="x"><apex:inputText value="{!mode}"/></apex:repeat>`, "j_id0:f1:j_id1:1:j_id2", "mode", "Seen posted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, idx := vfRender3Project(t, `<apex:page controller="PostController"><apex:form id="f1">`+tc.form+`<apex:commandButton value="Save" action="{!save}"/><apex:pageMessages/></apex:form></apex:page>`)
			saveProgram, err := vm.CompileAnonymous(`ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Seen ' + ApexPages.currentPage().getParameters().get('` + tc.param + `')));
return null;`)
			if err != nil {
				t.Fatal(err)
			}
			machine := testRunner(t)
			if err := machine.RegisterClass(vm.Class{Name: "PostController", Methods: map[string]vm.Method{
				"save": {Name: "PostController.save", ClassName: "PostController", ReturnType: "PageReference", Program: saveProgram},
			}}); err != nil {
				t.Fatal(err)
			}
			posted, err := RenderPage(PageRenderRequest{
				Project: p, VFIndex: idx, Machine: machine, PageName: "P", PageURL: "/apex/P", Action: "{!save}",
				FormValues: map[string]string{tc.post: "posted"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(posted.HTML, tc.want) {
				t.Fatalf("post under %s did not bind: %s", tc.post, posted.HTML)
			}
		})
	}
}

// Two forms with the same input id: a post of one form binds that form's input only.
func TestVFRender3TwoFormsSameInputID(t *testing.T) {
	p, idx := vfRender3Project(t, `<apex:page controller="PostController">
<apex:form id="f1"><apex:inputText value="{!first}" id="q"/></apex:form>
<apex:form id="f2"><apex:inputText value="{!second}" id="q"/><apex:commandButton value="Save" action="{!save}"/><apex:pageMessages/></apex:form>
</apex:page>`)
	saveProgram, err := vm.CompileAnonymous(`ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Seen ' + ApexPages.currentPage().getParameters().get('q')));
return null;`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{Name: "PostController", Methods: map[string]vm.Method{
		"save": {Name: "PostController.save", ClassName: "PostController", ReturnType: "PageReference", Program: saveProgram},
	}}); err != nil {
		t.Fatal(err)
	}
	posted, err := RenderPage(PageRenderRequest{
		Project: p, VFIndex: idx, Machine: machine, PageName: "P", PageURL: "/apex/P", Action: "{!save}",
		FormValues: map[string]string{"j_id0:f2:q": "two"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(posted.HTML, "Seen two") {
		t.Fatalf("second form's post did not bind: %s", posted.HTML)
	}
}
