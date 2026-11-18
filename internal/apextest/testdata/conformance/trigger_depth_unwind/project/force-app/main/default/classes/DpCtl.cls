public class DpCtl {
    public static Integer depth = 0;
    public static String mode = 'throw';
    public static String log = '';
    public static String counters = '';
    public static void add(String s) {
        log = (log == '' ? '' : ',') + s;
    }
    public static void reset(String m) {
        depth = 0;
        mode = m;
        log = '';
        counters = '';
    }
    public static String type(Exception e) {
        String t = e.getTypeName();
        if (t == 'System.DmlException') {
            return 'D';
        }
        if (t == 'System.LimitException') {
            return 'L';
        }
        return 'X';
    }
    public static String loc(Exception e) {
        return sloc(e.getMessage());
    }
    public static String sloc(String m) {
        if (m == null) {
            return 'none';
        }
        Integer i = m.indexOf('Trigger.');
        return i < 0 ? 'none' : m.substring(i, Math.min(i + 60, m.length()));
    }
}
