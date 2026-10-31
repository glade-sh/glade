// Duplicate upsert routes captured in upsert_duplicate_native at API 62 and 67.
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
                    if (saved.isSuccess()) System.assertEquals(shape == 'custom_insert', saved.isCreated(), shape+' / '+route);
                }
            } catch (DmlException failure) {
                status = String.valueOf(failure.getDmlStatusCode(0));
            }
            System.assertEquals(collide ? 'DUPLICATES_DETECTED' : 'ok', status, shape+' / '+route+' / '+collide);
            if (collide && (shape == 'custom_insert' || shape == 'custom_update')) System.assertEquals(null, row.Id, shape+' / '+route);
            else if (targetId != null) System.assertEquals(targetId, row.Id, shape+' / '+route);
            else System.assertNotEquals(null, row.Id);
            System.assertEquals(collide && shape == 'custom_insert' ? 1 : 2, [SELECT COUNT() FROM Contact]);
            if (targetId != null) {
                Contact stored = [SELECT Email FROM Contact WHERE Id=:targetId];
                System.assertEquals(collide ? 'original@example.invalid' : row.Email, stored.Email, shape+' / '+route);
            }
            Database.rollback(point);
        }
    }
}

// API 62/67 adjacent/istest/org.tsv: unchanged matching Email is allowed.
for (String shape : new List<String>{'implicit','explicit','custom'}) {
    for (Boolean partial : new List<Boolean>{false,true}) {
        Savepoint point = Database.setSavepoint();
        insert new Contact(LastName='Block', Email='same@example.invalid', RouteKey__c='block');
        Contact target = new Contact(LastName='Safe', Email='same@example.invalid', RouteKey__c='target');
        insert target;
        Id savedTargetId = target.Id;
        Contact row = new Contact(LastName='Block', Email='same@example.invalid');
        if (shape == 'custom') row.RouteKey__c='target'; else row.Id=target.Id;
        if (partial) {
            Database.UpsertResult saved;
            if (shape == 'custom') saved=Database.upsert(row, Contact.RouteKey__c, false);
            else if (shape == 'explicit') saved=Database.upsert(row, Contact.Id, false);
            else saved=Database.upsert(row, false);
            System.assertEquals(true, saved.isSuccess(), shape);
        } else {
            if (shape == 'custom') { upsert row RouteKey__c; }
            else if (shape == 'explicit') { upsert row Id; }
            else { upsert row; }
        }
        System.assertEquals(target.Id, row.Id, shape);
        Contact unchangedStored = [SELECT LastName FROM Contact WHERE Id=:savedTargetId];
        System.assertEquals('Block', unchangedStored.LastName);
        Database.rollback(point);
    }
}

// API 62/67 adjacent/istest/org.tsv: changed and inserted alert matches reject.
for (String shape : new List<String>{'implicit','explicit','custom'}) {
    for (Boolean insertion : new List<Boolean>{false,true}) {
        for (Boolean partial : new List<Boolean>{false,true}) {
            Savepoint point = Database.setSavepoint();
            insert new Contact(LastName='Alert', Email='same@example.invalid', RouteKey__c='block');
            Id targetId;
            if (!insertion) {
                Contact target = new Contact(LastName='Safe', Email='other@example.invalid', RouteKey__c='target');
                insert target; targetId=target.Id;
            }
            Contact row = new Contact(LastName='Alert', Email='same@example.invalid');
            if (shape == 'custom') row.RouteKey__c = insertion ? 'new' : 'target';
            else if (!insertion) row.Id=targetId;
            String status='ok';
            try {
                if (partial) {
                    Database.UpsertResult result;
                    if (shape == 'custom') result=Database.upsert(row, Contact.RouteKey__c, false);
                    else if (shape == 'explicit') result=Database.upsert(row, Contact.Id, false);
                    else result=Database.upsert(row, false);
                    if (!result.isSuccess()) status=String.valueOf(result.getErrors()[0].getStatusCode());
                } else if (shape == 'custom') { upsert row RouteKey__c; }
                else if (shape == 'explicit') { upsert row Id; }
                else { upsert row; }
            } catch (DmlException failure) { status=String.valueOf(failure.getDmlStatusCode(0)); }
            System.assertEquals('DUPLICATES_DETECTED', status, shape);
            if (!insertion) {
                Contact stored = [SELECT LastName FROM Contact WHERE Id=:targetId];
                System.assertEquals('Safe', stored.LastName);
            }
            Database.rollback(point);
        }
    }
}
