Test.startTest();
Savepoint sp2 = Database.setSavepoint();
Test.stopTest();
String r = 'stop';
try { Database.rollback(sp2); r += '|rollback-ok'; } catch (Exception e) { r += '|caught:' + e.getTypeName() + ':' + e.getMessage(); }
obs = r + '|' + Limits.getDmlStatements();
