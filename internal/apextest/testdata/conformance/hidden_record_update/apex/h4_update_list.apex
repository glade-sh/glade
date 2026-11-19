User u = [SELECT Id FROM User WHERE Alias = 'hid62' AND LastName = 'Hidden' LIMIT 1];
Id tid = [SELECT Id FROM HidRec__c WHERE Name = 'HiddenTarget' LIMIT 1].Id;
System.runAs(u) {
    try {
        update new List<HidRec__c>{ new HidRec__c(Id = tid, Flag__c = 'L') };
        System.debug('OBS|list|ok');
    } catch (Exception e) {
        System.debug('OBS|list|EX|' + e.getTypeName() + '|' + e.getMessage().left(90));
    }
}
