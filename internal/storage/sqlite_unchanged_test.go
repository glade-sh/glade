package storage

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func TestSQLiteSaveIfChanged(t *testing.T) {
	path := filepath.Join(t.TempDir(), "org.db")
	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	org := benchmarkFixtureOrg()
	if err := ApplyFixture(&org, benchmarkFixture(3)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	baseline, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	// An unchanged save must not invoke even a single DELETE.
	if _, err := store.db.Exec(`create trigger prohibit_delete before delete on records begin select raise(abort, 'write'); end`); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveIfChanged(org); err != nil {
		t.Fatalf("unchanged: %v", err)
	}
	// A changed sequence, metadata, definition, or record must still save and fail
	// atomically if the database rejects the rewrite.
	for name, mutate := range map[string]func(*OrgState){
		"sequence":   func(o *OrgState) { o.IDSequences["Account"]++ },
		"metadata":   func(o *OrgState) { o.Metadata.DataCategoryGroups = []DataCategoryGroup{{Name: "Changed"}} },
		"definition": func(o *OrgState) { x := o.Objects["Account"]; x.Definition.Label = "Changed"; o.Objects["Account"] = x },
		"record": func(o *OrgState) {
			x := o.Objects["Account"]
			for id, rec := range x.Records {
				rec.Fields["Name"] = StringValue("Changed")
				x.Records[id] = rec
				break
			}
			o.Objects["Account"] = x
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := org.Clone()
			mutate(&changed)
			if err := store.SaveIfChanged(changed); err == nil {
				t.Fatal("changed save succeeded despite rejecting trigger")
			}
			loaded, err := store.Load()
			if err != nil {
				t.Fatal(err)
			}
			if !loaded.PersistedFieldsIdentical(baseline) {
				t.Fatal("failed save changed persisted state")
			}
		})
	}
	if _, err := store.db.Exec(`drop trigger prohibit_delete`); err != nil {
		t.Fatal(err)
	}
	other, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	newer := org.Clone()
	newer.IDSequences["Account"]++
	if err := other.Save(newer); err != nil {
		t.Fatal(err)
	}
	// A stale state cannot be mistaken for an unchanged database.
	if err := store.SaveIfChanged(org); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded.IDSequences["Account"] != org.IDSequences["Account"] {
		t.Fatal("stale save incorrectly skipped")
	}
}

func BenchmarkSQLiteUnchangedSave(b *testing.B) {
	for _, mode := range []string{"baseline", "unchanged"} {
		b.Run(mode, func(b *testing.B) {
			store, err := OpenSQLite(filepath.Join(b.TempDir(), "org.db"))
			if err != nil {
				b.Fatal(err)
			}
			defer store.Close()
			org := benchmarkFixtureOrg()
			if err := ApplyFixture(&org, benchmarkFixture(5000)); err != nil {
				b.Fatal(err)
			}
			if err := store.Save(org); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if mode == "baseline" {
					err = store.Save(org)
				} else {
					err = store.SaveIfChanged(org)
				}
				if err != nil {
					b.Fatal(fmt.Errorf("save: %w", err))
				}
			}
		})
	}
}

func TestSQLiteSaveIfChangedConcurrentWriters(t *testing.T) {
	path := filepath.Join(t.TempDir(), "org.db")
	a, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	org := benchmarkFixtureOrg()
	if err := ApplyFixture(&org, benchmarkFixture(3)); err != nil {
		t.Fatal(err)
	}
	if err := a.Save(org); err != nil {
		t.Fatal(err)
	}
	b, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	first, second := org.Clone(), org.Clone()
	first.IDSequences["Account"] = 100
	second.IDSequences["Account"] = 200
	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i, store := range []*SQLiteStore{a, b} {
		state := first
		if i == 1 {
			state = second
		}
		wg.Add(1)
		go func() { defer wg.Done(); <-start; errs <- store.SaveIfChanged(state) }()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	loaded, err := a.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.IDSequences["Account"]; got != 100 && got != 200 {
		t.Fatalf("sequence = %d", got)
	}
	if len(loaded.Objects["Account"].Records) != 3 {
		t.Fatal("partial write")
	}
}

func TestSQLiteSaveIfChangedMatchesSave(t *testing.T) {
	for name, mutate := range map[string]func(*OrgState){
		"metadata":   func(o *OrgState) { o.Metadata.DataCategoryGroups = []DataCategoryGroup{{Name: "Changed"}} },
		"sequence":   func(o *OrgState) { o.IDSequences["Account"]++ },
		"definition": func(o *OrgState) { x := o.Objects["Account"]; x.Definition.Label = "Changed"; o.Objects["Account"] = x },
		"record": func(o *OrgState) {
			x := o.Objects["Account"]
			for id := range x.Records {
				delete(x.Records, id)
				break
			}
			o.Objects["Account"] = x
		},
	} {
		t.Run(name, func(t *testing.T) {
			a, err := OpenSQLite(filepath.Join(t.TempDir(), "a.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer a.Close()
			b, err := OpenSQLite(filepath.Join(t.TempDir(), "b.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer b.Close()
			org := benchmarkFixtureOrg()
			if err := ApplyFixture(&org, benchmarkFixture(3)); err != nil {
				t.Fatal(err)
			}
			if err := a.Save(org); err != nil {
				t.Fatal(err)
			}
			if err := b.Save(org); err != nil {
				t.Fatal(err)
			}
			mutate(&org)
			if err := a.SaveIfChanged(org); err != nil {
				t.Fatal(err)
			}
			if err := b.Save(org); err != nil {
				t.Fatal(err)
			}
			dump := func(s *SQLiteStore) []string {
				t.Helper()
				var result []string
				for _, q := range []string{`select key || ':' || quote(value) from org_meta order by key`, `select name || ':' || quote(definition_json) from object_definitions order by name`, `select object_name || ':' || id || ':' || quote(record_json) from records order by object_name, id`, `select object_name || ':' || sequence from id_sequences order by object_name`} {
					rows, err := s.db.Query(q)
					if err != nil {
						t.Fatal(err)
					}
					for rows.Next() {
						var row string
						if err := rows.Scan(&row); err != nil {
							t.Fatal(err)
						}
						result = append(result, row)
					}
					if err := rows.Err(); err != nil {
						t.Fatal(err)
					}
					rows.Close()
				}
				return result
			}
			if !reflect.DeepEqual(dump(a), dump(b)) {
				t.Fatal("persistent rows differ from Save")
			}
		})
	}
}

func TestSQLiteSaveIfChangedReadOnlyDatabase(t *testing.T) {
	path := filepath.Join(t.TempDir(), "org.db")
	store, err := OpenSQLite(path)
	if err != nil {
		t.Fatal(err)
	}
	org := benchmarkFixtureOrg()
	if err := ApplyFixture(&org, benchmarkFixture(3)); err != nil {
		t.Fatal(err)
	}
	if err := store.Save(org); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	readonly, err := OpenSQLite("file:" + filepath.ToSlash(path) + "?mode=ro")
	if err != nil {
		t.Fatal(err)
	}
	defer readonly.Close()
	if err := readonly.SaveIfChanged(org); err != nil {
		t.Fatalf("unchanged read-only save: %v", err)
	}
	org.IDSequences["Account"]++
	if err := readonly.SaveIfChanged(org); err == nil {
		t.Fatal("changed read-only save succeeded")
	}
}
