List<Account> l = [SELECT Id FROM Account WHERE Name LIKE 'QRSEED%' LIMIT 10000];
update l;
obs = l.size() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows() + '|' + Limits.getLimitDmlRows();
