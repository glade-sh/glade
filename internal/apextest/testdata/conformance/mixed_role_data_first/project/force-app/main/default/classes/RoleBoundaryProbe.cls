public class RoleBoundaryProbe {
    public static Map<String,Object> observe(Boolean roleFirst) {
        Map<String,Object> result = new Map<String,Object>();
        UserRole role = new UserRole(Name='Capture role ' + String.valueOf(DateTime.now().getTime()));
        Account account = new Account(Name='Capture account');
        if (roleFirst) { attempt('role', role, result); attempt('account', account, result); }
        else { attempt('account', account, result); attempt('role', role, result); }
        return result;
    }
    private static void attempt(String stage, SObject row, Map<String,Object> result) {
        try { insert row; result.put(stage, 'ok'); }
        catch (DmlException e) { result.put(stage, String.valueOf(e.getDmlStatusCode(0))); }
    }
}
