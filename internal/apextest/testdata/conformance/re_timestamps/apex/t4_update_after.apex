Long T0=System.now().getTime();
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<String> NM=new List<String>{'GLAR0','GLAR1','GLAR2','GLAR3','GLAR4'};
List<Account> rows=new List<Account>(); for(String n:NM){rows.add(new Account(Name=n));}
insert rows;
Map<String,List<Long>> before=new Map<String,List<Long>>();
for(Account a:[SELECT Name,CreatedDate,LastModifiedDate,SystemModstamp FROM Account]){before.put(a.Name,new List<Long>{a.CreatedDate.getTime(),a.LastModifiedDate.getTime(),a.SystemModstamp.getTime()});}
List<Account> extra=new List<Account>{new Account(Name='GLAR5'),new Account(Name='GLAR6')}; insert extra;
rows[2].Description='u2'; update rows[2];
Long T1=System.now().getTime();
for(Account a:[SELECT Name,CreatedDate,LastModifiedDate,SystemModstamp FROM Account]){
 List<Long> b=before.containsKey(a.Name)?before.get(a.Name):new List<Long>{-1,-1,-1};
 System.debug('T4|upd_after|R|'+a.Name+'|'+b[0]+'|'+b[1]+'|'+b[2]+'|'+a.CreatedDate.getTime()+'|'+a.LastModifiedDate.getTime()+'|'+a.SystemModstamp.getTime());
}
System.debug('T4|upd_after|T0|'+T0+'|'+T1);
