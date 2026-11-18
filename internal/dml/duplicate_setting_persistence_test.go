package dml_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func TestLegacySeededDuplicateRuleEnforcementAfterSQLiteReload(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "")
	for _, enabled := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "on"}[enabled], func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "org.db")
			store, err := storage.OpenSQLite(path)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			if err := store.Save(org); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			db, err := sql.Open("sqlite", path)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`update object_definitions set definition_json = replace(cast(definition_json as text), '"seeded":true,', '')`); err != nil {
				t.Fatal(err)
			}
			if _, err := db.Exec(`delete from org_meta where key = ?`, "glade.org.duplicateRuleOrigins"); err != nil {
				t.Fatal(err)
			}
			if err := db.Close(); err != nil {
				t.Fatal(err)
			}
			store, err = storage.OpenSQLite(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			loaded, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			storage.EnsureDeterministicPlatformData(&loaded)
			loaded.StandardDuplicateRules = enabled
			engine := dml.NewEngine(&loaded)
			engine.Options.EnforceDuplicateRules = true
			first, second := insertTestAccounts(engine)
			if !first.Success || blocked(second) != enabled || (!enabled && !second.Success) {
				t.Fatalf("enabled %v: %+v %+v", enabled, first, second)
			}
		})
	}
}
