public class DefaultFieldNativeObservation {
 public static User createActor(String key){
   Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
   String tag=key+String.valueOf(DateTime.now().getTime());
   User u=new User(Username='parcel-'+tag+'@example.invalid',Alias='parcel',Email='parcel@example.invalid',LastName='Parcel observation',ProfileId=p.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',LanguageLocaleKey='en_US',EmailEncodingKey='UTF-8');insert u;
   if(key!='FLS-DEF-08'){String permissionName=key=='FLS-DEF-07'?'ParcelWrite':'ParcelRead';PermissionSet ps=[SELECT Id FROM PermissionSet WHERE Name=:permissionName LIMIT 1];insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=ps.Id);}
  return u;
 }
 public static Map<String,Object> read(String key){
  Map<String,Object> out=new Map<String,Object>();
  try{User u=createActor(key);System.runAs(u){out=readCurrent(key);}}
  catch(Exception issue){out.put('type',issue.getTypeName());out.put('message',issue.getMessage());}
  return out;
 }
 public static Map<String,Object> readCurrent(String key){
  Map<String,Object> out=new Map<String,Object>{'row'=>key};
  try{
   Id currentId=UserInfo.getUserId();User current=[SELECT Profile.Name FROM User WHERE Id=:currentId];
   out.put('actorProfile',current.Profile.Name);
   if(current.Profile.Name!='Standard User'){out.put('nativeStatus','ACTOR_SESSION_REQUIRED');return out;}

    ParcelEntry__c row;
    if(key=='FLS-DEF-02')row=(ParcelEntry__c)JSON.deserialize('{"Name":"Owned","Payload__c":"value"}',ParcelEntry__c.class);
    else if(key=='FLS-DEF-04')row=(ParcelEntry__c)JSON.deserialize('{"Name":"Owned","Payload__c":"value","Ready__c":false}',ParcelEntry__c.class);
    else if(key=='FLS-DEF-06')row=(ParcelEntry__c)JSON.deserialize('{"Name":"Owned","Payload__c":"value","Ready__c":true}',ParcelEntry__c.class);
    else if(key=='FLS-DEF-03'||key=='FLS-DEF-07')row=new ParcelEntry__c(Name='Owned',Payload__c='value',Ready__c=false);
    else if(key=='FLS-DEF-05')row=new ParcelEntry__c(Name='Owned',Payload__c='value',Ready__c=true);
    else row=new ParcelEntry__c(Name='Owned',Payload__c='value');
    Schema.DescribeFieldResult field=ParcelEntry__c.Ready__c.getDescribe();
    out.put('objectName',ParcelEntry__c.SObjectType.getDescribe().getName());out.put('fieldName',field.getName());
    out.put('objectCreateable',ParcelEntry__c.SObjectType.getDescribe().isCreateable());out.put('fieldCreateable',field.isCreateable());out.put('fieldUpdateable',field.isUpdateable());out.put('fieldAccessible',field.isAccessible());out.put('fieldNillable',field.isNillable());out.put('fieldDefaultedOnCreate',field.isDefaultedOnCreate());out.put('fieldDefaultValue',field.getDefaultValue());
    out.put('originalBefore',JSON.serialize(row));out.put('populatedBefore',row.getPopulatedFieldsAsMap().keySet());out.put('isSetBefore',row.isSet(field.getName()));out.put('valueBefore',row.get(field.getName()));
    for(Boolean enforce:new List<Boolean>{true,false}){
     String mode=enforce?'rootEnforced':'rootNotEnforced';
     try{SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{row},enforce);SObject returned=d.getRecords()[0];out.put(mode,new Map<String,Object>{'removed'=>d.getRemovedFields(),'modified'=>d.getModifiedIndexes(),'returnedJSON'=>JSON.serialize(returned),'returnedPopulated'=>returned.getPopulatedFieldsAsMap().keySet(),'returnedIsSet'=>returned.isSet(field.getName()),'returnedValue'=>returned.get(field.getName())});}
     catch(Exception issue){out.put(mode,new Map<String,Object>{'type'=>issue.getTypeName(),'message'=>issue.getMessage()});}
    }
    out.put('originalAfter',JSON.serialize(row));out.put('populatedAfter',row.getPopulatedFieldsAsMap().keySet());out.put('isSetAfter',row.isSet(field.getName()));out.put('valueAfter',row.get(field.getName()));
  }catch(Exception issue){out.put('type',issue.getTypeName());out.put('message',issue.getMessage());}
  return out;
 }
}