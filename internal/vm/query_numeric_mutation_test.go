package vm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func queryNumericMutationTestOrg() storage.OrgState {
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Id"] = storage.Field{APIName: "Id", Type: storage.FieldID}
	account.Definition.Fields["AnnualRevenue"] = storage.Field{APIName: "AnnualRevenue", Type: storage.FieldDecimal, DisplayType: "CURRENCY", Precision: 18, Scale: 2, ScaleSpecified: true}
	account.Definition.Fields["NumberOfEmployees"] = storage.Field{APIName: "NumberOfEmployees", Type: storage.FieldInteger}
	org.Objects["Account"] = account
	org.Objects["ProjectionGauge__c"] = storage.ObjectState{
		Definition: storage.ObjectDefinition{APIName: "ProjectionGauge__c", KeyPrefix: "a00", Fields: map[string]storage.Field{
			"Name":       {APIName: "Name", Type: storage.FieldString},
			"Amount__c":  {APIName: "Amount__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", Precision: 18, ScaleSpecified: true},
			"Account__c": {APIName: "Account__c", Type: storage.FieldReference, ReferenceTo: []string{"Account"}, RelationshipName: "Account__r"},
		}},
		Records: make(map[storage.ID]storage.Record),
	}
	return org
}

func TestQueryNumericMutationCustomDynamicProjection(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Account parent=new Account(Name='parent'); insert parent;
ProjectionGauge__c seed=new ProjectionGauge__c(Name='gauge',Amount__c=12,Account__c=parent.Id); insert seed;
Id rid=seed.Id;
SObject q=Database.query('SELECT Id,Amount__c FROM ProjectionGauge__c WHERE Id=:rid');
q.put('Amount__c',(Decimal)q.get('Amount__c')+3);
Boolean omittedRejected=false;try {Object omitted=q.get('Account__c');} catch (SObjectException e) {omittedRejected=true;}System.assert(omittedRejected);
ProjectionGauge__c selected=[SELECT Id,Amount__c,Account__r.Name FROM ProjectionGauge__c WHERE Id=:rid];
selected.Amount__c+=3;
System.assertEquals(parent.Id,selected.Account__c);
System.assertEquals('parent',selected.Account__r.Name);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := queryNumericMutationTestOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}

		})
	}
}

// Native static and dynamic query rows retain omitted-field errors after
// numeric writes. Selected and explicitly assigned lookups remain controls.
func TestQuerySelectedNumericMutationOmittedLookup(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, isTest := range []bool{false, true} {
			for _, mutation := range []string{
				"q.AnnualRevenue += 3;", "q.AnnualRevenue -= 3;",
				"q.AnnualRevenue = q.AnnualRevenue + 3;", "q.put('AnnualRevenue',15);",
				"q.AnnualRevenue = q.AnnualRevenue;", "q.NumberOfEmployees += 1;",
				"ProjectionMutationBridge.adjust(q);",
			} {
				t.Run(fmt.Sprintf("%s/isTest=%t/%s", api, isTest, mutation), func(t *testing.T) {
					program, err := CompileAnonymousWithOptions(`
Account parent = new Account(Name='parent'); insert parent;
Account seed = new Account(Name='seed',AnnualRevenue=12,NumberOfEmployees=4,ParentId=parent.Id);
insert seed;
Account q = [SELECT Id,AnnualRevenue,NumberOfEmployees FROM Account WHERE Id=:seed.Id];
Account snapshot = q.clone(true,true,true,true);
`+mutation+`
Boolean directRejected=false; try {Object v=q.ParentId;} catch(SObjectException e) {directRejected=true;}
Boolean dynamicRejected=false; try {Object v=q.get('pArEnTiD');} catch(SObjectException e) {dynamicRejected=true;}
Boolean tokenRejected=false; try {Object v=q.get(Account.ParentId);} catch(SObjectException e) {tokenRejected=true;}
Map<String,Object> populated = q.getPopulatedFieldsAsMap();
String serialized = JSON.serialize(q);
Boolean snapshotRejected = false;
try { Object v=snapshot.ParentId; } catch (SObjectException e) { snapshotRejected=true; }
Boolean parentRejected = false;
try { Object v=q.Parent; } catch (SObjectException e) { parentRejected=true; }
Boolean parentMethodRejected = false;
try { Object v=q.getSObject('Parent'); } catch (SObjectException e) { parentMethodRejected=true; }
Boolean textRejected = false;
try { Object v=q.Description; } catch (SObjectException e) { textRejected=true; }
update q;
Boolean postDMLRejected = false;
try { Object v=q.ParentId; } catch (SObjectException e) { postDMLRejected=true; }
Boolean postDMLDynamicRejected = false;
try { Object v=q.get('ParentId'); } catch (SObjectException e) { postDMLDynamicRejected=true; }
Boolean postDMLTokenRejected = false;
try { Object v=q.get(Account.ParentId); } catch (SObjectException e) { postDMLTokenRejected=true; }
`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					org := queryNumericMutationTestOrg()
					machine := New(nil)
					machine.SetOrg(&org)
					bridge, err := CompileAnonymousWithOptions(`input.put('AnnualRevenue',(Decimal)input.get('AnnualRevenue')+3);`, CompileOptions{APIVersion: api})
					if err != nil {
						t.Fatal(err)
					}
					if err := machine.RegisterMethod(Method{Name: "ProjectionMutationBridge.adjust", ClassName: "ProjectionMutationBridge", ReturnType: "void", IsStatic: true, Params: []Param{{Name: "input", Type: "SObject"}}, Program: bridge}); err != nil {
						t.Fatal(err)
					}
					if isTest {
						machine.EnableTestContext()
					}
					if _, err := machine.Execute(program); err != nil {
						t.Fatal(err)
					}
					for _, name := range []string{"directRejected", "dynamicRejected", "tokenRejected", "snapshotRejected", "parentRejected", "parentMethodRejected", "textRejected", "postDMLRejected", "postDMLDynamicRejected", "postDMLTokenRejected"} {
						if value := machine.Globals[name]; value.Kind != ValueBool || !value.Bool {
							t.Fatalf("%s escaped query projection: %#v", name, value)
						}
					}
					if _, exists := machine.Globals["populated"].Map[mapKey(String("ParentId"))]; exists {
						t.Fatal("omitted lookup became populated")
					}
					if text := machine.Globals["serialized"].Text; strings.Contains(text, "__glade_") || strings.Contains(text, "ParentId") {
						t.Fatalf("query metadata leaked into serialization: %s", text)
					}
					q := machine.Globals["q"]
					if machine.queriedSObjectFieldsIncludes(q, "ParentId") {
						t.Fatal("omitted lookup widened SELECT projection")
					}
				})
			}
		}
	}
}

func TestQueryNumericMutationProjectionControls(t *testing.T) {
	for _, mutation := range []string{"", "q.Name='changed';", "q.AnnualRevenue=null;", "q.put('Name','changed');", "q=[SELECT Id FROM Account WHERE Id=:seed.Id]; q.AnnualRevenue=15; q.AnnualRevenue=16;", "q.AnnualRevenue=null; q.AnnualRevenue=15;"} {
		t.Run(mutation, func(t *testing.T) {
			program, err := CompileAnonymous(`
Account seed=new Account(Name='seed',AnnualRevenue=12); insert seed;
Account q=[SELECT Id,AnnualRevenue FROM Account WHERE Id=:seed.Id];
` + mutation + `
Boolean rejected=false;
try { Object v=q.ParentId; } catch (SObjectException e) { rejected=true; }
System.assert(rejected);
q.ParentId=null;
System.assertEquals(null,q.ParentId);
System.assertEquals(null,q.get('ParentId'));
Account selected=[SELECT Id,AnnualRevenue,ParentId FROM Account WHERE Id=:seed.Id];
selected.AnnualRevenue+=3;
System.assertEquals(null,selected.ParentId);
`)
			if err != nil {
				t.Fatal(err)
			}
			org := queryNumericMutationTestOrg()
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
