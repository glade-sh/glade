package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Packet108731B/SHA7693ce9552c378d68f0d36337568272cdef8c738f14eff24536f3c148cb15748.
// DescribeTabResult catalog586/member6; fully qualified viewing URL clause127-133.
// Primary5748B/SHA8a32940771ffb8572d9ac6fff03ac5e9bd9572d66dcbf5aeafb854bc18649a17.
// Program665B/SHA08f171397162f8d2d07b2b478534b8064a501c4f58ebf07b09bf1d3dee2bee12.
// Explicit controlled profile: synthetic user005000000000001 is stipulated
// to have the Sales app and its standard Account tab available. This one
// producer result models that admitted profile; it does not test filtering.
// Build tab/app values through current constructors after binding fresh org
// and user. Seed only the producer result; never inject url/mobileUrl fields.
// Default existing origin provider; no hostname/path/mobile URL oracle.
// Three URL, two public-tab and one construction assertions; reach unknown.
// No native availability/provenance, lower API, full-family or AC7 credit.
func TestExecDescribeTabViewingURLAPI67(t *testing.T) {
	program, err := CompileAnonymousWithOptions(`Schema.DescribeTabResult selected = null;
for (Schema.DescribeTabSetResult app : Schema.describeTabs()) {
    for (Schema.DescribeTabResult tab : app.getTabs()) {
        if (tab.getSobjectName() == 'Account' && !tab.isCustom()) selected = tab;
    }
}
System.assert(selected != null);
System.assertEquals('Account', selected.getSobjectName());
System.assertEquals(false, selected.isCustom());
String viewingUrl = selected.getUrl();
System.assert(viewingUrl != null);
Integer separator = viewingUrl.indexOf('://');
System.assert(separator > 0);
String authority = viewingUrl.substring(separator + 3).substringBefore('/');
System.assert(!String.isBlank(authority));
`, CompileOptions{APIVersion: "67.0"})
	if err != nil {
		t.Fatal(err)
	}
	if program.APIVersion != "67.0" {
		t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
	}

	org := testDataOrg()
	org.APIVersion = "67.0"
	machine := New(nil)
	machine.SetOrg(&org)
	machine.SetCurrentUser(storage.Record{
		ID:     "005000000000001",
		Object: "User",
		Fields: map[string]storage.Value{"Id": storage.IDValue("005000000000001")},
	})

	accountTab := machine.describeTabValue(storage.TabMetadata{
		Name:        "Accounts",
		Label:       "Accounts",
		SObjectName: "Account",
		Custom:      false,
	})
	salesApp := describeTabSetValue(describeTabSetTemplate{
		Name:      "Sales",
		Label:     "Sales",
		Namespace: "standard",
		Selected:  true,
	}, []Value{accountTab})
	availableApps := List(salesApp)
	machine.describeTabsCache = &availableApps

	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// Source-only construction preservation; zero Salesforce behavior credit.
func TestDescribeTabViewingURLPreservesOriginAndMobile(t *testing.T) {
	cases := []struct {
		name, configuredOrigin, wantOrigin string
	}{
		{name: "defaultOrigin", wantOrigin: "https://local.glade.example"},
		{name: "configuredOrigin", configuredOrigin: "https://trail.example.test:8443/", wantOrigin: "https://trail.example.test:8443"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			machine := New(nil)
			if tc.configuredOrigin != "" {
				machine.SetServerBaseURL(tc.configuredOrigin)
			}
			tab := machine.describeTabValue(storage.TabMetadata{Name: "Accounts", Label: "Accounts", SObjectName: "Account", Custom: false})
			// Schema describe R233: viewing URLs contain the object key prefix.
			const relativeURL = "/lightning/o/Accounts/list"
			viewing := tab.Fields["url"]
			if viewing.Kind != ValueString || viewing.Text != tc.wantOrigin+"/001/o" {
				t.Fatalf("viewing URL = %#v, want %q", viewing, tc.wantOrigin+"/001/o")
			}
			mobile := tab.Fields["mobileUrl"]
			if mobile.Kind != ValueString || mobile.Text != relativeURL {
				t.Fatalf("preserved mobile URL = %#v, want %q", mobile, relativeURL)
			}
		})
	}
}
