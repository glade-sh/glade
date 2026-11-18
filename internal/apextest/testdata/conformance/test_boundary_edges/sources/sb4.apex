Savepoint sp = Database.setSavepoint();
insert new Account(Name = 'sbdata');
Test.startTest();
String r = 'start';
try { Database.rollback(sp); r += '|rollback-ok|' + [SELECT COUNT() FROM Account WHERE Name = 'sbdata']; } catch (Exception e) { r += '|caught:' + e.getTypeName() + ':' + e.getMessage(); }
Test.stopTest();
obs = r;
