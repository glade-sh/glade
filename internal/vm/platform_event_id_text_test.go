package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Apex Rules of Conversion: IDs can always be assigned to Strings.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_rules_of_conversion.htm
// Platform-event R073/R077 retain incompatible-input and read-only-field checks.
func TestPlatformEventTextPutAcceptsIdScalar(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, tc := range []struct{ name, code string }{
			{"id", `row.put('Actor__c', actorId);`},
			{"erased id", `Object erased = actorId; row.put('Actor__c', erased);`},
			{"field token", `row.put(IdTextSignal__e.Actor__c, actorId);`},
			{"previous value", `row.Actor__c = 'before'; System.assertEquals('before', row.put('Actor__c', actorId));`},
			{"queried id", `Account source = new Account(Name = 'Source'); insert source; actorId = [SELECT Id FROM Account WHERE Id = :source.Id].Id; row.put('Actor__c', actorId);`},
			{"literal id control", `actorId = (Id)'001000000000001AAA'; row.put('Actor__c', actorId);`},
			{"string control", `String text = actorId; row.put('Actor__c', text);`},
		} {
			t.Run(api+"/"+tc.name, func(t *testing.T) {
				program, err := CompileAnonymousWithOptions(`
Id actorId = EventBus.publish(new IdTextSignal__e()).getId();
IdTextSignal__e row = new IdTextSignal__e();
`+tc.code+`
String expected = actorId;
System.assertEquals(expected, row.Actor__c);
try {
    row.put('Actor__c', 123);
    System.assert(false, 'Integer must remain incompatible');
} catch (SObjectException error) {
    System.assertEquals('Illegal assignment from Integer to String', error.getMessage());
}
try {
    row.put('ReplayId', 1);
    System.assert(false, 'ReplayId must remain read-only');
} catch (SObjectException error) {
    System.assertEquals('Field ReplayId is not editable', error.getMessage());
}
System.assertEquals(expected, row.Actor__c);
row.put('Actor__c', null);
System.assertEquals(null, row.Actor__c);
`, CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := testDataOrg()
				org.Objects["IdTextSignal__e"] = storage.ObjectState{Definition: storage.ObjectDefinition{
					APIName: "IdTextSignal__e", KeyPrefix: "e00",
					Metadata: map[string]string{"publishBehavior": "PublishImmediately"},
					Fields: map[string]storage.Field{
						"Actor__c": {APIName: "Actor__c", Type: storage.FieldString, Length: 18},
						"ReplayId": {APIName: "ReplayId", Type: storage.FieldInteger},
					},
				}}
				machine := New(nil)
				machine.SetOrg(&org)
				if _, err := machine.Execute(program); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
