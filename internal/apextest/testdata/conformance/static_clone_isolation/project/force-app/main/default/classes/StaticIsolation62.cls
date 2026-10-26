public class StaticIsolation62 {
    public static List<String> trace = new List<String>();
    public static Integer count = initialize();
    static { trace.add('block'); }
    private static Integer initialize() { trace.add('field'); return 3; }
}
