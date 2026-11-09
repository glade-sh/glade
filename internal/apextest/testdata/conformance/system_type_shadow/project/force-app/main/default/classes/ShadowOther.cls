public class ShadowOther {
    public static Map<String, Object> observe() {
        Map<String, Object> r = new Map<String, Object>();
        try {
            Account a = [SELECT Id FROM Account WHERE Name = 'no-such-shadow-row' LIMIT 1];
        } catch (QueryException e) {
            r.put('other_sys', e.getTypeName());
        }
        try { throw new ShadowHost.QueryException('m'); } catch (ShadowHost.QueryException e) { r.put('other_inner', e.getTypeName()); }
        ShadowHost.EventBus b = new ShadowHost.EventBus();
        r.put('other_bus', b.publish('y'));
        List<Database.SaveResult> res = new List<Database.SaveResult>();
        r.put('other_db', res.size());
        return r;
    }
}
