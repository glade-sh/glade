public class ApprovalReferenceCapture {
 public static String observe() {
  String suffix=String.valueOf(System.now().getTime()); String out='';
  PermissionSet ps=new PermissionSet(Name='RefGrant'+suffix,Label='Ref Grant '+suffix);insert ps;
  MutingPermissionSet mute=new MutingPermissionSet(DeveloperName='RefMute'+suffix,MasterLabel='Ref Mute '+suffix);insert mute;
  PermissionSetGroup groupRow=new PermissionSetGroup(DeveloperName='RefGroup'+suffix,MasterLabel='Ref Group '+suffix);insert groupRow;
  try {insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Contact',PermissionsRead=true);out+='ordinary=OK;';}catch(DmlException e){out+='ordinary='+String.valueOf(e.getDmlStatusCode(0))+';';}
  try {insert new ObjectPermissions(ParentId=mute.Id,SObjectType='Account',PermissionsModifyAllRecords=true);out+='muting=OK;';}catch(DmlException e){out+='muting='+String.valueOf(e.getDmlStatusCode(0))+';';}
  try {insert new PermissionSetGroupComponent(PermissionSetGroupId=groupRow.Id,PermissionSetId=mute.Id);out+='component=OK;';}catch(DmlException e){out+='component='+String.valueOf(e.getDmlStatusCode(0))+';';}
  try {insert new ObjectPermissions(ParentId=UserInfo.getUserId(),SObjectType='Account',PermissionsRead=true);out+='wrong=OK;';}catch(DmlException e){out+='wrong='+String.valueOf(e.getDmlStatusCode(0))+';';}
  return out;
 }
}
