Id uid = [SELECT Id FROM User WHERE Alias = 'hid62' AND LastName = 'Hidden' LIMIT 1].Id;
List<HidRec__c> tg = [SELECT Id FROM HidRec__c WHERE Name = 'HiddenTarget' LIMIT 1];
if (tg.isEmpty()) {
    insert new HidRec__c(Name = 'HiddenTarget', Flag__c = 'a');
} else {
    update new HidRec__c(Id = tg[0].Id, Flag__c = 'a');
}
List<HidRec__c> vg = [SELECT Id FROM HidRec__c WHERE Name = 'VisibleTarget' LIMIT 1];
if (vg.isEmpty()) {
    insert new HidRec__c(Name = 'VisibleTarget', Flag__c = 'a', OwnerId = uid);
} else {
    update new HidRec__c(Id = vg[0].Id, Flag__c = 'a', OwnerId = uid);
}
System.debug('OBS|setup|rec=' + [SELECT COUNT() FROM HidRec__c]);
