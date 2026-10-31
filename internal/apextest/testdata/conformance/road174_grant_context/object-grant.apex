User u=[SELECT Id FROM User WHERE LastName='Proof' AND IsActive=true LIMIT 1];
PermissionSet p=new PermissionSet(Name='Road174Grant',Label='Synthetic grant',PermissionsApiEnabled=true,PermissionsAuthorApex=true,PermissionsModifyMetadata=true,PermissionsViewRoles=true,PermissionsViewSetup=true);insert p;
insert new ObjectPermissions(ParentId=p.Id,SobjectType='Account',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true);
insert new ObjectPermissions(ParentId=p.Id,SobjectType='Order',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true,PermissionsDelete=true,PermissionsViewAllRecords=true,PermissionsModifyAllRecords=true);
insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=p.Id);
