package vm

import (
	"fmt"
	"strings"
	"testing"
)

func TestCompileTemporalQueryLiterals(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, literal := range []string{
			"2020-01-01T08:00:00Z",
			"2020-01-01T13:30:00+05:30",
			"2020-01-01T00:00:00-08:00",
			"2020-01-01T08:00:00.000Z",
		} {
			for _, query := range []string{
				"SELECT Id FROM Contact WHERE ObservedAt__c > " + literal,
				"SELECT Id FROM Contact WHERE LastName = :label AND ObservedAt__c IN (" + literal + ")",
				"FIND 'Temporal' IN ALL FIELDS RETURNING Contact(Id WHERE ObservedAt__c > " + literal + ")",
			} {
				t.Run(api+"/"+query, func(t *testing.T) {
					program, err := CompileAnonymousWithOptions("Object rows = ["+query+"];", CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if len(program.Instructions) != 1 || !strings.Contains(program.Instructions[0].Expr.Value, literal) {
						t.Fatalf("timestamp changed during query compilation: %#v", program.Instructions)
					}
					loop := fmt.Sprintf("for(Contact row : [SELECT Id FROM Contact WHERE ObservedAt__c > %s]) {}", literal)
					if _, err := CompileAnonymousWithOptions(loop, CompileOptions{APIVersion: api}); err != nil {
						t.Fatal(err)
					}
				})
			}
		}
	}
}

func TestTemporalQueryLiteralDoesNotChangeApexNumbers(t *testing.T) {
	for _, source := range []string{
		"Datetime value = 2020-01-01T08:00:00Z;",
		"Object rows = [SELECT Id FROM Contact WHERE Id = null]; Datetime value = 2020-01-01T08:00:00Z;",
		"Object rows = new List<Integer>{1}[0]; Datetime value = 2020-01-01T08:00:00Z;",
	} {
		if _, err := CompileAnonymous(source); err == nil {
			t.Fatalf("DateTime token accepted outside a query: %s", source)
		}
	}
}
