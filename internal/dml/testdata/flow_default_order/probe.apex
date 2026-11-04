List<Database.SaveResult> out = new List<Database.SaveResult>();
Account a = new Account(Name='DEFAULT_PROBE_OMITTED');
Account b = new Account(Name='DEFAULT_PROBE_EXPLICIT'); b.put('ProbeFlag__c', true);
Account d = new Account(Name='REJECT_PROBE');
Account c = new Account(Name='DEFAULT_PROBE_NULL'); c.put('ProbeFlag__c', null);
for (Account row : new List<Account>{a,b,c,d}) out.add(Database.insert(row, false));
for (Database.SaveResult result : out) System.debug('DEFAULT_ORDER|' + result.isSuccess() + '|' + (result.isSuccess() ? '' : result.getErrors()[0].getMessage()));
