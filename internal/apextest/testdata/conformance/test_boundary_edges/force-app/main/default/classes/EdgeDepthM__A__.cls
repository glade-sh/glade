public class EdgeDepthM__A__ {
    public static Integer n = 0;
    public static String mode = 'plain';
    public static String log = '';
    public static String scrub(String m, Id rid) {
        String id18 = String.valueOf(rid);
        return m.replace(id18, '<id>').replace(id18.substring(0, 15), '<id>');
    }
}
