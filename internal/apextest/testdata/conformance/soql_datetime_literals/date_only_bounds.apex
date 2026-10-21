Object r;
String lastName = 'Synthetic date only';
delete [SELECT Id FROM Contact WHERE LastName = :lastName];
insert new Contact(LastName = lastName, Birthdate = Date.newInstance(2020, 1, 1));
List<Contact> staticRows = [SELECT Id FROM Contact WHERE LastName = 'Synthetic date only' AND Birthdate < 2020-01-02];
List<Contact> dynamicRows = Database.query('SELECT Id FROM Contact WHERE LastName = \'Synthetic date only\' AND Birthdate < 2020-01-02');
r = String.valueOf(staticRows.size()) + '|' + String.valueOf(dynamicRows.size());
System.debug('SOQL_DATETIME|date_only_bounds|' + String.valueOf(r));
