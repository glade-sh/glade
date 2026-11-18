Long T0=System.now().getTime();
List<Contact> oc=[SELECT Id FROM Contact WHERE LastName LIKE 'GLC%']; if(!oc.isEmpty()){delete oc;}
List<Account> oo=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oo.isEmpty()){delete oo;}
Account p=new Account(Name='GLATP0'); insert p;
Long T1=System.now().getTime();
Contact c=new Contact(LastName='GLCTC0',AccountId=p.Id); insert c;
Long T2=System.now().getTime();
Account pq=[SELECT CreatedDate,LastModifiedDate,SystemModstamp FROM Account WHERE Id=:p.Id];
Contact cq=[SELECT CreatedDate,LastModifiedDate,SystemModstamp FROM Contact WHERE Id=:c.Id];
System.debug('T4|child|P|'+pq.CreatedDate.getTime()+'|'+pq.LastModifiedDate.getTime()+'|'+pq.SystemModstamp.getTime());
System.debug('T4|child|C|'+cq.CreatedDate.getTime()+'|'+cq.LastModifiedDate.getTime()+'|'+cq.SystemModstamp.getTime());
System.debug('T4|child|T|'+T0+'|'+T1+'|'+T2);
