package sema

import "testing"

// The v0.3.3 parent rejects these platform type references in every local
// declaration position. Preserve that local contract unless native rows cover
// a change to compound declarations.
func TestQualifiedLocalCompoundMatchesShippingParent(t *testing.T) {
	a := &Analyzer{known: map[string]TypeReference{"system": {}}}
	model := &semaTypeMemberView{
		state: &semaTypeMemberState{},
		current: map[string]typeMembers{
			"system.string":  {name: "System.String", platform: true},
			"system.integer": {name: "System.Integer", platform: true},
			"system.list":    {name: "System.List", platform: true},
		},
	}
	for _, name := range []string{"System.String", "System.Integer", "System.List"} {
		t.Run(name, func(t *testing.T) {
			if !a.qualifiedLifecycleLocalTypeRejected("Example", name, model) {
				t.Errorf("shipping parent rejected %s in compound local declarations", name)
			}
		})
	}
}
