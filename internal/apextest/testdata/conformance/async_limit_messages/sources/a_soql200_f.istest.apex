Test.startTest();
AsyncProbe__A__.fut('soql200');
Test.stopTest();
List<Account> r = [SELECT Description FROM Account WHERE Name = 'AOBS-soql200_f' LIMIT 1];
obs = r.isEmpty() ? 'NOREC' : r[0].Description;
