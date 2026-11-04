package vm

import (
	"runtime"
	"testing"
)

// The legacy rendering snapshot uses an explicitly configured fixture label.
// This local fixture contract is independent of the native catalog label.
func TestDescribeShareFrozenFixtureLabel(t *testing.T) {
	fields := describeShareFieldMapOf(t, describeShareUnprimed(), "Account")
	token := fields.Map[mapKey(String("DandbCompanyId"))]
	if got := token.Fields["label"].Text; got != "DandbCompanyId" {
		t.Fatalf("fixture label = %q; want DandbCompanyId", got)
	}
}

// Catalog changes can alter rendered metadata. Sharing must preserve the
// complete current rendering, including on a cache hit in another clone.
func TestDescribeShareCatalogRenderingEquivalence(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"account", `System.debug('S2:' + Schema.describeSObjects(new List<String>{'Account'})[0]); System.debug('S3:' + Schema.SObjectType.Account.fields);`},
		{"contact", `System.debug('S2:' + Schema.describeSObjects(new List<String>{'Contact'})[0]); System.debug('S3:' + Schema.SObjectType.Contact.fields);`},
		{"custom", `System.debug('S2:' + Widget__c.SObjectType.getDescribe()); System.debug('S3:' + Schema.SObjectType.Widget__c.fields);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := describeShareRow{name: tc.name, source: tc.source}
			org := describeShareOrgWithLegacyLabel(false)
			unshared := New(nil)
			unshared.SetOrg(&org)
			unshared.EnableTestContext()
			want, wantErr := describeShareRun(t, unshared, row)
			if wantErr != "" {
				t.Fatal(wantErr)
			}
			base, template := describeSharePrimedBaseWithOrg(t, describeShareOrgWithLegacyLabel(false))
			first := describeShareClone(base, template)
			for _, mode := range []string{"cold", "warm"} {
				machine := first
				if mode == "warm" {
					machine = describeShareClone(base, template)
				}
				got, errText := describeShareRun(t, machine, row)
				if errText != "" || got != want {
					t.Errorf("%s: got %q error %q; unshared %q", mode, got, errText, want)
				}
			}
			runtime.KeepAlive(first)
		})
	}
}
