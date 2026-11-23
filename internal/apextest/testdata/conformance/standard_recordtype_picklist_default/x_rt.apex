try {
  Id rt = Schema.SObjectType.X__c.getRecordTypeInfosByDeveloperName().get('RT').getRecordTypeId();
  X__c x = new X__c(Name='a', RecordTypeId=rt); insert x;
  System.debug('R5|x_rt|P=' + [SELECT P__c FROM X__c WHERE Id=:x.Id].P__c + '|inMemory=' + x.P__c);
} catch (Exception e) { System.debug('R5|x_rt|EXC|' + e.getMessage()); }
