@IsTest private class DefaultOrderProbeTest {
 @IsTest static void observe() {
  Account a = new Account(Name='DEFAULT_PROBE_OMITTED');
  Account b = new Account(Name='DEFAULT_PROBE_EXPLICIT'); b.put('ProbeFlag__c', true);
  Account d = new Account(Name='REJECT_PROBE');
Account c = new Account(Name='DEFAULT_PROBE_NULL'); c.put('ProbeFlag__c', null);
  List<String> rows = new List<String>();
  for (Account row : new List<Account>{a,b,c,d}) {
   Database.SaveResult r = Database.insert(row, false);
   rows.add(String.valueOf(r.isSuccess()) + '|' + (r.isSuccess() ? '' : r.getErrors()[0].getMessage()));
  }
  System.assert(false, 'DEFAULT_ORDER|' + JSON.serialize(rows));
 }
}
