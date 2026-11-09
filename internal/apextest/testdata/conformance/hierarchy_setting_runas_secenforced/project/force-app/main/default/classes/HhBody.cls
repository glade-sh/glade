public class HhBody {
    static String q = 'SELECT Val__c FROM V35Hh__c';
    static Object t(String name, Map<String,Object> m, Callable c) {
        try { m.put(name, c.call()); } catch (Exception e) { m.put(name, e.getTypeName()); }
        return null;
    }
    public interface Callable { Object call(); }
    public class SecStatic implements Callable { public Object call() { return [SELECT Val__c FROM V35Hh__c WITH SECURITY_ENFORCED].size(); } }
    public class SecDynamic implements Callable { public Object call() { return Database.query('SELECT Val__c FROM V35Hh__c WITH SECURITY_ENFORCED').size(); } }
    public static Map<String,Object> run() {
        Map<String,Object> m = new Map<String,Object>();
        t('secStatic', m, new SecStatic()); t('secDynamic', m, new SecDynamic());
        return m;
    }
}
