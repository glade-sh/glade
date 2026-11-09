package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// activity_account_derivation (API 62.0 and 67.0): $ObjectType deploys only
// in a validation rule, where ISNULL($ObjectType.Account.Fields.Name) is
// false (row V). Formula fields, defaults, workflow and flow formulas fail to
// deploy, and FormulaEval rejects it (deploy and F rows).
func TestObjectTypeFormulaAnswerOnlyInValidationRules(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Account")
	definition := org.Objects["Account"].Definition
	record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("LF")}}
	const formula = "ISNULL($ObjectType.Account.Fields.Name)"
	if fires, ok := evaluateValidationFormulaInOrgWithNamespace(formula, "", &org, definition, record, nil, true, ""); !ok || fires {
		t.Fatalf("validation rule: fires=%v ok=%v; want not null", fires, ok)
	}
	if isNull, ok := evaluateRecordFormulaInOrgWithContext(formula, &org, definition, record, nil, true); !ok || !isNull {
		t.Fatalf("other formula: isNull=%v ok=%v; want the pre-capture null answer", isNull, ok)
	}
	want := "Field $ObjectType.Account.Fields.Name does not exist. Check spelling."
	if got := ValidateFormulaInstance("IF(ISNULL($ObjectType.Account.Fields.Name), 'null', 'set')", &org, definition, "STRING", false); got != want {
		t.Fatalf("FormulaEval message = %q; want %q", got, want)
	}
}
