PermissionSet access = [SELECT Id FROM PermissionSet WHERE Name = 'MatrixFieldAccess' LIMIT 1];
insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = access.Id);
System.debug('FIELD_ACCESS_ASSIGNED');
