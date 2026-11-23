try {
  Id rt = Schema.SObjectType.Account.getRecordTypeInfosByDeveloperName().get('RT').getRecordTypeId();
  Account a = new Account(Name='a', RecordTypeId=rt); insert a;
  System.debug('R5|acct_rt|Industry=' + [SELECT Industry FROM Account WHERE Id=:a.Id].Industry);
} catch (Exception e) { System.debug('R5|acct_rt|EXC|' + e.getMessage()); }
