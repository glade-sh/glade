package storage

import "testing"

func TestFlowDeclarationsCloneIsolation(t *testing.T) {
	for name, clone := range map[string]func(OrgState) OrgState{
		"clone":             OrgState.Clone,
		"runtime":           OrgState.CloneRuntime,
		"frozen_definition": OrgState.CloneRuntimeFrozenDefinition,
		"rollback":          OrgState.CloneRollbackSnapshot,
	} {
		t.Run(name, func(t *testing.T) {
			org := NewOrgState()
			org.Metadata.FlowDeclarations = []FlowRule{{Name: "OwnedScreen", Variables: []FlowVariable{{Name: "recordId", DataType: "String", IsInput: true}}}}
			copied := clone(org)
			copied.Metadata.FlowDeclarations[0].Variables[0].Name = "changed"
			if org.Metadata.FlowDeclarations[0].Variables[0].Name != "recordId" {
				t.Fatal("clone mutated original declaration")
			}
		})
	}
}
