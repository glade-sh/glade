package vm

import (
	"errors"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/soql"
)

// Native bad-column versus aggregate-locator precedence remains unknown. This
// regression describes an isolated local candidate, not an accepted oracle.
func TestEmptyReferenceSumLocatorMissingCriterionPrecedenceCandidate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			t.Run(api+"/"+locatorPrecedenceRoute(named), func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine(api)
				err := executeLocatorPrecedenceBody(t, machine, api, named,
					"Set<Id> parents=new Set<Id>();",
					"Database.getQueryLocator('SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND MissingCriterion = \\'Synthetic\\' GROUP BY AccountId');")
				assertLocatorPrecedenceException(t, err, soql.MissingColumnMessage("MissingCriterion", "Opportunity"))
			})
		}
	}
}

// These scope controls retain prior errors/admission. API68 and the pending
// empty-SUM shape are local invariants, not additional native assertions.
func TestEmptyReferenceSumLocatorPrecedenceControls(t *testing.T) {
	const prefix = "SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND "
	const suffix = " GROUP BY AccountId"
	const aggregateError = "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			for _, tc := range []struct {
				name, declaration, query, action, message string
			}{
				{"validNoneligible", "Set<Id> parents=new Set<Id>();", prefix + "Amount >= 200" + suffix, "", aggregateError},
				{"eligibleEmpty", "Set<Id> parents=new Set<Id>();", prefix + "Amount > 200" + suffix, "Assert.isNotNull(handle);", ""},
				{"eligibleIterator", "Set<Id> parents=new Set<Id>();", prefix + "Amount > 200" + suffix, "handle.iterator();", aggregateError},
				{"nonemptyMissing", "Set<Id> parents=new Set<Id>{'001000000000099'};", prefix + "MissingCriterion = 'Synthetic'" + suffix, "", aggregateError},
				{"listMissing", "List<Id> parents=new List<Id>();", prefix + "MissingCriterion = 'Synthetic'" + suffix, "", aggregateError},
				{"numericMissing", "Set<Id> parents=new Set<Id>();", prefix + "MissingCriterion > 200" + suffix, "", aggregateError},
				{"malformedLiteral", "Set<Id> parents=new Set<Id>();", prefix + "Amount > 'invalid'" + suffix, "", aggregateError},
				{"malformedSyntax", "Set<Id> parents=new Set<Id>();", prefix + "Amount >" + suffix, "", "queryException"},
				{"ordinaryMissing", "Set<Id> parents=new Set<Id>();", "SELECT Id FROM Opportunity WHERE AccountId IN :parents AND MissingCriterion = 'Synthetic'", "", soql.MissingColumnMessage("MissingCriterion", "Opportunity")},
			} {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/"+tc.name, func(t *testing.T) {
					machine := emptyReferenceSumLocatorMachine(api)
					query := strings.ReplaceAll(tc.query, "'", "\\'")
					body := "Database.QueryLocator handle=Database.getQueryLocator('" + query + "');" + tc.action
					err := executeLocatorPrecedenceBody(t, machine, api, named, tc.declaration, body)
					if tc.message == "" {
						if err != nil {
							t.Fatalf("existing construction candidate changed: %v", err)
						}
					} else {
						assertLocatorPrecedenceException(t, err, tc.message)
					}
				})
			}
		}
	}
}

func executeLocatorPrecedenceBody(t *testing.T, machine *VM, api string, named bool, declaration, body string) error {
	t.Helper()
	if named {
		params := []Param{{Name: "parents", Type: "Set<Id>"}}
		if strings.HasPrefix(declaration, "List<Id>") {
			params[0].Type = "List<Id>"
		}
		program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
		if err != nil {
			t.Fatal(err)
		}
		if err := machine.RegisterClass(Class{Name: "LocatorCriteriaProbe", APIVersion: api, IsTest: true, Methods: map[string]Method{
			"observe": {Name: "LocatorCriteriaProbe.observe", ClassName: "LocatorCriteriaProbe", APIVersion: api, IsStatic: true, Params: params, Program: program},
		}}); err != nil {
			t.Fatal(err)
		}
		machine.EnableTestContext()
		body = "LocatorCriteriaProbe.observe(parents);"
	}
	program, err := CompileAnonymousWithOptions(declaration+body, CompileOptions{APIVersion: api})
	if err != nil {
		t.Fatal(err)
	}
	_, err = machine.Execute(program)
	return err
}

func locatorPrecedenceRoute(named bool) string {
	if named {
		return "istest"
	}
	return "anonymous"
}

func assertLocatorPrecedenceException(t *testing.T, err error, message string) {
	t.Helper()
	var runtimeErr *RuntimeError
	if !errors.As(err, &runtimeErr) || exceptionQualifiedTypeName(runtimeErr.Type) != "System.QueryException" {
		t.Fatalf("expected local QueryException: %v", err)
	}
	if message != "queryException" && runtimeErr.ExceptionMessage() != message {
		t.Fatalf("local candidate message = %q, want %q", runtimeErr.ExceptionMessage(), message)
	}
}
