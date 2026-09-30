package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestContractNumberDefaultPreservesImportedValuesAndAvoidsCollisions(t *testing.T) {
	org := testOrg()
	org.Objects["Contract"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Contract"}, Records: map[storage.ID]storage.Record{
		"800000000000001": {Object: "Contract", Fields: map[string]storage.Value{"ContractNumber": storage.StringValue("00000001")}},
		"800000000000002": {Object: "Contract", Fields: map[string]storage.Value{"ContractNumber": storage.StringValue("00000002")}},
	}}
	e := NewEngine(&org)
	e.IDs.Sequences["Contract"] = 0
	row := storage.Record{Object: "Contract"}
	e.applyContractNumberDefault("Contract", &row)
	if got := row.Fields["ContractNumber"].String; got != "00000003" {
		t.Fatalf("collision-safe number = %q", got)
	}
	if e.IDs.Sequences["Contract"] != 0 {
		t.Fatal("default changed allocation state before insert")
	}
	row.Fields["ContractNumber"] = storage.StringValue("Imported-value")
	e.applyContractNumberDefault("Contract", &row)
	if row.Fields["ContractNumber"].String != "Imported-value" {
		t.Fatal("existing number overwritten")
	}
	custom := storage.Record{Object: "Contract__c"}
	e.applyContractNumberDefault("Contract__c", &custom)
	if len(custom.Fields) != 0 {
		t.Fatal("platform default applied to custom object")
	}
	if org.Objects["Contract"].Records["800000000000001"].Fields["ContractNumber"].String != "00000001" {
		t.Fatal("stored value changed")
	}
}
