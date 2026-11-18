Savepoint sp = Database.setSavepoint();
Test.startTest();
String r = 'start';
try { Database.rollback(sp); r += '|rollback-ok'; } catch (Exception e) { r += '|caught:' + e.getTypeName() + ':' + e.getMessage(); }
Test.stopTest();
obs = r + '|' + Limits.getDmlStatements();
