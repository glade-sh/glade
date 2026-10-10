Savepoint outerPoint = Database.setSavepoint();
Account a = new Account(Name = 'Single');
insert a;
Id accountId = a.Id;
SObject row = Database.query('SELECT Id, Name FROM Account WHERE Id = :accountId LIMIT 1');
System.debug('Q1|' + (a == (Account)row) + '|' + JSON.serialize(a.getPopulatedFieldsAsMap()) + '|' + JSON.serialize(row.getPopulatedFieldsAsMap()));
Database.UndeleteResult active = Database.undelete(a, false);
System.debug('D01|' + active.isSuccess() + '|' + (active.getId() == a.Id) + '|' + active.getErrors()[0].getStatusCode());
Database.DeleteResult removed = Database.delete(a, false);
System.debug('D04|' + removed.isSuccess() + '|' + (removed.getId() == a.Id) + '|' + JSON.serialize(removed.getErrors()));
Database.UndeleteResult restored = Database.undelete(a, false);
System.debug('D05|' + restored.isSuccess() + '|' + (restored.getId() == a.Id) + '|' + JSON.serialize(restored.getErrors()));
Contact c = new Contact(LastName = 'Wrong type');
insert c;
try {
    Database.UndeleteResult wrong = Database.undelete(new Account(Id = c.Id), false);
    System.debug('D03|' + wrong.isSuccess() + '|' + (wrong.getId() == c.Id) + '|' + wrong.getErrors()[0].getStatusCode());
} catch (Exception e) { System.debug('D03|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
Savepoint missingPoint = Database.setSavepoint();
Account missing = new Account(Name = 'Missing');
insert missing;
Id missingId = missing.Id;
Database.rollback(missingPoint);
try {
    Database.UndeleteResult absent = Database.undelete(new Account(Id = missingId), false);
    System.debug('D02|' + absent.isSuccess() + '|' + (absent.getId() == missingId) + '|' + absent.getErrors()[0].getStatusCode());
} catch (Exception e) { System.debug('D02|EXC|' + e.getTypeName() + '|' + e.getMessage()); }
Database.rollback(outerPoint);
