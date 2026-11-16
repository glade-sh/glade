Test.startTest();
System.enqueueJob(new AsyncProbe__A__('getters'));
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-getters_q' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
