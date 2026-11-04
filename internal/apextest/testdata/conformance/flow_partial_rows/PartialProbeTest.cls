@IsTest private class PartialProbeTest { @IsTest static void observe() { List<String> observed=new List<String>();
for (Integer mode=0; mode<3; mode++) {
 Account bad = new Account(Id='001000000000001AAA',Name='BAD_ID');
 Account flow = new Account(Name='FLOW_REJECT');
 Account ok = new Account(Name='NORMAL');
 List<Account> items = mode==0 ? new List<Account>{bad,flow} : (mode==1 ? new List<Account>{flow,bad} : new List<Account>{ok,flow});
 List<Database.SaveResult> results=Database.insert(items,false);
 List<String> rows=new List<String>();
 for(Database.SaveResult r:results) rows.add(r.isSuccess() ? 'SUCCESS' : String.valueOf(r.getErrors()[0].getStatusCode()));
 observed.add(String.valueOf(mode)+'|'+JSON.serialize(rows));
}

System.assert(false,'PARTIAL_ROWS|'+JSON.serialize(observed)); } }
