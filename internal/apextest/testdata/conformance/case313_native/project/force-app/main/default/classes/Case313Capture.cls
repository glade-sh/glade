@IsTest private class Case313Capture {
 static User actor() {
  User u=new User(Username='probe.'+System.now().getTime()+'@example.invalid',Alias='probe',Email='probe@example.invalid',LastName='Probe',ProfileId=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1].Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');
  insert u; return u;
 }
 static ObjectPermissions grant(User u) {
  PermissionSet ps=new PermissionSet(Name='ProbeGrant',Label='Probe Grant'); insert ps;
  ObjectPermissions op=new ObjectPermissions(ParentId=ps.Id,SObjectType='ProbeItem__c',PermissionsRead=true); insert op;
  insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id); return op;
 }
 @IsTest static void scalarDelete() {
  User u=actor(); ObjectPermissions op=grant(u); Map<String,Object> out=new Map<String,Object>();
  System.runAs(u) { try { delete op; out.put('scalar','ok'); } catch(Exception e) { out.put('scalar',e.getTypeName()+'|'+e.getMessage()); } }
  System.assert(false,'OBS|'+JSON.serialize(out));
 }
 @IsTest static void listDelete() {
  User u=actor(); ObjectPermissions op=grant(u); Map<String,Object> out=new Map<String,Object>();
  System.runAs(u) { try { delete new List<ObjectPermissions>{op}; out.put('list','ok'); } catch(Exception e) { out.put('list',e.getTypeName()+'|'+e.getMessage()); } }
  System.assert(false,'OBS|'+JSON.serialize(out));
 }
 @IsTest static void nestedSnapshot() {
  User u=actor(); ObjectPermissions op=grant(u); Map<String,Object> out=new Map<String,Object>();
  System.runAs(u) {
   Schema.DescribeSObjectResult held=ProbeItem__c.SObjectType.getDescribe(); out.put('before',held.isAccessible());
   try { delete op; out.put('delete','ok'); } catch(Exception e) { out.put('delete',e.getTypeName()+'|'+e.getMessage()); }
   System.runAs(u) { out.put('held',held.isAccessible()); out.put('fresh',ProbeItem__c.SObjectType.getDescribe().isAccessible()); }
   out.put('restored',ProbeItem__c.SObjectType.getDescribe().isAccessible());
  }
  System.assert(false,'OBS|'+JSON.serialize(out));
 }
 @IsTest static void nestedControl() {
  User u=actor(); grant(u); Map<String,Object> out=new Map<String,Object>();
  System.runAs(u) {
   Schema.DescribeSObjectResult held=ProbeItem__c.SObjectType.getDescribe(); out.put('before',held.isAccessible());
   System.runAs(u) { out.put('held',held.isAccessible()); out.put('fresh',ProbeItem__c.SObjectType.getDescribe().isAccessible()); }
   out.put('restored',ProbeItem__c.SObjectType.getDescribe().isAccessible());
  }
  System.assert(false,'OBS|'+JSON.serialize(out));
 }
}
