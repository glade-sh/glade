package storage

import (
	"path/filepath"
	"testing"
)

func TestSQLiteStorePersistsOrgState(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oaer.db")
	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	org := NewOrgState()
	org.OrgID = "00D000000000001"
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{APIName: "Account", KeyPrefix: "001", Fields: map[string]Field{"Name": {APIName: "Name", Type: FieldString}}},
		Records: map[ID]Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]Value{"Name": StringValue("Acme")}},
		},
	}
	org.IDSequences["Account"] = 1
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.OrgID != org.OrgID || loaded.Objects["Account"].Records["001000000000001"].Fields["Name"].String != "Acme" {
		t.Fatalf("loaded = %#v", loaded)
	}
	summary, err := store.Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if summary.SchemaVersion != sqliteSchemaVersion || summary.Objects != 1 || summary.Records != 1 || summary.ByObject["Account"] != 1 {
		t.Fatalf("summary = %#v", summary)
	}
}

func TestSQLiteStoreAppliesMigrations(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oaer.db")
	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	version, err := store.SchemaVersion()
	if err != nil {
		t.Fatal(err)
	}
	if version != sqliteSchemaVersion {
		t.Fatalf("schema version = %d", version)
	}
	var applied int
	if err := store.db.QueryRow(`select count(*) from schema_migrations where version = ?`, sqliteSchemaVersion).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("applied migrations = %d", applied)
	}
	for _, table := range []string{"org_meta", "object_definitions", "records", "id_sequences"} {
		var name string
		if err := store.db.QueryRow(`select name from sqlite_master where type = 'table' and name = ?`, table).Scan(&name); err != nil {
			t.Fatalf("table %s missing: %v", table, err)
		}
	}
}
