System.debug(LoggingLevel.ERROR, 'TRACE|mapJsonOrder|before');
Map<String, Object> section = new Map<String, Object>();
section.put('section_key', 'value');
section.put('section_key_2', 'value2');
String mapJson = JSON.serialize(new Map<String, Object>{'section' => section});
System.debug(LoggingLevel.ERROR, 'OBS|mapJsonOrder|' + mapJson);
System.debug(LoggingLevel.ERROR, 'OBS|mapKeySetOrder|' + String.join(new List<String>(section.keySet()), ','));

System.debug(LoggingLevel.ERROR, 'TRACE|groupedAggregateAlias|before');
insert new List<Contact>{
    new Contact(LastName = 'Synthetic Aggregate A', LeadSource = 'Web'),
    new Contact(LastName = 'Synthetic Aggregate B', LeadSource = 'Social')
};
List<String> sources = new List<String>();
for (AggregateResult row : [
    SELECT LeadSource name FROM Contact
    WHERE LastName LIKE 'Synthetic Aggregate %'
    GROUP BY LeadSource
]) {
    sources.add(String.valueOf(row.get('name')));
}
sources.sort();
System.debug(LoggingLevel.ERROR, 'OBS|groupedAggregateAlias|' + JSON.serialize(sources));

System.debug(LoggingLevel.ERROR, 'TRACE|parentProjectionRequery|before');
Account parent = new Account(Name = 'Synthetic Parent');
insert parent;
Contact child = new Contact(LastName = 'Synthetic Child', AccountId = parent.Id);
insert child;
child = [SELECT Id, LastName, Account.Id FROM Contact WHERE Id = :child.Id];
String requeryText = 'SELECT Id, Account.Name, Account.Owner.Id FROM Contact WHERE Id = \'' +
    String.valueOf(child.Id) + '\' AND Account.Name = \'Synthetic Parent\' AND Account.Owner.Id != null';
List<Contact> refreshed = (List<Contact>) Database.query(requeryText);
Map<String, Object> parentObservation = new Map<String, Object>();
parentObservation.put('rowCount', refreshed.size());
parentObservation.put('hasParent', !refreshed.isEmpty() && refreshed[0].getPopulatedFieldsAsMap().containsKey('Account'));
parentObservation.put('parentName', refreshed.isEmpty() ? null : refreshed[0].Account.Name);
parentObservation.put('ownerIdPresent', !refreshed.isEmpty() && refreshed[0].Account.Owner.Id != null);
System.debug(LoggingLevel.ERROR, 'OBS|parentProjectionRequery|' + JSON.serialize(parentObservation));
