List<Contact> oc=[SELECT Id FROM Contact WHERE LastName LIKE 'GLC%']; if(!oc.isEmpty()){delete oc;}
List<Account> oa=[SELECT Id FROM Account WHERE Name LIKE 'GLA%']; if(!oa.isEmpty()){delete oa;}
List<String> PN=new List<String>{'GLAP00','GLAP01','GLAP02','GLAP03','GLAP04','GLAP05','GLAP06','GLAP07','GLAP08','GLAP09','GLAP10','GLAP11','GLAP12'};
List<Account> ps=new List<Account>(); for(String n:PN){ps.add(new Account(Name=n));}
insert ps;
Map<String,Id> pm=new Map<String,Id>(); for(Account p:ps){pm.put(p.Name,p.Id);}
List<String> CIN=new List<String>{'GLAP00:GLC00C4','GLAP05:GLC05C1','GLAP06:GLC06C3','GLAP04:GLC04C1','GLAP07:GLC07C1','GLAP08:GLC08C2','GLAP12:GLC12C1','GLAP05:GLC05C3','GLAP05:GLC05C0','GLAP08:GLC08C0','GLAP00:GLC00C1','GLAP04:GLC04C3','GLAP12:GLC12C4','GLAP10:GLC10C3','GLAP10:GLC10C0','GLAP09:GLC09C1','GLAP11:GLC11C0','GLAP01:GLC01C4','GLAP12:GLC12C3','GLAP01:GLC01C0','GLAP07:GLC07C0','GLAP03:GLC03C3','GLAP06:GLC06C1','GLAP10:GLC10C1','GLAP08:GLC08C4','GLAP09:GLC09C3','GLAP03:GLC03C0','GLAP03:GLC03C4','GLAP08:GLC08C1','GLAP09:GLC09C0','GLAP12:GLC12C0','GLAP05:GLC05C2','GLAP11:GLC11C2','GLAP07:GLC07C4','GLAP00:GLC00C0','GLAP00:GLC00C3','GLAP01:GLC01C1','GLAP07:GLC07C3','GLAP02:GLC02C0','GLAP06:GLC06C0','GLAP03:GLC03C1','GLAP05:GLC05C4','GLAP06:GLC06C2','GLAP10:GLC10C2','GLAP02:GLC02C2','GLAP09:GLC09C4','GLAP02:GLC02C4','GLAP03:GLC03C2','GLAP11:GLC11C4','GLAP10:GLC10C4','GLAP02:GLC02C3','GLAP00:GLC00C2','GLAP11:GLC11C3','GLAP07:GLC07C2','GLAP08:GLC08C3','GLAP01:GLC01C3','GLAP06:GLC06C4','GLAP04:GLC04C4','GLAP01:GLC01C2','GLAP04:GLC04C0','GLAP04:GLC04C2','GLAP11:GLC11C1','GLAP02:GLC02C1','GLAP12:GLC12C2','GLAP09:GLC09C2'};
List<Contact> cs=new List<Contact>();
for(String key:CIN){String[] parts=key.split(':');cs.add(new Contact(LastName=parts[1],AccountId=pm.get(parts[0])));}
insert cs;
List<Account> q=[SELECT Name,(SELECT LastName FROM Contacts WHERE LastName LIKE 'GLC%') FROM Account WHERE Name LIKE 'GLAP%'];
List<String> pq=new List<String>(); List<String> flat=new List<String>();
for(Account p:q){pq.add(p.Name);List<String> cn=new List<String>();for(Contact c:p.Contacts){cn.add(c.LastName);}flat.add(p.Name+'='+String.join(cn,'+'));}
System.debug('O1|HCH|IN|'+String.join(PN,','));
System.debug('O1|HCH|CI|'+String.join(CIN,','));
System.debug('O1|HCH|Q|'+String.join(pq,','));
System.debug('O1|HCH|C|'+String.join(flat,';'));
