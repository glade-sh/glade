package vm

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Owned source/transport API 62/67/68 observations cover empty construction and
// iteration, plus matching-input and grouped COUNT rejection controls.
func TestEmptyReferenceDateAggregateLocatorOrgConformance(t *testing.T) {
	type stage struct {
		Attempted, Created, QueryPreserved, HasNext bool
		Type, Message                               string
	}
	type input struct {
		BoundSize              int
		Construction, Iterator stage
	}
	var data struct {
		Cells []struct {
			SourceAPI, TransportAPI, Context, ID, Query string
			Observation                                 struct {
				Shape                  string
				SeededMatchingRows     int
				Empty, MatchingControl input
				Construction, Iterator stage
				BoundSize              int
			}
		}
	}
	raw, err := os.ReadFile("testdata/empty_reference_date_aggregate_locator/native-observations.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatal(err)
	}
	if len(data.Cells) != 24 {
		t.Fatalf("native matrix has %d cells, want 24", len(data.Cells))
	}
	seen := make(map[string]bool)
	queries := map[string]string{
		"maximum-date":            dateAggregateLocatorQuery("MAX(CloseDate)", "CloseDate < TODAY"),
		"maximum-date-or-decimal": dateAggregateLocatorQuery("MAX(CloseDate)", "CloseDate < TODAY OR Amount > 100"),
		"count-date":              dateAggregateLocatorQuery("COUNT(CloseDate)", "Amount < 200"),
		"negative-control":        "SELECT NumberOfEmployees,COUNT(Id) n FROM Account WHERE Id IN :owned GROUP BY NumberOfEmployees",
	}
	checked, iterators := 0, 0
	for _, cell := range data.Cells {
		key := cell.SourceAPI + "/" + cell.Context + "/" + cell.ID
		if seen[key] || queries[cell.ID] == "" || cell.Query != queries[cell.ID] || cell.SourceAPI != cell.TransportAPI || cell.Observation.Shape != cell.ID ||
			(cell.SourceAPI != "62.0" && cell.SourceAPI != "67.0" && cell.SourceAPI != "68.0") ||
			(cell.Context != "anonymous" && cell.Context != "istest") {
			t.Fatalf("invalid native cell: %+v", cell)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			quote := func(s string) string { return "'" + strings.ReplaceAll(s, "'", "\\'") + "'" }
			query := "String query=" + quote(cell.Query) + ";"
			run := func(t *testing.T, declaration, body string) error {
				t.Helper()
				machine := emptyReferenceSumLocatorMachine(cell.SourceAPI)
				named := cell.Context == "istest"
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: cell.SourceAPI})
				if err != nil {
					t.Fatal(err)
				}
				if err := machine.RegisterClass(Class{Name: "DateAggregateObservation", APIVersion: cell.SourceAPI, Methods: map[string]Method{
					"observe": {Name: "DateAggregateObservation.observe", ClassName: "DateAggregateObservation", APIVersion: cell.SourceAPI, IsStatic: named, Params: []Param{{Name: "parents", Type: "Set<Id>"}}, Program: program},
				}}); err != nil {
					t.Fatal(err)
				}
				call := "new DateAggregateObservation().observe(parents);"
				if named {
					machine.EnableTestContext()
					call = "DateAggregateObservation.observe(parents);"
				}
				entry, err := CompileAnonymousWithOptions(declaration+call, CompileOptions{APIVersion: cell.SourceAPI})
				if err != nil {
					t.Fatal(err)
				}
				_, err = machine.Execute(entry)
				return err
			}
			reject := func(t *testing.T, declaration, body string, want stage) {
				t.Helper()
				if !want.Attempted || want.Created || want.Type != "System.QueryException" || want.Message == "" {
					t.Fatalf("invalid native rejection: %+v", want)
				}
				assertLocatorPrecedenceException(t, run(t, declaration, body+query+"Database.getQueryLocator(query);"), want.Message)
			}
			if cell.ID == "negative-control" {
				if cell.Observation.BoundSize != 1 || cell.Observation.Iterator.Attempted {
					t.Fatal("negative control did not retain the matching constructor boundary")
				}
				reject(t, "Set<Id> parents=new Set<Id>();", "Account parent=new Account(Name='Owned negative control',NumberOfEmployees=3);insert parent;Set<Id> owned=new Set<Id>{parent.Id};", cell.Observation.Construction)
				checked++
				return
			}
			observation := cell.Observation
			if observation.SeededMatchingRows != 1 || observation.Empty.BoundSize != 0 || observation.MatchingControl.BoundSize != 1 || observation.MatchingControl.Iterator.Attempted {
				t.Fatal("invalid empty/matching setup observation")
			}
			seed := "Account parent=new Account(Name='Owned aggregate parent');insert parent;insert new Opportunity(Name='Owned aggregate child',AccountId=parent.Id,CloseDate=Date.today().addDays(-2),StageName='Prospecting',Amount=150);"
			t.Run("emptyConstruction", func(t *testing.T) {
				want := observation.Empty.Construction
				if !want.Attempted || !want.Created || !want.QueryPreserved || want.Type != "" {
					t.Fatalf("invalid native empty constructor: %+v", want)
				}
				body := "parents=parents.clone();" + query + "Database.QueryLocator handle=Database.getQueryLocator(query);Assert.isNotNull(handle);Assert.areEqual(query,handle.getQuery());"
				if err := run(t, seed+"Set<Id> parents=new Set<Id>();", body); err != nil {
					t.Fatalf("native empty construction: %v", err)
				}
			})
			t.Run("matchingConstruction", func(t *testing.T) {
				reject(t, seed+"Set<Id> parents=new Set<Id>{parent.Id};", "parents=parents.clone();", observation.MatchingControl.Construction)
			})
			checked += 2
			t.Run("emptyIterator", func(t *testing.T) {
				want := observation.Empty.Iterator
				if !want.Attempted || !want.Created || want.HasNext || want.Type != "" {
					t.Fatalf("invalid native empty iterator: %+v", want)
				}
				body := "parents=parents.clone();" + query + "Database.QueryLocator handle=Database.getQueryLocator(query);Assert.areEqual(query,handle.getQuery());Database.QueryLocatorIterator rows=handle.iterator();Assert.isNotNull(rows);Assert.isFalse(rows.hasNext());"
				if err := run(t, seed+"Set<Id> parents=new Set<Id>();", body); err != nil {
					t.Fatalf("native empty iteration: %v", err)
				}
			})
			iterators++
		})
	}
	if checked != 42 || iterators != 18 {
		t.Fatalf("native boundaries: constructors=%d iterators=%d, want 42/18", checked, iterators)
	}
}
