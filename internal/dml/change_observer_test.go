package dml

import (
	"errors"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The VM's native CDC operation/upsert/partial/rollback fixtures also enter
// through these engine methods. Pin the observer contract beneath those routes.
func TestChangeObserverMutationUpsertPartialAndTransactionRoutes(t *testing.T) {
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	engine := NewEngine(&org)
	var operations []string
	var beforeUpdate storage.Record
	observer := func(operation string, before, after storage.Record) {
		operations = append(operations, operation)
		if operation == "UPDATE" {
			beforeUpdate = before.Clone()
		}
	}
	checkpoint := func() func() {
		n := len(operations)
		return func() { operations = operations[:n] }
	}
	// Resolve optional hooks dynamically so this regression compiles on the
	// runtime before the observer contract was introduced.
	for name, hook := range map[string]any{"ChangeObserver": observer, "ChangeCheckpoint": checkpoint} {
		field := reflect.ValueOf(&engine).Elem().FieldByName(name)
		if !field.IsValid() || !field.CanSet() || field.Type() != reflect.TypeOf(hook) {
			t.Fatalf("engine mutation route has no %s hook", name)
		}
		field.Set(reflect.ValueOf(hook))
	}
	row := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("cedar-source")}}
	inserted := engine.Insert([]storage.Record{row, {Object: "Account"}})
	if !inserted[0].Success || inserted[1].Success {
		t.Fatalf("partial insert: %+v", inserted)
	}
	row.ID = inserted[0].ID
	row.Fields["Name"] = storage.StringValue("cedar-renamed")
	if result := engine.Update([]storage.Record{row}); !result[0].Success {
		t.Fatal(result)
	}
	if name, _ := beforeUpdate.GetField("Name"); name.String != "cedar-source" {
		t.Fatalf("before image: %+v", beforeUpdate)
	}
	if result := engine.Delete([]storage.Record{row}); !result[0].Success {
		t.Fatal(result)
	}
	if result := engine.Undelete([]storage.Record{row}); !result[0].Success {
		t.Fatal(result)
	}
	if result := engine.Upsert([]storage.Record{row}); !result[0].Success {
		t.Fatal(result)
	}
	row.ID = ""
	if result := engine.Upsert([]storage.Record{row}); !result[0].Success {
		t.Fatal(result)
	}
	want := []string{"CREATE", "UPDATE", "DELETE", "UNDELETE", "UPDATE", "CREATE"}
	if !reflect.DeepEqual(operations, want) {
		t.Fatalf("operations=%v, want %v", operations, want)
	}
	abort := errors.New("rollback")
	if err := engine.WithTransaction(func(e *Engine) error {
		if result := e.Insert([]storage.Record{row}); !result[0].Success {
			t.Fatal(result)
		}
		return abort
	}); !errors.Is(err, abort) {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(operations, want) {
		t.Fatalf("rolled-back changes escaped: %v", operations)
	}
}
