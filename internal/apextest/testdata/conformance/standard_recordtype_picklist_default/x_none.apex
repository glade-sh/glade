try {
  X__c x = new X__c(Name='a'); insert x;
  System.debug('R5|x_none|P=' + [SELECT P__c FROM X__c WHERE Id=:x.Id].P__c);
} catch (Exception e) { System.debug('R5|x_none|EXC|' + e.getMessage()); }
