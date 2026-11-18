@IsTest private class PsrFinishMixedTest {
 @IsTest static void setupObjectsAssignmentLast() {
  Id ps=PsrUtil.permSetId();
  List<SObject> items=new List<SObject>{new PermissionSet(Name='PsrOther',Label='Psr Other'),new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=ps)};
  String result='ok';
  try { insert as system items; } catch(Exception e) {result='ex:'+e.getTypeName()+':'+e.getMessage();}
  Boolean now=FeatureManagement.checkPermission('PsrPerm'); Boolean later=false;
  System.runAs(new User(Id=UserInfo.getUserId())) {later=FeatureManagement.checkPermission('PsrPerm');}
  System.assert(false,'insert='+result+';immediate='+now+';later='+later);
 }
}
