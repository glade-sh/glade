public class ViewAllFieldsObservation {
 static Map<String,Object> fail(Exception e){return new Map<String,Object>{'exceptionType'=>e.getTypeName(),'exceptionMessage'=>e.getMessage()};}
 public static Object observe(String key) {
  Map<String,Object> out=new Map<String,Object>();
  PermissionSet parent=new PermissionSet(Name='ViewAllObservation',Label='View All Observation');
  insert parent;
  if(key=='OPV7'||key=='OPV8') {
   Id missingParent; if(key=='OPV8') missingParent='0PS000000000001';
   ObjectPermissions grant=new ObjectPermissions(ParentId=missingParent,SObjectType='Contact',PermissionsRead=true);
   Database.SaveResult result=Database.insert(grant,false);
   out.put('success',result.isSuccess());
   List<String> errors=new List<String>();for(Database.Error error:result.getErrors())errors.add(String.valueOf(error.getStatusCode())+':'+error.getMessage()+':'+error.getFields());out.put('errors',errors);
  } else if(key=='OPV1'||key=='OPV6') {
   ObjectPermissions grant=new ObjectPermissions(ParentId=parent.Id,SObjectType='Contact',PermissionsRead=true);
   try { grant.put('PermissionsViewAllFields',key=='OPV1'); out.put('putReturned',true); } catch(Exception e) { out.put('put',fail(e)); }
   Database.SaveResult result;
   try { result=Database.insert(grant,false); out.put('success',result.isSuccess()); if(!result.isSuccess()){List<String> errors=new List<String>();for(Database.Error error:result.getErrors())errors.add(String.valueOf(error.getStatusCode())+':'+error.getMessage());out.put('errors',errors);} } catch(Exception e) { out.put('insert',fail(e)); }
   if(result!=null&&result.isSuccess()) {
    Id target=grant.Id;
    try { SObject s=Database.query('SELECT PermissionsRead,PermissionsViewAllFields FROM ObjectPermissions WHERE Id=:target'); out.put('viewAllFields',s.get('PermissionsViewAllFields')); } catch(Exception e) { out.put('query',fail(e)); }
    try { SObject s=Database.query('SELECT PermissionsRead FROM ObjectPermissions WHERE Id=:target'); out.put('read',s.get('PermissionsRead')); } catch(Exception e) { out.put('queryRead',fail(e)); }
   }
  } else if(key=='OPV2') {
   insert new ObjectPermissions(ParentId=parent.Id,SObjectType='Contact',PermissionsRead=true);
   try { List<SObject> rows=Database.query('SELECT Id,PermissionsViewAllFields FROM ObjectPermissions WHERE ParentId=\''+parent.Id+'\''); out.put('rows',rows.size()); out.put('value',rows[0].get('PermissionsViewAllFields')); } catch(Exception e) { out.put('query',fail(e)); }
  } else if(key=='OPV3') {
   ObjectPermissions grant=new ObjectPermissions();
   try { out.put('value',grant.get('PermissionsViewAllFields')); } catch(Exception e) { out.put('get',fail(e)); }
   try { out.put('populated',grant.isSet('PermissionsViewAllFields')); } catch(Exception e) { out.put('isSet',fail(e)); }
  } else if(key=='OPV4') {
   ObjectPermissions grant=new ObjectPermissions(SObjectType='Contact',PermissionsRead=true,PermissionsCreate=true);
   Database.SaveResult result=Database.insert(grant,false);
   out.put('success',result.isSuccess());
   List<String> errors=new List<String>();for(Database.Error error:result.getErrors())errors.add(String.valueOf(error.getStatusCode())+':'+error.getMessage()+':'+error.getFields());out.put('errors',errors);
  } else if(key=='OPV5') {
   Schema.DescribeSObjectResult d=Schema.getGlobalDescribe().get('ObjectPermissions').getDescribe();
   out.put('fieldMap',d.fields.getMap().containsKey('PermissionsViewAllFields'));
   out.put('fieldMapNames',d.fields.getMap().keySet().size());
   Map<String,Schema.SObjectField> viaGlobal=Schema.getGlobalDescribe().get('ObjectPermissions').getDescribe().fields.getMap();
   out.put('global',viaGlobal.containsKey('PermissionsViewAllFields'));
   Boolean listed=false;for(Schema.SObjectField f:d.fields.getMap().values())if(f.getDescribe().getName()=='PermissionsViewAllFields')listed=true;
   out.put('listed',listed);
  }
  return out;
 }
}
