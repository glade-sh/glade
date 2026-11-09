package apextest

import (
	"path/filepath"
	"testing"
)

// A definition kept under config/ that nothing names may let a class compile
// against the objects it enables, but the test org follows the named
// definition: Networks and MultiCurrency stay off at run time.
func TestUnreferencedScratchDefinitionsKeepRuntimeShapeOfNamedDefinition(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true,"definitionFile":"config/base-scratch-def.json"}]}`)
	writeFile(t, filepath.Join(root, "config/base-scratch-def.json"), `{"edition":"Developer","settings":{"communitiesSettings":{"enableNetworksEnabled":false}}}`)
	writeFile(t, filepath.Join(root, "config/advanced-scratch-def.json"), `{"edition":"Developer","features":["MultiCurrency","Communities"],"settings":{"currencySettings":{"enableMultiCurrency":true},"communitiesSettings":{"enableNetworksEnabled":true}}}`)
	body := `@IsTest private class UnrefShapeTest {
	@IsTest static void probe() {
		// Compiles because an unreferenced definition enables Networks.
		List<AccountBrand> brands;
		System.assertEquals(false, Schema.SObjectType.Opportunity.fields.getMap().containsKey('CurrencyIsoCode'), 'MultiCurrency must stay off');
		Boolean denied = false;
		try { Database.query('SELECT Id FROM Network LIMIT 1'); } catch (QueryException ex) { denied = true; }
		System.assert(denied, 'Network must stay unavailable at run time');
	}
}`
	path := filepath.Join(root, "force-app/main/default/classes/UnrefShapeTest.cls")
	writeFile(t, path, body)
	writeFile(t, path+"-meta.xml", "<ApexClass><apiVersion>67.0</apiVersion></ApexClass>")
	index := loadTestIndex(t, root)
	if index.HasErrors() {
		t.Fatalf("index: %v", index.Diagnostics)
	}
	result := Run(index, Options{SelectedClasses: []string{"UnrefShapeTest"}, SelectedMethod: "probe", NoDiskCache: true, Parallelism: 1})
	if len(result.Suites) != 1 || len(result.Suites[0].Cases) != 1 || result.Suites[0].Cases[0].Problem != nil {
		t.Fatalf("test runner: %+v %+v", result.Summary(), result.Suites)
	}
}
