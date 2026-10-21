package vm

import (
	"strings"
	"testing"
)

// These local boundaries are deliberately retained where the native captures
// establish no exception contract or admission behavior.
func TestCursorJSONTokenUnobservedAdmissionBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, api, expression string
	}{
		{"strict", "65.0", `JSON.deserializeStrict('{"queryId":"UnusableCursor67890"}',Database.Cursor.class)`},
		{"numeric", "65.0", `JSON.deserialize('{"queryId":12345}',Database.Cursor.class)`},
		{"unregistered multipart", "65.0", `JSON.deserialize('{"queryId":"unregistered-token"}',Database.Cursor.class)`},
		{"other API", "66.0", `JSON.deserialize('{"queryId":"UnusableCursor67890"}',Database.Cursor.class)`},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(test.expression+`;`, CompileOptions{APIVersion: test.api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			_, err = machine.Execute(program)
			if err == nil || !strings.Contains(err.Error(), "Database.Cursor JSON queryId resolution outside the local execution") {
				t.Fatalf("got %v; want retained unsupported admission boundary", err)
			}
			if len(machine.queryCursors) != 0 {
				t.Fatal("unobserved admission created a registered query")
			}
		})
	}
}

func TestCursorJSONTokenRetainsUnresolvedMethodBoundary(t *testing.T) {
	for _, operation := range []string{"shell.getNumRecords()", "shell.fetch(0,1)"} {
		t.Run(operation, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Database.Cursor shell=(Database.Cursor)JSON.deserialize('{"queryId":"UnusableCursor67890"}',Database.Cursor.class);
`+operation+`;`, CompileOptions{APIVersion: "65.0"})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			_, err = machine.Execute(program)
			if err == nil || !strings.Contains(err.Error(), "Database.Cursor JSON queryId resolution outside the local execution") {
				t.Fatalf("got %v; want retained unresolved method boundary", err)
			}
			shell := machine.Globals["shell"]
			if shell.Kind != ValueObject || shell.Type != "Database.Cursor" {
				t.Fatal("method boundary occurred before observed shell admission")
			}
			for _, field := range []string{"Records", "Query"} {
				if _, found := shell.Fields[field]; found {
					t.Fatalf("unresolved shell acquired invented %s", field)
				}
			}
			if len(machine.queryCursors) != 0 {
				t.Fatal("unresolved shell created a registered query")
			}
		})
	}
}
