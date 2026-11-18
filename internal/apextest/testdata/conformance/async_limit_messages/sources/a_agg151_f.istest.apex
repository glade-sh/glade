Test.startTest();
AsyncProbe__A__.fut('agg151');
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-agg151_f' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
