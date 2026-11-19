PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name = 'GapAccess' LIMIT 1];
Id pid = ps.Id;
if ([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :UserInfo.getUserId() AND PermissionSetId = :pid] == 0) {
    insert new PermissionSetAssignment(AssigneeId = UserInfo.getUserId(), PermissionSetId = pid);
}
System.debug('OBS|fls|assigned');
