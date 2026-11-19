Long T0=System.now().getTime();
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<Integer> SZ=new List<Integer>{2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2,2};
for(Integer b=0;b<SZ.size();b++){
 List<Account> chunk=new List<Account>();
 for(Integer r=0;r<SZ[b];r++){chunk.add(new Account(Name='GLAB'+b+'R'+r));}
 insert chunk;
}
Long T1=System.now().getTime();
for(Account a:[SELECT Name,CreatedDate,LastModifiedDate,SystemModstamp FROM Account]){
 System.debug('T4|s50x2|R|'+a.Name+'|'+a.CreatedDate.getTime()+'|'+a.LastModifiedDate.getTime()+'|'+a.SystemModstamp.getTime());
}
System.debug('T4|s50x2|T0|'+T0+'|'+T1);
System.debug('T4|s50x2|N|'+[SELECT COUNT() FROM Account]);
