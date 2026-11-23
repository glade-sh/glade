try {
  X__c x = new X__c(Name='a'); insert x;
  X__c q = [SELECT RecordTypeId, P__c FROM X__c WHERE Id=:x.Id];
  System.debug('R5|x_none2|P=' + q.P__c + '|rtNull=' + (q.RecordTypeId == null) + '|inMemRtNull=' + (x.RecordTypeId == null) + '|inMemP=' + x.P__c);
} catch (Exception e) { System.debug('R5|x_none2|EXC|' + e.getMessage()); }
