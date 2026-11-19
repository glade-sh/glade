List<Contact> oc=[SELECT Id FROM Contact WHERE LastName LIKE 'GLC%']; if(!oc.isEmpty()){delete oc;}
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<String> INS=new List<String>{'GLAA0002','GLAA0000','GLAA0001','GLAA0004','GLAA0003'};
List<Account> rows=new List<Account>();
for(String n:INS){rows.add(new Account(Name=n));}
insert rows;
List<Account> q=[SELECT Id,Name FROM Account WHERE Name LIKE 'GLA%'];
List<String> qn=new List<String>(); List<String> qi=new List<String>();
for(Account a:q){qn.add(a.Name);qi.add(String.valueOf(a.Id));}
System.debug('O1|A5|CHUNKS|1');
System.debug('O1|A5|IN|'+String.join(INS,','));
System.debug('O1|A5|Q|'+String.join(qn,','));
System.debug('O1|A5|IDS|'+String.join(qi,','));
