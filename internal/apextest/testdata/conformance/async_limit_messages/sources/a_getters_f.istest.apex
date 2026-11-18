Test.startTest();
AsyncProbe__A__.fut('getters');
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-getters_f' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
