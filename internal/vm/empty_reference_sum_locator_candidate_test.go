package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// This is a local compatibility candidate, not a Salesforce assertion. Native
// construction and iteration observations for the empty reference bind are
// pending. Before the candidate, Execute returns the aggregate QueryException.
func TestEmptyReferenceSumLocatorCandidateConstruction(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			name := api + "/anonymous"
			if named {
				name = api + "/named"
			}
			t.Run(name, func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine(api)
				body := `
Set<Id> parents = new Set<Id>();
String text = 'SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND Amount > 200 GROUP BY AccountId';
Database.QueryLocator handle = Database.getQueryLocator(text);
Assert.isNotNull(handle, 'Local construction candidate');
Assert.areEqual(text, handle.getQuery());
`
				if named {
					program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterClass(Class{Name: "EmptyHandleConstruction", Methods: map[string]Method{
						"observe": {Name: "EmptyHandleConstruction.observe", ClassName: "EmptyHandleConstruction", IsStatic: true, APIVersion: api, Program: program},
					}}); err != nil {
						t.Fatal(err)
					}
					machine.EnableTestContext()
					body = "EmptyHandleConstruction.observe();"
				}
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				if _, err := machine.Execute(program); err != nil {
					t.Fatalf("local construction candidate failed: %v", err)
				}
			})
		}
	}
}

// Scope invariants retain the prior rejection path. They are not native answers
// for filtered-out, deleted, null, erased, iteration, or other aggregate inputs.
func TestEmptyReferenceSumLocatorCandidateScope(t *testing.T) {
	const grouped = "SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND Amount > 200 GROUP BY AccountId"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct {
			name, declaration, query, call string
		}{
			{"nonemptyUnmatched", "Set<Id> parents=new Set<Id>{'001000000000099'};", grouped, "Database.getQueryLocator(text)"},
			{"filteredNonempty", "Account parent=new Account(Name='Synthetic parent');insert parent;Set<Id> parents=new Set<Id>{parent.Id};", grouped, "Database.getQueryLocator(text)"},
			{"matchingNonempty", "Account parent=new Account(Name='Synthetic parent');insert parent;insert new Opportunity(Name='Synthetic child',StageName='Prospecting',CloseDate=Date.newInstance(2024,2,3),Amount=500,AccountId=parent.Id);Set<Id> parents=new Set<Id>{parent.Id};", grouped, "Database.getQueryLocator(text)"},
			{"deletedNonempty", "Account parent=new Account(Name='Synthetic parent');insert parent;Set<Id> parents=new Set<Id>{parent.Id};delete parent;", grouped, "Database.getQueryLocator(text)"},
			{"emptyList", "List<Id> parents=new List<Id>();", grouped, "Database.getQueryLocator(text)"},
			{"emptyStringSet", "Set<String> parents=new Set<String>();", grouped, "Database.getQueryLocator(text)"},
			{"nullIdSet", "Set<Id> parents;", grouped, "Database.getQueryLocator(text)"},
			{"nullMember", "Set<Id> parents=new Set<Id>{null};", grouped, "Database.getQueryLocator(text)"},
			{"groupedCount", "Set<Id> parents=new Set<Id>();", strings.Replace(grouped, "SUM(Amount)", "COUNT(Id)", 1), "Database.getQueryLocator(text)"},
			{"ungroupedSum", "Set<Id> parents=new Set<Id>();", "SELECT SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND Amount > 200", "Database.getQueryLocator(text)"},
			{"missingAmountPredicate", "Set<Id> parents=new Set<Id>();", "SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents GROUP BY AccountId", "Database.getQueryLocator(text)"},
			{"literalEmpty", "Set<Id> parents=new Set<Id>();", strings.Replace(grouped, ":parents", "()", 1), "Database.getQueryLocator(text)"},
			{"cursor", "Set<Id> parents=new Set<Id>();", grouped, "Database.getCursor(text)"},
			{"explicitAccess", "Set<Id> parents=new Set<Id>();", grouped, "Database.getQueryLocator(text,AccessLevel.SYSTEM_MODE)"},
			{"mapBinds", "Set<Id> parents=new Set<Id>();Map<String,Object> values=new Map<String,Object>{'parents'=>parents};", grouped, "Database.getQueryLocatorWithBinds(text,values,AccessLevel.SYSTEM_MODE)"},
			{"forUpdate", "Set<Id> parents=new Set<Id>();", grouped + " FOR UPDATE", "Database.getQueryLocator(text)"},
			{"badField", "Set<Id> parents=new Set<Id>();", strings.Replace(grouped, "Amount >", "MissingMetric__c >", 1), "Database.getQueryLocator(text)"},
			{"badAmountType", "Set<Id> parents=new Set<Id>();", strings.Replace(grouped, "200", "'invalid'", 1), "Database.getQueryLocator(text)"},
			{"badSyntax", "Set<Id> parents=new Set<Id>();", strings.Replace(grouped, "Amount > 200", "Amount >", 1), "Database.getQueryLocator(text)"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine(api)
				text := strings.ReplaceAll(tc.query, "'", "\\'")
				program, err := CompileAnonymousWithOptions(tc.declaration+"String text='"+text+"';Object handle="+tc.call+";", CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != "System.QueryException" {
					t.Fatalf("out-of-scope input must remain rejected: %v", err)
				}
			})
		}
		// The admitted construction does not enable iteration. This is a local
		// boundary decision, not a captured native empty-iterator expectation.
		t.Run(api+"/constructionOnly", func(t *testing.T) {
			machine := emptyReferenceSumLocatorMachine(api)
			program, err := CompileAnonymousWithOptions("Set<Id> parents=new Set<Id>();String text='"+grouped+"';Database.QueryLocator handle=Database.getQueryLocator(text);handle.iterator();", CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != "System.QueryException" {
				t.Fatalf("iteration must remain outside the local candidate: %v", err)
			}
		})
	}
}

// Owned query_locators.json N015 and its cursor/FOR UPDATE controls establish
// these exact rejection messages at API62/67. API68 is not promoted here.
func TestEmptyReferenceSumLocatorKnownRejections(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, query, call, message string }{
			{"matchingGroupedCount", "SELECT NumberOfEmployees,COUNT(Id) n FROM Account WHERE Id IN :owned GROUP BY NumberOfEmployees", "Database.getQueryLocator(text)", "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"},
			{"matchingGroupedCursor", "SELECT NumberOfEmployees,COUNT(Id) n FROM Account WHERE Id IN :owned GROUP BY NumberOfEmployees", "Database.getCursor(text)", "Query contains aggregate expressions, these are not permitted in cursor queries"},
			{"ordinaryForUpdate", "SELECT Id,Name FROM Account WHERE Id IN :owned FOR UPDATE", "Database.getQueryLocator(text)", "You cannot lock records in Batch Apex using FOR UPDATE"},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine(api)
				body := "List<Account> seed=new List<Account>{new Account(Name='Synthetic alpha',NumberOfEmployees=0),new Account(Name='Synthetic beta',NumberOfEmployees=1),new Account(Name='Synthetic gamma',NumberOfEmployees=2),new Account(Name='Synthetic omega',NumberOfEmployees=null)};insert seed;Set<Id> owned=new Map<Id,Account>(seed).keySet();String text='" + tc.query + "';Object handle=" + tc.call + ";"
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(program)
				var runtimeErr *RuntimeError
				if !errors.As(err, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != "System.QueryException" || runtimeErr.ExceptionMessage() != tc.message {
					t.Fatalf("native rejection must remain unchanged: %v", err)
				}
			})
		}
	}
}

func emptyReferenceSumLocatorMachine(api string) *VM {
	org := storage.NewOrgState()
	org.APIVersion = api
	storage.EnsureDeterministicPlatformData(&org)
	storage.EnsureStandardObject(&org, "Opportunity")
	machine := New(nil)
	machine.SetOrg(&org)
	return machine
}
