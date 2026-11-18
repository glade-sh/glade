@IsTest private class ProbeV67 {
 static User actor(String name) {
  Profile p=[SELECT Id FROM Profile WHERE Name=:name LIMIT 1];
  return new User(Alias='vprobe',Email='probe@example.test',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',UserName='v036'+System.now().getTime()+name.length()+'@example.test');
 }
 static String sorted(Set<String> s) {List<String> v=new List<String>(s);v.sort();return String.join(v,',');}
 static void run(String profile, Boolean grant) {
  String out='';
  List<String> names=new List<String>{'Account','Contact','Lead','Case','Task','Product2','Pricebook2','Campaign'};
  List<String> checks=new List<String>{'IsPriorityRecord','DoNotCall','HasOptedOutOfEmail','HasOptedOutOfFax','IsClosed','IsStopped','IsEscalated','IsRecurrence2','IsRecurrence2Exception','IsRecurrence2Exclusion','ClosedOnCreate','ProductClass','IsSerialized','TransferRecordMode','ExpectedResponse','NumberSent'};
  User u=actor(profile);
  if (grant) { insert u; PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name='ProbeStandardFields' LIMIT 1]; insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id); }
  System.runAs(u) {
   for (String name:names) {
    Schema.SObjectType typ=Schema.getGlobalDescribe().get(name);
    SObject obj=typ.newSObject(null,true);
    Map<String,Schema.SObjectField> fields=typ.getDescribe().fields.getMap();
    out+=name+'|pop='+sorted(obj.getPopulatedFieldsAsMap().keySet());
    try { SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{obj},true); Set<String> removed=d.getRemovedFields().get(name); out+='|remove='+(removed==null?'':sorted(removed)); }
    catch(Exception e) { out+='|throw='+e.getTypeName(); }
    out+='|create='+typ.getDescribe().isCreateable()+'\n';
    for (String f:checks) if (fields.containsKey(f)) { Schema.DescribeFieldResult d=fields.get(f).getDescribe(); out+=name+'.'+f+'|read='+d.isAccessible()+'|create='+d.isCreateable()+'|default='+d.isDefaultedOnCreate()+'|pop='+obj.getPopulatedFieldsAsMap().containsKey(f)+'\n'; }
   }
  }
  System.assert(false,'CAPTURE\n'+out);
 }
 @IsTest static void admin() {run('System Administrator',false);}
 @IsTest static void standard() {run('Standard User',false);}
 @IsTest static void adminGrant() {run('System Administrator',true);}
 @IsTest static void standardGrant() {run('Standard User',true);}
}
