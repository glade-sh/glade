@IsTest(SeeAllData=false)
private without sharing class GapProbe62 {
 @TestSetup static void setupUsers(){
  List<User> users=new List<User>();
  users.add(new User(Alias='gpg',LastName='GapProbeGB',Email='capture@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_GB',TimeZoneSidKey='GMT',ProfileId=UserInfo.getProfileId(),Username='GapProbeGB.'+UserInfo.getOrganizationId()+'.'+System.now().getTime()+'@example.invalid'));
  insert as system users;
 }
 private static void emit(String id,Object o){ System.assert(false,'CAPTURE|'+id+'|'+JSON.serialize(o)); }
 private static String names(List<Contact> cs){ List<String> n=new List<String>(); for(Contact c:cs)n.add(c.LastName); return String.join(n,','); }
 @IsTest static void nullOrder(){
  Map<String,Object> o=new Map<String,Object>();
  try{
   Account a1=new Account(Name='GP a1'); insert as system a1; Account a2=new Account(Name='GP a2'); insert as system a2;
   Contact p1=new Contact(LastName='p1'); insert as system p1; Contact p2=new Contact(LastName='p2'); insert as system p2;
   List<Contact> seed=new List<Contact>{
    new Contact(LastName='c1',AccountId=a2.Id,ReportsToId=p2.Id,Birthdate=Date.newInstance(2021,1,1),Title='b'),
    new Contact(LastName='c2',AccountId=a1.Id,ReportsToId=p1.Id,Birthdate=Date.newInstance(2020,1,1),Title='a'),
    new Contact(LastName='c3')};
   insert as system seed;
   Set<Id> ids=new Map<Id,Contact>(seed).keySet();
   for(String f:new List<String>{'AccountId','ReportsToId','Birthdate','Title'}){
    for(String tail:new List<String>{'ASC','ASC NULLS FIRST','ASC NULLS LAST','DESC','DESC NULLS FIRST','DESC NULLS LAST'}){
     o.put(f+' '+tail, names(Database.query('SELECT Id,LastName FROM Contact WHERE Id IN :ids ORDER BY '+f+' '+tail+' , LastName')));
    }
   }
  }catch(Exception e){o.put('exception',e.getTypeName()+':'+e.getMessage());}
  emit('nullOrder',o);
 }
 @IsTest static void ties(){
  Map<String,Object> o=new Map<String,Object>();
  try{
   List<Account> seed=new List<Account>();
   for(String n:new List<String>{'b','B','a','A','SERVICE','Service','service'}) seed.add(new Account(Name='GPT '+n));
   for(Account a:seed) insert as system a;
   Set<Id> ids=new Map<Id,Account>(seed).keySet();
   List<String> r=new List<String>(); for(Account a:Database.query('SELECT Name FROM Account WHERE Id IN :ids ORDER BY Name')) r.add(a.Name);
   o.put('asc',String.join(r,'|'));
   r=new List<String>(); for(Account a:Database.query('SELECT Name FROM Account WHERE Id IN :ids ORDER BY Name DESC')) r.add(a.Name);
   o.put('desc',String.join(r,'|'));
   r=new List<String>(); for(Account a:Database.query('SELECT Name FROM Account WHERE Id IN :ids ORDER BY Name, Id DESC')) r.add(a.Name);
   o.put('ascIdDesc',String.join(r,'|'));
  }catch(Exception e){o.put('exception',e.getTypeName()+':'+e.getMessage());}
  emit('ties',o);
 }
 private static Map<String,Object> weeks(){
  Map<String,Object> o=new Map<String,Object>();
  Date today=Date.today(); o.put('today',today); o.put('locale',UserInfo.getLocale()); o.put('tz',UserInfo.getTimeZone().getID());
  o.put('dow', Math.mod(Date.newInstance(2023,1,1).daysBetween(today),7));
  List<Contact> seed=new List<Contact>();
  for(Integer i=-21;i<=21;i++) seed.add(new Contact(LastName='d'+(i+100),Birthdate=today.addDays(i)));
  insert as system seed;
  Set<Id> ids=new Map<Id,Contact>(seed).keySet();
  for(String lit:new List<String>{'THIS_WEEK','LAST_WEEK','NEXT_WEEK','LAST_N_WEEKS:1','NEXT_N_WEEKS:1','N_WEEKS_AGO:1','LAST_N_WEEKS:2','THIS_MONTH'}){
   List<Integer> offs=new List<Integer>();
   for(Contact c:Database.query('SELECT Birthdate FROM Contact WHERE Id IN :ids AND Birthdate='+lit+' ORDER BY Birthdate')) offs.add(today.daysBetween(c.Birthdate));
   o.put(lit, offs.isEmpty()?'none':(String.valueOf(offs[0])+'..'+String.valueOf(offs[offs.size()-1])+' n='+offs.size()));
  }
  return o;
 }
 @IsTest static void weeksGB(){
  Map<String,Object> o=new Map<String,Object>();
  try{ List<User> us=[SELECT Id FROM User WHERE LastName='GapProbeGB' WITH SYSTEM_MODE]; System.runAs(us[0]){ o=weeks(); } }catch(Exception e){o.put('exception',e.getTypeName()+':'+e.getMessage());}
  emit('weeksGB',o);
 }
 @IsTest static void weeksUS(){
  Map<String,Object> o=new Map<String,Object>();
  try{ o=weeks(); }catch(Exception e){o.put('exception',e.getTypeName()+':'+e.getMessage());}
  emit('weeksUS',o);
 }
}
