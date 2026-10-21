package vm

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Sharing applies to the contributing records, not to ownerless aggregate
// results. Explicit grants remain separate from record ownership and OWD.
func TestAggregateSharingUsesVisibleInputsAndHelperSource(t *testing.T) {
	for _, source := range []struct{ entry, helper string }{{"62.0", "62.0"}, {"66.0", "66.0"}, {"67.0", "67.0"}, {"68.0", "68.0"}, {"67.0", "66.0"}, {"66.0", "67.0"}} {
		for _, mode := range []string{"with sharing", "without sharing"} {
			for _, model := range []string{"Private", "ReadWrite"} {
				for _, shape := range []string{"flat", "sum", "grouped"} {
					t.Run(fmt.Sprintf("%s/%s/%s/%s/%s", source.entry, source.helper, mode, model, shape), func(t *testing.T) {
						machine := recordSharingControlMachine(t, model)
						query := "SELECT SUM(Amount__c) amount FROM Widget__c"
						if shape == "flat" {
							query = "SELECT Name, Amount__c FROM Widget__c"
						}
						if shape == "grouped" {
							query = "SELECT Name, SUM(Amount__c) amount FROM Widget__c GROUP BY Name"
						}
						for _, spec := range []struct{ name, api, sharing, body string }{{"SumBase", source.helper, "", "return Database.query('" + query + "');"}, {"ScopedSumReader", source.entry, mode, "return SumBase.read();"}} {
							body, err := CompileAnonymousWithOptions(spec.body, CompileOptions{APIVersion: spec.api})
							if err != nil {
								t.Fatal(err)
							}
							class := Class{Name: spec.name, APIVersion: spec.api}
							if spec.sharing != "" {
								class.Modifiers = []string{spec.sharing}
							}
							if err := machine.RegisterClass(class); err != nil {
								t.Fatal(err)
							}
							if err := machine.RegisterMethod(Method{Name: spec.name + ".read", ClassName: spec.name, IsStatic: true, Access: "public", APIVersion: spec.api, SourceContextBound: true, SharingMode: spec.sharing, Program: body}); err != nil {
								t.Fatal(err)
							}
						}
						value, err := machine.callMethod(machine.Methods["ScopedSumReader.read"], nil, &Result{})
						if err != nil {
							t.Fatal(err)
						}
						total := float64(0)
						for _, row := range value.List {
							n := row.Fields["amount"]
							if shape == "flat" {
								n = row.Fields["Amount__c"]
							}
							switch n.Kind {
							case ValueInt:
								total += float64(n.Int)
							case ValueDecimal:
								total += n.Decimal
							default:
								t.Fatalf("aggregate kind: %#v", n)
							}
						}
						want := float64(20)
						if model == "Private" && (mode == "with sharing" || source.helper == "67.0" || source.helper == "68.0") {
							want = 10
						}
						if total != want {
							t.Fatalf("total=%g want=%g", total, want)
						}
						if shape != "sum" {
							wantRows := 2
							if want == 10 {
								wantRows = 1
							}
							if len(value.List) != wantRows {
								t.Fatalf("groups=%d want=%d", len(value.List), wantRows)
							}
						}
					})
				}
			}
		}
	}
}

// The owned aggregation observations record a null SUM for no inputs. Sharing
// must produce that same empty-input result rather than sum hidden inputs.
func TestAggregateSharingWithNoVisibleInputs(t *testing.T) {
	for _, grouped := range []bool{false, true} {
		t.Run(fmt.Sprint(grouped), func(t *testing.T) {
			machine := recordSharingControlMachine(t, "Private")
			state := machine.Org.Objects["Widget__c"]
			for id, row := range state.Records {
				row.System.OwnerID = storage.ID("005000000000001")
				state.Records[id] = row
			}
			machine.Org.Objects["Widget__c"] = state
			query := "SELECT SUM(Amount__c) amount FROM Widget__c"
			if grouped {
				query = "SELECT Name, SUM(Amount__c) amount FROM Widget__c GROUP BY Name"
			}
			body, err := CompileAnonymousWithOptions("return Database.query('"+query+"');", CompileOptions{APIVersion: "66.0"})
			if err != nil {
				t.Fatal(err)
			}
			if err := machine.RegisterClass(Class{Name: "EmptySumReader", APIVersion: "66.0", Modifiers: []string{"with sharing"}}); err != nil {
				t.Fatal(err)
			}
			value, err := machine.callMethod(Method{Name: "EmptySumReader.read", ClassName: "EmptySumReader", IsStatic: true, APIVersion: "66.0", SourceContextBound: true, SharingMode: "with sharing", Program: body}, nil, &Result{})
			if err != nil {
				t.Fatal(err)
			}
			if grouped {
				if len(value.List) != 0 {
					t.Fatalf("empty groups=%d want=0", len(value.List))
				}
			} else if len(value.List) != 1 || value.List[0].Fields["amount"].Kind != ValueNull {
				t.Fatalf("empty SUM rows=%d want=1 with null amount", len(value.List))
			}
		})
	}
}

func TestAggregateSharingKeepsSourceAndActorCachesSeparate(t *testing.T) {
	machine := recordSharingControlMachine(t, "Private")
	state := machine.Org.Objects["Widget__c"]
	row := state.Records["a00000000000001"]
	row.Fields["Amount__c"] = storage.DecimalValue("100")
	state.Records[row.ID] = row
	machine.Org.Objects["Widget__c"] = state
	if err := machine.RegisterClass(Class{Name: "ActorSumReader", APIVersion: "66.0", Modifiers: []string{"with sharing"}}); err != nil {
		t.Fatal(err)
	}
	machine.currentClass = "ActorSumReader"
	machine.currentMethod = Method{ClassName: "ActorSumReader", APIVersion: "66.0", SourceContextBound: true, SharingMode: "with sharing"}
	for _, actor := range []struct {
		id     string
		amount float64
	}{{"005000000000002", 10}, {"005000000000001", 100}, {"005000000000002", 10}} {
		machine.testContext.CurrentUser.Fields["Id"] = String(actor.id)
		value, err := machine.executeSOQL("SELECT SUM(Amount__c) amount FROM Widget__c", &Result{})
		if err != nil {
			t.Fatal(err)
		}
		if len(value.List) != 1 || value.List[0].Fields["amount"].Decimal != actor.amount {
			t.Errorf("actor=%s SUM rows=%d want amount=%g", actor.id, len(value.List), actor.amount)
		}
		if len(machine.Org.Objects["Widget__c"].Records) != 2 {
			t.Fatal("sharing changed the source records")
		}
	}
}
