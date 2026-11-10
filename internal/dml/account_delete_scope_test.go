package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func scopeInsert(t *testing.T, engine *Engine, object string, fields map[string]storage.Value) storage.ID {
	t.Helper()
	res := engine.Insert([]storage.Record{{Object: object, Fields: fields}})
	if len(res) != 1 || !res[0].Success {
		t.Fatalf("insert %s: %#v", object, res)
	}
	return res[0].ID
}

func scopeOrg() (*storage.OrgState, *Engine) {
	org := testOrg()
	for _, o := range []string{"Contract", "Case", "Opportunity", "Contact"} {
		storage.EnsureStandardObject(&org, o)
	}
	e := NewEngine(&org)
	return &org, &e
}

// Only the captured Contract statuses (Draft, In Approval Process, Activated)
// get a derived StatusCode; any other status keeps the stored StatusCode
// untouched, so a configured activated-category status is not downgraded.
func TestContractStatusCodeOnlyForCapturedStatuses(t *testing.T) {
	_, engine := scopeOrg()
	account := scopeInsert(t, engine, "Account", map[string]storage.Value{"Name": storage.StringValue("Synthetic")})
	contract := scopeInsert(t, engine, "Contract", map[string]storage.Value{"AccountId": storage.IDValue(account), "Status": storage.StringValue("Draft")})
	stored := engine.Org.Objects["Contract"].Records[contract]
	stored.Fields["Status"] = storage.StringValue("Configured Live")
	stored.Fields["StatusCode"] = storage.StringValue("Activated")
	engine.Org.Objects["Contract"].Records[contract] = stored
	// An unrelated update leaves it alone too.
	engine.Update([]storage.Record{{Object: "Contract", ID: contract, Fields: map[string]storage.Value{"Description": storage.StringValue("x")}}})
	if got := engine.Org.Objects["Contract"].Records[contract].Fields["StatusCode"].String; got != "Activated" {
		t.Fatalf("unrelated update changed StatusCode to %q", got)
	}
}

// A Contract in an uncaptured status is not treated as cascadeable: the
// Account delete keeps failing as it did before the captured rules.
func TestAccountDeleteKeepsUncapturedContractStatus(t *testing.T) {
	org, engine := scopeOrg()
	account := scopeInsert(t, engine, "Account", map[string]storage.Value{"Name": storage.StringValue("Synthetic")})
	contract := scopeInsert(t, engine, "Contract", map[string]storage.Value{"AccountId": storage.IDValue(account), "Status": storage.StringValue("Draft")})
	c := org.Objects["Contract"].Records[contract]
	c.Fields["Status"] = storage.StringValue("Configured Live")
	delete(c.Fields, "StatusCode")
	org.Objects["Contract"].Records[contract] = c
	res := engine.Delete([]storage.Record{{Object: "Account", ID: account}})
	if len(res) != 1 || res[0].Success {
		t.Fatalf("delete with uncaptured Contract status: %#v", res)
	}
	if org.Objects["Account"].Records[account].System.IsDeleted || org.Objects["Contract"].Records[contract].System.IsDeleted {
		t.Fatal("records deleted")
	}
}

// Competing blocker types have no captured precedence: the delete fails, and
// none of the single-blocker native messages is chosen.
func TestAccountDeleteCompetingBlockersMakeNoPrecedenceClaim(t *testing.T) {
	org, engine := scopeOrg()
	account := scopeInsert(t, engine, "Account", map[string]storage.Value{"Name": storage.StringValue("Synthetic")})
	scopeInsert(t, engine, "Contract", map[string]storage.Value{"AccountId": storage.IDValue(account), "Status": storage.StringValue("Activated")})
	scopeInsert(t, engine, "Case", map[string]storage.Value{"AccountId": storage.IDValue(account), "Subject": storage.StringValue("s")})
	res := engine.Delete([]storage.Record{{Object: "Account", ID: account}})
	if len(res) != 1 || res[0].Success {
		t.Fatalf("delete: %#v", res)
	}
	for _, e := range append(res[0].Errors, Error{Message: res[0].Error}) {
		if strings.Contains(e.Message, "following active contracts") || strings.Contains(e.Message, "following cases") {
			t.Fatalf("single-blocker message chosen for competing blockers: %q", e.Message)
		}
	}
	if org.Objects["Account"].Records[account].System.IsDeleted {
		t.Fatal("account deleted")
	}
	// Each blocker alone keeps its captured message.
	for object, fields := range map[string]map[string]storage.Value{
		"Contract": {"Status": storage.StringValue("Activated")},
		"Case":     {"Subject": storage.StringValue("s")},
	} {
		_, e := scopeOrg()
		a := scopeInsert(t, e, "Account", map[string]storage.Value{"Name": storage.StringValue("Synthetic")})
		fields["AccountId"] = storage.IDValue(a)
		scopeInsert(t, e, object, fields)
		r := e.Delete([]storage.Record{{Object: "Account", ID: a}})
		want := "following cases"
		if object == "Contract" {
			want = "following active contracts"
		}
		if len(r) != 1 || r[0].Success || len(r[0].Errors) == 0 || !strings.Contains(r[0].Errors[0].Message, want) {
			t.Errorf("%s alone: %#v", object, r)
		}
	}
}
