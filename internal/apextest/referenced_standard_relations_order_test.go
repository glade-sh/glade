package apextest

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Project-inferred lookups arrive in a map. The relations they add to a
// standard object must keep a stable order across identical builds.
func TestApplyReferencedStandardFieldSetRelationsOrderIsStable(t *testing.T) {
	reference := func(name, parent string) storage.Field {
		return storage.Field{APIName: name + "Id", Type: storage.FieldReference, ReferenceTo: []string{parent}, RelationshipName: name}
	}
	build := func() string {
		org := storage.NewOrgState()
		org.Objects["Widget"] = storage.ObjectState{Definition: storage.ObjectDefinition{
			APIName: "Widget",
			Fields:  map[string]storage.Field{"Id": {APIName: "Id", Type: storage.FieldID}},
		}}
		applyReferencedStandardFieldSet(&org, map[string]map[string]storage.Field{
			"Widget": {
				"AlphaId":   reference("Alpha", "Account"),
				"BravoId":   reference("Bravo", "Contact"),
				"CharlieId": reference("Charlie", "Lead"),
				"DeltaId":   reference("Delta", "Case"),
				"EchoId":    reference("Echo", "User"),
			},
		}, map[string]struct{}{})
		order := make([]string, 0, 5)
		for _, relation := range org.Objects["Widget"].Definition.Relations {
			order = append(order, relation.Field)
		}
		return strings.Join(order, ",")
	}
	want := build()
	for i := 1; i < 20; i++ {
		if got := build(); got != want {
			t.Fatalf("build %d relations order:\n got %s\nwant %s", i, got, want)
		}
	}
}
