Test.startTest();
Database.executeBatch(new AsyncProbe__A__('agg76'));
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-agg76_b' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
