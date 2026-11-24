List<String> labels = new List<String>();
for (Account a : [SELECT AccountNumber FROM Account WHERE Name LIKE 'capture %' ORDER BY BillingCity ASC NULLS FIRST, AccountNumber ASC]) labels.add(a.AccountNumber);
System.debug('ROAD466ROW:'+UserInfo.getLocale()+':'+String.join(labels,','));
