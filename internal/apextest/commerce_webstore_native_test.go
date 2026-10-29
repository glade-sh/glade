package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/vm"
)

// commerce_enabled_surface/org.tsv:9,12,20,23,30-31 captures the constructor,
// direct member read, positive/negative membership and Boolean describe at
// APIs 62/67. commerce_extension_shapes/org.tsv's webstore_fields and
// webstore_null_boundary rows add direct assignment, dynamic put/get and
// null-to-false coercion. The shared helper repeats anonymous, named and @IsTest
// execution through cold and warm project runners. Source-only projects use
// the supported catalog as a compatibility fallback for an unspecified org.
func TestCommerceWebStoreNativeProjectRoutes(t *testing.T) {
	const body = `
WebStore store = new WebStore(Name='Owned commerce store', OptionsAutoFacetingEnabled=true);
System.assertEquals(true, store.OptionsAutoFacetingEnabled);
System.assertEquals('Owned commerce store', store.Name);
store.OptionsAutoFacetingEnabled = false;
System.assertEquals(false, store.OptionsAutoFacetingEnabled, 'direct false assignment');
store.put('OptionsAutoFacetingEnabled', true);
System.assertEquals(true, store.get('OptionsAutoFacetingEnabled'));
Map<String,Schema.SObjectField> fields = WebStore.SObjectType.getDescribe().fields.getMap();
System.assertEquals(true, fields.containsKey('OptionsAutoFacetingEnabled'));
System.assertEquals(false, fields.containsKey('OptionsSyntheticMissing'), 'missing field membership');
Schema.DescribeFieldResult field = fields.get('OptionsAutoFacetingEnabled').getDescribe();
System.assertEquals('BOOLEAN', String.valueOf(field.getType()));
System.assertEquals(true, field.isCreateable());
System.assertEquals(true, field.isUpdateable());
System.assertEquals(false, field.isNillable(), 'Boolean nillability');
WebStore nullBoundary = new WebStore(OptionsAutoFacetingEnabled=null);
System.assertEquals(false, nullBoundary.OptionsAutoFacetingEnabled, 'explicit null Boolean read');
`
	for _, api := range []string{"62.0", "67.0"} {
		for _, shape := range []string{"source-only", "commerce"} {
			t.Run(api+"/"+shape, func(t *testing.T) {
				root := t.TempDir()
				writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
				if shape == "commerce" {
					writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer","features":["B2BCommerce","Communities"],"settings":{"communitiesSettings":{"enableNetworksEnabled":true},"commerceSettings":{"commerceEnabled":true}}}`)
				}
				runStandardFieldEvidenceRoutes(t, root, api, body)
			})
		}
	}
}

// commerce_extension_shapes/org.tsv:50 observes this direct read at API62.
// Retain value/registration diagnostics so a project-linked representation
// cannot silently diverge from the lower-level SObject route.
func TestCommerceWebStoreNativeNullProjectValue(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	writeFile(t, filepath.Join(root, "force-app/main/default/classes/OwnedCommerceNull.cls"), `public class OwnedCommerceNull { public static Boolean observe(){ WebStore store=new WebStore(OptionsAutoFacetingEnabled=null); return store.OptionsAutoFacetingEnabled; } }`)
	index := loadTestIndex(t, root)
	org := orgFromIndex(index)
	runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
	machine := runner.newMachine()
	program, err := vm.CompileAnonymousWithOptions(`WebStore store=new WebStore(OptionsAutoFacetingEnabled=null); Boolean observed=store.OptionsAutoFacetingEnabled;`, vm.CompileOptions{APIVersion: "62.0"})
	if err != nil {
		t.Fatal(err)
	}
	result, err := machine.Execute(program)
	got := result.Vars["observed"]
	if err != nil || got.Kind != vm.ValueBool || got.Bool {
		class, registered := machine.Classes["WebStore"]
		object, installed := machine.Org.Objects["WebStore"]
		_, rawField := object.Definition.Fields["OptionsAutoFacetingEnabled"]
		t.Fatalf("native false read=%#v err=%v; receiver=%#v; installed=%t rawField=%t registered=%t platform=%t source=%q dependency=%t super=%q", got, err, result.Vars["store"], installed, rawField, registered, class.PlatformType, class.SourceUnit, class.Dependency, class.SuperClass)
	}
}
