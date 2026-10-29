public class PermissionOmissionObservation {
 public static Object observe(String key) {
  Map<String,Object> out=new Map<String,Object>();
  PermissionSet parent=new PermissionSet(Name='CheckboxObservation',Label='Checkbox Observation');
  insert parent;
  PermissionSet storedParent=[SELECT PermissionsEmailSingle,PermissionsCustomizeApplication FROM PermissionSet WHERE Id=:parent.Id];
  out.put('parentEmailSingle',storedParent.PermissionsEmailSingle);
  out.put('parentOmitted',storedParent.PermissionsCustomizeApplication);
  Boolean crud=key=='OP_2';
  ObjectPermissions grant=new ObjectPermissions(ParentId=parent.Id,SObjectType='Contact',PermissionsRead=true,PermissionsCreate=crud,PermissionsEdit=crud,PermissionsDelete=crud);
  List<String> names=new List<String>{'PermissionsRead','PermissionsCreate','PermissionsEdit','PermissionsDelete','PermissionsModifyAllRecords','PermissionsViewAllRecords'};
  Boolean viewFields=Schema.getGlobalDescribe().get('ObjectPermissions').getDescribe().fields.getMap().containsKey('PermissionsViewAllFields');
  out.put('viewAllFieldsAvailable',viewFields);
  if(viewFields)names.add('PermissionsViewAllFields');
  if(key=='OP_3')for(String name:names)if(name!='PermissionsRead')grant.put(name,false);
  if(key=='OP_R')grant.ParentId=null;
  Database.SaveResult result=Database.insert(grant,false);
  out.put('success',result.isSuccess());
  if(!result.isSuccess()) {
   List<String> errors=new List<String>();for(Database.Error error:result.getErrors())errors.add(String.valueOf(error.getStatusCode()));out.put('errors',errors);
  } else {
   Id target=grant.Id;SObject stored=Database.query('SELECT '+String.join(names,',')+' FROM ObjectPermissions WHERE Id=:target');
   Map<String,Object> values=new Map<String,Object>();for(String name:names)values.put(name,stored.get(name));out.put('stored',values);
  }
  return out;
 }
}
