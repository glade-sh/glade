public class PclLeadGrant {
    public static void grant(User u) {
        PermissionSet ps = new PermissionSet(Name = 'PclLeadRead', Label = 'PclLeadRead');
        insert ps;
        insert new ObjectPermissions(ParentId = ps.Id, SobjectType = 'Lead', PermissionsRead = true);
        insert new PermissionSetAssignment(AssigneeId = u.Id, PermissionSetId = ps.Id);
    }
}
