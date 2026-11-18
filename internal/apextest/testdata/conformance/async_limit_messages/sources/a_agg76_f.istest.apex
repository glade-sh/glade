Test.startTest();
AsyncProbe__A__.fut('agg76');
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-agg76_f' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
