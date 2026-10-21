public class GroupStatusObservation {
 public static Map<String,Object> observe(Boolean calculate) {
  String nonce=EncodingUtil.convertToHex(Crypto.generateAesKey(128)).substring(0,10);
  PermissionSetGroup emptyGroup=new PermissionSetGroup(DeveloperName='OwnedEmpty_'+nonce,MasterLabel='Owned empty '+nonce);
  PermissionSetGroup componentGroup=new PermissionSetGroup(DeveloperName='OwnedComponent_'+nonce,MasterLabel='Owned component '+nonce);
  insert new List<PermissionSetGroup>{emptyGroup,componentGroup};
  Map<String,Object> rows=new Map<String,Object>{'emptyInserted'=>snapshot(emptyGroup)};
  if(calculate) rows.put('emptyCalculated',recalculate(new List<Id>{emptyGroup.Id},false,emptyGroup));
  PermissionSet emptySet=new PermissionSet(Name='OwnedEmptySet_'+nonce,Label='Owned empty set '+nonce);
  insert emptySet;
  insert new PermissionSetGroupComponent(PermissionSetGroupId=componentGroup.Id,PermissionSetId=emptySet.Id);
  rows.put('componentInserted',snapshot(componentGroup));
  if(calculate) {
   rows.put('componentCalculated',recalculate(new List<Id>{componentGroup.Id},false,componentGroup));
   Map<String,Object> listRow=recalculate(new List<Id>{emptyGroup.Id,componentGroup.Id},true,componentGroup);
   listRow.put('emptyStatus',[SELECT Status FROM PermissionSetGroup WHERE Id=:emptyGroup.Id].Status);
   rows.put('listCalculated',listRow);
   insert new PermissionSetGroupComponent(PermissionSetGroupId=emptyGroup.Id,PermissionSetId=emptySet.Id);
   Map<String,Object> mixedRow=recalculate(new List<Id>{emptyGroup.Id,componentGroup.Id},true,componentGroup);
   mixedRow.put('emptyStatus',[SELECT Status FROM PermissionSetGroup WHERE Id=:emptyGroup.Id].Status);
   rows.put('mixedListCalculated',mixedRow);
   PermissionSet anotherSet=new PermissionSet(Name='OwnedAnotherSet_'+nonce,Label='Owned another set '+nonce);
   insert anotherSet;
   insert new List<PermissionSetGroupComponent>{new PermissionSetGroupComponent(PermissionSetGroupId=emptyGroup.Id,PermissionSetId=anotherSet.Id),new PermissionSetGroupComponent(PermissionSetGroupId=componentGroup.Id,PermissionSetId=anotherSet.Id)};
   Map<String,Object> outdatedRow=recalculate(new List<Id>{emptyGroup.Id,componentGroup.Id},true,componentGroup);
   outdatedRow.put('emptyStatus',[SELECT Status FROM PermissionSetGroup WHERE Id=:emptyGroup.Id].Status);
   rows.put('outdatedListCalculated',outdatedRow);
   rows.put('repeatCalculated',recalculate(new List<Id>{emptyGroup.Id},false,emptyGroup));
  }
  return rows;
 }
 public static Map<String,Object> snapshot(PermissionSetGroup groupRow) {
  return new Map<String,Object>{'callerStatus'=>groupRow.Status,'queriedStatus'=>[SELECT Status FROM PermissionSetGroup WHERE Id=:groupRow.Id].Status};
 }
 public static Map<String,Object> recalculate(List<Id> ids,Boolean listForm,PermissionSetGroup groupRow) {
  Integer statements=Limits.getDMLStatements(),dmlRows=Limits.getDMLRows(),queries=Limits.getQueries();
  Map<String,Object> row=new Map<String,Object>();
  try {if(listForm) Test.calculatePermissionSetGroup(ids);else Test.calculatePermissionSetGroup(ids[0]);row.put('succeeded',true);}
  catch(Exception error) {row.put('succeeded',false);row.put('errorType',error.getTypeName());row.put('error',error.getMessage());}
  row.put('dmlDelta',Limits.getDMLStatements()-statements);row.put('dmlRowsDelta',Limits.getDMLRows()-dmlRows);row.put('queryDelta',Limits.getQueries()-queries);
  row.putAll(snapshot(groupRow));return row;
 }
}
