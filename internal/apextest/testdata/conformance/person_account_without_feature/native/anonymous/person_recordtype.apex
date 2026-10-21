List<RecordType> rows = [SELECT Id, DeveloperName, IsPersonType FROM RecordType WHERE SObjectType = 'Account' AND IsPersonType = true];
System.debug('PERSON_RECORDTYPE|' + rows.size());
