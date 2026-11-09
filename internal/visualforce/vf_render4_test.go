package visualforce

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// A property bound in every apex:repeat iteration keeps the last iteration's
// value (native row RepTextMode of testdata/vf_render_markup/postback.tsv:
// mode=v2). The legacy binder's alias must let the later iteration win.
func TestVFRender4LegacyAliasLastIterationWins(t *testing.T) {
	for _, tc := range []struct {
		name, input, first, second, param string
	}{
		{"text no id", `<apex:inputText value="{!mode}"/>`, "j_id0:f1:j_id1:0:j_id2", "j_id0:f1:j_id1:1:j_id2", "mode"},
		{"text with id", `<apex:inputText value="{!mode}" id="m1"/>`, "j_id0:f1:j_id1:0:m1", "j_id0:f1:j_id1:1:m1", "m1"},
		{"hidden no id", `<apex:inputHidden value="{!mode}"/>`, "j_id0:f1:j_id1:0:j_id2", "j_id0:f1:j_id1:1:j_id2", "mode"},
		{"tenth iteration after second", `<apex:inputText value="{!mode}"/>`, "j_id0:f1:j_id1:2:j_id2", "j_id0:f1:j_id1:10:j_id2", "mode"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, idx := vfRender3Project(t, `<apex:page controller="PostController"><apex:form id="f1"><apex:repeat value="{!'a'}" var="x">`+tc.input+`</apex:repeat><apex:commandButton value="Save" action="{!save}"/><apex:pageMessages/></apex:form></apex:page>`)
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
				FormValues: map[string]string{tc.first: "earlier", tc.second: "later"},
			})
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(posted.HTML, "Seen later") {
				t.Fatalf("the later iteration did not win: %s", posted.HTML)
			}
		})
	}
}
