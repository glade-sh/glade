package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func childIndexBenchOrg(parents, perParent int) storage.OrgState {
	org := storage.NewOrgState()
	accounts := map[storage.ID]storage.Record{}
	contacts := map[storage.ID]storage.Record{}
	for i := 0; i < parents; i++ {
		aid := storage.ID(fmt.Sprintf("001%012d", i))
		accounts[aid] = storage.Record{ID: aid, Object: "Account"}
		for j := 0; j < perParent; j++ {
			cid := storage.ID(fmt.Sprintf("003%06d%06d", i, j))
			contacts[cid] = storage.Record{ID: cid, Object: "Contact", Fields: map[string]storage.Value{"AccountId": storage.IDValue(aid)}}
		}
	}
	org.Objects["Account"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Account"}, Records: accounts}
	org.Objects["Contact"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contact",
		Fields:    map[string]storage.Field{"AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}},
		Relations: []storage.Relationship{{Field: "AccountId", ParentObjects: []string{"Account"}, ParentRelationship: "Account", ChildRelationship: "Contacts"}},
	}, Records: contacts}
	return org
}

func BenchmarkChildRelationshipIndexBuild(b *testing.B) {
	org := childIndexBenchOrg(200, 3)
	obj := org.Objects["Contact"]
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		childRelationshipIndex(org, "Contact", obj, "AccountId", nil)
	}
}
