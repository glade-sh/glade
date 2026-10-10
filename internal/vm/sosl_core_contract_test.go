package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestSOSLReturningWhereSignedDecimalsAndNumericOrdering(t *testing.T) {
	program, err := CompileAnonymous(`
Account low = new Account(Name = 'Numeric Low', AnnualRevenue = -1.25);
Account middle = new Account(Name = 'Numeric Middle', AnnualRevenue = 2);
Account high = new Account(Name = 'Numeric High', AnnualRevenue = 10.5);
insert new List<Account>{low, middle, high};
Test.setFixedSearchResults(new List<Id>{low.Id, middle.Id, high.Id});
List<Account> literals = (List<Account>)Search.query(
 'FIND {candidate} RETURNING Account(Id, Name WHERE (AnnualRevenue >= -1.25 AND AnnualRevenue < 0) OR AnnualRevenue >= 10.5)'
)[0];
Set<Id> literalIds = new Set<Id>();
for (Account row : literals) literalIds.add(row.Id);
System.assertEquals(new Set<Id>{low.Id, high.Id}, literalIds);

Decimal lower = -1.25;
Decimal upper = 2.5;
List<Account> bound = (List<Account>)Search.query(
 'FIND {candidate} RETURNING Account(Id, Name WHERE (AnnualRevenue >= :lower AND AnnualRevenue < :upper))'
)[0];
Set<Id> boundIds = new Set<Id>();
for (Account row : bound) boundIds.add(row.Id);
System.assertEquals(new Set<Id>{low.Id, middle.Id}, boundIds);

List<Account> numeric = (List<Account>)Search.query(
 'FIND {candidate} RETURNING Account(Id, Name WHERE AnnualRevenue < 10)'
)[0];
Set<Id> numericIds = new Set<Id>();
for (Account row : numeric) numericIds.add(row.Id);
System.assertEquals(new Set<Id>{low.Id, middle.Id}, numericIds);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSOSLFindBooleanAndEscapedWildcardUsesOrgBackedSearch(t *testing.T) {
	program, err := CompileAnonymous(`
insert new List<Account>{
	new Account(Name = 'Red Alpha'),
	new Account(Name = 'Red Beta'),
	new Account(Name = 'Blue Alpha'),
	new Account(Name = 'Literal*Token'),
	new Account(Name = 'LiteralXToken')
};

List<Account> negative = (List<Account>)Search.query(
	'FIND {Red AND NOT Beta} RETURNING Account(Id, Name)'
)[0];
System.assertEquals(1, negative.size());
System.assertEquals('Red Alpha', negative[0].Name);

List<Account> associated = (List<Account>)Search.query(
	'FIND {Red AND NOT Blue AND Alpha} RETURNING Account(Id, Name)'
)[0];
Set<String> associatedNames = new Set<String>();
for (Account row : associated) associatedNames.add(row.Name);
System.assertEquals(new Set<String>{'Red Alpha', 'Red Beta'}, associatedNames);

List<Account> grouped = (List<Account>)Search.query(
	'FIND {(Red OR Blue) AND Alpha} RETURNING Account(Id, Name)'
)[0];
Set<String> groupedNames = new Set<String>();
for (Account row : grouped) groupedNames.add(row.Name);
System.assertEquals(new Set<String>{'Blue Alpha', 'Red Alpha'}, groupedNames);

List<Account> literal = (List<Account>)Search.query(
	'FIND {Literal\\*Token} RETURNING Account(Id, Name)'
)[0];
System.assertEquals(1, literal.size());
System.assertEquals('Literal*Token', literal[0].Name);

List<Account> wildcard = (List<Account>)Search.query(
	'FIND {Literal*} RETURNING Account(Id, Name)'
)[0];
Set<String> wildcardNames = new Set<String>();
for (Account row : wildcard) wildcardNames.add(row.Name);
System.assertEquals(new Set<String>{'Literal*Token', 'LiteralXToken'}, wildcardNames);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestSOSLReturningWhereComparisonsAndGroupsUseFixedCandidates(t *testing.T) {
	program, err := CompileAnonymous(`
Account alpha = new Account(Name = 'Alpha');
Account beta = new Account(Name = 'Beta');
Account gamma = new Account(Name = 'Gamma');
insert new List<Account>{alpha, beta, gamma};
Test.setFixedSearchResults(new List<Id>{alpha.Id, beta.Id, gamma.Id});

List<Account> less = (List<Account>)Search.query(
	'FIND {candidate} RETURNING Account(Id, Name WHERE Name < ''Beta'' ORDER BY Name)'
)[0];
System.assertEquals(1, less.size());
System.assertEquals('Alpha', less[0].Name);

List<Account> lessEqual = (List<Account>)Search.query(
	'FIND {candidate} RETURNING Account(Id, Name WHERE Name <= ''Beta'' ORDER BY Name)'
)[0];
System.assertEquals(2, lessEqual.size());
System.assertEquals('Alpha', lessEqual[0].Name);
System.assertEquals('Beta', lessEqual[1].Name);

List<Account> greater = (List<Account>)Search.query(
	'FIND {candidate} RETURNING Account(Id, Name WHERE Name > ''Beta'' ORDER BY Name)'
)[0];
System.assertEquals(1, greater.size());
System.assertEquals('Gamma', greater[0].Name);

List<Account> greaterEqual = (List<Account>)Search.query(
	'FIND {candidate} RETURNING Account(Id, Name WHERE Name >= ''Beta'' ORDER BY Name)'
)[0];
System.assertEquals(2, greaterEqual.size());
System.assertEquals('Beta', greaterEqual[0].Name);
System.assertEquals('Gamma', greaterEqual[1].Name);

List<Account> grouped = (List<Account>)Search.query(
	'FIND {candidate} RETURNING Account(Id, Name WHERE (Name = ''Alpha'' OR Name = ''Gamma'') AND Name != ''Gamma'')'
)[0];
System.assertEquals(1, grouped.size());
System.assertEquals('Alpha', grouped[0].Name);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := storage.NewOrgState()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	machine.EnableTestContext()
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
