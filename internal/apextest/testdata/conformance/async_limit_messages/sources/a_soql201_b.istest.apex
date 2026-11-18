Test.startTest();
Database.executeBatch(new AsyncProbe__A__('soql201'));
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-soql201_b' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
