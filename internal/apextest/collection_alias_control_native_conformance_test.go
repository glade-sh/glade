package apextest

import "testing"

func TestCollectionAliasControlNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "collection_alias_control", "NoteCollectorTest", []string{"67.0"}, 4)
}
