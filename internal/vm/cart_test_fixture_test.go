package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func cartFixtureStorage(t *testing.T) storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	for _, name := range []string{"Account", "WebStore", "WebCart"} {
		storage.EnsureStandardObject(&org, name)
		object := org.Objects[name]
		object.Records = map[storage.ID]storage.Record{}
		org.Objects[name] = object
	}
	return org
}

func createStorageCartFixture(t *testing.T, org *storage.OrgState) *VM {
	t.Helper()
	machine := New(nil)
	machine.SetOrg(org)
	machine.EnableTestContext()
	if _, handled := machine.cartTestFixture("createCart", []Value{{Kind: ValueObject, Type: "CartExtension.WebStoreTypeEnum", Text: "B2CE"}}); !handled {
		t.Fatal("cart fixture was not created")
	}
	return machine
}

func TestCartFixturePreservesSharedSnapshot(t *testing.T) {
	org := cartFixtureStorage(t)
	account := org.Objects["Account"]
	account.Records["001000000000001"] = storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Original")}}
	org.Objects["Account"] = account
	before := storage.SnapshotRuntimeOrg(&org)
	createStorageCartFixture(t, &org)
	if len(before.Objects["Account"].Records) != 1 {
		t.Fatal("cart creation changed a shared snapshot")
	}
}

func TestCartFixtureAppearsInIndexedQuery(t *testing.T) {
	org := cartFixtureStorage(t)
	cart := org.Objects["WebCart"]
	cart.Definition.Indexes = []storage.IndexDefinition{{Name: "WebCart.Name", Object: "WebCart", Fields: []string{"Name"}}}
	org.Objects["WebCart"] = cart
	storage.RebuildIndexes(&org)
	machine := createStorageCartFixture(t, &org)
	program, err := CompileAnonymous("System.assertEquals(1, [SELECT COUNT() FROM WebCart WHERE Name='My Cart']);")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
