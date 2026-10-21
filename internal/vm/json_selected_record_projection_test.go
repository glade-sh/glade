package vm

import "testing"

func TestDMLPopulatedRecordProjectionNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			runJSONRecordProjection(t, api, `
Contact record=new Contact(FirstName='Harbor',LastName='Projection');
insert record;
System.assertEquals(new Set<String>{'FirstName','Id','LastName'},record.getPopulatedFieldsAsMap().keySet());
Contact explicitOwner=new Contact(FirstName='Harbor',LastName='Explicit',OwnerId=UserInfo.getUserId());
insert explicitOwner;
System.assertEquals(new Set<String>{'FirstName','Id','LastName','OwnerId'},explicitOwner.getPopulatedFieldsAsMap().keySet());
`)
		})
	}
}

func TestJSONSelectedCompoundRecordProjectionNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			runJSONRecordProjection(t, api, `
Account record=new Account(Name='Projection',BillingStreet='1 Survey',BillingCity='Study');
insert record;
Id recordId=record.Id;
Account selected=[SELECT Id,Name,BillingAddress,CreatedDate FROM Account WHERE Id=:recordId];
System.assertEquals(new Set<String>{'BillingAddress','CreatedDate','Id','Name'},selected.getPopulatedFieldsAsMap().keySet());
Map<String,Object> payload=(Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(selected,false));
System.assertEquals(new Set<String>{'attributes','BillingAddress','CreatedDate','Id','Name'},payload.keySet());
Account hydrated=(Account)JSON.deserialize(JSON.serialize(selected,false),Account.class);
System.assertEquals('Study',((Address)hydrated.get('BillingAddress')).getCity());
System.assert(selected.CreatedDate==hydrated.CreatedDate);
`)
		})
	}
}
