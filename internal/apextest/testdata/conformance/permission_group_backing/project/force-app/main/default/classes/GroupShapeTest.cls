@IsTest private class GroupShapeTest {
 @IsTest static void backingPermissionSet() {
  PermissionSetGroup grp=[SELECT Id,DeveloperName FROM PermissionSetGroup WHERE DeveloperName='Grove_Access'];
  List<PermissionSet> rows=[SELECT Id,Name,Type,PermissionSetGroupId,IsCustom,IsOwnedByProfile,NamespacePrefix FROM PermissionSet WHERE PermissionSetGroupId=:grp.Id];
  Map<String,Object> result=new Map<String,Object>{'count'=>rows.size()};
  if (!rows.isEmpty()) { PermissionSet ps=rows[0];result.put('name',ps.Name);result.put('type',ps.Type);result.put('custom',ps.IsCustom);result.put('profileOwned',ps.IsOwnedByProfile);result.put('groupLinked',ps.PermissionSetGroupId==grp.Id); result.put('namespace',ps.NamespacePrefix); }
  System.assert(false,'OBSERVED|backingPermissionSet|'+JSON.serialize(result));
 }
 @IsTest static void groupAssignment() {
  PermissionSetGroup grp=[SELECT Id,Status FROM PermissionSetGroup WHERE DeveloperName='Grove_Access'];
  if(grp.Status!='Updated')Test.calculatePermissionSetGroup(grp.Id);
  PermissionSet backing=[SELECT Id FROM PermissionSet WHERE PermissionSetGroupId=:grp.Id];
  PermissionSetAssignment assignment=new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetGroupId=grp.Id);
  insert assignment;
  Boolean immediate=assignment.PermissionSetId!=null;
  PermissionSetAssignment stored=[SELECT Id,PermissionSetId,PermissionSetGroupId,PermissionSet.Name FROM PermissionSetAssignment WHERE Id=:assignment.Id];
  System.assert(false,'OBSERVED|groupAssignment|'+JSON.serialize(new Map<String,Object>{'immediatePermissionSet'=>immediate,'backingLinked'=>stored.PermissionSetId==backing.Id,'groupLinked'=>stored.PermissionSetGroupId==grp.Id,'name'=>stored.PermissionSet.Name}));
 }
}
