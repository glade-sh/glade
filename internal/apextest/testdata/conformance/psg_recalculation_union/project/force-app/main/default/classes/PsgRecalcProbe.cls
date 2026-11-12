@IsTest private class PsgRecalcProbe {
 static Map<String,Boolean> rights(Id groupId) {
  PermissionSet p=[SELECT PermissionsApiEnabled,PermissionsRunReports FROM PermissionSet WHERE PermissionSetGroupId=:groupId AND Type='Group' LIMIT 1];
  return new Map<String,Boolean>{'api'=>p.PermissionsApiEnabled,'reports'=>p.PermissionsRunReports};
 }
 @IsTest static void observe() {
  Map<String,Object> result = new Map<String,Object>();
  try {
   PermissionSet a=new PermissionSet(Name='UnionA',Label='Union A',PermissionsApiEnabled=true);insert a;
   PermissionSet b=new PermissionSet(Name='UnionB',Label='Union B',PermissionsApiEnabled=true,PermissionsRunReports=true);insert b;
   PermissionSetGroup g=new PermissionSetGroup(DeveloperName='UnionProbe',MasterLabel='Union Probe');insert g;
   PermissionSetGroupComponent ca=new PermissionSetGroupComponent(PermissionSetGroupId=g.Id,PermissionSetId=a.Id);insert ca;
   Test.calculatePermissionSetGroup(g.Id);result.put('first',rights(g.Id));
   PermissionSetGroupComponent cb=new PermissionSetGroupComponent(PermissionSetGroupId=g.Id,PermissionSetId=b.Id);insert cb;
   Test.calculatePermissionSetGroup(g.Id);result.put('both',rights(g.Id));
   delete ca;Test.calculatePermissionSetGroup(g.Id);result.put('second',rights(g.Id));
   delete cb;Test.calculatePermissionSetGroup(g.Id);result.put('none',rights(g.Id));
  } catch (Exception e) { result.put('setupError',e.getTypeName()+': '+e.getMessage()); }
  System.assert(false,'CAP|'+JSON.serialize(result));
 }
}
