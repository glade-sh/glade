List<String> rows = new List<String>();
Map<String,RecordType> types = new Map<String,RecordType>();
for (RecordType rt : [SELECT Id, DeveloperName, Name FROM RecordType WHERE SobjectType = 'Account']) { types.put(rt.DeveloperName, rt); }
for (String name : new List<String>{'Care','Member','Provider'}) {
 RecordType business = types.get('Business_' + name);
 RecordType person = types.get(name);
 rows.add(name + '|identity|' + String.valueOf(business.Id != person.Id) + '|' + String.valueOf(business.Name == name) + '|' + String.valueOf(person.Name == name + ' Label'));
 Account b = new Account(Name='Business row', RecordTypeId=business.Id);
 Database.SaveResult br = Database.insert(b, false);
 if (br.isSuccess()) { b = [SELECT RecordTypeId, IsPersonAccount FROM Account WHERE Id=:b.Id]; }
 rows.add(name + '|business|' + String.valueOf(br.isSuccess()) + '|' + String.valueOf(b.RecordTypeId == business.Id) + '|' + String.valueOf(b.IsPersonAccount));
 Account a = new Account(LastName='Person row', RecordTypeId=person.Id);
 Database.SaveResult ar = Database.insert(a, false);
 if (ar.isSuccess()) { a = [SELECT RecordTypeId, IsPersonAccount FROM Account WHERE Id=:a.Id]; }
 rows.add(name + '|person|' + String.valueOf(ar.isSuccess()) + '|' + String.valueOf(a.RecordTypeId == person.Id) + '|' + String.valueOf(a.IsPersonAccount));
 Database.SaveResult missing = Database.insert(new Account(RecordTypeId=person.Id), false);
 rows.add(name + '|missingLastName|' + String.valueOf(missing.getErrors()[0].getStatusCode()));
}
for (String row : rows) { System.debug('ROW|' + row); }
