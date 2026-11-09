package project

import "testing"

func TestSuggestOrgSetupNetworkSchemaReferences(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		want       bool
	}{
		{"network-token", `class A { Object x = Schema.SObjectType.Network; }`, true},
		{"member-token", `class A { Object x = Schema.SObjectType.NetworkMember; }`, true},
		{"member-describe", `class A { Object x = NetworkMember.SObjectType.getDescribe(); }`, true},
		{"dynamic-describe", `class A { Object x = Schema.getGlobalDescribe().get('Network'); }`, true},
		{"comment", `class A { /* Schema.SObjectType.Network */ }`, false},
		{"other", `class A { Object x = Schema.SObjectType.Account; }`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := suggestedFeatures(t, map[string]string{"force-app/c/classes/A.cls": tc.body})
			_, present := got["NetworksEnabled"]
			if present != tc.want {
				t.Fatalf("NetworksEnabled present=%t, want %t; suggestions=%+v", present, tc.want, got)
			}
		})
	}
}
