PermissionSet access = [SELECT Id FROM PermissionSet WHERE Name = 'ProbeCredentialAccess' LIMIT 1];
Id currentUserId = UserInfo.getUserId();
if ([SELECT COUNT() FROM PermissionSetAssignment WHERE AssigneeId = :currentUserId AND PermissionSetId = :access.Id] == 0) {
    insert new PermissionSetAssignment(AssigneeId = currentUserId, PermissionSetId = access.Id);
}
System.debug(LoggingLevel.ERROR, 'PROBE_ACCESS_READY');
