package vm

import (
	"strings"
	"testing"
)

// All five additional construction forms are isolated local candidates. Native
// construction answers are pending; these tests do not promote acceptance.
func TestEmptyReferenceSumLocatorCriteriaFormsCandidate(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			for _, tc := range locatorCriteriaForms() {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/"+tc.name, func(t *testing.T) {
					machine := emptyReferenceSumLocatorMachine(api)
					query := locatorCriteriaFormQuery(tc.criterion)
					body := "Database.QueryLocator handle=Database.getQueryLocator('" + strings.ReplaceAll(query, "'", "\\'") + "');Assert.isNotNull(handle);"
					err := executeLocatorPrecedenceBody(t, machine, api, named, "Set<Id> parents=new Set<Id>();", body)
					if err != nil {
						t.Fatalf("local criteria construction candidate failed: %v", err)
					}
				})
			}
		}
	}
}

func TestEmptyReferenceSumLocatorCriteriaFormsOrdinaryValidator(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			for _, tc := range locatorCriteriaForms() {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/"+tc.name, func(t *testing.T) {
					machine := emptyReferenceSumLocatorMachine(api)
					query := strings.ReplaceAll(locatorCriteriaFormQuery(tc.criterion), "'", "\\'")
					body := "List<SObject> rows=Database.query('" + query + "');Assert.areEqual(0,rows.size());"
					if err := executeLocatorPrecedenceBody(t, machine, api, named, "Set<Id> parents=new Set<Id>();", body); err != nil {
						t.Fatalf("ordinary validator rejected the criteria: %v", err)
					}
				})
			}
		}
	}
}

// Preserve rejection for unsupported forms and invalid inputs. An admitted
// construction remains tagged for the existing iterator rejection boundary.
func TestEmptyReferenceSumLocatorCriteriaFormsControls(t *testing.T) {
	const aggregateError = "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			for _, tc := range []struct{ name, declaration, criterion, call, action, message string }{
				{"nonempty", "Set<Id> parents=new Set<Id>{'001000000000099'};", "Amount < 200", "Database.getQueryLocator(text)", "", aggregateError},
				{"list", "List<Id> parents=new List<Id>();", "Amount < 200", "Database.getQueryLocator(text)", "", aggregateError},
				{"numericWrongField", "Set<Id> parents=new Set<Id>();", "ExpectedRevenue < 200", "Database.getQueryLocator(text)", "", aggregateError},
				{"numericWrongOperator", "Set<Id> parents=new Set<Id>();", "Amount >= 200", "Database.getQueryLocator(text)", "", aggregateError},
				{"dateWrongLiteral", "Set<Id> parents=new Set<Id>();", "CloseDate < YESTERDAY", "Database.getQueryLocator(text)", "", aggregateError},
				{"dateWrongOperator", "Set<Id> parents=new Set<Id>();", "CloseDate = TODAY", "Database.getQueryLocator(text)", "", aggregateError},
				{"stringWrongType", "Set<Id> parents=new Set<Id>();", "Name = 'Synthetic'", "Database.getQueryLocator(text)", "", aggregateError},
				{"orWrongOrder", "Set<Id> parents=new Set<Id>();", "(Amount > 100 OR CloseDate < TODAY)", "Database.getQueryLocator(text)", "", aggregateError},
				{"orWrongOperator", "Set<Id> parents=new Set<Id>();", "(CloseDate < TODAY OR Amount < 100)", "Database.getQueryLocator(text)", "", aggregateError},
				{"orMissingParentheses", "Set<Id> parents=new Set<Id>();", "CloseDate < TODAY OR Amount > 100", "Database.getQueryLocator(text)", "", aggregateError},
				{"andUnsupported", "Set<Id> parents=new Set<Id>();", "(CloseDate < TODAY AND Amount > 100)", "Database.getQueryLocator(text)", "", aggregateError},
				{"badField", "Set<Id> parents=new Set<Id>();", "MissingDate < TODAY", "Database.getQueryLocator(text)", "", aggregateError},
				{"badLiteral", "Set<Id> parents=new Set<Id>();", "Amount < 'invalid'", "Database.getQueryLocator(text)", "", aggregateError},
				{"badSyntax", "Set<Id> parents=new Set<Id>();", "Amount <", "Database.getQueryLocator(text)", "", "queryException"},
				{"cursor", "Set<Id> parents=new Set<Id>();", "Amount < 200", "Database.getCursor(text)", "", "Query contains aggregate expressions, these are not permitted in cursor queries"},
				{"explicitAccess", "Set<Id> parents=new Set<Id>();", "Amount < 200", "Database.getQueryLocator(text,AccessLevel.SYSTEM_MODE)", "", aggregateError},
				{"mapBinds", "Set<Id> parents=new Set<Id>();Map<String,Object> values=new Map<String,Object>{'parents'=>parents};", "Amount < 200", "Database.getQueryLocatorWithBinds(text,values,AccessLevel.SYSTEM_MODE)", "", aggregateError},
				{"forUpdate", "Set<Id> parents=new Set<Id>();", "Amount < 200", "Database.getQueryLocator(text+' FOR UPDATE')", "", aggregateError},
			} {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/"+tc.name, func(t *testing.T) {
					machine := emptyReferenceSumLocatorMachine(api)
					query := strings.ReplaceAll(locatorCriteriaFormQuery(tc.criterion), "'", "\\'")
					body := "String text='" + query + "';Object handle=" + tc.call + ";" + tc.action
					declaration, methodBody := tc.declaration, body
					if tc.name == "mapBinds" {
						declaration = "Set<Id> parents=new Set<Id>();"
						methodBody = "Map<String,Object> values=new Map<String,Object>{'parents'=>parents};" + body
					}
					err := executeLocatorPrecedenceBody(t, machine, api, named, declaration, methodBody)
					assertLocatorPrecedenceException(t, err, tc.message)
				})
			}
			for _, tc := range locatorCriteriaForms() {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/iterator/"+tc.name, func(t *testing.T) {
					machine := emptyReferenceSumLocatorMachine(api)
					query := strings.ReplaceAll(locatorCriteriaFormQuery(tc.criterion), "'", "\\'")
					body := "Database.QueryLocator handle=Database.getQueryLocator('" + query + "');handle.iterator();"
					err := executeLocatorPrecedenceBody(t, machine, api, named, "Set<Id> parents=new Set<Id>();", body)
					assertLocatorPrecedenceException(t, err, aggregateError)
				})
			}
		}
	}
}

func locatorCriteriaForms() []struct{ name, criterion string } {
	return []struct{ name, criterion string }{
		{"dateBeforeToday", "CloseDate < TODAY"},
		{"dateOrAmount", "(CloseDate < TODAY OR Amount > 100)"},
		{"picklistNotEqual", "StageName != 'Prospecting'"},
		{"amountLessThan", "Amount < 200"},
		{"picklistEqual", "StageName = 'Prospecting'"},
	}
}

func locatorCriteriaFormQuery(criterion string) string {
	return "SELECT AccountId,SUM(Amount) total FROM Opportunity WHERE AccountId IN :parents AND " + criterion + " GROUP BY AccountId"
}
