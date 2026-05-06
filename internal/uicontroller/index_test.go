package uicontroller

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/open-aer/oaer/internal/project"
	"github.com/open-aer/oaer/internal/schema"
	"github.com/open-aer/oaer/internal/typesys"
)

func TestBuildExtractsAuraControllerReferences(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WidgetController.cls"), `public class WidgetController {
  @AuraEnabled public static String save(Id recordId) { return 'ok'; }
  @AuraEnabled public static String load() { return 'loaded'; }
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/Widget.cmp"), `<aura:component controller="WidgetController">
  <c:lineItem value="{!v.item}" />
  <lightning:button label="Save" press="{!c.save}" />
</aura:component>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/Widget.app"), `<aura:application><c:lineItem /></aura:application>`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/Widget.evt"), `<aura:event type="APPLICATION" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/Widget.design"), `<design:component label="Widget" />`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/WidgetController.js"), `({
  load: function(component) {
    component.get("c.load");
  }
})`)
	writeFile(t, filepath.Join(root, "force-app/main/default/aura/Widget/WidgetHelper.js"), `({
  save: function(cmp) {
    cmp.get('c.save');
  }
})`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	apex := typesys.Build(p, schema.Schema{})
	idx, err := Build(p, apex)
	if err != nil {
		t.Fatal(err)
	}

	if len(idx.AuraBundles) != 1 {
		t.Fatalf("aura bundles = %#v", idx.AuraBundles)
	}
	bundle := idx.AuraBundles[0]
	if bundle.Name != "Widget" || len(bundle.Files) != 6 {
		t.Fatalf("bundle summary = %#v", bundle)
	}
	if len(bundle.ControllerReferences) != 1 || bundle.ControllerReferences[0].Name != "WidgetController" {
		t.Fatalf("controllers = %#v", bundle.ControllerReferences)
	}
	if !hasAuraComponent(bundle, "c", "lineItem") || !hasAuraComponent(bundle, "lightning", "button") {
		t.Fatalf("component refs = %#v", bundle.ComponentReferences)
	}
	if !hasAuraAction(bundle, "save", true) || !hasAuraAction(bundle, "load", true) {
		t.Fatalf("action refs = %#v", bundle.ActionReferences)
	}
	if len(idx.ApexMethods) < 2 {
		t.Fatalf("apex methods = %#v", idx.ApexMethods)
	}
}

func TestBuildExtractsLWCImportsWiresAndReactiveParameters(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/WidgetController.cls"), `public class WidgetController {
  @AuraEnabled(cacheable=true) public static String getWidget(String accountId) { return 'ok'; }
  @AuraEnabled public static void saveWidget(String accountId) {}
}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/lwc/widget/widget.js"), `import { LightningElement, wire } from 'lwc';
import getWidget from '@salesforce/apex/WidgetController.getWidget';
import saveWidget from '@salesforce/apex/WidgetController.saveWidget';
import Save from '@salesforce/label/c.Save';
import RES from '@salesforce/resourceUrl/WidgetAssets';
import ACCOUNT_NAME from '@salesforce/schema/Account.Name';
import { NavigationMixin } from 'lightning/navigation';
import { getRecord, updateRecord } from 'lightning/uiRecordApi';
import { getObjectInfo } from 'lightning/uiObjectInfoApi';
import child from 'c/child';

export default class Widget extends LightningElement {
  accountId;
  @wire(getWidget, { accountId: '$accountId', nested: '$filters.term' }) widget;
  @wire(getRecord, { recordId: '$accountId', fields: [ACCOUNT_NAME] }) record;
}`)

	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	apex := typesys.Build(p, schema.Schema{})
	idx, err := Build(p, apex)
	if err != nil {
		t.Fatal(err)
	}
	if len(idx.LWCBundles) != 1 {
		t.Fatalf("lwc bundles = %#v", idx.LWCBundles)
	}
	bundle := idx.LWCBundles[0]
	for _, want := range []string{"apex", "label", "resourceUrl", "schema", "lightning", "local"} {
		if !hasLWCImportKind(bundle, want) {
			t.Fatalf("missing import kind %s in %#v", want, bundle.Imports)
		}
	}
	if !hasLWCImport(bundle, "getWidget", "apex", "WidgetController", "getWidget") ||
		!hasLWCImport(bundle, "child", "local", "", "") {
		t.Fatalf("imports = %#v", bundle.Imports)
	}
	if len(bundle.Wires) != 2 {
		t.Fatalf("wires = %#v", bundle.Wires)
	}
	wire := findWire(bundle, "getWidget")
	if wire == nil || wire.AdapterKind != "apex" || wire.ApexClassName != "WidgetController" || wire.ApexMethodName != "getWidget" {
		t.Fatalf("apex wire = %#v", wire)
	}
	if !hasReactive(*wire, "accountId") || !hasReactive(*wire, "filters.term") {
		t.Fatalf("reactive params = %#v", wire.ReactiveParameters)
	}
	recordWire := findWire(bundle, "getRecord")
	if recordWire == nil || recordWire.AdapterKind != "lightning" || !hasReactive(*recordWire, "accountId") {
		t.Fatalf("record wire = %#v", recordWire)
	}
	if !hasResolvedMethod(idx, "WidgetController", "getWidget") || !hasResolvedMethod(idx, "WidgetController", "saveWidget") {
		t.Fatalf("apex methods = %#v", idx.ApexMethods)
	}
}

func hasAuraComponent(bundle AuraBundle, namespace, name string) bool {
	for _, ref := range bundle.ComponentReferences {
		if ref.Namespace == namespace && ref.Name == name {
			return true
		}
	}
	return false
}

func hasAuraAction(bundle AuraBundle, name string, resolved bool) bool {
	for _, ref := range bundle.ActionReferences {
		if ref.Name == name && ref.Resolved == resolved {
			return true
		}
	}
	return false
}

func hasLWCImportKind(bundle LWCBundle, kind string) bool {
	for _, imp := range bundle.Imports {
		if imp.Kind == kind {
			return true
		}
	}
	return false
}

func hasLWCImport(bundle LWCBundle, local, kind, className, method string) bool {
	for _, imp := range bundle.Imports {
		if imp.LocalName == local && imp.Kind == kind && imp.ClassName == className && imp.MethodName == method {
			return true
		}
	}
	return false
}

func findWire(bundle LWCBundle, adapter string) *LWCWire {
	for i := range bundle.Wires {
		if bundle.Wires[i].Adapter == adapter {
			return &bundle.Wires[i]
		}
	}
	return nil
}

func hasReactive(wire LWCWire, name string) bool {
	for _, param := range wire.ReactiveParameters {
		if param == name {
			return true
		}
	}
	return false
}

func hasResolvedMethod(idx Index, className, method string) bool {
	for _, ref := range idx.ApexMethods {
		if ref.ClassName == className && ref.MethodName == method && ref.Resolved {
			return true
		}
	}
	return false
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
