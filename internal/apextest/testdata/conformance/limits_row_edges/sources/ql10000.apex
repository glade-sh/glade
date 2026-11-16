Database.QueryLocator ql = Database.getQueryLocator([SELECT Id FROM Account WHERE Name LIKE 'QRSEED%' LIMIT 10000]);
obs = Limits.getQueryLocatorRows() + '|' + Limits.getLimitQueryLocatorRows() + '|' + Limits.getQueryRows();
