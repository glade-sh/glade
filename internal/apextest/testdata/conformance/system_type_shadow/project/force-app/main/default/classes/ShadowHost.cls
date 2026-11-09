public class ShadowHost {
    public class QueryException extends Exception {}
    public class EventBus {
        public String publish(String s) { return 'inner:' + s; }
        public Integer size() { return 7; }
    }
    public class ListException extends Exception {}

    public static Map<String, Object> observe() {
        Map<String, Object> r = new Map<String, Object>();
        try { throw new QueryException('m'); } catch (Exception e) { r.put('inner_type', e.getTypeName()); }
        try { throw new QueryException('m'); } catch (QueryException e) { r.put('inner_catch', e.getTypeName()); }
        try { throw new System.QueryException('m'); } catch (System.QueryException e) { r.put('sys_qualified', e.getTypeName()); }
        try {
            Account a = [SELECT Id FROM Account WHERE Name = 'no-such-shadow-row' LIMIT 1];
            r.put('sys_thrown', 'none');
        } catch (QueryException e) {
            r.put('sys_thrown', 'caught-inner:' + e.getTypeName());
        } catch (Exception e) {
            r.put('sys_thrown', 'caught-exception:' + e.getTypeName());
        }
        try {
            Account a = [SELECT Id FROM Account WHERE Name = 'no-such-shadow-row' LIMIT 1];
        } catch (System.QueryException e) {
            r.put('sys_thrown_qualified', e.getTypeName());
        }
        Exception ex = new QueryException('m');
        r.put('instanceof_inner', ex instanceof QueryException);
        r.put('instanceof_sys', ex instanceof System.QueryException);
        r.put('class_name', QueryException.class.getName());
        r.put('sys_class_name', System.QueryException.class.getName());
        r.put('new_inner_type', new QueryException('m').getTypeName());
        r.put('list_inner', new ListException('m').getTypeName());
        EventBus b = new EventBus();
        r.put('bus_inner', b.publish('x'));
        r.put('bus_size', b.size());
        return r;
    }
}
