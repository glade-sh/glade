package visualforce

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

// Point-source family: page-action rows 43-45, outputText row 159,
// inputField list row 160, and null-relative-comparison row 174.
func TestVisualforceExpressionContractFamily(t *testing.T) {
	t.Run("page-action-null-refresh-and-PageReference", func(t *testing.T) {
		t.Run("invokes-before-render-and-null-refreshes", func(t *testing.T) {
			p, idx := expressionContractPage(t, "ExpressionAction", `<apex:page controller="ExpressionActionController" action="{!step}"><apex:outputText value="{!status}"/></apex:page>`)
			program, err := vm.CompileAnonymous(`this.status = 'invoked'; return null;`)
			if err != nil {
				t.Fatal(err)
			}
			machine := testRunner(t)
			if err := machine.RegisterClass(vm.Class{
				Name:    "ExpressionActionController",
				Fields:  map[string]vm.Field{"status": {Name: "status", Type: "String", InitialValue: vm.String("before")}},
				Methods: map[string]vm.Method{"step": {Name: "ExpressionActionController.step", ClassName: "ExpressionActionController", ReturnType: "PageReference", Program: program}},
			}); err != nil {
				t.Fatal(err)
			}
			result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "ExpressionAction", PageURL: "/apex/ExpressionAction"})
			if err != nil {
				t.Fatal(err)
			}
			if result.Redirect || !strings.Contains(result.HTML, "invoked") {
				t.Fatalf("result redirect=%t url=%q html=%s", result.Redirect, result.RedirectURL, result.HTML)
			}
		})

		t.Run("PageReference-may-redirect", func(t *testing.T) {
			p, idx := expressionContractPage(t, "ExpressionRedirect", `<apex:page controller="ExpressionRedirectController" action="{!go}"><apex:outputText value="not redirected"/></apex:page>`)
			program, err := vm.CompileAnonymous(`PageReference target = new PageReference('/apex/ExpressionDone'); target.setRedirect(true); return target;`)
			if err != nil {
				t.Fatal(err)
			}
			machine := testRunner(t)
			if err := machine.RegisterClass(vm.Class{
				Name:    "ExpressionRedirectController",
				Methods: map[string]vm.Method{"go": {Name: "ExpressionRedirectController.go", ClassName: "ExpressionRedirectController", ReturnType: "PageReference", Program: program}},
			}); err != nil {
				t.Fatal(err)
			}
			result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: machine, PageName: "ExpressionRedirect", PageURL: "/apex/ExpressionRedirect"})
			if err != nil {
				t.Fatal(err)
			}
			if !result.Redirect || result.RedirectURL != "/apex/ExpressionDone" {
				t.Fatalf("redirect=%t url=%q", result.Redirect, result.RedirectURL)
			}
		})
	})

	t.Run("recordSetVar-display-after-action", func(t *testing.T) {
		p, idx := expressionContractPage(t, "ExpressionRecordSet", `<apex:page standardController="Account" recordSetVar="records" extensions="ExpressionRecordSetAction"><apex:outputText value="{!actionStatus}"/><apex:repeat value="{!records}" var="row"><apex:outputText value="{!row.Name}"/>|</apex:repeat></apex:page>`)
		program, err := vm.CompileAnonymous(`this.controller.setPageSize(2); this.actionStatus = 'action-ran'; return null;`)
		if err != nil {
			t.Fatal(err)
		}
		org := standardSetControllerOrg()
		machine := vm.New(nil)
		machine.SetOrg(&org)
		machine.SetCurrentUser(org.Objects["User"].Records[standardSetControllerUserID])
		if err := machine.RegisterClass(vm.Class{
			Name: "ExpressionRecordSetAction",
			Fields: map[string]vm.Field{
				"controller":   {Name: "controller", Type: "ApexPages.StandardSetController"},
				"actionStatus": {Name: "actionStatus", Type: "String"},
			},
			Methods: map[string]vm.Method{"limit": {Name: "ExpressionRecordSetAction.limit", ClassName: "ExpressionRecordSetAction", ReturnType: "PageReference", Program: program}},
		}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Org: &org, Machine: machine, PageName: "ExpressionRecordSet", PageURL: "/apex/ExpressionRecordSet", Action: "{!limit}"})
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{"action-ran", "Acme", "Acme Probe"} {
			if !strings.Contains(result.HTML, want) {
				t.Fatalf("html missing %q: %s", want, result.HTML)
			}
		}
	})

	t.Run("URLFOR-resource-page-action-redirect", func(t *testing.T) {
		p, idx := expressionContractPage(t, "ExpressionResource", `<apex:page action="{!URLFOR($Resource.ExpressionBundle)}"><apex:outputText value="not redirected"/></apex:page>`)
		writeFile(t, filepath.Join(p.Root, "force-app/main/default/staticresources/ExpressionBundle.resource"), "owned resource entry")
		result, err := RenderPage(PageRenderRequest{Project: p, VFIndex: idx, Machine: testRunner(t), PageName: "ExpressionResource", PageURL: "/apex/ExpressionResource"})
		if err != nil {
			t.Fatal(err)
		}
		if !result.Redirect || !strings.Contains(result.RedirectURL, "/resource/ExpressionBundle") {
			t.Fatalf("resource action redirect=%t url=%q", result.Redirect, result.RedirectURL)
		}
	})

	t.Run("outputText-escape-omitted-true-false", func(t *testing.T) {
		tree, err := ParseMarkupTree(`<apex:page><apex:outputText value="{!omitted}"/><apex:outputText escape="true" value="{!explicit}"/><apex:outputText escape="false" value="{!raw}"/></apex:page>`)
		if err != nil {
			t.Fatal(err)
		}
		controller := vm.Object("ExpressionOutputController")
		controller.Fields["omitted"] = vm.String("omitted<probe/>")
		controller.Fields["explicit"] = vm.String("explicit<probe/>")
		controller.Fields["raw"] = vm.String("raw<probe/>")
		rendered, err := RenderMarkupTree(tree, &RenderContext{Expression: &ExpressionContext{Controller: controller}})
		if err != nil {
			t.Fatal(err)
		}
		for _, prefix := range []string{"omitted", "explicit", "raw"} {
			if !strings.Contains(rendered, prefix) {
				t.Fatalf("rendered output missing %q: %s", prefix, rendered)
			}
		}
		if strings.Contains(rendered, "omitted<probe/>") || strings.Contains(rendered, "explicit<probe/>") {
			t.Fatalf("omitted/true escape emitted source markup literally: %s", rendered)
		}
		if !strings.Contains(rendered, "raw<probe/>") {
			t.Fatalf("escape=false did not emit supplied markup: %s", rendered)
		}
	})

	t.Run("inputField-list-literal-string-object-toString", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Label: "Name", Type: storage.FieldString}},
		}}
		machine := vm.New(nil)
		machine.SetOrg(&org)
		toStringProgram, err := vm.CompileAnonymous(`return 'object-toString-value';`)
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterClass(vm.Class{
			Name:    "ExpressionListChoice",
			Methods: map[string]vm.Method{"toString": {Name: "ExpressionListChoice.toString", ClassName: "ExpressionListChoice", ReturnType: "String", Program: toStringProgram}},
		}); err != nil {
			t.Fatal(err)
		}
		controller := vm.Object("ExpressionListController")
		account := vm.Object("Account")
		account.Fields["Name"] = vm.String("Acme")
		controller.Fields["Account"] = account
		controller.Fields["stringValues"] = vm.String("string-first,string-second")
		controller.Fields["escapedValues"] = vm.String("unsafe<&")
		controller.Fields["objectValues"] = vm.List(vm.String("object-list-string"), vm.Object("ExpressionListChoice"))
		tree, err := ParseMarkupTree(`<apex:page><apex:inputField id="literal" value="{!Account.Name}" list="literal-first,literal-second"/><apex:inputField id="string" value="{!Account.Name}" list="{!stringValues}"/><apex:inputField id="objects" value="{!Account.Name}" list="{!objectValues}"/><apex:inputField id="escaped" value="{!Account.Name}" list="{!escapedValues}"/></apex:page>`)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := RenderMarkupTree(tree, &RenderContext{VM: machine, Expression: &ExpressionContext{VM: machine, Controller: controller}})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Count(rendered, "<datalist"); got != 4 {
			t.Fatalf("rendered %d datalists for four listed inputs: %s", got, rendered)
		}
		for _, want := range []string{"literal-first", "literal-second", "string-first", "string-second", "object-list-string", "object-toString-value"} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("rendered datalist missing %q: %s", want, rendered)
			}
		}
		if !strings.Contains(rendered, `<option value="unsafe&lt;&amp;">unsafe&lt;&amp;</option>`) {
			t.Fatalf("datalist option was not escaped in attribute and text contexts: %s", rendered)
		}
		associations := regexp.MustCompile(`<input\b[^>]*\bid="([^"]+)"[^>]*\blist="([^"]+)"`).FindAllStringSubmatch(rendered, -1)
		if len(associations) != 4 {
			t.Fatalf("got %d input/datalist references, want four: %s", len(associations), rendered)
		}
		for _, association := range associations {
			inputID, listID := association[1], association[2]
			if listID != inputID+"-list" || !strings.Contains(rendered, `<datalist id="`+listID+`">`) {
				t.Fatalf("input id %q is not associated with its datalist %q: %s", inputID, listID, rendered)
			}
		}
	})

	t.Run("unnamed-fields-use-form-qualified-datalist-ids", func(t *testing.T) {
		org := storage.NewOrgState()
		org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "Account", Fields: map[string]storage.Field{"Name": {APIName: "Name", Label: "Name", Type: storage.FieldString}},
		}}
		machine := vm.New(nil)
		machine.SetOrg(&org)
		account := vm.Object("Account")
		account.Fields["Name"] = vm.String("Acme")
		controller := vm.Object("ExpressionListController")
		controller.Fields["Account"] = account
		tree, err := ParseMarkupTree(`<apex:page><apex:form id="formOne"><apex:inputField value="{!Account.Name}" list="first-choice"/></apex:form><apex:form id="formTwo"><apex:inputField value="{!Account.Name}" list="second-choice"/></apex:form></apex:page>`)
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := RenderMarkupTree(tree, &RenderContext{VM: machine, Expression: &ExpressionContext{VM: machine, Controller: controller}})
		if err != nil {
			t.Fatal(err)
		}
		associations := regexp.MustCompile(`<input\b[^>]*\bname="Account\.Name"[^>]*\blist="([^"]+)"`).FindAllStringSubmatch(rendered, -1)
		if len(associations) != 2 {
			t.Fatalf("got %d Account.Name input list references, want two: %s", len(associations), rendered)
		}
		firstList, secondList := associations[0][1], associations[1][1]
		if firstList == secondList || !strings.Contains(firstList, "formOne") || !strings.Contains(secondList, "formTwo") {
			t.Fatalf("unnamed datalist IDs are not naming-container-qualified: %q, %q", firstList, secondList)
		}
		if strings.Count(rendered, `name="Account.Name"`) != 2 {
			t.Fatalf("datalist IDs changed submitted field names: %s", rendered)
		}
		if !strings.Contains(rendered, `<datalist id="`+firstList+`"><option value="first-choice">first-choice</option></datalist>`) ||
			!strings.Contains(rendered, `<datalist id="`+secondList+`"><option value="second-choice">second-choice</option></datalist>`) {
			t.Fatalf("form inputs are not associated with their own option sets: %s", rendered)
		}
	})

	t.Run("null-relative-comparisons-error-on-either-side", func(t *testing.T) {
		ctx := &ExpressionContext{Variables: map[string]vm.Value{"nullable": vm.Null, "number": vm.Int(1)}}
		for _, expr := range []string{
			"NULL < 1", "NULL <= 1", "NULL > 1", "NULL >= 1",
			"1 < NULL", "1 <= NULL", "1 > NULL", "1 >= NULL",
			"nullable < 1", "1 >= nullable",
		} {
			t.Run(expr, func(t *testing.T) {
				if _, err := EvaluateExpression(expr, ctx); err == nil {
					t.Fatalf("EvaluateExpression(%q) succeeded; source requires an exception", expr)
				}
			})
		}
		for _, tc := range []struct{ expr, want string }{
			{"1 < 2", "true"},
			{"2 >= 2", "true"},
			{"NULL == NULL", "true"},
			{"NULL != 1", "true"},
			{"1 == 1", "true"},
			{"false && (nullable < 1)", "false"},
		} {
			got, err := EvaluateExpression(tc.expr, ctx)
			if err != nil || got != tc.want {
				t.Fatalf("EvaluateExpression(%q) = %q, %v; want %q, nil", tc.expr, got, err, tc.want)
			}
		}
		tree, err := ParseMarkupTree(`<apex:page><apex:outputText value="{!nullable &lt; 1}"/></apex:page>`)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := RenderMarkupTree(tree, &RenderContext{Expression: ctx}); err == nil {
			t.Fatal("RenderMarkupTree accepted a null-relative comparison in outputText")
		}
	})

	t.Run("null-relative-errors-stop-later-child-evaluation", func(t *testing.T) {
		program, err := vm.CompileAnonymous(`this.calls = this.calls + 1; return true;`)
		if err != nil {
			t.Fatal(err)
		}
		machine := vm.New(nil)
		className := "ExpressionErrorSideEffectController"
		if err := machine.RegisterClass(vm.Class{
			Name:   className,
			Fields: map[string]vm.Field{"calls": {Name: "calls", Type: "Integer", InitialValue: vm.Int(0)}},
			Methods: map[string]vm.Method{"bump": {
				Name: className + ".bump", ClassName: className, ReturnType: "Boolean", Program: program,
			}},
		}); err != nil {
			t.Fatal(err)
		}
		controller := vm.Object(className)
		controller.Fields["calls"] = vm.Int(0)
		ctx := &ExpressionContext{VM: machine, Controller: controller}
		for _, expr := range []string{"NULL < 1 || this.bump()", "IF(NULL < 1, true, this.bump())"} {
			if _, err := EvaluateExpression(expr, ctx); err == nil {
				t.Fatalf("EvaluateExpression(%q) succeeded", expr)
			}
			calls := ctx.Controller.Fields["calls"]
			if calls.Kind != vm.ValueInt || calls.Int != 0 {
				t.Fatalf("%s ran after its condition errored; calls=%s", expr, calls.String())
			}
		}
		for _, tc := range []struct{ expr, want string }{
			{"false && (NULL < 1)", "false"},
			{"true || (NULL < 1)", "true"},
			{"IF(true, true, NULL < 1)", "true"},
			{"IF(false, NULL < 1, true)", "true"},
		} {
			got, err := EvaluateExpression(tc.expr, ctx)
			if err != nil || got != tc.want {
				t.Fatalf("EvaluateExpression(%q) = %q, %v; want %q, nil", tc.expr, got, err, tc.want)
			}
		}
		if calls := ctx.Controller.Fields["calls"]; calls.Kind != vm.ValueInt || calls.Int != 0 {
			t.Fatalf("short-circuited expressions ran bump; calls=%s", calls.String())
		}
	})
}

func expressionContractPage(t *testing.T, name, markup string) (project.Project, Index) {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"67.0"}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages", name+".page"), markup)
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
