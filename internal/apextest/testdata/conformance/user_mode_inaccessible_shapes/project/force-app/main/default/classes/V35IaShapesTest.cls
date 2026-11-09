@IsTest(SeeAllData=false)
private without sharing class V35IaShapesTest {
 @IsTest static void baseline(){System.assertEquals(2,1+1,'BASELINE|arithmetic');}
 private static Map<String,Object> obs(Exception e){
  Map<String,Object> x=new Map<String,Object>{'t'=>e.getTypeName(),'m'=>e.getMessage()};
  if(e instanceof QueryException){
   Map<String,Set<String>> m=((QueryException)e).getInaccessibleFields();
   if(m==null){x.put('map','null');}else{
    List<String> ks=new List<String>(m.keySet());ks.sort();List<Object> out=new List<Object>();
    for(String k:ks){List<String> fs=new List<String>(m.get(k));fs.sort();out.add(new Map<String,Object>{'o'=>k,'f'=>fs});}
    x.put('map',out);}
  }
  return x;
 }
 private static void emit(String id,Map<String,Object> o){System.assert(false,'CAPTURE|'+id+'|'+JSON.serialize(o));}
 private static User reader(){List<User> us=[SELECT Id FROM User WHERE LastName='V35IaReader' WITH SYSTEM_MODE];System.assertEquals(1,us.size(),'fixture unique reader');return us[0];}
 private static Id ia(){return [SELECT Id FROM V35Ia__c WHERE Name='ia1' WITH SYSTEM_MODE].Id;}
 private static Id acct(){return [SELECT Id FROM Account WHERE Name='v35ia child' WITH SYSTEM_MODE].Id;}
 private static Id opp(){return [SELECT Id FROM Opportunity WHERE Name='v35ia opp' WITH SYSTEM_MODE].Id;}
 private static Id task(){return [SELECT Id FROM Task WHERE Subject='v35ia task' WITH SYSTEM_MODE].Id;}
 @TestSetup static void setup(){
  System.runAs(new User(Id=UserInfo.getUserId())){
   Profile pr=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' WITH SYSTEM_MODE];
   User u=new User(Alias='v35ia',LastName='V35IaReader',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='GMT',ProfileId=pr.Id,Username='v35iareader.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid');insert as system u;
   PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='V35IaReader' WITH SYSTEM_MODE];insert as system new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);
  }
  V35IaP__c p=new V35IaP__c(Name='p1',POk__c='a',PD1__c='b',PD2__c='c');insert as system p;
  V35IaX__c x=new V35IaX__c(Name='x1',XOk__c='a',XD__c='b');insert as system x;
  V35Ia__c r=new V35Ia__c(Name='ia1',Ok1__c='a',Ok2__c='b',Ok3__c='c',D1__c='d1',D2__c='d2',D3__c='d3',D4__c='d4',D5__c='d5',OkNum__c=1,DNum__c=2,OkDate__c=Date.today(),DDate__c=Date.today(),Parent__c=p.Id,DParent__c=p.Id,XRef__c=x.Id);insert as system r;
  insert as system new V35IaC__c(Name='c1',Ia__c=r.Id,COk__c='a',CD1__c='b',CD2__c='c');
  Account top=new Account(Name='v35ia top',AnnualRevenue=10);insert as system top;
  Account mid=new Account(Name='v35ia mid',AnnualRevenue=10,ParentId=top.Id);insert as system mid;
  Account a=new Account(Name='v35ia child',AccountNumber='n',AnnualRevenue=10,Description='d',Phone='1',ParentId=mid.Id);insert as system a;
  Opportunity op=new Opportunity(Name='v35ia opp',StageName='Prospecting',CloseDate=Date.today(),Amount=5,NextStep='n',LeadSource='Web',Description='d',AccountId=a.Id);insert as system op;
  insert as system new Task(Subject='v35ia task',Description='d',WhatId=a.Id,OwnerId=reader().Id);
 }
 @IsTest static void f5_mixed(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c, D2__c, D3__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c, D2__c, D3__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f5_mixed',o);
 }
 @IsTest static void f5_alldenied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c, D2__c, D3__c, D4__c, D5__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c, D2__c, D3__c, D4__c, D5__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f5_alldenied',o);
 }
 @IsTest static void f6_types(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c, DNum__c, DDate__c, Ok2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c, DNum__c, DDate__c, Ok2__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f6_types',o);
 }
 @IsTest static void f5_eq(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c, D2__c, D3__c FROM V35Ia__c WHERE Id = :rid WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c, D2__c, D3__c FROM V35Ia__c WHERE Id = :rid', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f5_eq',o);
 }
 @IsTest static void f5_orderdenied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c, Ok2__c, Ok3__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY D2__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c, Ok2__c, Ok3__c FROM V35Ia__c WHERE Id IN :ids ORDER BY D2__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f5_orderdenied',o);
 }
 @IsTest static void f4_orderdesc(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY Ok1__c DESC]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids ORDER BY Ok1__c DESC', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f4_orderdesc',o);
 }
 @IsTest static void f4_order2(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY Ok1__c, Ok2__c DESC]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids ORDER BY Ok1__c, Ok2__c DESC', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f4_order2',o);
 }
 @IsTest static void f3_orderselected(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c FROM V35Ia__c WHERE Id IN :ids ORDER BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_orderselected',o);
 }
 @IsTest static void f2_orderdenied_unselected(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE Id IN :ids ORDER BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f2_orderdenied_unselected',o);
 }
 @IsTest static void f3_limit(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE LIMIT 5]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids LIMIT 5', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_limit',o);
 }
 @IsTest static void f3_limit_offset(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE LIMIT 5 OFFSET 0]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids LIMIT 5 OFFSET 0', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_limit_offset',o);
 }
 @IsTest static void f2_where_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE D1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE D1__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f2_where_denied',o);
 }
 @IsTest static void f3_where_ok_denied_and(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c FROM V35Ia__c WHERE Ok1__c = 'x' AND D2__c = 'y' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c FROM V35Ia__c WHERE Ok1__c = \'x\' AND D2__c = \'y\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_where_ok_denied_and',o);
 }
 @IsTest static void f3_where_or(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, D1__c FROM V35Ia__c WHERE Id = :rid OR D2__c = 'y' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, D1__c FROM V35Ia__c WHERE Id = :rid OR D2__c = \'y\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_where_or',o);
 }
 @IsTest static void f3_where_nonid(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Ok1__c = 'a' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Ok1__c = \'a\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_where_nonid',o);
 }
 @IsTest static void f3_nowhere(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c FROM V35Ia__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c FROM V35Ia__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_nowhere',o);
 }
 @IsTest static void f1_nonid_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c FROM V35Ia__c WHERE Ok1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c FROM V35Ia__c WHERE Ok1__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f1_nonid_where',o);
 }
 @IsTest static void f4_eq_3denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c, D3__c FROM V35Ia__c WHERE Id = :rid WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c, D3__c FROM V35Ia__c WHERE Id = :rid', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f4_eq_3denied',o);
 }
 @IsTest static void f2_where_self(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c FROM V35Ia__c WHERE D1__c != null WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c FROM V35Ia__c WHERE D1__c != null', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f2_where_self',o);
 }
 @IsTest static void f2_where_inlist(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE D1__c IN ('a', 'b') WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE D1__c IN (\'a\', \'b\')', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f2_where_inlist',o);
 }
 @IsTest static void f3_where_number(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c, DNum__c FROM V35Ia__c WHERE DNum__c > 5 WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c, DNum__c FROM V35Ia__c WHERE DNum__c > 5', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_where_number',o);
 }
 @IsTest static void f3_case(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT id, d1__c, D2__C FROM v35ia__c WHERE id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT id, d1__c, D2__C FROM v35ia__c WHERE id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_case',o);
 }
 @IsTest static void f3_format(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, FORMAT(DNum__c) f, D1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, FORMAT(DNum__c) f, D1__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('f3_format',o);
 }
 @IsTest static void rel_parent_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Parent__r.POk__c, Parent__r.PD1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Parent__r.POk__c, Parent__r.PD1__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_parent_denied',o);
 }
 @IsTest static void rel_parent_own_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, Parent__r.PD1__c, Parent__r.PD2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, Parent__r.PD1__c, Parent__r.PD2__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_parent_own_denied',o);
 }
 @IsTest static void rel_parent_ok(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Parent__r.POk__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Parent__r.POk__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_parent_ok',o);
 }
 @IsTest static void rel_deniedlookup(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, DParent__c, DParent__r.POk__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, DParent__c, DParent__r.POk__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_deniedlookup',o);
 }
 @IsTest static void rel_deniedlookup_name(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, DParent__r.Name FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, DParent__r.Name FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_deniedlookup_name',o);
 }
 @IsTest static void rel_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE Parent__r.PD1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE Parent__r.PD1__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_where',o);
 }
 @IsTest static void rel_order(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY Parent__r.PD1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE Id IN :ids ORDER BY Parent__r.PD1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_order',o);
 }
 @IsTest static void rel_object_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, XRef__r.Name, XRef__r.XD__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, XRef__r.Name, XRef__r.XD__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_object_denied',o);
 }
 @IsTest static void rel_object_denied_ok(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, XRef__r.XOk__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, XRef__r.XOk__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_object_denied_ok',o);
 }
 @IsTest static void rel_semijoin(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D2__c FROM V35Ia__c WHERE Parent__c IN (SELECT Id FROM V35IaP__c WHERE PD1__c = 'x') WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D2__c FROM V35Ia__c WHERE Parent__c IN (SELECT Id FROM V35IaP__c WHERE PD1__c = \'x\')', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('rel_semijoin',o);
 }
 @IsTest static void sub_child(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT Id, CD1__c FROM Kids__r) FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT Id, CD1__c FROM Kids__r) FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('sub_child',o);
 }
 @IsTest static void sub_child_both(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, (SELECT COk__c, CD1__c, CD2__c FROM Kids__r) FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, (SELECT COk__c, CD1__c, CD2__c FROM Kids__r) FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('sub_child_both',o);
 }
 @IsTest static void sub_child_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT Id FROM Kids__r WHERE CD1__c = 'x') FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT Id FROM Kids__r WHERE CD1__c = \'x\') FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('sub_child_where',o);
 }
 @IsTest static void agg_count_field(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT(D1__c) c FROM V35Ia__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT(D1__c) c FROM V35Ia__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('agg_count_field',o);
 }
 @IsTest static void agg_max(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT MAX(DNum__c) m FROM V35Ia__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT MAX(DNum__c) m FROM V35Ia__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('agg_max',o);
 }
 @IsTest static void agg_group(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c, COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c, COUNT(Id) c FROM V35Ia__c GROUP BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('agg_group',o);
 }
 @IsTest static void agg_group2(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c, D2__c, COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY D1__c, D2__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c, D2__c, COUNT(Id) c FROM V35Ia__c GROUP BY D1__c, D2__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('agg_group2',o);
 }
 @IsTest static void agg_group_ok_sum_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, SUM(DNum__c) s FROM V35Ia__c WITH USER_MODE GROUP BY Ok1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, SUM(DNum__c) s FROM V35Ia__c GROUP BY Ok1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('agg_group_ok_sum_denied',o);
 }
 @IsTest static void agg_having(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY Ok1__c HAVING SUM(DNum__c) > 1]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c GROUP BY Ok1__c HAVING SUM(DNum__c) > 1', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('agg_having',o);
 }
 @IsTest static void count_where_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ Integer n = [SELECT COUNT() FROM V35Ia__c WHERE D1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>n}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ Integer n = Database.countQueryWithBinds('SELECT COUNT() FROM V35Ia__c WHERE D1__c = \'x\'', new Map<String,Object>{'rid'=>rid,'ids'=>ids,'aid'=>aid,'aids'=>aids,'oid'=>oid,'oids'=>oids,'tid'=>tid}, AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>n}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('count_where_denied',o);
 }
 @IsTest static void count_ok(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ Integer n = [SELECT COUNT() FROM V35Ia__c WHERE Ok1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>n}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ Integer n = Database.countQueryWithBinds('SELECT COUNT() FROM V35Ia__c WHERE Ok1__c = \'x\'', new Map<String,Object>{'rid'=>rid,'ids'=>ids,'aid'=>aid,'aids'=>aids,'oid'=>oid,'oids'=>oids,'tid'=>tid}, AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>n}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('count_ok',o);
 }
 @IsTest static void lock_for_update(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE FOR UPDATE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids FOR UPDATE', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('lock_for_update',o);
 }
 @IsTest static void allrows(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ALL ROWS]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c, D2__c FROM V35Ia__c WHERE Id IN :ids ALL ROWS', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('allrows',o);
 }
 @IsTest static void x_object_denied5(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Name, XOk__c, XD__c, CreatedDate FROM V35IaX__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Name, XOk__c, XD__c, CreatedDate FROM V35IaX__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('x_object_denied5',o);
 }
 @IsTest static void x_object_denied1(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35IaX__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35IaX__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('x_object_denied1',o);
 }
 @IsTest static void acct_f6(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Name, AccountNumber, AnnualRevenue, Description, Phone FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Name, AccountNumber, AnnualRevenue, Description, Phone FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_f6',o);
 }
 @IsTest static void acct_f4(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, AccountNumber, AnnualRevenue, Phone FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, AccountNumber, AnnualRevenue, Phone FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_f4',o);
 }
 @IsTest static void acct_rel(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Parent.AnnualRevenue, Parent.Description FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Parent.AnnualRevenue, Parent.Description FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_rel',o);
 }
 @IsTest static void acct_rel2(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Parent.Parent.AnnualRevenue FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Parent.Parent.AnnualRevenue FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_rel2',o);
 }
 @IsTest static void acct_rel_name(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Name, Parent.Name, Parent.Phone FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Name, Parent.Name, Parent.Phone FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_rel_name',o);
 }
 @IsTest static void acct_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, AccountNumber FROM Account WHERE AnnualRevenue > 5 WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, AccountNumber FROM Account WHERE AnnualRevenue > 5', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_where',o);
 }
 @IsTest static void acct_ok(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, AccountNumber, Industry FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, AccountNumber, Industry FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('acct_ok',o);
 }
 @IsTest static void opp_f7(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Name, StageName, CloseDate, Amount, NextStep, LeadSource FROM Opportunity WHERE Id IN :oids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Name, StageName, CloseDate, Amount, NextStep, LeadSource FROM Opportunity WHERE Id IN :oids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('opp_f7',o);
 }
 @IsTest static void opp_rel(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Name, Amount, Account.AnnualRevenue, Account.Name FROM Opportunity WHERE Id IN :oids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Name, Amount, Account.AnnualRevenue, Account.Name FROM Opportunity WHERE Id IN :oids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('opp_rel',o);
 }
 @IsTest static void opp_rel_deep(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Account.Parent.AnnualRevenue FROM Opportunity WHERE Id IN :oids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Account.Parent.AnnualRevenue FROM Opportunity WHERE Id IN :oids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('opp_rel_deep',o);
 }
 @IsTest static void opp_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, StageName FROM Opportunity WHERE Amount > 5 WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, StageName FROM Opportunity WHERE Amount > 5', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('opp_where',o);
 }
 @IsTest static void opp_f4(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, StageName, Amount, Description FROM Opportunity WHERE Id IN :oids WITH USER_MODE ORDER BY StageName]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, StageName, Amount, Description FROM Opportunity WHERE Id IN :oids ORDER BY StageName', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('opp_f4',o);
 }
 @IsTest static void typeof_when(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, TYPEOF What WHEN Account THEN Id, AnnualRevenue, Description ELSE Id END FROM Task WHERE Id = :tid WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, TYPEOF What WHEN Account THEN Id, AnnualRevenue, Description ELSE Id END FROM Task WHERE Id = :tid', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('typeof_when',o);
 }
 @IsTest static void typeof_when_ok(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, TYPEOF What WHEN Account THEN Id, AccountNumber ELSE Id END FROM Task WHERE Id = :tid WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, TYPEOF What WHEN Account THEN Id, AccountNumber ELSE Id END FROM Task WHERE Id = :tid', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('typeof_when_ok',o);
 }
 @IsTest static void task_f4(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Subject, Description, ActivityDate FROM Task WHERE Id = :tid WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Subject, Description, ActivityDate FROM Task WHERE Id = :tid', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('task_f4',o);
 }
 @IsTest static void m_where_then_order(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE D2__c = 'x' WITH USER_MODE ORDER BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE D2__c = \'x\' ORDER BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_where_then_order',o);
 }
 @IsTest static void m_order_select(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D3__c, D1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY D2__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D3__c, D1__c FROM V35Ia__c WHERE Id IN :ids ORDER BY D2__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_order_select',o);
 }
 @IsTest static void m_rel_before_own(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Parent__r.PD1__c, D1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Parent__r.PD1__c, D1__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_rel_before_own',o);
 }
 @IsTest static void m_sub_before_own(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT Id, CD1__c FROM Kids__r), D1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT Id, CD1__c FROM Kids__r), D1__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_sub_before_own',o);
 }
 @IsTest static void m_rel_order_select(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY Parent__r.PD1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c FROM V35Ia__c WHERE Id IN :ids ORDER BY Parent__r.PD1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_rel_order_select',o);
 }
 @IsTest static void m_rel_where_own_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Ok1__c FROM V35Ia__c WHERE Parent__r.PD1__c = 'x' AND D1__c = 'y' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Ok1__c FROM V35Ia__c WHERE Parent__r.PD1__c = \'x\' AND D1__c = \'y\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_rel_where_own_where',o);
 }
 @IsTest static void m_dup_where_select(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, D1__c FROM V35Ia__c WHERE D1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, D1__c FROM V35Ia__c WHERE D1__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_dup_where_select',o);
 }
 @IsTest static void m_x_name_only(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Name FROM V35IaX__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Name FROM V35IaX__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_x_name_only',o);
 }
 @IsTest static void m_x_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35IaX__c WHERE XD__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35IaX__c WHERE XD__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_x_where',o);
 }
 @IsTest static void m_x_order(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, XOk__c FROM V35IaX__c WITH USER_MODE ORDER BY XD__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, XOk__c FROM V35IaX__c ORDER BY XD__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_x_order',o);
 }
 @IsTest static void m_rel_x_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35Ia__c WHERE XRef__r.XD__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35Ia__c WHERE XRef__r.XD__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_rel_x_where',o);
 }
 @IsTest static void m_denied_lookup_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35Ia__c WHERE DParent__r.POk__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35Ia__c WHERE DParent__r.POk__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_denied_lookup_where',o);
 }
 @IsTest static void m_denied_lookup_parent_denied(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, DParent__r.PD1__c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, DParent__r.PD1__c FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_denied_lookup_parent_denied',o);
 }
 @IsTest static void m_agg_count_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT(Id) c FROM V35Ia__c WHERE D1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT(Id) c FROM V35Ia__c WHERE D1__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_agg_count_where',o);
 }
 @IsTest static void m_sub_child_order(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT Id FROM Kids__r ORDER BY CD1__c) FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT Id FROM Kids__r ORDER BY CD1__c) FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_sub_child_order',o);
 }
 @IsTest static void m_sub_child_parent_field(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT Id, Ia__r.D1__c FROM Kids__r) FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT Id, Ia__r.D1__c FROM Kids__r) FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_sub_child_parent_field',o);
 }
 @IsTest static void m_antijoin(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35Ia__c WHERE Id NOT IN (SELECT Ia__c FROM V35IaC__c WHERE CD1__c = 'x') WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35Ia__c WHERE Id NOT IN (SELECT Ia__c FROM V35IaC__c WHERE CD1__c = \'x\')', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_antijoin',o);
 }
 @IsTest static void m_owner_rel(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, Owner.Email FROM Account WHERE Id IN :aids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, Owner.Email FROM Account WHERE Id IN :aids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_owner_rel',o);
 }
 @IsTest static void m_createdby_rel(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, CreatedBy.Email FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, CreatedBy.Email FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_createdby_rel',o);
 }
 @IsTest static void m_case_rel(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, parent__r.pd1__c FROM v35ia__c WHERE id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, parent__r.pd1__c FROM v35ia__c WHERE id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_case_rel',o);
 }
 @IsTest static void m_child_to_parent_own(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, CD1__c, Ia__r.D1__c, Ia__r.Ok1__c FROM V35IaC__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, CD1__c, Ia__r.D1__c, Ia__r.Ok1__c FROM V35IaC__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('m_child_to_parent_own',o);
 }
 @IsTest static void g_group_in(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE GROUP BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids GROUP BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_in',o);
 }
 @IsTest static void g_group_in_having(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE GROUP BY Ok1__c HAVING SUM(DNum__c) > 1]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids GROUP BY Ok1__c HAVING SUM(DNum__c) > 1', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_in_having',o);
 }
 @IsTest static void g_having_count(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY Ok1__c HAVING COUNT(D1__c) > 1]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c GROUP BY Ok1__c HAVING COUNT(D1__c) > 1', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_having_count',o);
 }
 @IsTest static void g_where_group(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WHERE D1__c = 'x' WITH USER_MODE GROUP BY Ok1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WHERE D1__c = \'x\' GROUP BY Ok1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_where_group',o);
 }
 @IsTest static void g_group_only(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT(Id) c FROM V35Ia__c GROUP BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_only',o);
 }
 @IsTest static void g_group_only_in(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE GROUP BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids GROUP BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_only_in',o);
 }
 @IsTest static void g_sum_in(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT SUM(DNum__c) s FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT SUM(DNum__c) s FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_sum_in',o);
 }
 @IsTest static void g_group_in_having_both(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE GROUP BY D1__c HAVING SUM(DNum__c) > 1]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids GROUP BY D1__c HAVING SUM(DNum__c) > 1', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_in_having_both',o);
 }
 @IsTest static void g_min_two(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT MIN(DNum__c) a, MAX(DDate__c) b FROM V35Ia__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT MIN(DNum__c) a, MAX(DDate__c) b FROM V35Ia__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_min_two',o);
 }
 @IsTest static void g_group_two_in(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT D1__c, D2__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE GROUP BY D1__c, D2__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT D1__c, D2__c, COUNT(Id) c FROM V35Ia__c WHERE Id IN :ids GROUP BY D1__c, D2__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_two_in',o);
 }
 @IsTest static void g_group_order(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY Ok1__c ORDER BY COUNT(D1__c)]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c GROUP BY Ok1__c ORDER BY COUNT(D1__c)', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('g_group_order',o);
 }
 @IsTest static void o_where_group(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT(Id) c FROM V35Ia__c WHERE D2__c = 'x' WITH USER_MODE GROUP BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT(Id) c FROM V35Ia__c WHERE D2__c = \'x\' GROUP BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_where_group',o);
 }
 @IsTest static void o_where_having(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WHERE D2__c = 'x' WITH USER_MODE GROUP BY Ok1__c HAVING SUM(DNum__c) > 1]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Ok1__c, COUNT(Id) c FROM V35Ia__c WHERE D2__c = \'x\' GROUP BY Ok1__c HAVING SUM(DNum__c) > 1', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_where_having',o);
 }
 @IsTest static void o_group_having(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY D1__c HAVING COUNT(D2__c) > 1]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT(Id) c FROM V35Ia__c GROUP BY D1__c HAVING COUNT(D2__c) > 1', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_group_having',o);
 }
 @IsTest static void o_where_semi_then_own(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35Ia__c WHERE D1__c = 'x' AND Id IN (SELECT Ia__c FROM V35IaC__c WHERE CD1__c = 'y') WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35Ia__c WHERE D1__c = \'x\' AND Id IN (SELECT Ia__c FROM V35IaC__c WHERE CD1__c = \'y\')', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_where_semi_then_own',o);
 }
 @IsTest static void o_semi_then_where_own(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id FROM V35Ia__c WHERE Id IN (SELECT Ia__c FROM V35IaC__c WHERE CD1__c = 'y') AND D1__c = 'x' WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id FROM V35Ia__c WHERE Id IN (SELECT Ia__c FROM V35IaC__c WHERE CD1__c = \'y\') AND D1__c = \'x\'', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_semi_then_where_own',o);
 }
 @IsTest static void o_child_select_where(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT CD1__c FROM Kids__r WHERE CD2__c = 'x') FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT CD1__c FROM Kids__r WHERE CD2__c = \'x\') FROM V35Ia__c WHERE Id IN :ids', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_child_select_where',o);
 }
 @IsTest static void o_child_where_then_order(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT Id, (SELECT Id FROM Kids__r WHERE CD1__c = 'x') FROM V35Ia__c WHERE Id IN :ids WITH USER_MODE ORDER BY D1__c]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT Id, (SELECT Id FROM Kids__r WHERE CD1__c = \'x\') FROM V35Ia__c WHERE Id IN :ids ORDER BY D1__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_child_where_then_order',o);
 }
 @IsTest static void o_datefn(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT CALENDAR_YEAR(DDate__c) y, COUNT(Id) c FROM V35Ia__c WITH USER_MODE GROUP BY CALENDAR_YEAR(DDate__c)]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT CALENDAR_YEAR(DDate__c) y, COUNT(Id) c FROM V35Ia__c GROUP BY CALENDAR_YEAR(DDate__c)', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_datefn',o);
 }
 @IsTest static void o_count_distinct(){
  Map<String,Object> o=new Map<String,Object>();
  Id rid=ia();Set<Id> ids=new Set<Id>{rid};Id aid=acct();Set<Id> aids=new Set<Id>{aid};Id oid=opp();Set<Id> oids=new Set<Id>{oid};Id tid=task();
  System.runAs(reader()){
   try{ List<SObject> r = [SELECT COUNT_DISTINCT(D1__c) c FROM V35Ia__c WITH USER_MODE]; o.put('s', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('s', obs(e)); }
   try{ List<SObject> r = Database.query('SELECT COUNT_DISTINCT(D1__c) c FROM V35Ia__c', AccessLevel.USER_MODE); o.put('d', new Map<String,Object>{'ok'=>r.size()}); }catch(Exception e){ o.put('d', obs(e)); }
  }
  emit('o_count_distinct',o);
 }
}
