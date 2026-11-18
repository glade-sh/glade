public class DupProbe {
 public static Map<Id,String> lab=new Map<Id,String>();
 public static String L(Id i) {return lab.containsKey(i)?lab.get(i):'?';}
 public static String mf(String t) {return t=='DupGate__c'?'Mode__c':'Mode'+t.substring(3).replace('__c','')+'__c';}
 public static SObject mk(String t,String name,String key,String alt,String scope) {
  SObject s=Schema.getGlobalDescribe().get(t).newSObject();s.put('Name',name);s.put('Key__c',key);s.put('Alt__c',alt);s.put(mf(t),scope);return s;
 }
 public static void reg(SObject s) {lab.put(s.Id,(String)s.get('Name'));}
 public static void regAll(List<SObject> rows) {for(SObject s:rows) reg(s);}
 public static Map<String,Object> dr(Datacloud.DuplicateResult d) {
  List<Object> ms=new List<Object>();
  for(Datacloud.MatchResult m:d.getMatchResults()) {
   List<Object> rs=new List<Object>();
   for(Datacloud.MatchRecord r:m.getMatchRecords()) {
    List<String> fds=new List<String>();for(Datacloud.FieldDiff f:r.getFieldDiffs()) fds.add(f.getName()+'='+String.valueOf(f.getDifference()));fds.sort();
    List<String> ai=new List<String>();for(Datacloud.AdditionalInformationMap a:r.getAdditionalInformation()) ai.add(a.getName()+'='+a.getValue());ai.sort();
    rs.add(new Map<String,Object>{'n'=>L(r.getRecord().Id),'c'=>r.getMatchConfidence(),'fd'=>fds,'ai'=>ai});
   }
   ms.add(new Map<String,Object>{'rule'=>m.getRule(),'ent'=>m.getEntityType(),'eng'=>m.getMatchEngine(),'size'=>m.getSize(),'ok'=>m.isSuccess(),'recs'=>rs});
  }
  return new Map<String,Object>{'rule'=>d.getDuplicateRule(),'ent'=>d.getDuplicateRuleEntityType(),'allow'=>d.isAllowSave(),'msg'=>d.getErrorMessage(),'m'=>ms};
 }
 public static Map<String,Object> sv(Database.SaveResult r) {
  List<Object> es=new List<Object>();
  for(Database.Error e:r.getErrors()) {
   Map<String,Object> x=new Map<String,Object>{'st'=>String.valueOf(e.getStatusCode()),'msg'=>e.getMessage()};
   if(e instanceof Database.DuplicateError) x.put('dup',dr(((Database.DuplicateError)e).getDuplicateResult()));
   es.add(x);
  }
  return new Map<String,Object>{'ok'=>r.isSuccess(),'err'=>es};
 }
 public static List<Object> svs(List<Database.SaveResult> rs) {List<Object> o=new List<Object>();for(Database.SaveResult r:rs) o.add(sv(r));return o;}
 public static List<Object> fnd(List<SObject> rows) {
  List<Object> out=new List<Object>();
  for(Datacloud.FindDuplicatesResult f:Datacloud.FindDuplicates.findDuplicates(rows)) {
   List<Object> ds=new List<Object>();for(Datacloud.DuplicateResult d:f.getDuplicateResults()) ds.add(dr(d));
   out.add(new Map<String,Object>{'ok'=>f.isSuccess(),'d'=>ds});
  }
  return out;
 }
 public static List<Object> fbi(List<Id> ids) {
  List<Object> out=new List<Object>();
  for(Datacloud.FindDuplicatesResult f:Datacloud.FindDuplicatesByIds.findDuplicatesByIds(ids)) {
   List<Object> ds=new List<Object>();for(Datacloud.DuplicateResult d:f.getDuplicateResults()) ds.add(dr(d));
   out.add(new Map<String,Object>{'ok'=>f.isSuccess(),'d'=>ds});
  }
  return out;
 }
 public static Integer cnt(List<Object> found,String name) {
  Integer n=0;
  for(Object f:found) for(Object d:(List<Object>)((Map<String,Object>)f).get('d')) for(Object m:(List<Object>)((Map<String,Object>)d).get('m')) for(Object r:(List<Object>)((Map<String,Object>)m).get('recs')) if(name==null||((Map<String,Object>)r).get('n')==name) n++;
  return n;
 }
 public static Database.DMLOptions allow() {Database.DMLOptions o=new Database.DMLOptions();o.DuplicateRuleHeader.allowSave=true;return o;}
 public static void ord(String obj,String scope,String mode,Map<String,Object> o) {
  SObject a=mk(obj,'seed','k1',null,'seed');insert a;reg(a);
  if(mode=='upd'||mode=='updopt') {
   SObject b=mk(obj,'cand','k2',null,scope);insert b;reg(b);b.put('Key__c','k1');
   if(mode=='updopt') o.put('upd',sv(Database.update(b,allow()))); else o.put('upd',sv(Database.update(b,false)));
  } else {
   SObject b=mk(obj,'cand','k1',null,scope);
   if(mode=='opt') o.put('ins',sv(Database.insert(b,allow()))); else o.put('ins',sv(Database.insert(b,false)));
  }
  o.put('find',fnd(new List<SObject>{mk(obj,'q','k1',null,scope)}));
  o.put('n',Database.countQuery('SELECT count() FROM '+obj));
 }
 public static void gate(String key,Map<String,Object> o) {
  String scope=key=='mbBlank'?'mb':key;String alt=scope=='items'?'a':null;String k=key=='mbBlank'?null:'k';
  String ch=scope=='items'?'Alt__c':'Key__c';String orig=scope=='items'?'a':k;
  SObject a=mk('DupGate__c','A',k,alt,scope);SObject b=mk('DupGate__c','B',k,alt,scope);b.put('Other__c','o1');
  List<SObject> rows=new List<SObject>{a,b};o.put('seed',svs(Database.insert(rows,false)));regAll(rows);
  b.put('Other__c','o2');o.put('u1',sv(Database.update(b,false)));
  b.put(ch,orig);o.put('u2',sv(Database.update(b,false)));
  b.put(ch,'k3');o.put('u3',sv(Database.update(b,false)));
  b.put(ch,orig);o.put('u4',sv(Database.update(b,false)));
 }
 public static void x99(Map<String,Object> o) {
  String a99='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa'.substring(0,99);
  String a98=a99.substring(0,98);String e=String.fromCharArray(new List<Integer>{233});
  List<List<String>> vs=new List<List<String>>{
   new List<String>{'last100',a99+'X',a99+'Y'},new List<String>{'c99',a99+'X',a98+'bX'},new List<String>{'same',a99+'X',a99+'X'},
   new List<String>{'q99',a99+'X',a99},new List<String>{'uni',a98+e+'X',a98+e+'Y'},new List<String>{'uniDiff',a98+e+'X',a98+'eX'},
   new List<String>{'c98',a99+'X',a98.substring(0,97)+'bXX'}};
  List<Object> out=new List<Object>();
  for(List<String> v:vs) {
   Savepoint sp=Database.setSavepoint();
   SObject s=mk('DupKey__c','S',v[1],null,'seed');insert s;reg(s);
   SObject q=mk('DupKey__c','Q',v[2],null,'x99');
   Map<String,Object> r=new Map<String,Object>{'v'=>v[0],'find'=>cnt(fnd(new List<SObject>{q}),null)};
   r.put('dml',sv(Database.insert(mk('DupKey__c','Q',v[2],null,'x99'),false)));
   out.add(r);Database.rollback(sp);
  }
  o.put('x',out);
 }
 public static void cap(Map<String,Object> o) {
  List<Object> out=new List<Object>();
  for(String pos:new List<String>{'last','first','mid','none'}) {
   Savepoint sp=Database.setSavepoint();
   List<SObject> rows=new List<SObject>();
   for(Integer i=0;i<121;i++) {
    Boolean t=(pos=='last'&&i==120)||(pos=='first'&&i==0)||(pos=='mid'&&i==60);
    rows.add(mk('DupKey__c',t?'target':'r'+i,'ck',t?'target':'n'+i,'seed'));
   }
   insert rows;regAll(rows);
   String qa=pos=='none'?'nomatch':'target';
   List<Object> f=fnd(new List<SObject>{mk('DupKey__c','Q','ck',qa,'cap')});
   Map<String,Object> r=new Map<String,Object>{'pos'=>pos,'total'=>cnt(f,null),'target'=>cnt(f,'target')};
   r.put('dml',sv(Database.insert(mk('DupKey__c','Q','ck',qa,'cap'),false)));
   out.add(r);Database.rollback(sp);
  }
  o.put('c',out);
 }
 public static void mapc(Map<String,Object> o) {
  SObject t=mk('DupTgt__c','tgt','m1',null,'seed');insert t;reg(t);
  List<Object> out=new List<Object>();
  List<List<String>> vs=new List<List<String>>{new List<String>{'mapped','m1','zz'},new List<String>{'unequal','m2','zz'},new List<String>{'sameName',null,null},new List<String>{'unmappedKey','zz','zz'}};
  for(List<String> v:vs) {
   String key=v[0]=='unmappedKey'?'m1':null;String alt=v[0]=='mapped'?'m1':(v[0]=='unequal'?'m2':(v[0]=='unmappedKey'?'zz':null));
   Savepoint sp=Database.setSavepoint();
   Map<String,Object> r=new Map<String,Object>{'v'=>v[0]};
   r.put('find',fnd(new List<SObject>{mk('DupSrc__c','Q',key,alt,'xmap')}));
   r.put('dml',sv(Database.insert(mk('DupSrc__c','Q',key,alt,'xmap'),false)));
   out.add(r);Database.rollback(sp);
  }
  o.put('m',out);
 }

 public static void sgate(String key,Map<String,Object> o) {
  String scope=key=='mbBlank'?'mb':key;String ch=scope=='items'?'Alt__c':'Key__c';
  String origCh=key=='mbBlank'?null:(scope=='items'?'a':'k');
  String k=key=='mbBlank'?null:'k';String alt=scope=='items'?'a':null;
  SObject a=mk('DupGate__c','A',k,alt,scope);insert a;reg(a);
  SObject b=mk('DupGate__c','B',k,alt,scope);b.put(ch,scope=='items'?'b':'k2');insert b;reg(b);
  SObject s1=b.getSObjectType().newSObject(b.Id);s1.put('Other__c','o2');o.put('u1',sv(Database.update(s1,false)));
  SObject s2=b.getSObjectType().newSObject(b.Id);s2.put(ch,origCh);o.put('u2',sv(Database.update(s2,false)));
  SObject c=mk('DupGate__c','C',k,alt,scope);insert c;reg(c);
  SObject s3=c.getSObjectType().newSObject(c.Id);s3.put('Other__c','x');o.put('u3',sv(Database.update(s3,false)));
  SObject s4=c.getSObjectType().newSObject(c.Id);s4.put(ch,origCh);o.put('u4',sv(Database.update(s4,false)));
  SObject s5=c.getSObjectType().newSObject(c.Id);s5.put(ch,'k5');o.put('u5',sv(Database.update(s5,false)));
  SObject s6=c.getSObjectType().newSObject(c.Id);s6.put(ch,origCh);o.put('u6',sv(Database.update(s6,false)));
 }
 public static void fdiff2(Map<String,Object> o) {
  Contact c=contact();seed(c);lab.put(c.Id,'contact');
  Contact c2=contact();c2.LastName='Orchardson';c2.Email='iris2@orchard.invalid';c2.Phone=null;c2.MailingStreet=null;c2.MailingCity=null;seed(c2);lab.put(c2.Id,'contactNoPhone');
  Contact q1=new Contact(FirstName='Iriss',LastName='Orchard',Email='iris@orchard.invalid',Phone='4155550143');
  o.put('fuzzyFirst',fnd(new List<SObject>{q1}));
  Contact q2=new Contact(FirstName='Iris',LastName='Orchardson',Email='iris2@orchard.invalid',Phone='4155550143',MailingStreet='1 X St',MailingCity='Salem');
  o.put('queryHasStoredNull',fnd(new List<SObject>{q2}));
  Account a=new Account(Name='Orchard Studio',Phone='4155550142',BillingStreet='17 Orchard Way',BillingCity='Portland',BillingState='OR',BillingPostalCode='97201',BillingCountry='USA',Website='https://orchard.invalid');
  Database.SaveResult ar=Database.insert(a,allow());lab.put(a.Id,'acct');
  Account qa=new Account(Name='Orchard Studio',Phone='4155550199',BillingStreet='17 Orchard Way',BillingCity='Salem',BillingPostalCode='97201');
  o.put('acct',fnd(new List<SObject>{qa}));
  Lead l=lead();seed(l);lab.put(l.Id,'lead');
  Lead ql=lead();ql.Phone='4155550100';ql.Company='Other Co';ql.Street=null;
  o.put('lead',fnd(new List<SObject>{ql}));
  o.put('dmlContact',sv(Database.insert(new Contact(FirstName='Iris',LastName='Orchard',Email='iris@orchard.invalid',Phone='4155550999'),false)));
 }

 public static void sgate2(Map<String,Object> o) {
  SObject a=mk('DupGate__c','A','k',null,'na');insert a;reg(a);
  SObject d=mk('DupGate__c','D','k2',null,'na');insert d;reg(d);
  SObject s1=d.getSObjectType().newSObject(d.Id);s1.put('Key__c','k5');o.put('awayOk',sv(Database.update(s1,false)));
  SObject s2=d.getSObjectType().newSObject(d.Id);s2.put('Key__c','k');o.put('backIntoDup',sv(Database.update(s2,false)));
  SObject e=mk('DupGate__c','E','k2',null,'na');insert e;reg(e);
  SObject t1=e.getSObjectType().newSObject(e.Id);t1.put('Key__c','k');o.put('firstInto',sv(Database.update(t1,false)));
  SObject t2=e.getSObjectType().newSObject(e.Id);t2.put('Key__c','k');o.put('repeatInto',sv(Database.update(t2,false)));
  SObject f=mk('DupGate__c','F','k2',null,'na');insert f;reg(f);
  SObject u1=f.getSObjectType().newSObject(f.Id);u1.put('Other__c','z');o.put('otherFirst',sv(Database.update(u1,false)));
  SObject u2=f.getSObjectType().newSObject(f.Id);u2.put('Key__c','k');o.put('thenInto',sv(Database.update(u2,false)));
 }
 public static void fcase(Map<String,Object> o) {
  List<Object> out=new List<Object>();
  for(List<String> v:new List<List<String>>{new List<String>{'case','AbC','abc'},new List<String>{'trail','abc','abc '},new List<String>{'lead','abc',' abc'},new List<String>{'inner','a b','a  b'}}) {
   Savepoint sp=Database.setSavepoint();
   SObject s=mk('DupKey__c','S',v[1],null,'seed');insert s;reg(s);
   SObject q=mk('DupKey__c','Q',v[2],null,'x99');
   List<Object> f=fnd(new List<SObject>{q});
   out.add(new Map<String,Object>{'v'=>v[0],'find'=>f});
   Database.rollback(sp);
  }
  o.put('x',out);
 }
 public static Map<String,Object> exc(Exception e) {return new Map<String,Object>{'x'=>e.getTypeName(),'xm'=>e.getMessage()};}
 public static void none(Map<String,Object> o) {
  List<String> kinds=new List<String>{'DupNone__c','DupNone2__c','Opportunity'};
  for(String k:kinds) {
   Map<String,Object> r=new Map<String,Object>();
   SObject q;
   if(k=='Opportunity') q=new Opportunity(Name='opp',StageName='Prospecting',CloseDate=Date.today()); else q=mk(k,'q','k1',null,'off');
   try {r.put('find',fnd(new List<SObject>{q}));} catch(Exception e) {r.put('find',exc(e));}
   SObject s; if(k=='Opportunity') s=new Opportunity(Name='opp',StageName='Prospecting',CloseDate=Date.today()); else s=mk(k,'q','k1',null,'off');
   try {insert s;r.put('ins',true);} catch(Exception e) {r.put('ins',exc(e));}
   try {r.put('byIds',fbi(new List<Id>{s.Id}));} catch(Exception e) {r.put('byIds',exc(e));}
   try {r.put('findSaved',fnd(new List<SObject>{s}));} catch(Exception e) {r.put('findSaved',exc(e));}
   o.put(k,r);
  }
 }
 public static Contact contact() {return new Contact(FirstName='Iris',LastName='Orchard',Email='iris@orchard.invalid',Phone='4155550143',MailingStreet='18 Orchard Way',MailingCity='Portland',MailingState='OR',MailingPostalCode='97201',MailingCountry='USA');}
 public static Lead lead() {return new Lead(FirstName='Iris',LastName='Orchard',Company='Orchard',Email='iris@orchard.invalid',Phone='4155550143',Street='18 Orchard Way',City='Portland',State='OR',PostalCode='97201',Country='USA');}
 public static void seed(SObject row) {Database.SaveResult r=Database.insert(row,allow());System.assert(r.isSuccess(),'seed failed');}
 public static void mix(Map<String,Object> o) {
  Contact c=contact();seed(c);lab.put(c.Id,'contact');Lead l=lead();seed(l);lab.put(l.Id,'lead');
  try {o.put('lc',fnd(new List<SObject>{lead(),contact()}));} catch(Exception e) {o.put('lc',exc(e));}
  try {o.put('cl',fnd(new List<SObject>{contact(),lead()}));} catch(Exception e) {o.put('cl',exc(e));}
  try {o.put('idLc',fbi(new List<Id>{l.Id,c.Id}));} catch(Exception e) {o.put('idLc',exc(e));}
  try {o.put('idCl',fbi(new List<Id>{c.Id,l.Id}));} catch(Exception e) {o.put('idCl',exc(e));}
 }
 public static void diff(Map<String,Object> o) {
  Contact c=contact();seed(c);lab.put(c.Id,'contact');
  Contact same=contact();
  Contact other=contact();other.Phone='4155550199';other.MailingCity='Salem';
  Contact blank=contact();blank.Phone=null;blank.MailingStreet=null;
  o.put('same',fnd(new List<SObject>{same}));o.put('other',fnd(new List<SObject>{other}));o.put('blank',fnd(new List<SObject>{blank}));
  SObject a=mk('DupGate__c','A','k',null,'mb');SObject b=mk('DupGate__c','B',null,null,'mb');b.put('Other__c','x');
  List<SObject> rows=new List<SObject>{a,b};insert rows;regAll(rows);
  o.put('mbKey',fnd(new List<SObject>{mk('DupGate__c','Q','k',null,'mb')}));
  o.put('mbNull',fnd(new List<SObject>{mk('DupGate__c','Q',null,null,'mb')}));
 }
 public static void optBlk(Map<String,Object> o) {
  SObject p=mk('DupOpt__c','P','k',null,'seed');insert p;reg(p);
  List<String> modes=new List<String>{'allOrNoneFalse','allOrNoneTrue','unset','boolFalse','default'};
  List<Object> out=new List<Object>();
  for(String m:modes) {
   Savepoint sp=Database.setSavepoint();
   List<SObject> rows=new List<SObject>{mk('DupOpt__c','U1','u1',null,'blk'),mk('DupOpt__c','D','k',null,'blk'),mk('DupOpt__c','U2','u2',null,'blk')};
   Map<String,Object> r=new Map<String,Object>{'m'=>m};
   try {
    List<Database.SaveResult> rs;
    if(m=='boolFalse') rs=Database.insert(rows,false);
    else if(m=='default') rs=Database.insert(rows);
    else {Database.DMLOptions d=new Database.DMLOptions();if(m=='allOrNoneFalse') d.optAllOrNone=false;if(m=='allOrNoneTrue') d.optAllOrNone=true;rs=Database.insert(rows,d);}
    r.put('rs',svs(rs));
   } catch(Exception e) {r.put('exc',exc(e));}
   r.put('persisted',Database.countQuery('SELECT count() FROM DupOpt__c WHERE ModeOpt__c=\'blk\''));
   out.add(r);Database.rollback(sp);
  }
  o.put('o',out);
 }
 public static void optAlrt(Map<String,Object> o) {
  SObject p=mk('DupOpt__c','P','k',null,'seed');insert p;reg(p);
  List<Object> out=new List<Object>();
  for(String m:new List<String>{'setOptIns','overIns','setOptBoolIns','listOverIns','plainIns','setOptUpd','overUpd','plainUpd','setOptUpdBool'}) {
   Savepoint sp=Database.setSavepoint();
   Map<String,Object> r=new Map<String,Object>{'m'=>m};
   try {
    if(m.endsWith('Ins')||m=='setOptBoolIns') {
     SObject b=mk('DupOpt__c','C','k',null,'alrt');
     if(m=='setOptIns') {b.setOptions(allow());insert b;r.put('saved',true);}
     else if(m=='overIns') r.put('rs',sv(Database.insert(b,allow())));
     else if(m=='setOptBoolIns') {b.setOptions(allow());r.put('rs',sv(Database.insert(b,false)));}
     else if(m=='listOverIns') r.put('rs',svs(Database.insert(new List<SObject>{b},allow())));
     else r.put('rs',sv(Database.insert(b,false)));
    } else {
     SObject b=mk('DupOpt__c','C','k2',null,'alrt');insert b;b.put('Key__c','k');
     if(m=='setOptUpd') {b.setOptions(allow());update b;r.put('saved',true);}
     else if(m=='overUpd') r.put('rs',sv(Database.update(b,allow())));
     else if(m=='setOptUpdBool') {b.setOptions(allow());r.put('rs',sv(Database.update(b,false)));}
     else r.put('rs',sv(Database.update(b,false)));
    }
   } catch(Exception e) {r.put('exc',exc(e));}
   out.add(r);Database.rollback(sp);
  }
  o.put('o',out);
 }
 public static void repWrite(String v) {
  List<SObject> n=new List<SObject>{mk('DupOpt__c','N1_'+v,'rN'+v,null,'rep'),mk('DupOpt__c','N2_'+v,'rN'+v,null,'rep')};
  insert n;
  SObject s=mk('DupOpt__c','S0_'+v,'rS'+v,null,'seed');insert s;
  List<SObject> m=new List<SObject>{mk('DupOpt__c','S1_'+v,'rS'+v,null,'rep'),mk('DupOpt__c','S2_'+v,'rS'+v,null,'rep')};
  insert m;
  SObject c=mk('DupOpt__c','U0_'+v,'rU'+v,null,'seed');insert c;
  SObject d=mk('DupOpt__c','U1_'+v,'rUx'+v,null,'rep');insert d;d.put('Key__c','rU'+v);update d;
  System.debug('REPWOBS|'+v+'|written');
 }
 public static void repRead(String v) {
  Map<Id,String> nm=new Map<Id,String>();
  for(DupOpt__c r:[SELECT Id,Name FROM DupOpt__c WHERE Name LIKE :('%_'+v)]) nm.put(r.Id,r.Name);
  Map<Id,List<String>> sets=new Map<Id,List<String>>();
  Integer totalSets=[SELECT count() FROM DuplicateRecordSet];
  for(DuplicateRecordItem i:[SELECT DuplicateRecordSetId,RecordId FROM DuplicateRecordItem]) {
   if(!nm.containsKey(i.RecordId)) continue;
   if(!sets.containsKey(i.DuplicateRecordSetId)) sets.put(i.DuplicateRecordSetId,new List<String>());
   sets.get(i.DuplicateRecordSetId).add(nm.get(i.RecordId));
  }
  List<String> out=new List<String>();
  for(List<String> l:sets.values()) {l.sort();out.add(String.join(l,'+'));}
  out.sort();
  System.debug('REPROBS|'+v+'|'+JSON.serialize(new Map<String,Object>{'sets'=>out,'totalSets'=>totalSets,'records'=>nm.size()}));
 }
 public static void lb(String t) {for(SObject r:Database.query('SELECT Id,Name FROM '+t)) lab.put(r.Id,(String)r.get('Name'));}
 public static Map<String,Object> ins1(String t,String name,String key,String scope,Boolean opt) {
  SObject s=mk(t,name,key,null,scope);
  Map<String,Object> r=sv(opt?Database.insert(s,allow()):Database.insert(s,false));
  if(s.Id!=null) reg(s);
  return r;
 }
 public static Map<String,Object> up1(String t,String name,String f,String val,Boolean opt) {
  lb(t);
  SObject r=Database.query('SELECT Id FROM '+t+' WHERE Name=\''+name+'\' LIMIT 1');
  SObject s=r.getSObjectType().newSObject(r.Id);s.put(f,val);
  return opt?sv(Database.update(s,allow())):sv(Database.update(s,false));
 }
 public static void rptSame(Map<String,Object> o) {
  String t='DupGate__c';
  o.put('insA',ins1(t,'A','pk','na',false));o.put('insC',ins1(t,'C','pk','na',false));
  o.put('toK5',up1(t,'C','Key__c','pk5',false));o.put('back',up1(t,'C','Key__c','pk',false));
  o.put('toK5b',up1(t,'C','Key__c','pk5',false));o.put('backb',up1(t,'C','Key__c','pk',false));
 }
 public static void rptOther(Map<String,Object> o) {
  String t='DupGate__c';
  ins1(t,'A','pk','na',false);ins1(t,'B','pk7','na',false);
  o.put('insC',ins1(t,'C','pk','na',false));
  o.put('other',up1(t,'C','Other__c','o2',false));
  o.put('to7',up1(t,'C','Key__c','pk7',false));
  o.put('back',up1(t,'C','Key__c','pk',false));
  o.put('insF',ins1(t,'F','pk','na',false));
  o.put('fAway',up1(t,'F','Key__c','pk5',false));
  o.put('fTo7',up1(t,'F','Key__c','pk7',false));
  o.put('insG',ins1(t,'G','pk','na',false));
  o.put('gDirect7',up1(t,'G','Key__c','pk7',false));
 }
 public static void rptCtrl(Map<String,Object> o) {
  String t='DupGate__c';
  ins1(t,'A','pk','na',false);ins1(t,'B','pk7','na',false);
  o.put('insE',ins1(t,'E','pk2','na',false));
  o.put('into',up1(t,'E','Key__c','pk',false));o.put('intoAgain',up1(t,'E','Key__c','pk',false));
  o.put('away',up1(t,'E','Key__c','pk5',false));o.put('backInto',up1(t,'E','Key__c','pk',false));
  o.put('insD',ins1(t,'D','pk2','na',false));
  o.put('dAway',up1(t,'D','Key__c','pk5',false));o.put('dTo7',up1(t,'D','Key__c','pk7',false));
 }
 public static void rptAlrt(Map<String,Object> o) {
  String t='DupOpt__c';
  ins1(t,'A','pk','alrt',false);
  o.put('insC',ins1(t,'C','pk','alrt',true));
  o.put('toK5',up1(t,'C','Key__c','pk5',false));o.put('back',up1(t,'C','Key__c','pk',false));o.put('backAgain',up1(t,'C','Key__c','pk',false));
  o.put('insD',ins1(t,'D','pk2','alrt',false));
  o.put('into',up1(t,'D','Key__c','pk',false));o.put('intoAgain',up1(t,'D','Key__c','pk',false));
  o.put('dAway',up1(t,'D','Key__c','pk5',false));o.put('dBack',up1(t,'D','Key__c','pk',false));
 }
 public static void rptAllow(Map<String,Object> o) {
  String t='DupGate__c';
  ins1(t,'A','pk','na',false);
  o.put('insE',ins1(t,'E','pk2','na',false));
  o.put('blocked',up1(t,'E','Key__c','pk',false));o.put('retryAllow',up1(t,'E','Key__c','pk',true));
  o.put('away',up1(t,'E','Key__c','pk5',false));o.put('backPlain',up1(t,'E','Key__c','pk',false));
  o.put('insF',ins1(t,'F','pk2','na',false));
  o.put('fAllow',up1(t,'F','Key__c','pk',true));
  o.put('fAway',up1(t,'F','Key__c','pk5',false));o.put('fBack',up1(t,'F','Key__c','pk',false));
 }
 public static Map<String,Object> tk(String tok,String obj) {
  String kp=Schema.getGlobalDescribe().get(obj).getDescribe().getKeyPrefix();
  return new Map<String,Object>{'len'=>tok.length(),'p3'=>tok.length()>=3?tok.substring(0,3):tok,'isName'=>tok==obj,'isKeyPrefix'=>tok==kp,'startsKeyPrefix'=>tok.startsWith(kp),'kp'=>kp,'raw'=>tok};
 }
 public static Map<String,Object> entOf(List<Datacloud.FindDuplicatesResult> fr,String obj) {
  List<Object> rs=new List<Object>();
  for(Datacloud.FindDuplicatesResult f:fr) for(Datacloud.DuplicateResult d:f.getDuplicateResults()) {
   List<Object> ms=new List<Object>();
   for(Datacloud.MatchResult m:d.getMatchResults()) ms.add(new Map<String,Object>{'n'=>m.getSize(),'ent'=>tk(m.getEntityType(),obj)});
   rs.add(new Map<String,Object>{'top'=>tk(d.getDuplicateRuleEntityType(),obj),'m'=>ms});
  }
  return new Map<String,Object>{'obj'=>obj,'r'=>rs};
 }
 public static void ent(Map<String,Object> o) {
  List<Object> out=new List<Object>();
  for(List<String> v:new List<List<String>>{new List<String>{'DupGate__c','na'},new List<String>{'DupOpt__c','blk'},new List<String>{'DupKey__c','x99'}}) {
   String obj=v[0];
   Map<String,Object> r=new Map<String,Object>{'obj'=>obj};
   r.put('empty',entOf(Datacloud.FindDuplicates.findDuplicates(new List<SObject>{mk(obj,'q','zz-none',null,v[1])}),obj));
   SObject s=mk(obj,'S','zz-seed',null,v[1]);insert s;reg(s);
   r.put('matched',entOf(Datacloud.FindDuplicates.findDuplicates(new List<SObject>{mk(obj,'q','zz-seed',null,v[1])}),obj));
   SObject u=mk(obj,'U','zz-solo',null,v[1]);insert u;reg(u);
   r.put('byIdsEmpty',entOf(Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{u.Id}),obj));
   out.add(r);
  }
  o.put('custom',out);
  List<Object> std=new List<Object>();
  Contact c=new Contact(FirstName='Zq',LastName='Xyzzy');
  std.add(entOf(Datacloud.FindDuplicates.findDuplicates(new List<SObject>{c}),'Contact'));
  Account a=new Account(Name='Zqxyzzy Holdings');
  std.add(entOf(Datacloud.FindDuplicates.findDuplicates(new List<SObject>{a}),'Account'));
  Lead l=new Lead(FirstName='Zq',LastName='Xyzzy',Company='Zqxyzzy Co');
  std.add(entOf(Datacloud.FindDuplicates.findDuplicates(new List<SObject>{l}),'Lead'));
  o.put('std',std);
  List<Object> msgs=new List<Object>();
  for(String k:new List<String>{'DupNone__c','DupNone2__c'}) {
   SObject s=mk(k,'q','k1',null,'off');insert s;
   Map<String,Object> r=new Map<String,Object>{'obj'=>k};
   try {Datacloud.FindDuplicatesByIds.findDuplicatesByIds(new List<Id>{s.Id});} catch(Exception e) {
    Matcher m=Pattern.compile('for the (\\S+) object').matcher(e.getMessage());
    r.put('msg',e.getMessage().replaceFirst('for the \\S+ object','for the TOKEN object'));
    if(m.find()) r.put('tok',tk(m.group(1),k));
   }
   msgs.add(r);
  }
  o.put('noreg',msgs);
 }
 public static void rptPhase(String key,Map<String,Object> o) {
  String t='DupGate__c';
  if(key=='RPTM_1') {o.put('insA',ins1(t,'MA','mk','na',false));o.put('insC',ins1(t,'MC','mk','na',false));}
  else if(key=='RPTM_2'||key=='RPTM_4') o.put('toK5',up1(t,'MC','Key__c','mk5',false));
  else if(key=='RPTM_3'||key=='RPTM_5') o.put('back',up1(t,'MC','Key__c','mk',false));
  else if(key=='RPTN_1') {ins1(t,'NA','nk','na',false);ins1(t,'NB','nk7','na',false);o.put('insC',ins1(t,'NC','nk','na',false));o.put('insD',ins1(t,'ND','nk','na',false));}
  else if(key=='RPTN_2') {o.put('other',up1(t,'NC','Other__c','o2',false));o.put('dAway',up1(t,'ND','Key__c','nk5',false));}
  else if(key=='RPTN_3') {o.put('to7',up1(t,'NC','Key__c','nk7',false));o.put('dTo7',up1(t,'ND','Key__c','nk7',false));}
  else if(key=='RPTN_4') o.put('back',up1(t,'NC','Key__c','nk',false));
  else if(key=='RPTO_1') {ins1(t,'OA','ok','na',false);o.put('insE',ins1(t,'OE','ok2','na',false));}
  else if(key=='RPTO_2'||key=='RPTO_3') o.put('into',up1(t,'OE','Key__c','ok',false));
  else if(key=='RPTO_4') o.put('away',up1(t,'OE','Key__c','ok5',false));
  else if(key=='RPTO_5') o.put('backInto',up1(t,'OE','Key__c','ok',false));
 }
 public static void phase(String key) {
  Map<String,Object> o=new Map<String,Object>();
  try {rptPhase(key,o);} catch(Exception e) {o.put('x',e.getTypeName());o.put('xm',e.getMessage());}
  System.debug('RPTOBS|'+key+'|'+JSON.serialize(o));
 }
 public static void run(String key,Map<String,Object> o) {
  List<String> p=key.split('_');
  if(p[0]=='ORD') ord('DupOrd__c',p[1],p[2],o);
  else if(p[0]=='ORD2') ord('DupOrd2__c',p[1],p[2],o);
  else if(p[0]=='GATE') gate(p[1],o);
  else if(key=='X99') x99(o);
  else if(key=='CAPQ') cap(o);
  else if(key=='XMAP') mapc(o);
  else if(key=='NOREG') none(o);
  else if(key=='MIXT') mix(o);
  else if(key=='FDIFF') diff(o);
  else if(key=='SGATE2') sgate2(o);
  else if(key=='FCASE') fcase(o);
  else if(key=='FDIFF2') fdiff2(o);
  else if(p[0]=='SGATE') sgate(p[1],o);
  else if(key=='OPT_blk') optBlk(o);
  else if(key=='OPT_alrt') optAlrt(o);
  else if(key=='RPT_same') rptSame(o);
  else if(key=='RPT_other') rptOther(o);
  else if(key=='RPT_ctrl') rptCtrl(o);
  else if(key=='RPT_alrt') rptAlrt(o);
  else if(key=='RPT_allow') rptAllow(o);
  else if(key=='ENT') ent(o);
 }
 public static void emit(String key) {
  Map<String,Object> o=new Map<String,Object>();
  try {run(key,o);} catch(Exception e) {o.put('x',e.getTypeName());o.put('xm',e.getMessage());}
  System.assert(false,'DUPOBS|'+key+'|'+JSON.serialize(o));
 }
}
