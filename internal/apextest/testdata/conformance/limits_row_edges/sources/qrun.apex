List<Account> l = [SELECT Id FROM Account WHERE Name LIKE 'QRSEED%'];
obs = l.size() + '|' + Limits.getQueryRows() + '|' + Limits.getLimitQueryRows();
