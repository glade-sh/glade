// Local shipping-behavior control; not a Salesforce parity observation.
insert new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='seed');
List<Contact> rows = new List<Contact>{new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='new')};
List<Database.UpsertResult> saved = Database.upsert(rows, Contact.RouteKey__c, false);
System.assertEquals(true, saved[0].isSuccess());
System.assertEquals(2, [SELECT COUNT() FROM Contact]);
