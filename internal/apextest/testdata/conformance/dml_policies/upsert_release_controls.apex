// Release preservation controls, not Salesforce observations. Only unmatched
// Id insertion has captured duplicate-rule outcomes in routes/native.
for (String shape : new List<String>{'custom_insert','implicit_id_update','explicit_id_update','custom_update'}) {
    for (String route : new List<String>{'keyword','database_partial','database_atomic'}) {
        for (Boolean collide : new List<Boolean>{false,true}) {
            Savepoint point = Database.setSavepoint();
            insert new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='block');
            Id targetId;
            if (shape != 'custom_insert') {
                Contact target = new Contact(LastName='Block', Email='original@example.invalid', RouteKey__c='target');
                insert target;
                targetId = target.Id;
            }
            Contact row = new Contact(LastName='Block', Email=collide ? 'duplicate@example.invalid' : 'original@example.invalid');
            if (shape == 'implicit_id_update' || shape == 'explicit_id_update') row.Id = targetId;
            if (shape == 'custom_insert' || shape == 'custom_update') row.RouteKey__c = shape == 'custom_insert' ? 'new' : 'target';
            String status = 'ok';
            try {
                if (route == 'keyword') {
                    if (shape == 'custom_insert' || shape == 'custom_update') { upsert row RouteKey__c; }
                    else if (shape == 'explicit_id_update') { upsert row Id; }
                    else { upsert row; }
                } else {
                    Boolean atomic = route == 'database_atomic';
                    Database.UpsertResult saved;
                    if (shape == 'custom_insert' || shape == 'custom_update') saved = Database.upsert(row, Contact.RouteKey__c, atomic);
                    else if (shape == 'explicit_id_update') saved = Database.upsert(row, Contact.Id, atomic);
                    else saved = Database.upsert(row, atomic);
                    if (!saved.isSuccess()) status = String.valueOf(saved.getErrors()[0].getStatusCode());
                    System.assertEquals(shape == 'custom_insert', saved.isCreated(), shape+' / '+route);
                }
            } catch (DmlException failure) {
                status = String.valueOf(failure.getDmlStatusCode(0));
            }
            System.assertEquals('ok', status, shape+' / '+route+' / '+collide);
            System.assertNotEquals(null, row.Id);
            System.assertEquals(2, [SELECT COUNT() FROM Contact]);
            if (targetId != null) {
                Contact stored = [SELECT Email FROM Contact WHERE Id=:targetId];
                System.assertEquals(row.Email, stored.Email);
            }
            Database.rollback(point);
        }
    }
}
