Long T0=System.now().getTime();
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
Long T1=System.now().getTime();
Account a=new Account(Name='C0'); insert a;
Long T2=System.now().getTime();
Account q=[SELECT CreatedDate,LastModifiedDate,SystemModstamp FROM Account WHERE Id=:a.Id];
System.debug('T4|CLK|'+T0+'|'+T1+'|'+T2+'|'+q.CreatedDate.getTime()+'|'+q.LastModifiedDate.getTime()+'|'+q.SystemModstamp.getTime());
