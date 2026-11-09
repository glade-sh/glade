package apextest

import (
	"os"
	"path/filepath"
	"testing"
)

// standard_webstore_preserve_null/org.tsv:2-7 records unset, false and null
// inserts at APIs 62 and 67 and a later update to false for each.
func TestWebStorePreserveNullNativeRoutes(t *testing.T) {
	const body = `
 for (String mode : new List<String>{'unset','false','null'}) {
   WebStore s = new WebStore(Name='Null route '+mode);
   if (mode == 'false') s.put('OptionsPreserveGuestCartEnabled', false);
   if (mode == 'null') s.put('OptionsPreserveGuestCartEnabled', null);
   insert s;
   WebStore stored = [SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id=:s.Id];
   System.assertEquals(true, stored.OptionsPreserveGuestCartEnabled, mode+' insert');
   s.put('OptionsPreserveGuestCartEnabled', false);
   update s;
   stored = [SELECT OptionsPreserveGuestCartEnabled FROM WebStore WHERE Id=:s.Id];
   System.assertEquals(false, stored.OptionsPreserveGuestCartEnabled, mode+' update');
 }`
	definition, err := os.ReadFile("testdata/conformance/standard_webstore_options/definition.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"`+api+`","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(definition))
			runStandardFieldEvidenceRoutes(t, root, api, body)
		})
	}
}
