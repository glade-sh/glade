public class CovTernary {
    public static String pick(Integer n) {
        String label = n > 10
            ? 'big'
            : 'small';
        return label;
    }

    public static Integer clamp(Integer n) {
        Integer out = (n < 0) ? 0 : n;
        return out;
    }

    public static String never(Integer n) {
        String s = n == 999
            ? 'match'
            : 'miss';
        return s;
    }
}
