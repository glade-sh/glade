public class CovStaticInit {
    public static Integer seed = 7;
    public static Integer doubled;
    public static List<String> tags;

    static {
        doubled = seed * 2;
        tags = new List<String>{ 'cov' };
    }

    public static Integer value() {
        return doubled;
    }
}
