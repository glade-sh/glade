package soql

import (
	"strings"
	"testing"
)

func TestBindColumnsDistinguishesOccurrencesAndSubqueries(t *testing.T) {
	input := "SELECT Id FROM Account WHERE Name=:wanted AND Id=:wanted AND Id IN (SELECT AccountId FROM Contact WHERE LastName=:wanted)"
	columns, err := BindColumns(input)
	if err != nil {
		t.Fatal(err)
	}
	remaining := input
	offset := 0
	for _, expected := range []BindColumn{{Object: "Account", Field: "Name"}, {Object: "Account", Field: "Id"}, {Object: "Contact", Field: "LastName"}} {
		at := strings.Index(remaining, ":wanted")
		if at < 0 {
			t.Fatal("missing bind")
		}
		if got := columns[offset+at]; got != expected {
			t.Fatalf("expected %#v, got %#v", expected, got)
		}
		offset += at + len(":wanted")
		remaining = input[offset:]
	}
	if len(columns) != 3 {
		t.Fatalf("bind columns: %#v", columns)
	}
}
