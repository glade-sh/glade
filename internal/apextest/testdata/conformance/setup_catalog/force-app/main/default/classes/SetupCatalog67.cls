public class SetupCatalog67 {
 public static Map<String,Object> permissionObservation() {
  Id profileId=UserInfo.getProfileId();
  List<PermissionSet> owned=[SELECT Id,ProfileId,IsOwnedByProfile FROM PermissionSet WHERE ProfileId=:profileId];
  List<PermissionSet> standard=[SELECT Name,Label,IsOwnedByProfile FROM PermissionSet WHERE Name='CommerceUser'];
  Map<String,Object> result=new Map<String,Object>{'profilePresent'=>!owned.isEmpty(),'standardCount'=>standard.size()};
  if(!standard.isEmpty()) { result.put('name',standard[0].Name); result.put('label',standard[0].Label); result.put('owned',standard[0].IsOwnedByProfile); }
  return result;
 }
 public static Map<String,Object> catalogObservation(Boolean populated) {

  EntityDefinition row;
  if(populated) row=[SELECT QualifiedApiName,(SELECT DeveloperName,DataType FROM Fields WHERE IsCalculated=true AND QualifiedApiName='Computed__c') FROM EntityDefinition WHERE QualifiedApiName='CatalogSample__c'];
  else row=[SELECT QualifiedApiName,(SELECT DeveloperName,DataType FROM Fields WHERE IsCalculated=true AND QualifiedApiName='Stored__c') FROM EntityDefinition WHERE QualifiedApiName='CatalogSample__c'];
  List<SObject> children=row.getSObjects('Fields');
  Map<String,Object> result=new Map<String,Object>{'isNull'=>children==null,'directSize'=>row.Fields.size()};
  if(children!=null) { result.put('size',children.size()); if(!children.isEmpty()) result.put('developerName',children[0].get('DeveloperName')); }
  return result;
 }
 public static Map<String,Object> scanObservation() {
  Boolean nullFailure=false;
  try {
   for(EntityDefinition row:[SELECT QualifiedApiName,(SELECT DeveloperName,DataType FROM Fields WHERE IsCalculated=true AND QualifiedApiName='Stored__c') FROM EntityDefinition WHERE QualifiedApiName='CatalogSample__c']) {
    for(FieldDefinition field:row.getSObjects('Fields')) { String name=field.DeveloperName; }
   }
  } catch(NullPointerException e) { nullFailure=true; }
  Boolean assignedNullFailure=false;
  EntityDefinition emptyRow=[SELECT QualifiedApiName,(SELECT DeveloperName FROM Fields WHERE QualifiedApiName='NoSuchField__c') FROM EntityDefinition WHERE QualifiedApiName='CatalogSample__c'];
  List<SObject> assigned=emptyRow.getSObjects('Fields');
  try { for(SObject field:assigned) {} } catch(NullPointerException e) { assignedNullFailure=true; }
  Boolean explicitNullFailure=false; List<FieldDefinition> missing=null;
  try { for(FieldDefinition field:missing) {} } catch(NullPointerException e) { explicitNullFailure=true; }
  return new Map<String,Object>{'nullFailure'=>nullFailure,'explicitNullFailure'=>explicitNullFailure,'assignedNullFailure'=>assignedNullFailure};
 }
 public static Map<String,Object> ordinaryObservation() {
  Account parent=new Account(Name='Owned parent'); insert parent;
  Account row=[SELECT Id,(SELECT Id FROM Contacts) FROM Account WHERE Id=:parent.Id];
  return new Map<String,Object>{'stringNull'=>row.getSObjects('Contacts')==null,'tokenNull'=>row.getSObjects(Contact.AccountId)==null,'directSize'=>row.Contacts.size()};
 }
}
