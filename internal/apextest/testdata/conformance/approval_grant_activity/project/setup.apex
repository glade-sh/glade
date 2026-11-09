try {
 PermissionSet ps=new PermissionSet(Name='ApprovalGrant',Label='Approval Grant'); insert ps;
 insert new ObjectPermissions(ParentId=ps.Id,SObjectType='Account',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true,PermissionsViewAllRecords=true,PermissionsModifyAllRecords=true);
 PermissionSetGroup pg=new PermissionSetGroup(DeveloperName='ApprovalGroup',MasterLabel='Approval Group'); insert pg;
 insert new PermissionSetGroupComponent(PermissionSetGroupId=pg.Id,PermissionSetId=ps.Id);
 MutingPermissionSet mute=new MutingPermissionSet(DeveloperName='ApprovalMute',MasterLabel='Approval Mute'); insert mute;
 insert new ObjectPermissions(ParentId=mute.Id,SObjectType='Account',PermissionsModifyAllRecords=true);
 insert new PermissionSetGroupComponent(PermissionSetGroupId=pg.Id,PermissionSetId=mute.Id);
 System.debug('APPROVAL_GRANT_SETUP_OK');
} catch(Exception e) {System.debug('APPROVAL_GRANT_SETUP_FAIL|'+e.getMessage());}
