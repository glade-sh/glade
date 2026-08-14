package vm

import (
	"encoding/json"
	"testing"

	"github.com/glade-sh/glade/internal/ir"
)

func TestCompileDMLAccessModesPreservePrefixAndSuffixSyntax(t *testing.T) {
	for _, test := range []struct {
		name string
		src  string
		mode ir.DMLMode
	}{
		{name: "insert prefix", src: "insert as user record;", mode: ir.DMLModeUser},
		{name: "update suffix", src: "update record as system;", mode: ir.DMLModeSystem},
		{name: "upsert suffix", src: "upsert record External_Id__c as user;", mode: ir.DMLModeUser},
		{name: "merge prefix", src: "merge as system master duplicate;", mode: ir.DMLModeSystem},
		{name: "merge suffix", src: "merge master duplicate as user;", mode: ir.DMLModeUser},
		{name: "default", src: "delete record;", mode: ir.DMLModeDefault},
	} {
		t.Run(test.name, func(t *testing.T) {
			program, err := CompileAnonymous(test.src)
			if err != nil {
				t.Fatalf("CompileAnonymous(%q): %v", test.src, err)
			}
			if len(program.Instructions) != 1 || program.Instructions[0].DMLMode != test.mode {
				t.Fatalf("DML mode for %q = %#v, want %d", test.src, program.Instructions, test.mode)
			}
		})
	}
}

func TestCompileDMLAccessModesRejectDuplicatesAndSurviveJSON(t *testing.T) {
	if _, err := CompileAnonymous("insert as user record as system;"); err == nil {
		t.Fatal("duplicate DML access modes were accepted")
	}
	program, err := CompileAnonymous("undelete as system record;")
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	var roundTrip ir.Program
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatal(err)
	}
	if got := roundTrip.Instructions[0].DMLMode; got != ir.DMLModeSystem {
		t.Fatalf("JSON DML mode = %d, want %d", got, ir.DMLModeSystem)
	}
}
