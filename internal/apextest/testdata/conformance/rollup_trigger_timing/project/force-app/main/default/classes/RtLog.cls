public class RtLog {
    public static String obs = '';
    public static Integer delCount = 0;
    public static List<String> delSizes = new List<String>();
    public static void reset() {
        obs = '';
        delCount = 0;
        delSizes = new List<String>();
    }
    public static void add(String s) {
        obs = (obs == '' ? '' : ';') + s;
    }
    public static String take() {
        String s = obs;
        obs = '';
        return s;
    }
}
