public class HhBody {
    static String q = 'SELECT Val__c FROM V35Hh__c';
    static Object t(String name, Map<String,Object> m, Callable c) {
        try { m.put(name, c.call()); } catch (Exception e) { m.put(name, e.getTypeName()); }
        return null;
    }
    public interface Callable { Object call(); }
    public class LocUser implements Callable { public Object call() { return Database.getQueryLocator(q, AccessLevel.USER_MODE).iterator().hasNext() ? 1 : 0; } }
    public class LocSystem implements Callable { public Object call() { return Database.getQueryLocator(q, AccessLevel.SYSTEM_MODE).iterator().hasNext() ? 1 : 0; } }
    public class LocDefault implements Callable { public Object call() { return Database.getQueryLocator(q).iterator().hasNext() ? 1 : 0; } }
    public class CurUser implements Callable { public Object call() { return Database.getCursor(q, AccessLevel.USER_MODE).getNumRecords(); } }
    public class CurSystem implements Callable { public Object call() { return Database.getCursor(q, AccessLevel.SYSTEM_MODE).getNumRecords(); } }
    public class CurDefault implements Callable { public Object call() { return Database.getCursor(q).getNumRecords(); } }
    public static Map<String,Object> run() {
        Map<String,Object> m = new Map<String,Object>();
        t('locUser', m, new LocUser()); t('locSystem', m, new LocSystem()); t('locDefault', m, new LocDefault());
        t('curUser', m, new CurUser()); t('curSystem', m, new CurSystem()); t('curDefault', m, new CurDefault());
        return m;
    }
}
