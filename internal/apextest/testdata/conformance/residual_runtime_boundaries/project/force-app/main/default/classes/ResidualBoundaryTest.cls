@IsTest private class ResidualBoundaryTest {
 private static void emit(Object value){System.assert(false,'BOUNDARY|'+JSON.serialize(value));}
 @IsTest static void recordIdentity(){emit(ResidualBoundaryProbe.recordIdentity());}
 @IsTest static void automaticActivity(){emit(ResidualBoundaryProbe.emailActivity(true,false));}
 @IsTest static void duplicateActivity(){emit(ResidualBoundaryProbe.emailActivity(true,true));}
 @IsTest static void manualActivity(){emit(ResidualBoundaryProbe.emailActivity(false,true));}
 @IsTest static void readTraversal(){emit(ResidualBoundaryProbe.readTraversal());}
 @IsTest static void mutateTraversal(){ResidualBoundaryProbe.mutateTraversal();System.assert(false,'unreachable');}
 @IsTest static void nameNoGrant(){observeNames(false);}
 @IsTest static void nameObjectGrant(){observeNames(true);}
 private static void observeNames(Boolean granted){
  Profile profile=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1];
  User actor=new User(ProfileId=profile.Id,Alias='rbound',LastName='Owned',Email='owned@example.invalid',Username='boundary.'+String.valueOf(DateTime.now().getTime())+'@example.invalid',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles');insert actor;
  if(granted){
   PermissionSet permission=new PermissionSet(Name='OwnedBoundaryGrant',Label='Owned boundary grant');insert permission;
   for(String objectName:new List<String>{'Account','BoundaryCard__c','BoundarySequence__c'})insert new ObjectPermissions(ParentId=permission.Id,SObjectType=objectName,PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true);
   insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=permission.Id);
  }
  Map<String,Object> out=new Map<String,Object>();
  System.runAs(actor){
   for(String objectName:new List<String>{'Account','BoundaryCard__c','BoundarySequence__c'}){
    Schema.DescribeSObjectResult objectDescribe=Schema.getGlobalDescribe().get(objectName).getDescribe();
    Schema.DescribeFieldResult field=objectDescribe.fields.getMap().get('Name').getDescribe();
    out.put(objectName,new List<Boolean>{objectDescribe.isAccessible(),objectDescribe.isCreateable(),objectDescribe.isUpdateable(),field.isAccessible(),field.isCreateable(),field.isUpdateable()});
   }
   Schema.DescribeFieldResult optional=BoundaryCard__c.Value__c.getDescribe();
   out.put('optional',new List<Boolean>{optional.isAccessible(),optional.isCreateable(),optional.isUpdateable()});
   Schema.DescribeFieldResult audit=Account.CreatedDate.getDescribe();
   out.put('auditWrite',new List<Boolean>{audit.isCreateable(),audit.isUpdateable()});
  }
  emit(out);
 }
}
