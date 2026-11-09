package project

import "testing"

func TestSuggestOrgSetupNetworkFieldReferences(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		body string
		want bool
	}{
		{"network-import", "force-app/c/lwc/example/example.js", "import FIELD from '@salesforce/schema/Network.Name';", true},
		{"member-import", "force-app/c/lwc/example/example.js", "import FIELD from '@salesforce/schema/NetworkMember.MemberId';", true},
		{"network-metadata", "force-app/c/permissionsets/Example.permissionset-meta.xml", "<PermissionSet><fieldPermissions><field>Network.Name</field></fieldPermissions></PermissionSet>", true},
		{"unrelated", "force-app/c/lwc/example/example.js", "import FIELD from '@salesforce/schema/Account.Name';", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := suggestedFeatures(t, map[string]string{tc.file: tc.body})
			_, present := got["NetworksEnabled"]
			if present != tc.want {
				t.Fatalf("NetworksEnabled present=%t, want %t; suggestions=%+v", present, tc.want, got)
			}
		})
	}
}
