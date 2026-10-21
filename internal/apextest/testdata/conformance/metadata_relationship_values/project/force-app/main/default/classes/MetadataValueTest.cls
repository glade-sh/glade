@IsTest private class MetadataValueTest {
 private static EntityDefinition entity() {
  return (EntityDefinition)JSON.deserialize('{"QualifiedApiName":"Account","DurableId":"Account"}',EntityDefinition.class);
 }
 private static ViewConfig__mdt configuration() {
  return (ViewConfig__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Entity__c'=>'Account','Entity__r'=>entity(),'TextValue__c'=>'sample'}),Type.forName('ViewConfig__mdt'));
 }
 private static String objectName(ViewConfig__mdt config) {
  if(config==null) return null;
  if(config.Entity__r!=null) return config.Entity__r.QualifiedApiName;
  return config.Entity__c;
 }
 @IsTest static void nestedGraph() {
  ViewConfig__mdt config=configuration();
  ViewGroup__mdt holder=(ViewGroup__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Config__r'=>config}),Type.forName('ViewGroup__mdt'));
  ViewConfig__mdt scalar=new ViewConfig__mdt(Entity__c='Account');
  ViewGroup__mdt fallback=(ViewGroup__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Config__r'=>scalar}),ViewGroup__mdt.class);
  System.assert(false,'OBSERVED|nestedGraph|'+JSON.serialize(new Map<String,Object>{'groupPresent'=>holder.Config__r!=null,'entityPresent'=>holder.Config__r.Entity__r!=null,'name'=>objectName(holder.Config__r),'scalar'=>holder.Config__r.Entity__c,'fallback'=>objectName(fallback.Config__r)}));
 }
 @IsTest static void flatScalarCopy() {
  ViewConfig__mdt source=configuration();
  ViewConfig__mdt copy=new ViewConfig__mdt();
  for(SObjectField field:new List<SObjectField>{ViewConfig__mdt.Entity__c,ViewConfig__mdt.TextValue__c}) copy.put(field,source.get(field));
  ViewConfig__mdt direct=new ViewConfig__mdt(Entity__c='Account');
  System.assert(false,'OBSERVED|flatScalarCopy|'+JSON.serialize(new Map<String,Object>{'sourceParent'=>source.Entity__r!=null,'copyParent'=>copy.Entity__r!=null,'copyDynamicParent'=>copy.getSObject('Entity__r')!=null,'directParent'=>direct.Entity__r!=null,'copyScalar'=>copy.Entity__c,'text'=>copy.TextValue__c}));
 }
 @IsTest static void relationshipWrites() {
  ViewConfig__mdt config=new ViewConfig__mdt();
  ViewGroup__mdt holder=new ViewGroup__mdt();
  Map<String,Object> result=new Map<String,Object>();
  try { config.putSObject('Entity__r',entity());result.put('entityName','accepted'); }
  catch(Exception issue) { result.put('entityName',issue.getTypeName()+'|'+issue.getMessage()); }
  try { config.putSObject(ViewConfig__mdt.Entity__c,entity());result.put('entityToken','accepted'); }
  catch(Exception issue) { result.put('entityToken',issue.getTypeName()+'|'+issue.getMessage()); }
  try { config.put('Entity__r',entity());result.put('entityPut','accepted'); }
  catch(Exception issue) { result.put('entityPut',issue.getTypeName()+'|'+issue.getMessage()); }
  try { holder.putSObject('Config__r',configuration());result.put('configName','accepted'); }
  catch(Exception issue) { result.put('configName',issue.getTypeName()+'|'+issue.getMessage()); }
  try { holder.put('Config__r',configuration());result.put('configPut','accepted'); }
  catch(Exception issue) { result.put('configPut',issue.getTypeName()+'|'+issue.getMessage()); }
  result.put('entityPresent',config.Entity__r!=null);
  result.put('configPresent',holder.Config__r!=null);
  System.assert(false,'OBSERVED|relationshipWrites|'+JSON.serialize(result));
 }
 @IsTest static void queriedAndCached() {
  ViewConfig__mdt joined=[SELECT Entity__c,Entity__r.QualifiedApiName FROM ViewConfig__mdt WHERE DeveloperName='Example' WITH SYSTEM_MODE LIMIT 1];
  ViewConfig__mdt flat=[SELECT Entity__c FROM ViewConfig__mdt WHERE DeveloperName='Example' WITH SYSTEM_MODE LIMIT 1];
  ViewConfig__mdt cached=ViewConfig__mdt.getInstance('Example');
  ViewConfig__mdt allRows=ViewConfig__mdt.getAll().get('Example');
  Map<String,Object> result=new Map<String,Object>{'joinedParent'=>joined.Entity__r!=null,'joinedName'=>joined.Entity__r.QualifiedApiName,'joinedScalar'=>joined.Entity__c,'cachedParent'=>cached.Entity__r!=null,'cachedScalar'=>cached.Entity__c,'allParent'=>allRows.Entity__r!=null};
  try { result.put('flatParent',flat.Entity__r!=null); }
  catch(Exception issue) { result.put('flatError',issue.getTypeName()+'|'+issue.getMessage()); }
  System.assert(false,'OBSERVED|queriedAndCached|'+JSON.serialize(result));
 }
 @IsTest static void fieldDefinitionValues() {
  FieldDefinition column=(FieldDefinition)JSON.deserialize('{"QualifiedApiName":"Name","DurableId":"Account.Name"}',FieldDefinition.class);
  ViewConfig__mdt source=(ViewConfig__mdt)JSON.deserialize(JSON.serialize(new Map<String,Object>{'Entity__c'=>'Account','Field__c'=>'Account.Name','Field__r'=>column}),ViewConfig__mdt.class);
  ViewConfig__mdt copy=new ViewConfig__mdt();
  for(SObjectField token:new List<SObjectField>{ViewConfig__mdt.Entity__c,ViewConfig__mdt.Field__c}) copy.put(token,source.get(token));
  ViewConfig__mdt direct=new ViewConfig__mdt(Entity__c='Account',Field__c='Account.Name');
  ViewConfig__mdt joined=[SELECT Field__c,Field__r.QualifiedApiName FROM ViewConfig__mdt WHERE DeveloperName='Example' WITH SYSTEM_MODE LIMIT 1];
  ViewConfig__mdt flat=[SELECT Field__c FROM ViewConfig__mdt WHERE DeveloperName='Example' WITH SYSTEM_MODE LIMIT 1];
  ViewConfig__mdt cached=ViewConfig__mdt.getInstance('Example');
  ViewConfig__mdt allRows=ViewConfig__mdt.getAll().get('Example');
  Map<String,Object> result=new Map<String,Object>{'sourceParent'=>source.Field__r!=null,'sourceName'=>source.Field__r.QualifiedApiName,'copyParent'=>copy.Field__r!=null,'copyDynamicParent'=>copy.getSObject('Field__r')!=null,'directParent'=>direct.Field__r!=null,'copyScalar'=>copy.Field__c,'joinedName'=>joined.Field__r.QualifiedApiName,'joinedScalar'=>joined.Field__c,'cachedParent'=>cached.Field__r!=null,'allParent'=>allRows.Field__r!=null};
  try { result.put('flatParent',flat.Field__r!=null); }
  catch(Exception issue) { result.put('flatError',issue.getTypeName()+'|'+issue.getMessage()); }
  System.assert(false,'OBSERVED|fieldDefinitionValues|'+JSON.serialize(result));
 }
 @IsTest static void fieldDefinitionEntityName() {
  ViewConfig__mdt joined=[SELECT Field__c,Field__r.QualifiedApiName FROM ViewConfig__mdt WHERE DeveloperName='Example' WITH SYSTEM_MODE LIMIT 1];
  Map<String,Object> result=new Map<String,Object>();
  try {
   result.put('fieldName',joined.Field__r.QualifiedApiName);
   result.put('entityName',joined.Field__r.EntityDefinition.QualifiedApiName);
  } catch(Exception issue) {
   result.put('errorType',issue.getTypeName());
   result.put('errorMessage',issue.getMessage());
  }
  System.assert(false,'OBSERVED|fieldDefinitionEntityName|'+JSON.serialize(result));
 }
}
