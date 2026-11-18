public class DthHashSupport {
    public static Id profileId() {
        return [SELECT Id FROM Profile WHERE Name = 'Standard User' LIMIT 1].Id;
    }

    public static void grantProbe(Id userId) {
        Id psId = [SELECT Id FROM PermissionSet WHERE Name = 'Dth_Probe_Access' LIMIT 1].Id;
        insert new PermissionSetAssignment(AssigneeId = userId, PermissionSetId = psId);
    }
}
