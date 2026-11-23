package apextest

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A subclass of a project subclass of CheckoutPlaceOrder that reaches the
// platform validate through its parent fails like the direct subclass rows in
// testdata/conformance/commerce_place_order_super_validate/org.tsv (NullPointerException).
func TestCommercePlaceOrderSuperValidateTwoLevelSubclass(t *testing.T) {
	const fixture = "testdata/conformance/commerce_place_order_super_validate"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			body, err := os.ReadFile(filepath.Join(fixture, "project-scratch-def.json"))
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "config/project-scratch-def.json"), string(body))
			err = filepath.WalkDir(filepath.Join(fixture, "project/force-app"), func(path string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				name, _ := filepath.Rel(filepath.Join(fixture, "project"), path)
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(data), "__API__", api))
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/R5PoDeep.cls"), `@IsTest private class R5PoDeep {
  public virtual class Mid extends CartExtension.CheckoutPlaceOrder {}
  public class Deep extends Mid {
    public virtual override CartExtension.PlaceOrderResponse validate(CartExtension.PlaceOrderRequest r, List<String> d) { return super.validate(r, d); }
  }
  @IsTest static void deep() {
    Account a = new Account(Name = 'acc'); insert a;
    WebStore ws = new WebStore(Name = 'ws'); insert ws;
    WebCart wc = new WebCart(Name = 'cart', WebStoreId = ws.Id, AccountId = a.Id); insert wc;
    String msg;
    try { new Deep().validate(new CartExtension.PlaceOrderRequest(CartExtension.CartTestUtil.getCart(wc.Id)), new List<String>{'PRICING'}); msg = 'ok'; }
    catch (Exception e) { msg = e.getTypeName() + '|' + e.getMessage(); }
    System.assert(false, 'R5|deep|' + msg);
  }
}`)
			writeFile(t, filepath.Join(root, "force-app/main/default/classes/R5PoDeep.cls-meta.xml"), `<?xml version="1.0" encoding="UTF-8"?><ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			index := loadTestIndex(t, root)
			run := Run(index, Options{SelectedClasses: []string{"R5PoDeep"}, SelectedMethod: "deep", NoDiskCache: true, Parallelism: 1})
			if run.Summary().Total != 1 || len(run.Suites[0].Cases) != 1 || run.Suites[0].Cases[0].Problem == nil {
				t.Fatalf("unexpected selection: %+v", run.Summary())
			}
			want := "R5|deep|System.NullPointerException|Attempt to de-reference a null object"
			if got := run.Suites[0].Cases[0].Problem.Message; !strings.Contains(got, want) {
				t.Fatalf("NATIVE commerce_place_order_super_validate/org.tsv %s istest (two-level subclass): want %q; Glade %q", api, want, got)
			}
		})
	}
}
