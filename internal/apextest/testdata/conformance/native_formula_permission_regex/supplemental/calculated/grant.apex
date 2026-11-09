PermissionSet p=[SELECT Id FROM PermissionSet WHERE Name='Probe_Bypass_Access' LIMIT 1];insert new PermissionSetAssignment(AssigneeId=UserInfo.getUserId(),PermissionSetId=p.Id);
