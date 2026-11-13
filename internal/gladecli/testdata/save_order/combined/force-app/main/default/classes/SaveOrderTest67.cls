@IsTest private class SaveOrderTest67 { @IsTest static void combined() {
Savepoint sp = Database.setSavepoint();
SaveOrderTrace.events = '';
List<Account> rows = new List<Account>{new Account(Name='ok'),new Account(Name='reject-validation'),new Account(Name='reject-after')};
Database.SaveResult[] results = Database.insert(rows, false);
List<String> parts = new List<String>();
for (Database.SaveResult r : results) parts.add(r.isSuccess() ? 'ok' : String.valueOf(r.getErrors()[0].getStatusCode()));
List<Account> saved = [SELECT Name, Description FROM Account WHERE Name IN ('ok','reject-validation','reject-after') ORDER BY Name];
String observed = 'test:' + Test.isRunningTest() + '|' + String.join(parts, ',') + '|' + SaveOrderTrace.events + '|saved:' + saved.size() + ':' + (saved.isEmpty() ? '' : saved[0].Description);
Database.rollback(sp);
System.assertEquals('EXPECTED', observed); } }
