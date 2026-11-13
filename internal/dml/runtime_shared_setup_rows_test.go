package dml

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Read-only setup rows that a runtime org clone shares with its template.
var runtimeSharedSetupObjects = []string{"UserLicense", "PermissionSetLicense", "OpportunityStage", "TaskStatus", "LeadStatus", "TaskPriority", "DuplicateRule", "MatchingRule"}

func sharedSetupTemplate(t *testing.T) storage.OrgState {
	t.Helper()
	org := storage.NewOrgState()
	storage.EnsureDeterministicPlatformData(&org)
	seeded := 0
	for _, name := range runtimeSharedSetupObjects {
		if len(org.Objects[name].Records) > 0 {
			seeded++
		}
	}
	if seeded < 5 {
		t.Fatalf("only %d setup objects seeded", seeded)
	}
	return org
}

func setupRowsDigest(org storage.OrgState) string {
	names := append([]string{"Account"}, runtimeSharedSetupObjects...)
	sort.Strings(names)
	out := ""
	for _, name := range names {
		out += fmt.Sprintf("%s=%v\n", name, org.Objects[name].Records)
	}
	return out
}

func firstRow(org storage.OrgState, name string) (storage.Record, bool) {
	ids := make([]string, 0, len(org.Objects[name].Records))
	for id := range org.Objects[name].Records {
		ids = append(ids, string(id))
	}
	if len(ids) == 0 {
		return storage.Record{}, false
	}
	sort.Strings(ids)
	return org.Objects[name].Records[storage.ID(ids[0])].Clone(), true
}

// Every DML route against a clone's shared setup rows, whether the engine
// accepts or rejects it, leaves the template and a sibling clone unchanged.
func TestDMLOnClonedSetupRowsLeavesTemplateUnchanged(t *testing.T) {
	template := sharedSetupTemplate(t)
	want := setupRowsDigest(template)
	for _, name := range runtimeSharedSetupObjects {
		row, ok := firstRow(template, name)
		if !ok {
			continue
		}
		routes := map[string]func(e *Engine){
			"insert": func(e *Engine) {
				e.Insert([]storage.Record{{Object: name, Fields: map[string]storage.Value{"MasterLabel": storage.StringValue("New"), "DeveloperName": storage.StringValue("New_Value")}}})
			},
			"update": func(e *Engine) {
				update := storage.Record{Object: name, ID: row.ID, Fields: map[string]storage.Value{"MasterLabel": storage.StringValue("Changed")}}
				e.Update([]storage.Record{update})
			},
			"delete": func(e *Engine) {
				e.Delete([]storage.Record{{Object: name, ID: row.ID}})
			},
			"undelete": func(e *Engine) {
				e.Delete([]storage.Record{{Object: name, ID: row.ID}})
				e.Undelete([]storage.Record{{Object: name, ID: row.ID}})
			},
			"upsert": func(e *Engine) {
				e.Upsert([]storage.Record{{Object: name, ID: row.ID, Fields: map[string]storage.Value{"MasterLabel": storage.StringValue("Upserted")}}})
			},
			"rolled back transaction": func(e *Engine) {
				_ = e.WithTransaction(func(tx *Engine) error {
					tx.Update([]storage.Record{{Object: name, ID: row.ID, Fields: map[string]storage.Value{"MasterLabel": storage.StringValue("Rolled back")}}})
					tx.Delete([]storage.Record{{Object: name, ID: row.ID}})
					return errors.New("roll back")
				})
			},
			"account insert": func(e *Engine) {
				e.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Acme")}}})
			},
		}
		for label, route := range routes {
			clone := template.CloneRuntimeFrozenShared()
			sibling := template.CloneRuntimeFrozenShared()
			if !clone.Objects[name].RecordsShared {
				t.Fatalf("%s rows are not shared with the clone", name)
			}
			engine := NewEngine(&clone)
			route(&engine)
			if setupRowsDigest(template) != want {
				t.Fatalf("%s %s reached the template", name, label)
			}
			if setupRowsDigest(sibling) != want {
				t.Fatalf("%s %s reached a sibling clone", name, label)
			}
		}
	}
}

// Parallel test classes run DML on clones of one template. Run with -race.
func TestDMLOnClonedSetupRowsParallelClasses(t *testing.T) {
	template := sharedSetupTemplate(t)
	want := setupRowsDigest(template)
	var wg sync.WaitGroup
	for worker := 0; worker < 6; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i, name := range runtimeSharedSetupObjects {
				row, ok := firstRow(template, name)
				if !ok {
					continue
				}
				clone := template.CloneRuntimeFrozenShared()
				engine := NewEngine(&clone)
				if (worker+i)%2 == 0 {
					engine.Update([]storage.Record{{Object: name, ID: row.ID, Fields: map[string]storage.Value{"MasterLabel": storage.StringValue(fmt.Sprintf("w%d", worker))}}})
				} else {
					engine.Delete([]storage.Record{{Object: name, ID: row.ID}})
				}
				_ = reflect.DeepEqual(clone.Objects[name].Records, template.Objects[name].Records)
			}
		}(worker)
	}
	wg.Wait()
	if setupRowsDigest(template) != want {
		t.Fatal("parallel clone DML reached the template")
	}
}
