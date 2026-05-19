package storage

import "testing"

func TestRebuildIndexesBuildsSingleFieldEntries(t *testing.T) {
	org := NewOrgState()
	org.Objects["Account"] = ObjectState{
		Definition: ObjectDefinition{
			APIName: "Account",
			Indexes: []IndexDefinition{{
				Name:   "Account.Name",
				Object: "Account",
				Fields: []string{"Name"},
			}},
		},
		Records: map[ID]Record{
			"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]Value{"Name": StringValue("Acme")}},
			"001000000000002": {ID: "001000000000002", Object: "Account", Fields: map[string]Value{"Name": StringValue("Beta")}},
		},
	}

	RebuildIndexes(&org)
	ids, ok := LookupIndex(org.Objects["Account"], "Name", StringValue("Acme"))
	if !ok || len(ids) != 1 || ids[0] != "001000000000001" {
		t.Fatalf("index lookup = %#v ok=%t", ids, ok)
	}
}

func TestLookupIndexMatchesFifteenAndEighteenCharacterIDs(t *testing.T) {
	org := NewOrgState()
	org.Objects["User"] = ObjectState{
		Definition: ObjectDefinition{
			APIName: "User",
			Indexes: []IndexDefinition{{
				Name:   "User.Id",
				Object: "User",
				Fields: []string{"Id"},
			}},
		},
		Records: map[ID]Record{
			"00500000000LFLV": {ID: "00500000000LFLV", Object: "User"},
		},
	}

	RebuildIndexes(&org)
	ids, ok := LookupIndex(org.Objects["User"], "Id", IDValue("00500000000LFLVAA4"))
	if !ok || len(ids) != 1 || ids[0] != "00500000000LFLV" {
		t.Fatalf("18-character lookup = %#v ok=%t", ids, ok)
	}
	ids, ok = LookupIndex(org.Objects["User"], "Id", StringValue("00500000000lflvaa4"))
	if !ok || len(ids) != 1 || ids[0] != "00500000000LFLV" {
		t.Fatalf("case-insensitive lookup = %#v ok=%t", ids, ok)
	}
}
