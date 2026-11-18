PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'GapAccess' LIMIT 1];
if ([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId() AND PermissionSetId = :ps.Id] == 0) { insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = ps.Id); }
System.debug('OBS|fls|assigned');