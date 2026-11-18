Database.QueryLocator ql = Database.getQueryLocator([SELECT Id FROM Account WHERE Name LIKE 'QRSEED%']);
obs = Limits.getQueryLocatorRows() + '|' + Limits.getLimitQueryLocatorRows() + '|' + Limits.getQueryRows();
