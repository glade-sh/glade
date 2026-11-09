public class PsaInvalidProbe {
    static String attempt(String label, PermissionSetAssignment psa) {
        String r;
        try { insert psa; r = 'inserted'; } catch (Exception e) { r = e.getTypeName() + ':' + e.getMessage(); }
        String r2;
        try {
            Database.SaveResult sr = Database.insert(new PermissionSetAssignment(AssigneeId = psa.AssigneeId, PermissionSetId = psa.PermissionSetId), false);
            if (sr.isSuccess()) { r2 = 'ok'; } else { Database.Error er = sr.getErrors()[0]; r2 = String.valueOf(er.getStatusCode()) + ':' + er.getMessage() + ':' + er.getFields(); }
        } catch (Exception e) { r2 = 'EX ' + e.getTypeName() + ':' + e.getMessage(); }
        return (r + ' ~ ' + r2).replace(String.valueOf(UserInfo.getUserId()), '<me>');
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        Id me = UserInfo.getUserId();
        o.put('unknownPrefix0PS', attempt('a', new PermissionSetAssignment(AssigneeId = me, PermissionSetId = '0PS0000000ZZZZZ')));
        o.put('nullSet', attempt('c', new PermissionSetAssignment(AssigneeId = me, PermissionSetId = null)));
        o.put('wrongType', attempt('d', new PermissionSetAssignment(AssigneeId = me, PermissionSetId = me)));
        PermissionSet ps = [SELECT Id FROM PermissionSet WHERE IsOwnedByProfile = false LIMIT 1];
        o.put('unknownAssignee', attempt('e', new PermissionSetAssignment(AssigneeId = '005000000000001AAA', PermissionSetId = ps.Id)));
        o.put('nullAssignee', attempt('f', new PermissionSetAssignment(AssigneeId = null, PermissionSetId = ps.Id)));
        return o;
    }
}
