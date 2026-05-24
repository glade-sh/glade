package storage

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestStressSQLiteLargeFixtureRoundTrip(t *testing.T) {
	const records = 1200
	store, err := OpenSQLite(filepath.Join(t.TempDir(), "glade.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	org := benchmarkFixtureOrg()
	fixture := benchmarkFixture(records)
	for i := range fixture.Objects[0].Records {
		fixture.Objects[0].Records[i].Fields["External_Key__c"] = StringValue(fmt.Sprintf("EXT-%04d", i))
	}
	account := org.Objects["Account"]
	account.Definition.Fields["External_Key__c"] = Field{APIName: "External_Key__c", Type: FieldString, ExternalID: true, Unique: true}
	org.Objects["Account"] = account
	if err := ApplyFixture(&org, fixture); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := len(loaded.Objects["Account"].Records); got != records {
		t.Fatalf("loaded Account records = %d", got)
	}
	exported := FixtureFromOrg(loaded)
	reimported := benchmarkFixtureOrg()
	reimportAccount := reimported.Objects["Account"]
	reimportAccount.Definition.Fields["External_Key__c"] = account.Definition.Fields["External_Key__c"]
	reimported.Objects["Account"] = reimportAccount
	if err := ApplyFixture(&reimported, exported); err != nil {
		t.Fatal(err)
	}
	if got := len(reimported.Objects["Account"].Records); got != records {
		t.Fatalf("reimported Account records = %d", got)
	}
}
