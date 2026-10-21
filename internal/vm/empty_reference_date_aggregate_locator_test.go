package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The parsed aggregate and schema types, not just the predicate text, explain
// why replacing every projection with SUM(Amount) was an incomplete local proof.
func TestEmptyReferenceDateAggregateLocatorInputShape(t *testing.T) {
	for _, tc := range dateAggregateLocatorShapes() {
		t.Run(tc.name, func(t *testing.T) {
			machine := emptyReferenceSumLocatorMachine("67.0")
			program, err := CompileAnonymous("Set<Id> parents=new Set<Id>();parents=parents.clone();")
			if err != nil {
				t.Fatal(err)
			}
			if _, err = machine.Execute(program); err != nil {
				t.Fatal(err)
			}
			query, err := machine.parseSOQLAt(dateAggregateLocatorQuery(tc.aggregate, tc.criterion))
			if err != nil {
				t.Fatal(err)
			}
			if query.Count || len(query.Aggregates) != 1 || query.Aggregates[0].Field != "CloseDate" || len(query.GroupBy) != 1 || query.GroupBy[0] != "AccountId" || query.Where == nil || len(query.Where.And) != 2 {
				t.Fatalf("unexpected aggregate AST: %+v", query)
			}
			membership, criterion := query.Where.And[0], query.Where.And[1]
			if len(membership.Values) != 1 || membership.Values[0].Kind != storage.ValueID || string(membership.Values[0].ID) != ":parents" {
				t.Fatalf("unexpected bind AST: %+v", membership)
			}
			value := machine.Globals["parents"]
			if machine.VarTypes["parents"] != "Set<Id>" || value.Kind != ValueSet || len(value.Set) != 0 {
				t.Fatalf("cloned bind lost declared view: %q, %v", machine.VarTypes["parents"], value)
			}
			definition := machine.Org.Objects["Opportunity"].Definition
			if definition.Fields["CloseDate"].Type != storage.FieldDate || definition.Fields["Amount"].Type != storage.FieldDecimal || definition.Fields["AccountId"].Type != storage.FieldReference {
				t.Fatal("unexpected schema types")
			}
			t.Logf("aggregate=%s(%s), Count=%t, bind=%s/empty, criterion parenthesized=%t field=%s op=%s range=%t orBranches=%d", query.Aggregates[0].Func, query.Aggregates[0].Field, query.Count, machine.VarTypes["parents"], criterion.Parenthesized, criterion.Field, criterion.Op, criterion.Range, len(criterion.Or))
			body := "parents=parents.clone();Assert.areEqual(0,Database.query('" + dateAggregateLocatorQuery(tc.aggregate, tc.criterion) + "').size());"
			if err := executeLocatorPrecedenceBody(t, machine, "67.0", true, "Set<Id> parents=new Set<Id>();", body); err != nil {
				t.Fatalf("ordinary query validator: %v", err)
			}
		})
	}
}

// These retain local scope and validator boundaries, without claiming native
// acceptance for an aggregate constructor or changing its error precedence.
func TestEmptyReferenceDateAggregateLocatorControls(t *testing.T) {
	const aggregateError = "Aggregate query does not support queryMore(), use LIMIT to restrict the results to a single batch"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, named := range []bool{false, true} {
			for _, tc := range []struct{ name, aggregate, criterion, suffix, call, message string }{
				{"minimum", "MIN(CloseDate)", "CloseDate < TODAY", "", "", ""},
				{"average", "AVG(Amount)", "Amount < 200", "", "", ""},
				{"maximumDecimal", "MAX(Amount)", "Amount < 200", "", "", ""},
				{"maximumDatetime", "MAX(CreatedDate)", "CloseDate < TODAY", "", "", ""},
				{"countId", "COUNT(Id)", "Amount < 200", "", "", ""},
				{"countDecimal", "COUNT(Amount)", "Amount < 200", "", "", ""},
				{"countDistinct", "COUNT_DISTINCT(CloseDate)", "Amount < 200", "", "", ""},
				{"countNoField", "COUNT()", "Amount < 200", "", "", "use countQuery() for [select count()...] queries"},
				{"dateLiteral", "MAX(CloseDate)", "CloseDate < YESTERDAY", "", "", ""},
				{"dateOperator", "MAX(CloseDate)", "CloseDate <= TODAY", "", "", ""},
				{"dateWrongField", "MAX(CloseDate)", "CreatedDate < TODAY", "", "", ""},
				{"dateOtherDate", "MAX(CloseDate)", "LastActivityDate < TODAY", "", "", ""},
				{"unparenthesized", "MAX(CloseDate)", "CloseDate < TODAY", "", "", ""},
				{"groupId", "MAX(CloseDate)", "CloseDate < TODAY", "", "", ""},
				{"dateMissing", "MAX(CloseDate)", "MissingDate < TODAY", "", "", ""},
				{"dateString", "MAX(CloseDate)", "CloseDate < 'invalid'", "", "", ""},
				{"numberString", "COUNT(CloseDate)", "Amount < 'invalid'", "", "", ""},
				{"numberOperator", "COUNT(CloseDate)", "Amount >= 200", "", "", ""},
				{"numberMissing", "COUNT(CloseDate)", "MissingNumber < 200", "", "", ""},
				{"orReversed", "MAX(CloseDate)", "Amount > 100 OR CloseDate < TODAY", "", "", ""},
				{"orWrongOperator", "MAX(CloseDate)", "CloseDate < TODAY OR Amount < 100", "", "", ""},
				{"andNested", "MAX(CloseDate)", "CloseDate < TODAY AND Amount > 100", "", "", ""},
				{"orExtra", "MAX(CloseDate)", "CloseDate < TODAY OR Amount > 100 OR Amount < 0", "", "", ""},
				{"malformedSyntax", "MAX(CloseDate)", "CloseDate <", "", "", "queryException"},
				{"multipleAggregates", "MAX(CloseDate),COUNT(CloseDate)", "CloseDate < TODAY", "", "", ""},
				{"ordered", "MAX(CloseDate)", "CloseDate < TODAY", " ORDER BY AccountId", "", ""},
				{"limited", "MAX(CloseDate)", "CloseDate < TODAY", " LIMIT 10", "", ""},
				{"forUpdate", "MAX(CloseDate)", "CloseDate < TODAY", " FOR UPDATE", "", ""},
				{"cursor", "MAX(CloseDate)", "CloseDate < TODAY", "", "Database.getCursor(text)", "Query contains aggregate expressions, these are not permitted in cursor queries"},
				{"access", "MAX(CloseDate)", "CloseDate < TODAY", "", "Database.getQueryLocator(text,AccessLevel.SYSTEM_MODE)", ""},
				{"mapBinds", "MAX(CloseDate)", "CloseDate < TODAY", "", "Database.getQueryLocatorWithBinds(text,new Map<String,Object>{'parents'=>parents},AccessLevel.SYSTEM_MODE)", ""},
			} {
				t.Run(api+"/"+locatorPrecedenceRoute(named)+"/"+tc.name, func(t *testing.T) {
					machine := emptyReferenceSumLocatorMachine(api)
					query := strings.ReplaceAll(dateAggregateLocatorQuery(tc.aggregate, tc.criterion)+tc.suffix, "'", "\\'")
					if tc.name == "unparenthesized" {
						query = strings.ReplaceAll(strings.ReplaceAll(query, "AND (", "AND "), ") GROUP BY", " GROUP BY")
					}
					if tc.name == "groupId" {
						query = strings.ReplaceAll(query, "AccountId", "Id")
					}
					if tc.name == "countNoField" {
						query = "SELECT COUNT() FROM Opportunity WHERE AccountId IN :parents AND Amount < 200"
					}
					call := tc.call
					if call == "" {
						call = "Database.getQueryLocator(text)"
					}
					body := "parents=parents.clone();String text='" + query + "';Object handle=" + call + ";"
					err := executeLocatorPrecedenceBody(t, machine, api, named, "Set<Id> parents=new Set<Id>();", body)
					message := tc.message
					if message == "" {
						message = aggregateError
					}
					assertLocatorPrecedenceException(t, err, message)
				})
			}
		}
	}
}

func TestEmptyReferenceDateAggregateLocatorBindControls(t *testing.T) {
	for _, tc := range dateAggregateLocatorShapes() {
		for _, source := range []string{"nonempty", "list", "objectView", "stringSet", "null", "closed", "getter", "static"} {
			t.Run(tc.name+"/"+source, func(t *testing.T) {
				machine := emptyReferenceSumLocatorMachine("67.0")
				empty := Set()
				empty.Type = "Set<Id>"
				machine.Globals["parents"] = empty
				machine.VarTypes["parents"] = "Set<Id>"
				switch source {
				case "nonempty":
					machine.Globals["parents"] = Set(String("001000000000099"))
				case "list":
					machine.Globals["parents"] = List()
					machine.VarTypes["parents"] = "List<Id>"
				case "objectView":
					machine.VarTypes["parents"] = "Object"
				case "stringSet":
					machine.VarTypes["parents"] = "Set<String>"
				case "null":
					machine.Globals["parents"] = Null
				case "closed":
					delete(machine.Globals, "parents")
				case "getter", "static":
					delete(machine.Globals, "parents")
					class := Class{Name: "AggregateBindSource", APIVersion: "67.0", StaticFields: map[string]Field{"calls": {Name: "calls", Type: "Integer", Static: true, Value: Int(0)}}}
					if source == "static" {
						class.StaticFields["parents"] = Field{Name: "parents", Type: "Set<Id>", Static: true, Value: empty}
					} else {
						getter, err := CompileAnonymous("calls++;return new Set<Id>();")
						if err != nil {
							t.Fatal(err)
						}
						class.Fields = map[string]Field{"parents": {Name: "parents", Type: "Set<Id>", Property: true, Getter: &Method{Name: "AggregateBindSource.parents.get", ClassName: "AggregateBindSource", ReturnType: "Set<Id>", Program: getter}}}
						machine.Globals["this"] = Object(class.Name)
					}
					if err := machine.RegisterClass(class); err != nil {
						t.Fatal(err)
					}
					machine.currentClass = class.Name
					machine.currentMethod = Method{ClassName: class.Name, IsStatic: source == "static"}
				}
				_, err := machine.databaseQueryHandle(String(dateAggregateLocatorQuery(tc.aggregate, tc.criterion)), Null, Null, false, &Result{})
				assertEmptyReferenceSumBindingScopeRejection(t, err)
				if source == "getter" {
					calls, err := machine.lookup("AggregateBindSource.calls")
					if err != nil || calls.Int != 0 {
						t.Fatalf("getter evaluated: %v, %v", calls, err)
					}
				}
			})
		}
	}
}

func dateAggregateLocatorShapes() []struct{ name, aggregate, criterion string } {
	return []struct{ name, aggregate, criterion string }{
		{"maximumDate", "MAX(CloseDate)", "CloseDate < TODAY"},
		{"maximumDateOrDecimal", "MAX(CloseDate)", "CloseDate < TODAY OR Amount > 100"},
		{"countDate", "COUNT(CloseDate)", "Amount < 200"},
	}
}

func dateAggregateLocatorQuery(aggregate, criterion string) string {
	return "SELECT AccountId," + aggregate + " resultValue FROM Opportunity WHERE AccountId IN :parents AND (" + criterion + ") GROUP BY AccountId"
}
