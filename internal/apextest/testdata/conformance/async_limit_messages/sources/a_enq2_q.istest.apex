Test.startTest();
System.enqueueJob(new AsyncProbe__A__('enq2'));
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-enq2_q' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
