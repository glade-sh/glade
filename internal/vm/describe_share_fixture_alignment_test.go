package vm

import "testing"

func TestDescribeShareNamespacedFixtureUsesSameAccountIDLabel(t *testing.T) {
	plain := describeShareOrg().Objects["Account"].Definition.Fields["Id"].Label
	namespaced := describeShareNamespacedOrg().Objects["Account"].Definition.Fields["Id"].Label
	if plain != namespaced {
		t.Fatalf("fixture Id labels differ: plain=%q namespaced=%q", plain, namespaced)
	}
}
