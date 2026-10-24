package storage

import "testing"

func TestNetworkConfigurationEmptySeedLifecycleRoutes(t *testing.T) {
	for _, route := range []string{"default", "Sites", "Communities"} {
		t.Run(route, func(t *testing.T) {
			org := NewOrgState()
			EnsureDeterministicPlatformData(&org)
			if route != "default" {
				ApplyOrgShape(&org, []string{route})
			}
			if len(org.Objects["Network"].Records) != 0 || len(org.Objects["NetworkMember"].Records) != 0 {
				t.Fatal("fresh org must not contain synthetic Network records or members")
			}
			if route == "Communities" {
				state := org.Objects["Network"]
				if state.Definition.APIName != "Network" {
					t.Fatal("enabled Network schema missing")
				}
				state.Records = map[ID]Record{"0DB000000000099": {ID: "0DB000000000099", Object: "Network", Fields: map[string]Value{"Name": StringValue("Synthetic Portal")}}}
				org.Objects["Network"] = state
				EnsureDeterministicPlatformData(&org)
				ApplyOrgShape(&org, []string{"Communities"})
				if len(org.Objects["Network"].Records) != 1 {
					t.Fatal("repeated shape application changed deployed Networks")
				}
			} else if _, ok := org.Objects["Network"]; ok {
				t.Fatal("disabled Network schema must be absent")
			}
		})
	}
}
