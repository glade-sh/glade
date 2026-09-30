package visualforce

import (
	"strings"
	"testing"

	nethtml "golang.org/x/net/html"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/vm"
)

func TestRenderStandardControllerAddFieldsResetTwoRequestsAPI67(t *testing.T) {
	const (
		pageName      = "StandardControllerAddFieldsResetOracle"
		extensionName = "SCAddFieldsResetOracleExt"
		accountID     = storage.ID("001000000000001AAA")
		ownerID       = storage.ID("005000000000001AAA")
		accountName   = "GladeVFReset67"
		accountPhone  = "4155550137"
	)

	root := t.TempDir()
	writeFile(t, root+"/sfdx-project.json", `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, root+"/force-app/main/default/pages/"+pageName+`.page`, `<apex:page standardController="Account" extensions="SCAddFieldsResetOracleExt"
           action="{!runProbe}" showHeader="false" sidebar="false">
    <!-- Static Name reference supplies the reset request's field. -->
    <span id="static-account-name"><apex:outputField value="{!Account.Name}"/></span>
    <span id="text-account-name"><apex:outputText value="{!Account.Name}"/></span>
    <span id="probe-ran"><apex:outputText value="{!probeRan}"/></span>
    <apex:outputPanel rendered="{!isAddFieldsProbe}" layout="none">
        <span id="addfields-initial-name"><apex:outputText value="{!initialName}"/></span>
        <span id="addfields-phone"><apex:outputText value="{!addedPhone}"/></span>
        <span id="addfields-owner-name"><apex:outputText value="{!addedOwnerName}"/></span>
        <!-- Runtime-computed Phone binding is only in the addFields request. -->
        <span id="dynamic-phone"><apex:outputField value="{!Account[dynamicFieldName]}"/></span>
    </apex:outputPanel>
    <apex:outputPanel rendered="{!isResetProbe}" layout="none">
        <span id="reset-initial-name"><apex:outputText value="{!initialName}"/></span>
        <span id="reset-after-name"><apex:outputText value="{!afterResetName}"/></span>
        <span id="unsaved-edit-discarded"><apex:outputText value="{!unsavedEditDiscarded}"/></span>
    </apex:outputPanel>
</apex:page>`)
	writeFile(t, root+"/force-app/main/default/pages/"+pageName+`.page-meta.xml`, `<?xml version="1.0" encoding="UTF-8"?>
<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>67.0</apiVersion>
    <availableInTouch>true</availableInTouch>
    <confirmationTokenRequired>false</confirmationTokenRequired>
    <label>StandardControllerAddFieldsResetOracle</label>
</ApexPage>`)
	writeFile(t, root+"/force-app/main/default/classes/"+extensionName+`.cls`, `public with sharing class SCAddFieldsResetOracleExt {
    private static final String UNSAVED_NAME = 'UNSAVED-VF-RESET-67';
    private final ApexPages.StandardController standardController;
    private final String probeMode;

    public String initialName { get; private set; }
    public String addedPhone { get; private set; }
    public String addedOwnerName { get; private set; }
    public String afterResetName { get; private set; }
    public Boolean unsavedEditDiscarded { get; private set; }
    public Boolean probeRan { get; private set; }

    public class UnknownProbeModeException extends Exception {}

    public SCAddFieldsResetOracleExt(ApexPages.StandardController controller) {
        standardController = controller;
        probeMode = ApexPages.currentPage().getParameters().get('probe');
        probeRan = false;
        if (probeMode == 'addfields') {
            // Declare explicit fields before the first getRecord().
            standardController.addFields(new List<String>{'Phone', 'Owner.Name'});
            Account loaded = (Account) standardController.getRecord();
            initialName = loaded.Name;
            addedPhone = loaded.Phone;
            addedOwnerName = loaded.Owner.Name;
        } else if (probeMode != 'reset') {
            throw new UnknownProbeModeException();
        }
    }

    public Boolean getIsAddFieldsProbe() { return probeMode == 'addfields'; }
    public Boolean getIsResetProbe() { return probeMode == 'reset'; }
    public String getDynamicFieldName() {
        if (probeMode != 'addfields') throw new UnknownProbeModeException();
        return 'Phone';
    }

    // Page action runs outside the constructor. Neither request performs DML.
    public PageReference runProbe() {
        if (probeMode == 'reset') {
            Account beforeReset = (Account) standardController.getRecord();
            initialName = beforeReset.Name;
            beforeReset.Name = UNSAVED_NAME;
            standardController.reset();
            // Source-required immediate order; Name is already referenced statically.
            standardController.addFields(new List<String>{'Name'});
            Account afterReset = (Account) standardController.getRecord();
            afterResetName = afterReset.Name;
            unsavedEditDiscarded = afterResetName == initialName && afterResetName != UNSAVED_NAME;
        }
        probeRan = true;
        return null;
    }
}`)
	writeFile(t, root+"/force-app/main/default/classes/"+extensionName+`.cls-meta.xml`, `<?xml version="1.0" encoding="UTF-8"?>
<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata">
    <apiVersion>67.0</apiVersion>
    <status>Active</status>
</ApexClass>`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := LoadProject(p)
	if err != nil {
		t.Fatal(err)
	}

	const ownerName = "VF Reset Seed Owner"
	seedUser := storage.Record{
		ID:     ownerID,
		Object: "User",
		Fields: map[string]storage.Value{"Name": storage.StringValue(ownerName)},
	}
	seedOrg := storage.NewOrgState()
	seedOrg.Objects["User"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "User",
			KeyPrefix: "005",
			Fields: map[string]storage.Field{
				"Name": {APIName: "Name", Label: "Full Name", Type: storage.FieldString},
			},
		},
		Records: map[storage.ID]storage.Record{ownerID: seedUser},
	}
	seedOrg.Objects["Account"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{
			APIName:   "Account",
			KeyPrefix: "001",
			Fields: map[string]storage.Field{
				"Name":    {APIName: "Name", Label: "Account Name", Type: storage.FieldString},
				"Phone":   {APIName: "Phone", Label: "Phone", Type: storage.FieldString},
				"OwnerId": {APIName: "OwnerId", Label: "Owner ID", Type: storage.FieldReference, ReferenceTo: []string{"User"}, RelationshipName: "Owner"},
			},
		},
		Records: map[storage.ID]storage.Record{
			accountID: {
				ID:     accountID,
				Object: "Account",
				Fields: map[string]storage.Value{
					"Name":    storage.StringValue(accountName),
					"Phone":   storage.StringValue(accountPhone),
					"OwnerId": storage.IDValue(ownerID),
				},
			},
		},
	}
	seededOwner := seedOrg.Objects["User"].Records[ownerID]
	expectedOwnerName := seededOwner.Fields["Name"].String
	// RenderPage's unit-level VM does not compile Apex source files from Project.
	// Register the fixture extension's same constructor/action/getters explicitly;
	// the .cls above still fixes the intended source and API version.
	constructor, err := vm.CompileAnonymous(`
this.standardController = controller;
this.probeMode = ApexPages.currentPage().getParameters().get('probe');
this.probeRan = false;
if (this.probeMode == 'addfields') {
    this.standardController.addFields(new List<String>{'Phone', 'Owner.Name'});
    Account loaded = (Account) this.standardController.getRecord();
    this.initialName = loaded.Name;
    this.addedPhone = loaded.Phone;
    this.addedOwnerName = loaded.Owner.Name;
}`)
	if err != nil {
		t.Fatal(err)
	}
	runProbe, err := vm.CompileAnonymous(`
if (this.probeMode == 'reset') {
    Account beforeReset = (Account) this.standardController.getRecord();
    this.initialName = beforeReset.Name;
    beforeReset.Name = 'UNSAVED-VF-RESET-67';
    this.standardController.reset();
    this.standardController.addFields(new List<String>{'Name'});
    Account afterReset = (Account) this.standardController.getRecord();
    this.afterResetName = afterReset.Name;
    this.unsavedEditDiscarded = this.afterResetName == this.initialName && this.afterResetName != 'UNSAVED-VF-RESET-67';
}
this.probeRan = true;
return null;`)
	if err != nil {
		t.Fatal(err)
	}
	isAddFieldsProbe, err := vm.CompileAnonymous(`return this.probeMode == 'addfields';`)
	if err != nil {
		t.Fatal(err)
	}
	isResetProbe, err := vm.CompileAnonymous(`return this.probeMode == 'reset';`)
	if err != nil {
		t.Fatal(err)
	}
	dynamicFieldName, err := vm.CompileAnonymous(`return 'Phone';`)
	if err != nil {
		t.Fatal(err)
	}

	renderRequest := func(probe string) string {
		t.Helper()
		requestOrg := seedOrg.Clone()
		machine := vm.New(nil)
		machine.SetOrg(&requestOrg)
		machine.SetCurrentUser(seededOwner)
		if err := machine.RegisterClass(vm.Class{
			Name: extensionName,
			Fields: map[string]vm.Field{
				"standardController":   {Name: "standardController", Type: "ApexPages.StandardController"},
				"probeMode":            {Name: "probeMode", Type: "String"},
				"initialName":          {Name: "initialName", Type: "String"},
				"addedPhone":           {Name: "addedPhone", Type: "String"},
				"addedOwnerName":       {Name: "addedOwnerName", Type: "String"},
				"afterResetName":       {Name: "afterResetName", Type: "String"},
				"unsavedEditDiscarded": {Name: "unsavedEditDiscarded", Type: "Boolean"},
				"probeRan":             {Name: "probeRan", Type: "Boolean"},
			},
			Constructors: []vm.Method{{Name: extensionName + ".<init>", ClassName: extensionName,
				Params:  []vm.Param{{Name: "controller", Type: "ApexPages.StandardController"}},
				Program: constructor, IsConstructor: true}},
			Methods: map[string]vm.Method{
				"runProbe": {Name: extensionName + ".runProbe", ClassName: extensionName,
					ReturnType: "PageReference", Program: runProbe},
				"getIsAddFieldsProbe": {Name: extensionName + ".getIsAddFieldsProbe", ClassName: extensionName,
					ReturnType: "Boolean", Program: isAddFieldsProbe},
				"getIsResetProbe": {Name: extensionName + ".getIsResetProbe", ClassName: extensionName,
					ReturnType: "Boolean", Program: isResetProbe},
				"getDynamicFieldName": {Name: extensionName + ".getDynamicFieldName", ClassName: extensionName,
					ReturnType: "String", Program: dynamicFieldName},
			},
		}); err != nil {
			t.Fatal(err)
		}
		result, err := RenderPage(PageRenderRequest{
			Project:  p,
			VFIndex:  idx,
			Org:      &requestOrg,
			Machine:  machine,
			PageName: pageName,
			PageURL:  "/apex/" + pageName + "?id=" + string(accountID) + "&probe=" + probe,
		})
		if err != nil {
			t.Fatalf("render %s request: %v", probe, err)
		}
		if result.Error != nil {
			t.Fatalf("render %s request: %v", probe, result.Error)
		}
		return result.HTML
	}

	addFieldsHTML := renderRequest("addfields")
	addFieldsSelectors := standardControllerAddFieldsResetSelectorValues(t, addFieldsHTML)
	assertStandardControllerAddFieldsResetSelectors(t, "constructor-addfields-dynamic-readback", addFieldsSelectors, []standardControllerAddFieldsResetSelector{
		{id: "probe-ran", sourceExpected: "true", localExpected: "true", present: true},
		{id: "static-account-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "text-account-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "addfields-initial-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "addfields-phone", sourceExpected: accountPhone, localExpected: accountPhone, present: true},
		{id: "addfields-owner-name", sourceExpected: "seed Account.Owner.Name", localExpected: expectedOwnerName, present: true},
		{id: "dynamic-phone", sourceExpected: accountPhone, localExpected: accountPhone, present: true},
		{id: "reset-initial-name", sourceExpected: "<absent>", localExpected: "<absent>", present: false},
		{id: "reset-after-name", sourceExpected: "<absent>", localExpected: "<absent>", present: false},
		{id: "unsaved-edit-discarded", sourceExpected: "<absent>", localExpected: "<absent>", present: false},
	})

	resetHTML := renderRequest("reset")
	resetSelectors := standardControllerAddFieldsResetSelectorValues(t, resetHTML)
	assertStandardControllerAddFieldsResetSelectors(t, "reset-static-name-discard", resetSelectors, []standardControllerAddFieldsResetSelector{
		{id: "probe-ran", sourceExpected: "true", localExpected: "true", present: true},
		{id: "static-account-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "text-account-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "reset-initial-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "reset-after-name", sourceExpected: accountName, localExpected: accountName, present: true},
		{id: "unsaved-edit-discarded", sourceExpected: "true", localExpected: "true", present: true},
		{id: "addfields-phone", sourceExpected: "<absent>", localExpected: "<absent>", present: false},
		{id: "addfields-owner-name", sourceExpected: "<absent>", localExpected: "<absent>", present: false},
		{id: "dynamic-phone", sourceExpected: "<absent>", localExpected: "<absent>", present: false},
	})
	t.Log("reset boundary: this request observes static Name edit discard only; no post-reset dynamic field read, so VF59.1 newly referenced-field reacquisition is unobservable")
}

func TestStandardControllerRecordRootPreservesExpressionPrecedence(t *testing.T) {
	record := vm.Object("Account")
	record.Fields["Name"] = vm.String("record")
	standardController := vm.Object("ApexPages.StandardController")
	standardController.Fields["record"] = record
	ctx := &ExpressionContext{StandardController: standardController}

	assertRoot := func(want string) {
		t.Helper()
		got, err := EvaluateExpression("Account.Name", ctx)
		if err != nil || got != want {
			t.Fatalf("Account.Name = %q, %v; want %q", got, err, want)
		}
	}
	assertRoot("record")
	if _, ok := resolveRootValue(ctx, "Contact"); ok {
		t.Fatal("unrelated SObject root resolved through standard controller")
	}

	extension := vm.Object("Extension")
	extensionAccount := vm.Object("Account")
	extensionAccount.Fields["Name"] = vm.String("extension")
	extension.Fields["Account"] = extensionAccount
	ctx.Extensions = []vm.Value{extension}
	assertRoot("extension")
	ctx.Extensions = nil

	variableAccount := vm.Object("Account")
	variableAccount.Fields["Name"] = vm.String("variable")
	ctx.Variables = map[string]vm.Value{"Account": variableAccount}
	assertRoot("variable")
}

func TestStandardControllerRecordRootDoesNotMaskThrowingExtensionGetter(t *testing.T) {
	record := vm.Object("Account")
	record.Fields["Name"] = vm.String("record")
	standardController := vm.Object("ApexPages.StandardController")
	standardController.Fields["record"] = record
	throwingGetter, err := vm.CompileAnonymous(`throw new VisualforceException('extension getter failed');`)
	if err != nil {
		t.Fatal(err)
	}
	machine := vm.New(nil)
	if err := machine.RegisterClass(vm.Class{
		Name: "ThrowingAccountExtension",
		Methods: map[string]vm.Method{
			"getAccount": {
				Name: "ThrowingAccountExtension.getAccount", ClassName: "ThrowingAccountExtension",
				ReturnType: "Account", Program: throwingGetter,
			},
		},
	}); err != nil {
		t.Fatal(err)
	}
	ctx := &ExpressionContext{
		StandardController: standardController,
		Extensions:         []vm.Value{vm.Object("ThrowingAccountExtension")},
		VM:                 machine,
	}
	if _, err := EvaluateExpression("Account.Name", ctx); err == nil || !strings.Contains(err.Error(), "extension getter failed") {
		t.Fatalf("Account.Name error = %v, want extension getter failure", err)
	}
}

type standardControllerAddFieldsResetSelector struct {
	id             string
	sourceExpected string
	localExpected  string
	present        bool
}

func assertStandardControllerAddFieldsResetSelectors(t *testing.T, request string, actual map[string][]string, expected []standardControllerAddFieldsResetSelector) {
	t.Helper()
	for _, check := range expected {
		values := actual[check.id]
		localActual := "<absent>"
		if len(values) == 1 {
			localActual = values[0]
		}
		t.Logf("request=%s selector=#%s source_expected=%q local_actual=%q present=%t", request, check.id, check.sourceExpected, localActual, len(values) > 0)
		if check.present && len(values) != 1 {
			t.Errorf("request=%s selector=#%s occurrence count=%d, want exactly one", request, check.id, len(values))
		}
		if !check.present && len(values) != 0 {
			t.Errorf("request=%s selector=#%s unexpectedly present with values %q", request, check.id, values)
		}
		if check.present && localActual != check.localExpected {
			t.Errorf("request=%s selector=#%s local actual=%q, want staged local value %q (source expectation %q)", request, check.id, localActual, check.localExpected, check.sourceExpected)
		}
	}
}

func standardControllerAddFieldsResetSelectorValues(t *testing.T, rendered string) map[string][]string {
	t.Helper()
	doc, err := nethtml.Parse(strings.NewReader(rendered))
	if err != nil {
		t.Fatalf("parse rendered Visualforce HTML: %v", err)
	}
	values := make(map[string][]string)
	var appendText func(*strings.Builder, *nethtml.Node)
	appendText = func(builder *strings.Builder, node *nethtml.Node) {
		if node.Type == nethtml.TextNode {
			builder.WriteString(node.Data)
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			appendText(builder, child)
		}
	}
	var visit func(*nethtml.Node)
	visit = func(node *nethtml.Node) {
		if node.Type == nethtml.ElementNode && node.Data == "span" {
			for _, attr := range node.Attr {
				if attr.Key == "id" {
					var text strings.Builder
					appendText(&text, node)
					values[attr.Val] = append(values[attr.Val], strings.TrimSpace(text.String()))
					break
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return values
}
