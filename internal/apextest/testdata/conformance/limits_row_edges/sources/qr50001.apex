List<Account> l = [SELECT Id FROM Account WHERE Name LIKE 'QRSEED%' LIMIT 50001];
obs = l.size() + '|' + Limits.getQueryRows() + '|' + Limits.getLimitQueryRows();
