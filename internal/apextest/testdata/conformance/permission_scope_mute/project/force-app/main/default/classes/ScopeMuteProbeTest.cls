@IsTest private class ScopeMuteProbeTest {
 static Id seedId;
 static String t(Exception e) { return e.getTypeName(); }
 static User mk(String alias) {
  User u=new User(Alias=alias,Email=alias+'@example.invalid',EmailEncodingKey='UTF-8',LastName=alias,LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username=alias+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce'].Id); insert u; return u;
 }
 static Id ps(String n) { return [SELECT Id FROM PermissionSet WHERE Name=:n].Id; }
 static void give(User u,String n) { insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps(n)); }
 static void giveGroup(User u) { insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetGroupId=[SELECT Id FROM PermissionSetGroup WHERE DeveloperName='GrpMuted'].Id); }
 static void seed() { give(new User(Id=UserInfo.getUserId()),'PsBase'); }
 static void seedRecord() { Cap__c s=new Cap__c(Name='seed',F__c='v'); insert s; seedId=s.Id; Test.setFixedSearchResults(new List<Id>{s.Id}); }
 static Map<String,Object> probe(AccessLevel lvl,Boolean nameOnly) {
  Map<String,Object> o=new Map<String,Object>();
  try { Cap__c c=new Cap__c(Name='n'); if(!nameOnly){c.F__c='v';} Database.insert(c,lvl); o.put('insert','ok'); } catch(Exception e) {o.put('insert',t(e));}
  try { Database.query('SELECT Id, F__c FROM Cap__c',lvl); o.put('queryF','ok'); } catch(Exception e) {o.put('queryF',t(e));}
  try { Database.query('SELECT Id, Name FROM Cap__c',lvl); o.put('queryName','ok'); } catch(Exception e) {o.put('queryName',t(e));}
  try { Cap__c s=new Cap__c(Id=seedId,F__c='w'); Database.update(s,lvl); o.put('updateF','ok'); } catch(Exception e) {o.put('updateF',t(e));}
  try { List<List<SObject>> r=Search.query('FIND \'seed\' RETURNING Cap__c(Id, F__c)',lvl); o.put('sosl',r[0].size()); } catch(Exception e) {o.put('sosl',t(e));}
  return o;
 }
 static List<Boolean> describe() {
  Schema.DescribeSObjectResult d=Cap__c.SObjectType.getDescribe(); Schema.DescribeFieldResult f=Cap__c.F__c.getDescribe();
  return new List<Boolean>{d.isAccessible(),d.isCreateable(),f.isAccessible(),f.isUpdateable(),f.isCreateable()};
 }
 static Boolean stripKeepsF() {
  SObjectAccessDecision d=Security.stripInaccessible(AccessType.READABLE,new List<Cap__c>{new Cap__c(Id=seedId,Name='seed',F__c='v')});
  return ((Cap__c)d.getRecords()[0]).getPopulatedFieldsAsMap().containsKey('F__c');
 }
 static void emit(String m,Map<String,Object> o) { System.assert(false,'OBSERVED|'+m+'|'+JSON.serialize(o)); }
 static void asUser(String m,String alias,String[] sets,Boolean viaGroup,String scope,Boolean nameOnly,Boolean extra) {
  seed(); User u=mk(alias); for(String s:sets){give(u,s);} if(viaGroup){giveGroup(u);} System.runAs(new User(Id=UserInfo.getUserId())){ seedRecord(); }
  AccessLevel lvl=scope==null?AccessLevel.USER_MODE:AccessLevel.USER_MODE.withPermissionSetId(ps(scope));
  Map<String,Object> o; System.runAs(u){ o=probe(lvl,nameOnly); if(extra){o.put('describe',describe());o.put('strip',stripKeepsF());} } emit(m,o);
 }
 @IsTest static void u1Plain(){ asUser('u1Plain','pa',new String[]{'PsBase'},false,null,false,false); }
 @IsTest static void u1ScopedOther(){ asUser('u1ScopedOther','pb',new String[]{'PsBase'},false,'PsOther',false,false); }
 @IsTest static void u1ScopedOtherNameOnly(){ asUser('u1ScopedOtherNameOnly','pc',new String[]{'PsBase'},false,'PsOther',true,false); }
 @IsTest static void u1ScopedBase(){ asUser('u1ScopedBase','pd',new String[]{'PsBase'},false,'PsBase',false,false); }
 @IsTest static void u1ScopedFalse(){ asUser('u1ScopedFalse','pe',new String[]{'PsBase'},false,'PsFalse',false,false); }
 @IsTest static void u2ScopedBase(){ asUser('u2ScopedBase','pf',new String[]{},false,'PsBase',false,false); }
 @IsTest static void u2Plain(){ asUser('u2Plain','pg',new String[]{},false,null,false,false); }
 @IsTest static void u3ScopedOther(){ asUser('u3ScopedOther','ph',new String[]{},false,'PsOther',false,false); }
 @IsTest static void u4Plain(){ asUser('u4Plain','pi',new String[]{'PsObjOnly'},false,null,false,false); }
 @IsTest static void u4ScopedBase(){ asUser('u4ScopedBase','pj',new String[]{'PsObjOnly'},false,'PsBase',false,false); }
 @IsTest static void u4ScopedOther(){ asUser('u4ScopedOther','pk',new String[]{'PsObjOnly'},false,'PsOther',false,false); }
 @IsTest static void u4ScopedOtherNameOnly(){ asUser('u4ScopedOtherNameOnly','pl',new String[]{'PsObjOnly'},false,'PsOther',true,false); }
 @IsTest static void groupMuted(){ asUser('groupMuted','pm',new String[]{},true,null,false,true); }
 @IsTest static void groupMutedNameOnly(){ asUser('groupMutedNameOnly','pn',new String[]{},true,null,true,false); }
 @IsTest static void groupMutedPlusDirect(){ asUser('groupMutedPlusDirect','po',new String[]{'PsGrant'},true,null,false,true); }
 static Map<String,Object> adminOps(AccessLevel lvl,Id scope) {
  Map<String,Object> o=new Map<String,Object>();
  try { Database.query('SELECT Id, Name FROM Account',lvl); o.put('qAccount','ok'); } catch(Exception e) {o.put('qAccount',t(e));}
  try { Database.query('SELECT Id, LastName FROM Contact',lvl); o.put('qContact','ok'); } catch(Exception e) {o.put('qContact',t(e));}
  try { Database.insert(new Account(Name='a'),lvl); o.put('insAccount','ok'); } catch(Exception e) {o.put('insAccount',t(e));}
  try { Database.query('SELECT Id, UserRecordAccess.HasAllAccess FROM Account',lvl); o.put('qURA','ok'); } catch(Exception e) {o.put('qURA',t(e));}
  if (scope != null) { try { SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<Account>{new Account(Name='n',Phone='1')},false,scope); o.put('stripPhoneKept',((Account)d.getRecords()[0]).getPopulatedFieldsAsMap().containsKey('Phone')); } catch(Exception e) {o.put('strip',t(e));} }
  return o;
 }
 @IsTest static void adminPlain(){ emit('adminPlain',adminOps(AccessLevel.USER_MODE,null)); }
 @IsTest static void adminScopedOther(){ Id s=ps('PsOther'); emit('adminScopedOther',adminOps(AccessLevel.USER_MODE.withPermissionSetId(s),s)); }
 @IsTest static void adminScopedFalse(){ Id s=ps('PsFalse'); emit('adminScopedFalse',adminOps(AccessLevel.USER_MODE.withPermissionSetId(s),s)); }
}
