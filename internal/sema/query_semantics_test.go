package sema

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestQuerySemanticsRejectsWrongSOSLAssignmentType(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { List<Account> values = [FIND 'probe' RETURNING Account(Id)]; } }`,
	})
	if !result.HasErrors() {
		t.Fatalf("SOSL result assigned to List<Account> was accepted: %#v", result.Diagnostics)
	}
}

func TestAnalyzeSOQLQueryDiagnosticsUseSchemaResolution(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> a = [SELECT Missing__c FROM Account];
    List<Account> b = [SELECT Owner.Missing__c FROM Account];
    List<Account> c = [SELECT Id, (SELECT LastName FROM BadContacts) FROM Account];
    List<Account> d = [SELECT Id FROM Missing__c];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_FIELD", "Missing__c", 4, 31)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_FIELD", "Owner.Missing__c", 5, 31)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "BadContacts", 6, 57)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_OBJECT", "Missing__c", 7, 39)
}

func TestQuerySemanticsFailsClosedOnParserErrors(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    List<Account> brokenSoql = [SELECT Id];
    List<List<SObject>> brokenSosl = [FIND 'probe'];
  }
}
`)
	if !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_PARSE") || !hasDiagnosticCode(diagnostics, "GLADESEMA_SOSL_PARSE") {
		t.Fatalf("expected query parser diagnostics: %#v", diagnostics)
	}
}

func TestQuerySemanticsRejectsInvalidQueryShapes(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<AggregateResult> limited = [SELECT COUNT(Id) total FROM Account LIMIT 1];
    List<AggregateResult> rollup = [SELECT Name, Rating, Type, COUNT(Id) total FROM Account GROUP BY ROLLUP(Name, Rating, Type)];
    List<Account> locked = [SELECT Id FROM Account ORDER BY Name FOR UPDATE];
    List<Account> allFields = [SELECT FIELDS(ALL) FROM Account];
  }
}
`, queryDiagnosticSchema())
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("expected query contract diagnostics: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsAllowsUngroupedScalarCountWithLimit(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    Integer countRows = [SELECT COUNT() FROM Account LIMIT 1];
  }
}
`, queryDiagnosticSchema())
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("ungrouped scalar COUNT() with LIMIT was rejected: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsUngroupedAggregateWithLimit(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<AggregateResult> aggregateRows = [SELECT COUNT(Id) total FROM Account LIMIT 1];
  }
}
`, queryDiagnosticSchema())
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("ungrouped aggregate with LIMIT was accepted: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsAggregateOnNonAggregatableField(t *testing.T) {
	no := false
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<AggregateResult> values = [SELECT SUM(Amount__c) total FROM Account];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Amount__c", Type: "Number", Aggregatable: &no}}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CAPABILITY") {
		t.Fatalf("SUM on a non-aggregatable field was accepted: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsAggregateOnStandardTextField(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { List<AggregateResult> values = [SELECT SUM(Name) total FROM Account]; } }`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("SUM on standard Account.Name was accepted: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsFieldsAllInApexEvenWithLimit(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { List<Account> values = [SELECT FIELDS(ALL) FROM Account LIMIT 1]; } }`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("FIELDS(ALL) in Apex was accepted: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsSelfSemiJoin(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Id IN (SELECT Id FROM Account)];
  }
}
`, queryDiagnosticSchema())
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CONTRACT") {
		t.Fatalf("expected self semi-join contract: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsEnforcesDescribeBackedFieldCapabilities(t *testing.T) {
	no := false
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<Account> filtered = [SELECT Id FROM Account WHERE NoFilter__c = 'x'];
    List<AggregateResult> grouped = [SELECT NoGroup__c, COUNT(Id) total FROM Account GROUP BY NoGroup__c];
    List<Account> sorted = [SELECT Id FROM Account ORDER BY NoSort__c];
    List<AggregateResult> aggregated = [SELECT SUM(NoAggregate__c) total FROM Account];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{
		{Name: "NoFilter__c", Type: "Text", Filterable: &no},
		{Name: "NoGroup__c", Type: "Text", Groupable: &no},
		{Name: "NoSort__c", Type: "Text", Sortable: &no},
		{Name: "NoAggregate__c", Type: "Number", Aggregatable: &no},
	}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_CAPABILITY") {
		t.Fatalf("expected describe capability diagnostics: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsMissingInlineBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :missing];
  }
}
`)
	if !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected missing bind diagnostic: %#v", diagnostics)
	}
}

func TestQuerySemanticsValidatesDottedBindReceivers(t *testing.T) {
	for name, test := range map[string]struct {
		source     string
		wantReject bool
	}{
		"declared receiver": {
			source: `
public class QueryProbe {
  public void run(Account account) {
    List<Account> accounts = [SELECT Id FROM Account WHERE Id = :account.Id];
  }
}
`,
		},
		"this field receiver": {
			source: `
public class QueryProbe {
  private Id accountId;
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Id = :this.accountId];
  }
}
`,
		},
		"different field type": {
			source: `
public class QueryProbe {
  public void run(Account account) {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :account.Id];
  }
}
`,
		},
		"missing receiver": {
			source: `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Id = :missing.Id];
  }
}
	`,
			wantReject: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", test.source)
			gotReject := hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND")
			if gotReject != test.wantReject {
				t.Fatalf("diagnostics = %#v, want reject=%v", diagnostics, test.wantReject)
			}
		})
	}
}

func TestQuerySemanticsAcceptsInlineBooleanLiteralExpression(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Active__c = :true];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("inline Boolean literal expression was rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsAcceptsDeclaredInlineBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    String name = 'ok';
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :name];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("declared bind rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsAcceptsQualifiedParameterBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run(Outer.Response response) {
    List<Account> accounts = [SELECT Id FROM Account WHERE Id IN :response.CartIds];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("qualified parameter bind rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsRequiresNumericLimitAndOffsetBinds(t *testing.T) {
	invalid := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    String limitValue = '1';
    List<Account> accounts = [SELECT Id FROM Account LIMIT :limitValue];
  }
}

`)
	if !hasDiagnosticCode(invalid, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected nonnumeric LIMIT bind diagnostic: %#v", invalid)
	}
	valid := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    Integer limitValue = 1;
    Long offsetValue = 0;
    List<Account> accounts = [SELECT Id FROM Account LIMIT :limitValue OFFSET :offsetValue];
  }
}
`)
	if hasDiagnosticCode(valid, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("numeric query window binds rejected: %#v", valid)
	}
}

func TestQuerySemanticsRequiresNumericSOSLLimitBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    String limitValue = '1';
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id) LIMIT :limitValue];
  }
}

`)
	if !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected nonnumeric SOSL LIMIT bind diagnostic: %#v", diagnostics)
	}
}

func TestQuerySemanticsRequiresNumericSOSLReturningLimitAndOffsetBinds(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    String returningLimit = '1';
    String offsetValue = '0';
    Integer limitValue = 1;
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id LIMIT :returningLimit OFFSET :offsetValue) LIMIT :limitValue];
  }
}
`)
	count := 0
	for _, item := range diagnostics {
		if item.Code == "GLADESEMA_QUERY_BIND" {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("numeric SOSL returning LIMIT and OFFSET diagnostics = %d, want 2: %#v", count, diagnostics)
	}
}

func TestQuerySemanticsRequiresStringSOSLDivisionBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    Integer division = 1;
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id) WITH DIVISION = :division];
  }
}
`)
	if !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected non-string SOSL WITH DIVISION bind diagnostic: %#v", diagnostics)
	}
}

func TestQuerySemanticsAcceptsDocumentedSOSLBindClauses(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    String term = 'aaa';
    String name = 'bbb';
    Integer returningLimit = 1;
    Integer returningOffset = 0;
    String division = 'Global';
    Integer limitValue = 2;
    List<List<SObject>> rows = [FIND :term IN ALL FIELDS RETURNING Account(Id, Name WHERE Name LIKE :name LIMIT :returningLimit OFFSET :returningOffset) WITH DIVISION = :division LIMIT :limitValue];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") || hasDiagnosticCode(diagnostics, "GLADESEMA_SOSL_PARSE") {
		t.Fatalf("documented SOSL bind clauses rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsRequiresNumericSOSLLimitBindInOneLineMethod(t *testing.T) {
	result := analyzeQueryProbe(t, "public class QueryProbe { public void run() { String limitValue = '1'; List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id) LIMIT :limitValue]; } }", schema.Schema{})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected nonnumeric one-line SOSL LIMIT bind diagnostic: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsIncompatibleSOSLWhereBind(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    Integer value = 1;
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id WHERE Name = :value)];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "Text"}}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected incompatible SOSL WHERE bind diagnostic: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsIncompatibleSOSLLikeBind(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    Integer value = 1;
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id, Name WHERE Name LIKE :value)];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "Text"}}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expected incompatible SOSL LIKE bind diagnostic: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsAcceptsExpressionWindowBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account LIMIT :Limits.getLimitQueries()];
  }
}

`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("expression window bind was rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsAcceptsDocumentedLiteralMethodBind(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :'XXXX'.substring(0, 3)];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	if result.HasErrors() {
		t.Fatalf("documented literal method bind was rejected: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsAcceptsConcatenatedMethodCallBind(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    Integer countJobs = [SELECT COUNT() FROM CronJobDetail WHERE Name = :QueryProbe.class.getName() + ' Test'];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "CronJobDetail", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	if result.HasErrors() {
		t.Fatalf("concatenated method-call bind was rejected: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsChecksSourceWithNestedTypesOnce(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public class Nested {}
  public void run() {
    List<Account> accounts = [SELECT Missing__c FROM Account];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Id", Type: "Id"}}}}})
	var count int
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA_QUERY_FIELD" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("query field diagnostic count = %d, want 1: %#v", count, result.Diagnostics)
	}
}

func TestQuerySemanticsAcceptsCollectionConstructorBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run(Id firstId, Id secondId) {
    List<Account> accounts = [SELECT Id FROM Account WHERE Id IN :new Set<Id>{firstId, secondId}];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("collection constructor bind was rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsRejectsIncompatibleFieldBindType(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    Integer name = 1;
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :name];
  }
}
`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("incompatible field bind accepted: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsRejectsIncompatibleStandardFieldBindType(t *testing.T) {
	if _, ok := newQuerySemanticsChecker(typesys.Index{}).field("Account", "Name"); !ok {
		t.Fatal("standard Account.Name metadata is unavailable")
	}
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public void run() { Integer numberValue = 1; List<Account> values = [SELECT Id FROM Account WHERE Name = :numberValue]; } }`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("incompatible standard-field bind accepted: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsAcceptsCollectionBindInEqualityAndValidatesElementType(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    List<Integer> numbers = new List<Integer>{1};
    List<String> names = new List<String>{'Acme'};
	List<Account> validEquality = [SELECT Id FROM Account WHERE Name = :names];
    List<Account> wrongElement = [SELECT Id FROM Account WHERE Name IN :numbers];
    List<Account> valid = [SELECT Id FROM Account WHERE Name IN :names];
  }
}
	`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	var count int
	for _, item := range result.Diagnostics {
		if item.Code == "GLADESEMA_QUERY_BIND" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("collection bind diagnostics = %d, want 1: %#v", count, result.Diagnostics)
	}
}

func TestQuerySemanticsAcceptsArrayCollectionBind(t *testing.T) {
	result := analyzeQueryProbe(t, `
public class QueryProbe {
  public void run() {
    String[] names = new String[]{'Acme'};
    List<Account> accounts = [SELECT Id FROM Account WHERE Name IN :names];
  }
}
	`, schema.Schema{Objects: []schema.Object{{Name: "Account", Fields: []schema.Field{{Name: "Name", Type: "Text"}}}}})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("array collection bind rejected: %#v", result.Diagnostics)
	}
}

func TestQuerySemanticsResolvesBindsInTheirLexicalMethodScope(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void first() {
    String elsewhere = 'x';
  }
  public void second() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :elsewhere];
    List<Account> later = [SELECT Id FROM Account WHERE Name = :declaredLater];
    String declaredLater = 'x';
  }
}
`)
	if !hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("out-of-scope and later bind names were accepted: %#v", diagnostics)
	}
}

func TestQuerySemanticsAcceptsMethodParameterBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run(String name) {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :name];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("method parameter bind rejected: %#v", diagnostics)
	}
}

func TestSemaBindingResolverFindsGenericMethodParameters(t *testing.T) {
	source := `
public class QueryProbe {
  public virtual List<Account> run(Set<String> newEmails, List<Account> newAccounts) {
    return [SELECT Id FROM Account WHERE Name IN :newEmails AND Id NOT IN :newAccounts];
  }
}`
	offset := strings.Index(source, ":newEmails")
	bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
	if got := bindings["newemails"]; got != "Set<String>" {
		t.Fatalf("newEmails binding = %q, want Set<String>; all = %#v", got, bindings)
	}
	if got := bindings["newaccounts"]; got != "List<Account>" {
		t.Fatalf("newAccounts binding = %q, want List<Account>; all = %#v", got, bindings)
	}
}

func TestSemaBindingResolverKeepsLocalAfterNestedBlock(t *testing.T) {
	source := `
public class QueryProbe {
  public void run() {
    Id recordId = '001000000000001';
    if (true) { System.debug(recordId); }
    List<Account> accounts = [SELECT Id FROM Account WHERE Id = :recordId];
  }
}`
	offset := strings.Index(source, ":recordId")
	bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
	if got := bindings["recordid"]; got != "Id" {
		t.Fatalf("recordId binding = %q, want Id; all = %#v", got, bindings)
	}
}

func TestSemaBindingResolverFindsLocalAfterMultilineCall(t *testing.T) {
	source := `
@IsTest
public class QueryProbe {
  private static BundleComponentRequest setupDonationProductAndBuildComponentRequest() {
    return setupDonationProductAndBuildComponentRequest(null);
  }
  private static BundleComponentRequest run(String nameOverride) {
    Entity__c entity = DataFactoryEntity.insertEntity();
    Product__c product = DataFactoryProduct.insertDonationProduct('name', entity.Id);
    Account account = DataFactoryAccount.insertIndividualAccount();
    CurrencyService.Instance.mockCurrencyIsoCode = 'USD';
    Id priceClassId = DataFactoryPriceClass.insertDefaultPriceClass().Id;
    MembershipType__c membershipType = DataFactoryMembershipType.insertDefaultMembershipType(entity.Id);
    Id mtplId = DataFactoryMembershipTypeProductLink.insertMembershipTypeProdLink(
        membershipType.Id,
        product.Id,
        Constant.STAGE_BOTH,
        Constant.PURPOSE_DONATION).Id;
    if (String.isNotBlank(nameOverride)) {
      MembershipTypeProductLink__c mtpl = [SELECT Id FROM MembershipTypeProductLink__c WHERE Id = :mtplId LIMIT 1];
    }
  }
}`
	offset := strings.LastIndex(source, ":mtplId")
	bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
	if got := bindings["mtplid"]; got != "Id" {
		t.Fatalf("mtplId binding = %q, want Id; all = %#v", got, bindings)
	}
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", source)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("outer local bind inside an if block rejected: %#v", diagnostics)
	}
}

func TestSemaBindingResolverFindsAnnotatedStaticField(t *testing.T) {
	source := `
public class QueryProbe {
  @testVisible private static final String queryName = 'name';
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :queryName];
  }
}`
	offset := strings.Index(source, ":queryName")
	bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
	if got := bindings["queryname"]; got != "String" {
		t.Fatalf("queryName binding = %q, want String; all = %#v", got, bindings)
	}
}

func TestSemaBindingResolverLexicalBindingsShadowFields(t *testing.T) {
	for name, test := range map[string]struct {
		source     string
		want       string
		wantReject bool
	}{
		"parameter": {
			source: `
public class QueryProbe {
  public void run(String value) {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :value];
  }
  private Integer value;
}`,
			want: "String",
		},
		"local": {
			source: `
public class QueryProbe {
  public void run() {
    Integer value = 1;
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :value];
  }
  private String value;
}`,
			want:       "Integer",
			wantReject: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			offset := strings.Index(test.source, ":value")
			bindings := newSemaBindingResolver(test.source, newSemaCodeSpans(test.source)).bindingsAt(offset)
			if got := bindings["value"]; got != test.want {
				t.Fatalf("shadowed value binding = %q, want %q; all = %#v", got, test.want, bindings)
			}
			result := analyzeDeclarationProject(t, map[string]string{"QueryProbe.cls": test.source})
			if gotReject := hasDiagnosticCode(result.Diagnostics, "GLADESEMA_QUERY_BIND"); gotReject != test.wantReject {
				t.Fatalf("query bind rejection = %v, want %v; diagnostics=%#v", gotReject, test.wantReject, result.Diagnostics)
			}
		})
	}
}

func TestQuerySemanticsAcceptsInstanceFieldBind(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  private String name;
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :name];
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("instance field bind rejected: %#v", diagnostics)
	}
}

func TestQuerySemanticsAcceptsStaticFieldBindInsideRunAs(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  private static Id runAsId = '005000000000001';
  public void run() {
    System.runAs(new User(Id = runAsId)) {
      List<Account> accounts = [SELECT Id FROM Account WHERE OwnerId = :runAsId];
    }
  }
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("static field bind inside runAs rejected: %#v", diagnostics)
	}
}

func TestSemaBindingResolverFindsOuterLocalInsideRunAs(t *testing.T) {
	source := `
@IsTest
private class QueryProbe {
  @IsTest static void run() {
    Account evtReg = new Account();
    User testUser = new User();
    System.runAs(testUser) {
      List<Account> accounts = [SELECT Id FROM Account WHERE Id = :evtReg.Id];
    }
  }
}`
	locations := semaMethodLocations(source, newSemaCodeSpans(source))
	if len(locations) != 1 {
		t.Fatalf("method locations = %d, want 1: %#v", len(locations), locations)
	}
	offset := strings.Index(source, ":evtReg")
	bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
	if got := bindings["evtreg"]; got != "Account" {
		t.Fatalf("evtReg binding = %q, want Account; all = %#v", got, bindings)
	}
}

func TestQuerySemanticsAcceptsAnonymousBlockLocalBind(t *testing.T) {
	source := "Id accountId = '001000000000001'; Integer count = [SELECT COUNT() FROM Account WHERE Id = :accountId];"
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("anonymous.apex", source)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("anonymous-block local bind rejected: %#v", diagnostics)
	}
}

func TestSemaBindingResolverFindsPropertyAccessorLocal(t *testing.T) {
	source := `
public class QueryProbe {
  public List<Account> accounts {
    private get {
      String name = 'Acme';
      return [SELECT Id FROM Account WHERE Name = :name];
    }
  }
}`
	offset := strings.Index(source, ":name")
	bindings := newSemaBindingResolver(source, newSemaCodeSpans(source)).bindingsAt(offset)
	if got := bindings["name"]; got != "String" {
		t.Fatalf("name binding = %q, want String; all = %#v", got, bindings)
	}
}

func TestQuerySemanticsAcceptsInstanceFieldDeclaredAfterMethod(t *testing.T) {
	diagnostics := newQuerySemanticsChecker(typesys.Index{}).checkFile("QueryProbe.cls", `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id FROM Account WHERE Name = :name];
  }
  private String name;
}
`)
	if hasDiagnosticCode(diagnostics, "GLADESEMA_QUERY_BIND") {
		t.Fatalf("later instance field bind rejected: %#v", diagnostics)
	}
}
func TestQuerySemanticsFieldProviderKeepsProjectAuthority(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Objects: []schema.Object{{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Name", Type: "Number"},
				{Name: "OwnerId", Type: "Lookup", ReferenceTo: []string{"pkg__Queue__c"}, RelationshipName: "pkg__Owner"},
				{Name: "pkg__Flag__c", Type: "Checkbox"},
			},
		}},
	})

	name, ok := checker.field("Account", "name")
	if !ok || name.Type != "Number" {
		t.Fatalf("query Account.Name = %#v, %v; want project Number", name, ok)
	}
	owner, ok := checker.field("Account", "OwnerId")
	if !ok || owner.Name != "OwnerId" || len(owner.ReferenceTo) != 1 || owner.ReferenceTo[0] != "pkg__Queue__c" || owner.RelationshipName != "pkg__Owner" {
		t.Fatalf("query Account.OwnerId authority = %#v, %v", owner, ok)
	}
	flag, ok := checker.field("Account", "Flag__c")
	if !ok || flag.Name != "pkg__Flag__c" {
		t.Fatalf("query namespace alias = %#v, %v; want canonical pkg__Flag__c", flag, ok)
	}
	if _, ok := checker.field("Account", "CreatedDate"); !ok {
		t.Fatal("query checker omitted standard Account.CreatedDate")
	}
}

func TestQuerySemanticsCorrectsPartialStandardFieldsWithoutChangingProjectRelationship(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "pkg"},
		Objects: []schema.Object{{
			Name:    "Contact",
			Partial: true,
			Fields: []schema.Field{
				{Name: "AccountId", Type: "Id", ReferenceTo: []string{"Name"}, RelationshipName: "Account"},
				{Name: "HasOptedOutOfEmail", Type: "String"},
				{Name: "pkg__ProjectParent__c", Type: "Lookup", ReferenceTo: []string{"pkg__ProjectTarget__c"}, RelationshipName: "pkg__ProjectParent__r"},
			},
		}},
	})

	account, ok := checker.field("Contact", "AccountId")
	if !ok || len(account.ReferenceTo) != 1 || account.ReferenceTo[0] != "Account" || account.RelationshipName != "Account" {
		t.Fatalf("query Contact.AccountId = %#v, %v; want standard Account relationship", account, ok)
	}
	optOut, ok := checker.field("Contact", "HasOptedOutOfEmail")
	if !ok || !strings.EqualFold(optOut.Type, "Boolean") {
		t.Fatalf("query Contact.HasOptedOutOfEmail = %#v, %v; want Boolean", optOut, ok)
	}
	projectParent, ok := checker.field("Contact", "ProjectParent__c")
	if !ok || len(projectParent.ReferenceTo) != 1 || projectParent.ReferenceTo[0] != "pkg__ProjectTarget__c" || projectParent.RelationshipName != "pkg__ProjectParent__r" {
		t.Fatalf("query project relationship authority = %#v, %v", projectParent, ok)
	}
}

func TestQuerySemanticsProviderKeepsDeclaredCountsAndMissingDiagnostics(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{
		Name:   "Account",
		Fields: []schema.Field{{Name: "ProjectOnly__c", Type: "Text"}},
	}}})
	if got := checker.declaredFields[normalizeName("Account")]; got != 1 {
		t.Fatalf("declared Account field count = %d, want 1 before provider enrichment", got)
	}
	if _, ok := checker.field("Account", "ProjectOnly__c"); !ok {
		t.Fatal("query provider omitted project field")
	}
	if _, ok := checker.field("Account", "CreatedDate"); !ok {
		t.Fatal("query provider omitted standard field")
	}
	if _, ok := checker.field("Account", "DefinitelyMissing__c"); ok {
		t.Fatal("authoritative Account accepted a missing field")
	}
}

func TestQuerySemanticsProviderKeepsPartialAndExternalObjectsOpen(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{
		Project: typesys.ProjectInfo{Namespace: "local"},
		Objects: []schema.Object{
			{Name: "Partial__c", Partial: true, Fields: []schema.Field{{Name: "Known__c", Type: "Text"}}},
			{Name: "vend__External__c", Fields: []schema.Field{{Name: "vend__Known__c", Type: "Text"}}},
		},
	})
	if checker.hasFieldMetadata("Partial__c") {
		t.Fatal("partial object became authoritative after provider enrichment")
	}
	if !checker.allowsIncompleteExternalManagedPackageObject("vend__External__c") {
		t.Fatal("external managed object stopped failing open")
	}
}

func TestQuerySemanticsProviderKeepsHierarchySetupOwnerTarget(t *testing.T) {
	declared := schema.Object{
		Name:               "Settings__c",
		CustomSettingsType: "Hierarchy",
		Fields:             []schema.Field{{Name: "SetupOwnerId", Type: "Lookup"}},
	}
	enriched := semaEnrichSchemaObject(declared)
	provided, ok := newSemaSObjectFieldProvider("", declared).lookup("SetupOwnerId")
	if !ok || len(provided.ReferenceTo) != 1 || provided.ReferenceTo[0] != "Name" {
		t.Fatalf("declared SetupOwner provider = %#v, %v; want Name", provided, ok)
	}
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{enriched}}, declared)
	field, target, ok := checker.relationshipField("Settings__c", "SetupOwner")
	if !ok || !strings.EqualFold(target, "Name") {
		t.Fatalf("SetupOwner relationship = %#v, %q, %v; want Name", field, target, ok)
	}
}

func TestQuerySemanticsKeepsDeclaredFieldsOnObjectsAndStandardFieldsLayered(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{
		Name:   "Account",
		Fields: []schema.Field{{Name: "ProjectOnly__c", Type: "Text"}},
	}}})

	account, ok := checker.object("Account")
	if !ok {
		t.Fatal("query checker omitted Account")
	}
	if got := account.Fields; len(got) != 1 || got[0].Name != "ProjectOnly__c" {
		t.Fatalf("query Account.Fields = %#v, want declared fields only", got)
	}
	if _, ok := checker.field("Account", "CreatedDate"); !ok {
		t.Fatal("layered query provider omitted Account.CreatedDate")
	}
	account, _ = checker.object("Account")
	if len(account.Fields) != 1 || account.Fields[0].Name != "ProjectOnly__c" {
		t.Fatalf("standard lookup mutated Account.Fields: %#v", account.Fields)
	}
}

func TestQuerySemanticsDefersStandardProviderDecodeUntilLookup(t *testing.T) {
	const objectName = "CareProgram"
	semaStandardSObjectFieldProviders.Delete(normalizeName(objectName))
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{
		Name:   objectName,
		Fields: []schema.Field{{Name: "ProjectOnly__c", Type: "Text"}},
	}}})

	provider, ok := checker.providers[normalizeName(objectName)].(*semaLayeredSObjectFieldProvider)
	if !ok || len(provider.layers) < 3 {
		t.Fatalf("CareProgram provider = %#v, %v; want layered provider", provider, ok)
	}
	standard, ok := provider.layers[2].(*semaStandardSObjectFieldProvider)
	if !ok || standard == nil {
		t.Fatalf("CareProgram standard layer = %#v, %v", provider.layers[2], ok)
	}
	if standard.fields != nil {
		t.Fatal("CareProgram standard fields decoded during checker construction")
	}
	if _, ok := checker.field(objectName, "ParentProgramId"); !ok {
		t.Fatal("CareProgram provider omitted standard ParentProgramId")
	}
	if standard.fields == nil {
		t.Fatal("CareProgram standard fields did not decode on lookup")
	}
}

func TestQuerySemanticsDefersStandardProviderForCommonAndMissingCustomFields(t *testing.T) {
	const objectName = "Account"
	semaStandardSObjectFieldProviders.Delete(normalizeName(objectName))
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{
		Name: objectName,
		Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "Name", Type: "Text"},
			{Name: "Declared__c", Type: "Checkbox"},
		},
	}}})
	provider := checker.providers[normalizeName(objectName)].(*semaLayeredSObjectFieldProvider)
	standard := provider.layers[2].(*semaStandardSObjectFieldProvider)

	for _, fieldName := range []string{"iD", "nAmE"} {
		if _, ok := checker.field(objectName, fieldName); !ok {
			t.Fatalf("Account provider omitted common %s", fieldName)
		}
	}
	if field, ok := checker.field(objectName, "dEcLaReD__C"); !ok || !strings.EqualFold(field.Type, "Checkbox") {
		t.Fatalf("declared Account custom field = %#v, %v", field, ok)
	}
	if _, ok := checker.field(objectName, "mIsSiNgBeNcHmArKfIeLd__C"); ok {
		t.Fatal("Account provider accepted a missing custom field")
	}
	if standard.fields != nil {
		t.Fatal("common and missing custom fields decoded the full Account standard provider")
	}
	if _, ok := checker.field(objectName, "Website"); !ok {
		t.Fatal("Account provider omitted standard Website")
	}
	if standard.fields == nil {
		t.Fatal("real standard field did not decode the Account standard provider")
	}

	const customObject = "ProbeTestObject__c"
	semaStandardSObjectFieldProviders.Delete(normalizeName(customObject))
	customChecker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{Name: customObject}}})
	if field, ok := customChecker.field(customObject, "Name__c"); !ok || field.Type == "" {
		t.Fatalf("standard custom-suffix object field = %#v, %v", field, ok)
	}
}

func TestQuerySemanticsProviderReturnsIndependentFieldValues(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{Name: "Account"}}})
	first, ok := checker.field("Account", "OwnerId")
	if !ok || len(first.ReferenceTo) == 0 {
		t.Fatalf("Account.OwnerId = %#v, %v", first, ok)
	}
	want := first.ReferenceTo[0]
	first.ReferenceTo[0] = "Mutated__c"
	second, ok := checker.field("Account", "OwnerId")
	if !ok || len(second.ReferenceTo) == 0 || second.ReferenceTo[0] != want {
		t.Fatalf("second Account.OwnerId = %#v, %v; shared field data escaped", second, ok)
	}
}

func TestQuerySemanticsActivityFieldsStayInProviderOverlay(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{
		{Name: "Activity", Fields: []schema.Field{{Name: "ActivityOnly__c", Type: "Text"}}},
		{Name: "Task", Fields: []schema.Field{{Name: "TaskOnly__c", Type: "Text"}}},
		{Name: "Event"},
	}})

	for _, target := range []string{"Task", "Event"} {
		if _, ok := checker.field(target, "ActivityOnly__c"); !ok {
			t.Fatalf("%s provider omitted Activity field", target)
		}
		object, _ := checker.object(target)
		for _, field := range object.Fields {
			if field.Name == "ActivityOnly__c" {
				t.Fatalf("Activity field was eagerly copied into %s.Fields", target)
			}
		}
	}
	if _, ok := checker.field("Task", "TaskOnly__c"); !ok {
		t.Fatal("Task provider lost its declared field")
	}
}

func TestQuerySemanticsLayeredFieldLookupAllocationBound(t *testing.T) {
	checker := newQuerySemanticsChecker(typesys.Index{Objects: []schema.Object{{
		Name:   "Account",
		Fields: []schema.Field{{Name: "Id", Type: "Id"}, {Name: "Name", Type: "Text"}},
	}}})
	if _, ok := checker.field("Account", "Name"); !ok {
		t.Fatal("query checker omitted Account.Name")
	}

	allocs := testing.AllocsPerRun(100, func() {
		if _, ok := checker.field("Account", "Name"); !ok {
			t.Fatal("query checker omitted Account.Name")
		}
	})
	if allocs > 3 {
		t.Fatalf("layered Account.Name lookup allocated %.0f times, want <= 3", allocs)
	}
}

func TestAnalyzeSOQLRelationshipAndAggregateAliasDiagnostics(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> badRelationship = [SELECT Bogus.Name FROM Account];
    AggregateResult okAlias = [SELECT COUNT(Id) total FROM Account ORDER BY total];
    List<AggregateResult> groupedAlias = [SELECT CustomFlag__c flag FROM Account GROUP BY CustomFlag__c];
  }
}
`
	sch := queryDiagnosticSchema()
	sch.Objects[0].Fields = append(sch.Objects[0].Fields, schema.Field{Name: "CustomFlag__c", Type: "Checkbox"})
	result := analyzeQueryProbe(t, source, sch)

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Bogus.Name", 4, 45)
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "total")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "CustomFlag__c flag")
}

func TestAnalyzeQuerySemanticsAcceptsKnownStandardObjectsWithoutProjectMetadata(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id, Name FROM Account];
    List<Contact> contacts = [SELECT Id, LastName FROM Contact];
    List<Profile> profiles = [SELECT Id, Name FROM Profile];
    List<CustomPermission> permissions = [SELECT Id FROM CustomPermission];
    List<ApexClass> classes = [SELECT Id, Name FROM ApexClass];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{
		{Name: "Expression_Function__mdt"},
	}})

	for _, objectName := range []string{"Account", "Contact", "Profile", "CustomPermission", "ApexClass"} {
		assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", objectName)
	}
}

func TestAnalyzeQuerySemanticsUsesGeneratedStandardRelationshipShape(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run(Set<Id> accountIds) {
    List<User> selected = [SELECT Id, Contact.AccountId FROM User WHERE Contact.AccountId IN :accountIds];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{
		Name: "User",
		Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
			{Name: "ContactId", Type: "Id"},
		},
	}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Contact.AccountId")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Contact.AccountId")
}

func TestAnalyzeQuerySemanticsMergesStandardFieldsIntoProjectStandardObject(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id, Name, CustomFlag__c FROM Account];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "CustomFlag__c", Type: "Checkbox"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Account.Id")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Account.Name")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "CustomFlag__c")
}

func TestAnalyzeQuerySemanticsResolvesProjectLocalObjectNamesAgainstNamespacedSchema(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Thing__c> things = [SELECT Name__c FROM Thing__c];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "pkg__Thing__c",
			Fields: []schema.Field{
				{Name: "pkg__Name__c", Type: "Text"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", "Thing__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Name__c")
}

func TestAnalyzeQuerySemanticsResolvesNamespacedStandardObjectExtensionField(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT pkg__PrimaryAffiliation__c FROM Account];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "PrimaryAffiliation__c", Type: "Lookup", ReferenceTo: []string{"Account"}},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "pkg__PrimaryAffiliation__c")
}

func TestAnalyzeQuerySemanticsAllowsCurrentObjectQualifiedFields(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    Integer matches = [SELECT Count() FROM Account WHERE Account.pkg__Status__c = 'Open'];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Status__c", Type: "Text"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Account.pkg__Status__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "pkg__Status__c")
}

func TestAnalyzeQuerySemanticsCopiesActivityFieldsToTaskAndEvent(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Task> tasks = [SELECT pkg__Engagement_Plan__c FROM Task];
    List<Event> events = [SELECT pkg__Engagement_Plan__c FROM Event];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "Activity",
			Fields: []schema.Field{
				{Name: "pkg__Engagement_Plan__c", Type: "Lookup", ReferenceTo: []string{"pkg__Engagement_Plan__c"}, RelationshipName: "pkg__Engagement_Plan__r"},
			},
		},
		{Name: "pkg__Engagement_Plan__c"},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Task.pkg__Engagement_Plan__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Event.pkg__Engagement_Plan__c")
}

func TestAnalyzeQuerySemanticsIncludesEventIsClosed(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Event> events = [SELECT Id, IsClosed FROM Event];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{}, schema.Schema{Objects: []schema.Object{{Name: "Event"}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Event.IsClosed")
}

func TestAnalyzeQuerySemanticsMergesExtensionFieldsOnDuplicateObjects(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<vend__Order__c> orders = [SELECT vend__Total__c, pkg__State__c FROM vend__Order__c];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "vend__Order__c",
			Fields: []schema.Field{
				{Name: "pkg__State__c", Type: "Picklist"},
			},
		},
		{
			Name: "vend__Order__c",
			Fields: []schema.Field{
				{Name: "vend__Total__c", Type: "Currency"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "pkg__State__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "vend__Total__c")
}

func TestAnalyzeQuerySemanticsAddsSystemFieldsToCustomObjects(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Thing__c> things = [SELECT Id, Name, Owner.Name, Owner.Type, CreatedDate, LastActivityDate, IsDeleted, CustomFlag__c, RecordType.Name FROM Thing__c ALL ROWS];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{
		{
			Name: "Thing__c",
			Fields: []schema.Field{
				{Name: "CustomFlag__c", Type: "Checkbox"},
				{Name: "RecordTypeId", Type: "Lookup"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Thing__c.Id")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Thing__c.Name")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "CreatedDate")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "LastActivityDate")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "IsDeleted")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Owner.Name")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Owner.Type")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "RecordType.Name")
}

func TestAnalyzeQuerySemanticsAcceptsSetupOwnerTypeOnHierarchySettings(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Settings__c> settings = [SELECT SetupOwner.Name, SetupOwner.Type FROM Settings__c];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{
		Name:               "Settings__c",
		CustomSettingsType: "Hierarchy",
		Fields: []schema.Field{
			{Name: "SetupOwnerId", Type: "Lookup"},
		},
	}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "SetupOwner.Name")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "SetupOwner.Type")
}

func TestAnalyzeQuerySemanticsAddsFeatureAndMetadataStandardFields(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [
      SELECT FirstName, LastName, IsPersonAccount, PersonContactId,
             PersonEmail, PersonBirthdate, PersonMailingStreet,
             PersonMailingAddress, PersonIndividual.Name, PersonAlias__c
      FROM Account
    ];
    List<Feature__mdt> features = [SELECT DeveloperName, NamespacePrefix, QualifiedAPIName FROM Feature__mdt];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{Name: "Account", Fields: []schema.Field{{Name: "pkg__PersonAlias__c", Type: "Text"}}},
		{Name: "Feature__mdt"},
	}})

	for _, field := range []string{
		"FirstName", "LastName", "IsPersonAccount", "PersonContactId",
		"PersonEmail", "PersonBirthdate", "PersonMailingStreet", "PersonMailingAddress",
		"PersonIndividual.Name", "PersonAlias__c",
		"DeveloperName", "NamespacePrefix", "QualifiedAPIName",
	} {
		assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", field)
	}
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "PersonIndividual.Name")
}

func TestLayeredModelCutoverQueryAcceptsFeatureFieldsAndRejectsCrossObjectLeak(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [
      SELECT PersonContactId, FirstName, PersonDoNotCall,
             BillingCountryCode, ShippingStateCode, CurrencyIsoCode
      FROM Account
    ];
    List<Group> groups = [SELECT PersonContactId FROM Group];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "BillingCountryCode", Type: "Text"},
				{Name: "ShippingStateCode", Type: "Text"},
				{Name: "CurrencyIsoCode", Type: "Text"},
			},
		},
		{Name: "Group", Fields: []schema.Field{{Name: "Id", Type: "Id"}}},
	}})

	for _, field := range []string{
		"PersonContactId", "FirstName", "PersonDoNotCall",
		"BillingCountryCode", "ShippingStateCode", "CurrencyIsoCode",
	} {
		assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "Account."+field)
	}
	for _, item := range result.Diagnostics {
		if item.Code == "GLADESEMA_QUERY_FIELD" && strings.Contains(item.Message, "CurrencyIsoCode") {
			t.Fatalf("declared multi-currency field produced a query diagnostic: %#v", item)
		}
	}
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_FIELD", "PersonContactId", 9, 34)
}

func TestAnalyzeQuerySemanticsRejectsOwnerOnCustomMetadata(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Feature__mdt> features = [SELECT Owner.Name FROM Feature__mdt];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{Name: "Feature__mdt"}}})
	for _, diag := range result.Diagnostics {
		if diag.Code == "GLADESEMA_QUERY_RELATIONSHIP" && strings.Contains(diag.Message, "Owner.Name") {
			return
		}
	}
	t.Fatalf("missing custom-metadata Owner relationship diagnostic: %#v", result.Diagnostics)
}

func TestAnalyzeQuerySemanticsUsesStandardChildRelationshipFallback(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT Id, (SELECT Id, LastName FROM Contacts) FROM Account];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{Name: "Account"}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "Contacts")
}

func TestAnalyzeQuerySemanticsUsesProjectReferencedPackageFieldShape(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run(Id entityId) {
    pkg__Entity__c entityRecord = new pkg__Entity__c();
    pkg__EntityPaymentGatewayLink__c gatewayLink = new pkg__EntityPaymentGatewayLink__c(
      pkg__Entity__c = entityRecord.Id
    );
    pkg__ProcessingFee__c fee = new pkg__ProcessingFee__c(
      pkg__PaymentMethodType__c = 'Credit Card',
      pkg__Mandatory__c = false,
      pkg__EntityPaymentGatewayLink__c = gatewayLink.Id
    );
    List<pkg__ProcessingFee__c> fees = [
      SELECT Id, pkg__PaymentMethodType__c, pkg__Mandatory__c
      FROM pkg__ProcessingFee__c
      WHERE pkg__EntityPaymentGatewayLink__r.pkg__Entity__r.Id = :entityId
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "pkg"}, schema.Schema{Objects: []schema.Object{
		{Name: "pkg__ProcessingFee__c"},
		{Name: "pkg__EntityPaymentGatewayLink__c"},
		{Name: "pkg__Entity__c"},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "pkg__PaymentMethodType__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "pkg__Mandatory__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "pkg__EntityPaymentGatewayLink__r.pkg__Entity__r.Id")
}

func TestAnalyzeQuerySemanticsLeavesExternalManagedPackageShapeOpen(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<vend__OrderItem__c> rows = [
      SELECT Id, vend__MembershipType2__r.vend__Term__c
      FROM vend__OrderItem__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "localpkg"}, schema.Schema{Objects: []schema.Object{
		{
			Name: "vend__OrderItem__c",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "vend__MembershipType2__c", Type: "Lookup", ReferenceTo: []string{"vend__MembershipType2__c"}, RelationshipName: "vend__MembershipType2__r"},
			},
		},
		{
			Name: "vend__MembershipType2__c",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
			},
		},
	}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "vend__MembershipType2__r.vend__Term__c")
}

func TestAnalyzeQuerySemanticsKeepsLocalManagedPackageShapeStrict(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<localpkg__OrderItem__c> rows = [
      SELECT Id, localpkg__Missing__c
      FROM localpkg__OrderItem__c
    ];
  }
}
`
	result := analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{Namespace: "localpkg"}, schema.Schema{Objects: []schema.Object{{
		Name: "localpkg__OrderItem__c",
		Fields: []schema.Field{
			{Name: "Id", Type: "Id"},
		},
	}}})

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_FIELD", "localpkg__Missing__c", 5, 18)
}

func TestQuerySemanticsEmailMessageParentPrefersStandardCase(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<EmailMessage> rows = [
      SELECT Subject, parent.Status, parent.caseNumber, parent.Contact.Name, parent.IsEscalated, parent.Subject
      FROM EmailMessage
    ];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{
		Name: "Case",
		Fields: []schema.Field{
			{Name: "HVEMPreviousQueue__c", Type: "Text"},
		},
	}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "parent.Status")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "parent.caseNumber")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_RELATIONSHIP", "parent.Contact.Name")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "parent.IsEscalated")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "parent.Subject")
}

func TestQuerySemanticsKeepsInferredObjectsPartial(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run(Id caseId) {
    List<ASR_Survey_Log__c> logs = [
      SELECT Id
      FROM ASR_Survey_Log__c
      WHERE Case__c = :caseId
    ];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_OBJECT", "ASR_Survey_Log__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "ASR_Survey_Log__c.Case__c")
}

func TestAnalyzeQuerySemanticsIgnoresSOQLComments(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Event__c> events = [
      SELECT
        Name,
        // HiddenRevenue__c,
        /* HiddenCost__c, */
        TotalRevenue__c
      FROM Event__c
    ];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{
		Name: "Event__c",
		Fields: []schema.Field{
			{Name: "Name", Type: "Text"},
			{Name: "TotalRevenue__c", Type: "Currency"},
		},
	}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "HiddenRevenue__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "HiddenCost__c")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "TotalRevenue__c")
}

func TestAnalyzeQuerySemanticsAcceptsLocationComponentFields(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Account> accounts = [SELECT PrimaryLocation__Latitude__s, PrimaryLocation__Longitude__s FROM Account];
  }
}
`
	result := analyzeQueryProbe(t, source, schema.Schema{Objects: []schema.Object{{
		Name: "Account",
		Fields: []schema.Field{
			{Name: "PrimaryLocation__c", Type: "Location"},
		},
	}}})

	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "PrimaryLocation__Latitude__s")
	assertNoDiagnosticContaining(t, result, "GLADESEMA_QUERY_FIELD", "PrimaryLocation__Longitude__s")
}

func TestAnalyzeSOQLTypeofBranchObjectDiagnostics(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<Event> rows = [SELECT TYPEOF What WHEN Missing__c THEN Name END FROM Event];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_OBJECT", "Missing__c", 4, 49)
}

func TestAnalyzeSOSLReturningFieldDiagnostics(t *testing.T) {
	t.Parallel()
	source := `
public class QueryProbe {
  public void run() {
    List<List<SObject>> rows = [FIND 'acme' RETURNING Account(Id, Missing__c), Missing__c(Id)];
  }
}
`
	result := analyzeQueryProbe(t, source, queryDiagnosticSchema())

	assertDiagnosticAt(t, result, "GLADESEMA_SOSL_FIELD", "Missing__c", 4, 67)
	assertDiagnosticAt(t, result, "GLADESEMA_QUERY_OBJECT", "Missing__c", 4, 80)
}

func analyzeQueryProbe(t *testing.T, source string, sch schema.Schema) Result {
	t.Helper()
	return analyzeQueryProbeWithProject(t, source, typesys.ProjectInfo{}, sch)
}

func analyzeQueryProbeWithProject(t *testing.T, source string, info typesys.ProjectInfo, sch schema.Schema) Result {
	t.Helper()
	root := t.TempDir()
	classPath := filepath.Join(root, "QueryProbe.cls")
	writeSemaFile(t, classPath, source)
	index := typesys.Build(project.Project{Root: root, ApexFiles: []string{classPath}}, sch)
	index.Project = info
	return Analyze(index)
}

func queryDiagnosticSchema() schema.Schema {
	return schema.Schema{Objects: []schema.Object{
		{
			Name: "Account",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "Text"},
				{Name: "OwnerId", Type: "Lookup", RelationshipName: "Owner", ReferenceTo: []string{"User"}},
			},
		},
		{
			Name: "Contact",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "LastName", Type: "Text"},
				{Name: "AccountId", Type: "Lookup", ChildRelationshipName: "Contacts", ReferenceTo: []string{"Account"}},
			},
		},
		{
			Name: "Event",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "WhatId", Type: "Lookup", RelationshipName: "What", ReferenceTo: []string{"Account", "Contact"}},
			},
		},
		{
			Name: "User",
			Fields: []schema.Field{
				{Name: "Id", Type: "Id"},
				{Name: "Name", Type: "Text"},
			},
		},
	}}
}

func assertDiagnosticAt(t *testing.T, result Result, code, text string, line, column int) diagnostic.Diagnostic {
	t.Helper()
	var candidates []string
	for _, diag := range result.Diagnostics {
		if diag.Code != code || diag.Range == nil || !containsString(diag.Message, text) {
			continue
		}
		candidates = append(candidates, fmt.Sprintf("%s %d:%d %s", diag.Code, diag.Range.Start.Line, diag.Range.Start.Column, diag.Message))
		if diag.Range.Start.Line == line && diag.Range.Start.Column == column && containsString(diag.Message, text) {
			return diag
		}
	}
	t.Fatalf("missing diagnostic code=%s text=%q at %d:%d candidates=%#v all=%#v", code, text, line, column, candidates, result.Diagnostics)
	return diagnostic.Diagnostic{}
}

func assertNoDiagnosticContaining(t *testing.T, result Result, code, text string) {
	t.Helper()
	for _, diag := range result.Diagnostics {
		if diag.Code == code && containsString(diag.Message, text) {
			t.Fatalf("unexpected diagnostic code=%s text=%q in %#v", code, text, result.Diagnostics)
		}
	}
}

func containsString(s, sub string) bool {
	return len(sub) == 0 || (len(sub) <= len(s) && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
