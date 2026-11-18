List<Contact> oc=[SELECT Id FROM Contact WHERE LastName LIKE 'GLC%']; if(!oc.isEmpty()){delete oc;}
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<String> INS=new List<String>{'GLAE0014','GLAE0005','GLAE0009','GLAE0000','GLAE0013','GLAE0007','GLAE0010','GLAE0006','GLAE0002','GLAE0011','GLAE0003','GLAE0016','GLAE0012','GLAE0008','GLAE0004','GLAE0015','GLAE0001'};
List<Account> rows=new List<Account>();
for(String n:INS){rows.add(new Account(Name=n));}
insert rows;
List<Account> q=[SELECT Id,Name FROM Account WHERE Name LIKE 'GLA%'];
List<String> qn=new List<String>(); List<String> qi=new List<String>();
for(Account a:q){qn.add(a.Name);qi.add(String.valueOf(a.Id));}
System.debug('O1|H17|CHUNKS|1');
System.debug('O1|H17|IN|'+String.join(INS,','));
System.debug('O1|H17|Q|'+String.join(qn,','));
System.debug('O1|H17|IDS|'+String.join(qi,','));
