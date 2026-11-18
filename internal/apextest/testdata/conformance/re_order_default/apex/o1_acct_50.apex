List<Contact> oc=[SELECT Id FROM Contact WHERE LastName LIKE 'GLC%']; if(!oc.isEmpty()){delete oc;}
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<String> INS=new List<String>{'GLAB0043','GLAB0002','GLAB0001','GLAB0004','GLAB0041','GLAB0032','GLAB0024','GLAB0009','GLAB0039','GLAB0034','GLAB0014','GLAB0016','GLAB0033','GLAB0015','GLAB0046','GLAB0028','GLAB0020','GLAB0031','GLAB0017','GLAB0000','GLAB0038','GLAB0049','GLAB0030','GLAB0047','GLAB0037','GLAB0040','GLAB0012','GLAB0025','GLAB0018','GLAB0011','GLAB0022','GLAB0007','GLAB0003','GLAB0036','GLAB0005','GLAB0042','GLAB0035','GLAB0010','GLAB0021','GLAB0023','GLAB0029','GLAB0013','GLAB0019','GLAB0044','GLAB0048','GLAB0027','GLAB0006','GLAB0026','GLAB0008','GLAB0045'};
List<Account> rows=new List<Account>();
for(String n:INS){rows.add(new Account(Name=n));}
insert rows;
List<Account> q=[SELECT Id,Name FROM Account WHERE Name LIKE 'GLA%'];
List<String> qn=new List<String>(); List<String> qi=new List<String>();
for(Account a:q){qn.add(a.Name);qi.add(String.valueOf(a.Id));}
System.debug('O1|A50|CHUNKS|1');
System.debug('O1|A50|IN|'+String.join(INS,','));
System.debug('O1|A50|Q|'+String.join(qn,','));
System.debug('O1|A50|IDS|'+String.join(qi,','));
