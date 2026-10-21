@IsTest private class StubObjectCollisionProbeTest {
 private static void emit(String key,Map<String,Object> result){System.assert(false,'OBSERVED|'+key+'|'+JSON.serialize(result));}
 @IsTest static void observeCollision(){
  Map<String,Object> result=new Map<String,Object>();
  try{Folder target=(Folder)Test.createStub(Folder.class,new CanopyStubProvider());result.put('value',target.answer());}
  catch(Exception issue){result.put('type',issue.getTypeName());result.put('message',issue.getMessage());}
  emit('observeCollision',result);
 }
 @IsTest static void observePlain(){
  Map<String,Object> result=new Map<String,Object>();
  try{CanopyStubTarget target=(CanopyStubTarget)Test.createStub(CanopyStubTarget.class,new CanopyStubProvider());result.put('value',target.answer());}
  catch(Exception issue){result.put('type',issue.getTypeName());result.put('message',issue.getMessage());}
  emit('observePlain',result);
 }
 @IsTest static void observeSchema(){
  Map<String,Object> result=new Map<String,Object>();
  try{Object target=Test.createStub(Schema.Folder.class,new CanopyStubProvider());result.put('created',target!=null);}
  catch(Exception issue){result.put('type',issue.getTypeName());result.put('message',issue.getMessage());}
  emit('observeSchema',result);
 }
 @IsTest static void observeRecordTypeDefault(){
  Map<String,Object> result=new Map<String,Object>();
  try{CanopyEntry__c row=new CanopyEntry__c(Name='first');result.put('fresh',row.TypeName__c);result.put('freshTypePresent',row.RecordTypeId!=null);insert row;result.put('caller',row.TypeName__c);result.put('callerTypePresent',row.RecordTypeId!=null);row=[SELECT TypeName__c,RecordTypeId FROM CanopyEntry__c WHERE Id=:row.Id];result.put('persisted',row.TypeName__c);result.put('persistedTypePresent',row.RecordTypeId!=null);}
  catch(Exception issue){result.put('type',issue.getTypeName());result.put('message',issue.getMessage());}
  emit('observeRecordTypeDefault',result);
 }
 @IsTest static void observeExplicitRecordTypeDefault(){
  Map<String,Object> result=new Map<String,Object>();
  try{Id typeId=CanopyEntry__c.SObjectType.getDescribe().getRecordTypeInfosByDeveloperName().get('Alpha').getRecordTypeId();CanopyEntry__c row=new CanopyEntry__c(Name='explicit',RecordTypeId=typeId);result.put('fresh',row.TypeName__c);result.put('freshTypeMatches',row.RecordTypeId==typeId);Database.insert(row,AccessLevel.SYSTEM_MODE);result.put('caller',row.TypeName__c);row=[SELECT TypeName__c,RecordTypeId FROM CanopyEntry__c WHERE Id=:row.Id];result.put('persisted',row.TypeName__c);result.put('persistedTypeMatches',row.RecordTypeId==typeId);}
  catch(Exception issue){result.put('type',issue.getTypeName());result.put('message',issue.getMessage());}
  emit('observeExplicitRecordTypeDefault',result);
 }
}
