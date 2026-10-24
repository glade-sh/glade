public without sharing class RuntimeShapeProbe {
 public static Map<String,Object> observe(String shape) {
  try {
   if(shape=='asyncStamp') return asyncStamp();
   if(shape=='hierarchyDefault') return hierarchyDefault();
   if(shape=='profileAssignment') return profileAssignment();
   if(shape=='deniedRead') return deniedRead();
   if(shape=='entityJoined') return entityJoined();
   if(shape=='entityOverride') return entityOverride();
   if(shape=='entityOperations') return entityOperations();
   if(shape=='taskReference') return taskReference();
   if(shape=='userDelete') return userDelete();
   if(shape=='plainPrefix') return prefix(null,'SliceRecord__c');
   if(shape=='qualifiedPrefix') return prefix('demo','demo__SliceRecord__c');
   if(shape=='accessIdentity') return accessIdentity();
   if(shape=='profileCreatable') return profileCreatable();
   if(shape=='activitySuccess') return activity(false);
   if(shape=='activityFailure') return activity(true);
   if(shape=='listFlag') return listFlag();
   return new Map<String,Object>{'unknownShape'=>true};
  } catch(Exception issue) { return new Map<String,Object>{'errorType'=>issue.getTypeName()}; }
 }
 private static User actor() {
  User value=new User(Alias='slice',Email='slice@example.test',EmailEncodingKey='UTF-8',LastName='Slice',
   LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',
   ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id,
   Username='slice.'+String.valueOf(Crypto.getRandomLong())+'.'+UserInfo.getOrganizationId()+'@example.test');
  insert value;return value;
 }
 public class StampRoot implements Queueable {
  public void execute(QueueableContext context) {
   Account value=new Account(Name='slice-root',Description=String.valueOf(context.getJobId()));insert value;
   System.enqueueJob(new StampChild(value.Id));
  }
 }
 public class StampChild implements Queueable {
  private Id recordId;
  public StampChild(Id value) { recordId=value; }
  public void execute(QueueableContext context) { update new Account(Id=recordId,Name='slice-terminal'); }
 }
 private static Map<String,Object> asyncStamp() {
  if(!Test.isRunningTest()) return new Map<String,Object>{'boundary'=>'requiresTestDrain'};
  Test.startTest();Id rootId=System.enqueueJob(new StampRoot());String drainError;
  try { Test.stopTest(); } catch(Exception issue) { drainError=issue.getTypeName(); }
  List<Account> rows=[SELECT Name,Description FROM Account WHERE Name LIKE 'slice-%'];
  return new Map<String,Object>{'drainError'=>drainError,'count'=>rows.size(),'terminal'=>rows.isEmpty()?false:rows[0].Name=='slice-terminal','rootStamp'=>rows.isEmpty()?false:(Id)rows[0].Description==rootId};
 }
 private static Map<String,Object> hierarchyDefault() {
  insert new SlicePreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Disabled__c=true);
  if(!Test.isRunningTest()) return new Map<String,Object>{'disabled'=>SlicePreference__c.getInstance().Disabled__c,'boundary'=>'runAsOnlyInTests'};
  User reader=actor();Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader) {
   result.put('disabled',SlicePreference__c.getInstance().Disabled__c);Id orgId=UserInfo.getOrganizationId();
   List<SlicePreference__c> rows=[SELECT Disabled__c FROM SlicePreference__c WHERE SetupOwnerId=:orgId];result.put('orgRows',rows.size());
  }
  return result;
 }
 private static Map<String,Object> profileAssignment() {
  Id profileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id;
  List<PermissionSet> sets=[SELECT Id,IsOwnedByProfile FROM PermissionSet WHERE ProfileId=:profileId];User reader=actor();
  Integer assignments=[SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId=:reader.Id AND PermissionSet.IsOwnedByProfile=true];
  return new Map<String,Object>{'profileSets'=>sets.size(),'profileOwned'=>!sets.isEmpty() && sets[0].IsOwnedByProfile,'explicitAssignments'=>assignments};
 }
 private static Map<String,Object> deniedRead() {
  if(!Test.isRunningTest()) return new Map<String,Object>{'boundary'=>'runAsOnlyInTests'};
  User reader=actor();System.runAs(new User(Id=UserInfo.getUserId())) { insert new SliceRecord__c(Name='hidden'); }
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader) {
   result.put('accessible',SliceRecord__c.SObjectType.getDescribe().isAccessible());
   try { result.put('rows',Database.query('SELECT Id FROM SliceRecord__c WITH USER_MODE').size()); }
   catch(Exception issue) { result.put('errorType',issue.getTypeName()); }
  }
  return result;
 }
 private static Map<String,Object> entityJoined() {
  SliceRule__mdt row=[SELECT Group__r.Source__r.Entity__r.QualifiedApiName FROM SliceRule__mdt WHERE DeveloperName='First' LIMIT 1];
  return new Map<String,Object>{'objectName'=>row.Group__r.Source__r.Entity__r.QualifiedApiName};
 }
 private static Map<String,Object> entityOverride() {
  SliceSource__mdt source=(SliceSource__mdt)JSON.deserialize('{"Entity__c":"Account","Entity__r":{"attributes":{"type":"EntityDefinition"},"QualifiedApiName":"Account"}}',SliceSource__mdt.class);
  SliceGroup__mdt holder=(SliceGroup__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Source__r'=>source}),SliceGroup__mdt.class);
  SliceRule__mdt rule=(SliceRule__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Group__r'=>holder,'Operation__c'=>'BEFORE_INSERT'}),SliceRule__mdt.class);
  List<SliceRule__mdt> overrideRows=new List<SliceRule__mdt>{rule};Map<String,Integer> counts=new Map<String,Integer>();
  for(SliceRule__mdt value:overrideRows) counts.put(value.Group__r.Source__r.Entity__r.QualifiedApiName,1);
  return new Map<String,Object>{'accountRules'=>counts.get('Account')};
 }
 private static Map<String,Object> entityOperations() {
  List<SliceRule__mdt> rows=[SELECT Operation__c,Group__r.Source__r.Entity__r.QualifiedApiName FROM SliceRule__mdt];Map<String,Integer> counts=new Map<String,Integer>();
  for(SliceRule__mdt value:rows) { if(value.Group__r.Source__r.Entity__r.QualifiedApiName=='Account') counts.put(value.Operation__c,1); }
  return new Map<String,Object>{'beforeInsert'=>counts.get('BEFORE_INSERT'),'beforeUpdate'=>counts.get('BEFORE_UPDATE'),'absent'=>counts.get('AFTER_DELETE')};
 }
 private static Map<String,Object> taskReference() {
  Account parent=new Account(Name='reference');insert parent;Database.SaveResult good=Database.insert(new Task(Subject='valid',WhatId=parent.Id),false);
  Id missing=(Id)(Account.SObjectType.getDescribe().getKeyPrefix()+'000000000123');Database.SaveResult bad=Database.insert(new Task(Subject='missing',WhatId=missing),false);
  return new Map<String,Object>{'valid'=>good.isSuccess(),'missing'=>bad.isSuccess(),'missingStatus'=>bad.isSuccess()?null:String.valueOf(bad.getErrors()[0].getStatusCode())};
 }
 private static Map<String,Object> userDelete() {
  if(!Test.isRunningTest()) return new Map<String,Object>{'boundary'=>'runAsOnlyInTests'};
  User reader=actor();SliceRecord__c value=new SliceRecord__c(Name='protected');System.runAs(new User(Id=UserInfo.getUserId())) { insert value; }
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader) {
   try { Database.delete(new List<SObject>{value},true,AccessLevel.USER_MODE);result.put('deleted',true); }
   catch(Exception issue) { result.put('errorType',issue.getTypeName()); }
  }
  result.put('remaining',[SELECT COUNT() FROM SliceRecord__c WHERE Id=:value.Id]);return result;
 }
 private static Map<String,Object> prefix(String namespaceValue,String objectName) {
  EntityDefinition entity=(EntityDefinition)JSON.deserialize(JSON.serialize(new Map<String,Object>{'QualifiedApiName'=>objectName,'NamespacePrefix'=>namespaceValue}),EntityDefinition.class);
  SliceSource__mdt source=(SliceSource__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Entity__c'=>objectName,'Entity__r'=>entity}),SliceSource__mdt.class);
  SliceSource__mdt copy=new SliceSource__mdt();copy.put(SliceSource__mdt.Entity__c,source.get(SliceSource__mdt.Entity__c));
  return new Map<String,Object>{'name'=>source.Entity__r.QualifiedApiName,'prefix'=>source.Entity__r.NamespacePrefix,'copyHasParent'=>copy.Entity__r!=null,'scalar'=>copy.Entity__c};
 }
 public class AccessIdentityJob implements Queueable {
  private AccessLevel level;
  public AccessIdentityJob(AccessLevel value) { level=value; }
  public void execute(QueueableContext context) { insert new Account(Name='slice-level',Description=String.valueOf(level==AccessLevel.USER_MODE)); }
 }
 private static Map<String,Object> accessIdentity() {
  AccessLevel original=AccessLevel.USER_MODE;AccessLevel copy=(AccessLevel)original.clone();
  Map<String,Object> result=new Map<String,Object>{'cloneSame'=>original==copy,'cloneEqual'=>original.equals(copy)};
  try { JSON.deserialize(JSON.serialize(original),AccessLevel.class);result.put('json','accepted'); }
  catch(Exception issue) {result.put('json',issue.getTypeName());}
  if(Test.isRunningTest()) {
   Test.startTest();System.enqueueJob(new AccessIdentityJob(original));Test.stopTest();
   Account row=[SELECT Description FROM Account WHERE Name='slice-level' LIMIT 1];result.put('queueSame',row.Description=='true');
  }
  return result;
 }
 private static Map<String,Object> profileCreatable() {
  Boolean rejected=false;
  try { Database.insert(new List<SObject>{new Profile(Name='Synthetic Profile')}); } catch(Exception issue) { rejected=true; }
  return new Map<String,Object>{'creatable'=>Profile.SObjectType.getDescribe().isCreateable(),'rejected'=>rejected};
 }
 private static Map<String,Object> activity(Boolean invalid) {
  Account parent=new Account(Name='activity');insert parent;
  Contact target=new Contact(LastName='Recipient',Email='recipient@example.test',AccountId=parent.Id);insert target;
  Messaging.SingleEmailMessage message=new Messaging.SingleEmailMessage();message.setTargetObjectId(target.Id);message.setWhatId(invalid?UserInfo.getUserId():parent.Id);
  message.setSubject('Synthetic activity');message.setPlainTextBody('Synthetic content');message.setSaveAsActivity(true);
  Messaging.SendEmailResult[] results=Messaging.sendEmail(new Messaging.SingleEmailMessage[]{message},false);
  return new Map<String,Object>{'sent'=>results[0].isSuccess(),'errors'=>results[0].getErrors().size(),'tasks'=>[SELECT COUNT() FROM Task WHERE WhoId=:target.Id]};
 }
 private static Map<String,Object> listFlag() {
  insert new SliceFlag__c(Name='FeatureOne',Value__c='enabled');SliceFlag__c cached=SliceFlag__c.getInstance('FeatureOne');SliceFlag__c queried=[SELECT Value__c FROM SliceFlag__c WHERE Name='FeatureOne' LIMIT 1];
  return new Map<String,Object>{'cached'=>cached.Value__c=='enabled','queried'=>queried.Value__c=='enabled','missing'=>SliceFlag__c.getInstance('Absent')==null};
 }
}
