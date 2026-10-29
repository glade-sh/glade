package vm

import (
	"strings"
	"testing"
)

// Regression controls for bind provenance during COUNT route validation.
// Ordinary queries use lexical scope; only WithBinds uses the supplied map.
func TestScalarCountRouteBindSources(t *testing.T) {
	const countError = "use countQuery() for [select count()...] queries"
	for _, api := range []string{"62.0", "67.0"} {
		for _, named := range []bool{false, true} {
			for _, tc := range []struct {
				name, declaration, body string
				message                 string
				missingMapKey           bool
			}{
				{
					name: "locatorLocalSet",
					body: "Decimal ceiling=200;Database.getQueryLocator('SELECT COUNT() FROM Opportunity WHERE AccountId IN :parents AND Amount < :ceiling');",
				},
				{
					name:        "locatorLocalList",
					declaration: "List<Id> parents=new List<Id>{'001000000000099'};",
					body:        "Database.getQueryLocator('SELECT COUNT() FROM Opportunity WHERE AccountId IN :parents');",
				},
				{
					name:        "locatorEmptyList",
					declaration: "List<Id> parents=new List<Id>();",
					body:        "Database.getQueryLocator('SELECT COUNT() FROM Opportunity WHERE AccountId IN :parents');",
				},
				{
					name:    "locatorLiteralEmptyList",
					body:    "Database.getQueryLocator('SELECT COUNT() FROM Opportunity WHERE AccountId IN ()');",
					message: "unexpected token: 'AccountId IN ()'",
				},
				{
					name: "locatorLocalScalar",
					body: "String wanted='Absent';Database.getQueryLocator('SELECT COUNT() FROM Opportunity WHERE Name = :wanted');",
				},
				{
					name: "locatorInline",
					body: "Decimal ceiling=200;Database.getQueryLocator([SELECT COUNT() FROM Opportunity WHERE AccountId IN :parents AND Amount < :ceiling]);",
				},
				{
					name: "queryLocal",
					body: "Decimal ceiling=200;Database.query('SELECT COUNT() FROM Opportunity WHERE AccountId IN :parents AND Amount < :ceiling');",
				},
				{
					name: "queryMapOnly",
					body: "Database.queryWithBinds('SELECT COUNT() FROM Opportunity WHERE Amount < :ceiling',new Map<String,Object>{'ceiling'=>200},AccessLevel.USER_MODE);",
				},
				{
					name: "queryMapShadowsLocal",
					body: "String ceiling='invalid';Database.queryWithBinds('SELECT COUNT() FROM Opportunity WHERE Amount < :ceiling',new Map<String,Object>{'ceiling'=>200},AccessLevel.USER_MODE);",
				},
				{
					name:          "queryMapDoesNotFallBackToLocal",
					body:          "Decimal ceiling=200;Database.queryWithBinds('SELECT COUNT() FROM Opportunity WHERE Amount < :ceiling',new Map<String,Object>(),AccessLevel.USER_MODE);",
					missingMapKey: true,
				},
			} {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/"+tc.name, func(t *testing.T) {
					declaration := tc.declaration
					if declaration == "" {
						declaration = "Set<Id> parents=new Set<Id>();"
					}
					err := executeLocatorPrecedenceBody(t, emptyReferenceSumLocatorMachine(api), api, named, declaration, tc.body)
					if tc.missingMapKey {
						assertLocatorPrecedenceException(t, err, "queryException")
						if !strings.Contains(err.Error(), "bindMap") || !strings.Contains(err.Error(), "ceiling") {
							t.Fatalf("expected missing map key instead of local scope fallback: %v", err)
						}
						return
					}
					message := tc.message
					if message == "" {
						message = countError
					}
					assertLocatorPrecedenceException(t, err, message)
				})
			}
		}
	}
}
