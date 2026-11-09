public class HhBody {
    static String q = 'SELECT Val__c FROM V35Hh__c';
    static Object t(String name, Map<String,Object> m, Callable c) {
        try { m.put(name, c.call()); } catch (Exception e) { m.put(name, e.getTypeName()); }
        return null;
    }
    public interface Callable { Object call(); }
    public class PagUser implements Callable { public Object call() { return Database.getPaginationCursor(q, AccessLevel.USER_MODE).getNumRecords(); } }
    public class PagSystem implements Callable { public Object call() { return Database.getPaginationCursor(q, AccessLevel.SYSTEM_MODE).getNumRecords(); } }
    public class PagDefault implements Callable { public Object call() { return Database.getPaginationCursor(q).getNumRecords(); } }
    public static Map<String,Object> run() {
        Map<String,Object> m = new Map<String,Object>();
        t('pagUser', m, new PagUser()); t('pagSystem', m, new PagSystem()); t('pagDefault', m, new PagDefault());
        return m;
    }
}
