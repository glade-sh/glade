package visualforce

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderPagePostPassesSubmittedValuesToAction(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page controller="EditController">
  <apex:form>
    <apex:pageMessages/>
    <apex:inputText value="{!mode}"/>
    <apex:commandButton value="Save" action="{!save}"/>
  </apex:form>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`
String mode = ApexPages.currentPage().getParameters().get('mode');
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Saved ' + mode));
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "EditController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "EditController.save",
				ClassName:  "EditController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "Edit",
		PageURL:  "/apex/Edit",
		Action:   "{!save}",
		FormValues: map[string]string{
			"mode": "quick",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "Saved quick") {
		t.Fatalf("html = %s", result.HTML)
	}
}

func TestRenderPageStopsActionDispatchAfterExtensionRuntimeError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page standardController="Account" extensions="BlockExtension">
  <apex:outputText value="ready"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`throw new VisualforceException('blocked');`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "BlockExtension",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "BlockExtension.save",
				ClassName:  "BlockExtension",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	_, err = RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "Edit",
		PageURL:  "/apex/Edit",
		Action:   "{!save}",
	})
	if err == nil || !strings.Contains(err.Error(), "blocked") {
		t.Fatalf("err = %v", err)
	}
}

func TestRenderPageRunsPageActionBeforeRendering(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ProbeLifecycleAction.page"), `<apex:page controller="ProbeLifecycleActionController" action="{!step}">
  <apex:outputText value="{!status}"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	stepProgram, err := vm.CompileAnonymous(`
this.status = 'stepped';
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "ProbeLifecycleActionController",
		Fields: map[string]vm.Field{
			"status": {Name: "status", Type: "String", InitialValue: vm.String("probe ready")},
		},
		Methods: map[string]vm.Method{
			"step": {Name: "ProbeLifecycleActionController.step", ClassName: "ProbeLifecycleActionController", ReturnType: "PageReference", Program: stepProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "ProbeLifecycleAction",
		PageURL:  "/apex/ProbeLifecycleAction",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "stepped") || strings.Contains(result.HTML, "probe ready") {
		t.Fatalf("html = %s", result.HTML)
	}
}

func TestRenderPageStandardControllerSaveUpdatesCurrentPageRecord(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page standardController="Account">
  <apex:form>
    <apex:inputField value="{!Account.Name}"/>
    <apex:commandButton value="Save" action="{!save}"/>
  </apex:form>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {
				ID:     "001000000000001",
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name": storage.StringValue("Original"),
				},
			},
		},
	}
	machine := vm.New(nil)
	machine.SetOrg(&org)

	_, err = RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Edit",
		PageURL:  "/apex/Edit?id=001000000000001",
		Action:   "{!save}",
		FormValues: map[string]string{
			"Name": "Updated",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	got := org.Objects["Account"].Records["001000000000001"].Fields["Name"].String
	if got != "Updated" {
		t.Fatalf("stored Account.Name = %q, want Updated", got)
	}
}

func TestRenderPageStandardSetRecordSetVarUsesOrgRecords(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Accounts.page"), `<apex:page standardController="Account" recordSetVar="accounts">
  <apex:pageBlock>
    <apex:pageBlockTable value="{!accounts}" var="a">
      <apex:column value="{!a.Name}"/>
    </apex:pageBlockTable>
  </apex:pageBlock>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := standardSetControllerOrg()
	machine := vm.New(nil)
	machine.SetOrg(&org)

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Accounts",
		PageURL:  "/apex/Accounts",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Acme", "Acme Probe", "Global Media", "salesforce.com", "Sample Account for Entitlements"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestRenderPageStandardSetExtensionReceivesController(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Accounts.page"), `<apex:page standardController="Account" recordSetVar="accounts" extensions="ProbeStandardSetControllerExtension">
  <apex:outputText value="{!resultSize}"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	getResultSize, err := vm.CompileAnonymous(`return controller.getResultSize();`)
	if err != nil {
		t.Fatal(err)
	}
	ctorProgram, err := vm.CompileAnonymous(`
this.controller = controller;
this.controller.setPageSize(2);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := standardSetControllerOrg()
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(vm.Class{
		Name: "ProbeStandardSetControllerExtension",
		Fields: map[string]vm.Field{
			"controller": {Name: "controller", Type: "ApexPages.StandardSetController"},
			"resultSize": {Name: "resultSize", Type: "Integer", Getter: &vm.Method{
				Name: "ProbeStandardSetControllerExtension.getResultSize", ClassName: "ProbeStandardSetControllerExtension", ReturnType: "Integer", Program: getResultSize,
			}},
		},
		Constructors: []vm.Method{{
			Name:          "ProbeStandardSetControllerExtension.<init>",
			ClassName:     "ProbeStandardSetControllerExtension",
			Params:        []vm.Param{{Name: "controller", Type: "ApexPages.StandardSetController"}},
			Program:       ctorProgram,
			IsConstructor: true,
		}},
		Methods: map[string]vm.Method{
			"getResultSize": {Name: "ProbeStandardSetControllerExtension.getResultSize", ClassName: "ProbeStandardSetControllerExtension", ReturnType: "Integer", Program: getResultSize},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "Accounts",
		PageURL:  "/apex/Accounts",
	})
	if err != nil {
		t.Fatal(err)
	}
	visibleHTML := result.HTML
	if idx := strings.Index(visibleHTML, `<input type="hidden" name="com.salesforce.visualforce.ViewState"`); idx >= 0 {
		visibleHTML = visibleHTML[:idx]
	}
	if !strings.Contains(visibleHTML, "5") {
		t.Fatalf("html missing result size 5: %s", result.HTML)
	}
}

func TestRenderPageStandardSetControllerReflectsExtensionPageSizeInRecordSetVar(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ProbeStandardSetPagination.page"), `<apex:page standardController="Account" recordSetVar="accounts" extensions="ProbeStandardSetPageSizeExtension">
  <apex:outputText value="{!rowCount}"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	getRowCount, err := vm.CompileAnonymous(`return controller.getRecords().size();`)
	if err != nil {
		t.Fatal(err)
	}
	ctorProgram, err := vm.CompileAnonymous(`
this.controller = controller;
this.controller.setPageSize(5);
`)
	if err != nil {
		t.Fatal(err)
	}
	org := standardSetControllerOrg()
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(vm.Class{
		Name: "ProbeStandardSetPageSizeExtension",
		Fields: map[string]vm.Field{
			"controller": {Name: "controller", Type: "ApexPages.StandardSetController"},
			"rowCount": {Name: "rowCount", Type: "Integer", Getter: &vm.Method{
				Name: "ProbeStandardSetPageSizeExtension.getRowCount", ClassName: "ProbeStandardSetPageSizeExtension", ReturnType: "Integer", Program: getRowCount,
			}},
		},
		Constructors: []vm.Method{{
			Name:          "ProbeStandardSetPageSizeExtension.<init>",
			ClassName:     "ProbeStandardSetPageSizeExtension",
			Params:        []vm.Param{{Name: "controller", Type: "ApexPages.StandardSetController"}},
			Program:       ctorProgram,
			IsConstructor: true,
		}},
		Methods: map[string]vm.Method{
			"getRowCount": {Name: "ProbeStandardSetPageSizeExtension.getRowCount", ClassName: "ProbeStandardSetPageSizeExtension", ReturnType: "Integer", Program: getRowCount},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "ProbeStandardSetPagination",
		PageURL:  "/apex/ProbeStandardSetPagination",
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "2") {
		t.Fatalf("html missing row count 2: %s", result.HTML)
	}
}

func TestRenderPageStandardSetRecordSetVarRefreshesAfterActionChangesPageSize(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ProbeStandardSetPagination.page"), `<apex:page standardController="Account" recordSetVar="accounts" extensions="ProbeStandardSetActionExtension">
  <apex:repeat value="{!accounts}" var="a"><apex:outputText value="{!a.Name}"/>|</apex:repeat>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	limitProgram, err := vm.CompileAnonymous(`
this.controller.setPageSize(2);
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	org := standardSetControllerOrg()
	machine := vm.New(nil)
	machine.SetOrg(&org)
	if err := machine.RegisterClass(vm.Class{
		Name: "ProbeStandardSetActionExtension",
		Fields: map[string]vm.Field{
			"controller": {Name: "controller", Type: "ApexPages.StandardSetController"},
		},
		Methods: map[string]vm.Method{
			"limit": {Name: "ProbeStandardSetActionExtension.limit", ClassName: "ProbeStandardSetActionExtension", ReturnType: "PageReference", Program: limitProgram},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "ProbeStandardSetPagination",
		PageURL:  "/apex/ProbeStandardSetPagination",
		Action:   "{!limit}",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Acme", "Acme Probe"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
	if strings.Contains(result.HTML, "Global Media") {
		t.Fatalf("recordSetVar did not refresh after action page size change: %s", result.HTML)
	}
}

func TestRenderPageStandardSetTableIncludesHeaderAndOrgRows(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/ProbeStandardSetTable.page"), `<apex:page standardController="Account" recordSetVar="accounts">
  <apex:pageBlockTable value="{!accounts}" var="a">
    <apex:column value="{!a.Name}"/>
  </apex:pageBlockTable>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	org := standardSetControllerOrg()
	machine := vm.New(nil)
	machine.SetOrg(&org)

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Org:      &org,
		Machine:  machine,
		PageName: "ProbeStandardSetTable",
		PageURL:  "/apex/ProbeStandardSetTable",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Account Name", "Acme", "Acme Probe", "Global Media", "salesforce.com", "Sample Account for Entitlements"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func standardSetControllerOrg() storage.OrgState {
	org := storage.NewOrgState()
	org.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]storage.Field{
			"Name": {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
		}},
		Records: map[storage.ID]storage.Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme Probe")}},
			"001000000000003": {ID: "001000000000003", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Global Media")}},
			"001000000000004": {ID: "001000000000004", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("salesforce.com")}},
			"001000000000005": {ID: "001000000000005", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Sample Account for Entitlements")}},
		},
	}
	return org
}

func TestRenderPageRestoresExtensionFieldsFromViewState(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Ext.page"), `<apex:page controller="HostController" extensions="TrailExtension">
  <apex:outputText value="{!note}"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{Name: "HostController"}); err != nil {
		t.Fatal(err)
	}
	if err := machine.RegisterClass(vm.Class{
		Name: "TrailExtension",
		Fields: map[string]vm.Field{
			"note": {Name: "note", Type: "String"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "Ext",
		PageURL:  "/apex/Ext",
		ViewState: &ViewStatePayload{
			PageName:        "Ext",
			ControllerType:  "HostController",
			ExtensionFields: []map[string]string{{"note": "restored"}},
			CSRF:            "token",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result.HTML, "restored") {
		t.Fatalf("html missing restored extension field: %s", result.HTML)
	}
}

func TestRenderPagePreservesViewStatePageMessagesOnPostback(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Messages.page"), `<apex:page controller="MessageController">
  <apex:form>
    <apex:pageMessages/>
    <apex:commandButton value="Save" action="{!save}"/>
  </apex:form>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`
ApexPages.addMessage(new ApexPages.Message(ApexPages.Severity.INFO, 'Fresh message'));
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "MessageController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "MessageController.save",
				ClassName:  "MessageController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "Messages",
		PageURL:  "/apex/Messages",
		ViewState: &ViewStatePayload{
			PageName:     "Messages",
			CSRF:         "token",
			PageMessages: []string{"Prior message"},
		},
		Action: "{!save}",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Prior message", "Fresh message"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
	decoded, err := DecodeViewState(result.ViewState, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Prior message", "Fresh message"} {
		if !stringSliceContains(decoded.PageMessages, want) {
			t.Fatalf("view state messages = %#v, want %q", decoded.PageMessages, want)
		}
	}
}

func TestRenderPageRejectsOversizedViewStatePayload(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Large.page"), `<apex:page controller="LargeController">
  <apex:form/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{Name: "LargeController"}); err != nil {
		t.Fatal(err)
	}

	_, err = RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "Large",
		PageURL:  "/apex/Large",
		ViewState: &ViewStatePayload{
			PageName:         "Large",
			CSRF:             "token",
			ControllerFields: map[string]string{"blob": strings.Repeat("x", MaxVisualforceViewStateBytes)},
		},
	})
	if !errors.Is(err, ErrVisualforceLimitExceeded) {
		t.Fatalf("err = %#v, want ErrVisualforceLimitExceeded", err)
	}
}

func TestRenderPageIteratesControllerGetterLists(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/List.page"), `<apex:page controller="ListController">
  <apex:repeat value="{!rows}" var="row"><apex:outputText value="{!row}"/><br/></apex:repeat>
  <apex:dataTable value="{!accounts}" var="a"><apex:column value="{!a.Name}"/></apex:dataTable>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	rowsGetter, err := vm.CompileAnonymous(`return new List<String>{'alpha', 'bravo', 'charlie'};`)
	if err != nil {
		t.Fatal(err)
	}
	accountsGetter, err := vm.CompileAnonymous(`
return new List<Account>{
    new Account(Name = 'Acme Probe'),
    new Account(Name = 'Birch Probe'),
    new Account(Name = 'Cedar Probe')
};
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "ListController",
		Fields: map[string]vm.Field{
			"rows": {Name: "rows", Type: "List<String>", Getter: &vm.Method{
				Name: "ListController.getRows", ClassName: "ListController", ReturnType: "List<String>", Program: rowsGetter,
			}},
			"accounts": {Name: "accounts", Type: "List<Account>", Getter: &vm.Method{
				Name: "ListController.getAccounts", ClassName: "ListController", ReturnType: "List<Account>", Program: accountsGetter,
			}},
		},
		Methods: map[string]vm.Method{
			"getRows":     {Name: "ListController.getRows", ClassName: "ListController", ReturnType: "List<String>", Program: rowsGetter},
			"getAccounts": {Name: "ListController.getAccounts", ClassName: "ListController", ReturnType: "List<Account>", Program: accountsGetter},
		},
	}); err != nil {
		t.Fatal(err)
	}
	controller, err := machine.ConstructController("ListController")
	if err != nil {
		t.Fatal(err)
	}
	rowsValue, ok, err := machine.ReadInstanceProperty(controller, "rows")
	if err != nil {
		t.Fatal(err)
	}
	if !ok || rowsValue.Kind != vm.ValueList || len(rowsValue.List) != 3 {
		t.Fatalf("ReadInstanceProperty(rows) = %#v ok=%v", rowsValue, ok)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:  p,
		VFIndex:  idx,
		Machine:  machine,
		PageName: "List",
		PageURL:  "/apex/List",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"alpha", "bravo", "charlie", "Acme Probe", "Birch Probe", "Cedar Probe"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestVisualforcePostLifecycleAppliesSubmittedValuesBeforeActionAndRendersAfter(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Lifecycle.page"), `<apex:page controller="LifecycleController">
  <apex:form id="form">
    <apex:inputText id="name" value="{!name}"/>
    <apex:commandButton value="Save" action="{!save}"/>
    <apex:outputText value="{!trace}"/>
  </apex:form>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`
this.trace = 'setter:' + this.name + '|action:save';
return null;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "LifecycleController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "LifecycleController.save",
				ClassName:  "LifecycleController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   machine,
		PageName:  "Lifecycle",
		PageURL:   "/apex/Lifecycle",
		ViewState: &ViewStatePayload{PageName: "Lifecycle", ControllerType: "LifecycleController", CSRF: "token"},
		Action:    "{!save}",
		FormValues: map[string]string{
			"form:name": "changed",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"setter:changed", "action:save"} {
		if !strings.Contains(result.HTML, want) {
			t.Fatalf("html missing %q: %s", want, result.HTML)
		}
	}
}

func TestVisualforceActionPageReferenceResultBecomesRedirect(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page controller="EditController">
  <apex:form><apex:commandButton value="Save" action="{!save}"/></apex:form>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`
PageReference next = new PageReference('/apex/Done');
next.setRedirect(true);
return next;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "EditController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "EditController.save",
				ClassName:  "EditController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   machine,
		PageName:  "Edit",
		PageURL:   "/apex/Edit",
		ViewState: &ViewStatePayload{PageName: "Edit", ControllerType: "EditController", CSRF: "token"},
		Action:    "{!save}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RedirectURL != "/apex/Done" || !result.Redirect {
		t.Fatalf("redirect = %q %t", result.RedirectURL, result.Redirect)
	}
}

func TestVisualforceActionPageReferenceWithoutRedirectRendersTargetPage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page controller="EditController">
  <apex:form><apex:commandButton value="Save" action="{!save}"/></apex:form>
</apex:page>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Done.page"), `<apex:page>
  <apex:outputText value="Done Rendered"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`
PageReference next = new PageReference('/apex/Done');
next.setRedirect(false);
return next;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "EditController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "EditController.save",
				ClassName:  "EditController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   machine,
		PageName:  "Edit",
		PageURL:   "/apex/Edit",
		ViewState: &ViewStatePayload{PageName: "Edit", ControllerType: "EditController", CSRF: "token"},
		Action:    "{!save}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RedirectURL != "" || !strings.Contains(result.HTML, "Done Rendered") {
		t.Fatalf("result redirect=%q html=%s", result.RedirectURL, result.HTML)
	}
}

func TestVisualforceActionPageReferenceWithoutRedirectDoesNotTreatRecordURLAsApexPage(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/pages/Edit.page"), `<apex:page controller="EditController">
  <apex:form><apex:commandButton value="Save" action="{!save}"/></apex:form>
  <apex:outputText value="Still Here"/>
</apex:page>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}
	saveProgram, err := vm.CompileAnonymous(`
PageReference next = new PageReference('/001000000000001AAA');
next.setRedirect(false);
return next;
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := testRunner(t)
	if err := machine.RegisterClass(vm.Class{
		Name: "EditController",
		Methods: map[string]vm.Method{
			"save": {
				Name:       "EditController.save",
				ClassName:  "EditController",
				ReturnType: "PageReference",
				Program:    saveProgram,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}

	result, err := RenderPage(PageRenderRequest{
		Project:   p,
		VFIndex:   idx,
		Machine:   machine,
		PageName:  "Edit",
		PageURL:   "/apex/Edit",
		ViewState: &ViewStatePayload{PageName: "Edit", ControllerType: "EditController", CSRF: "token"},
		Action:    "{!save}",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.RedirectURL != "" || !strings.Contains(result.HTML, "Still Here") {
		t.Fatalf("result redirect=%q html=%s", result.RedirectURL, result.HTML)
	}
}

func stringSliceContains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
