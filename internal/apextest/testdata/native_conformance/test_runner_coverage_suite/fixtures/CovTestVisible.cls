public class CovTestVisible {
    @TestVisible
    private static Integer bump(Integer n) {
        return n + 1;
    }

    @TestVisible
    private String echo(String s) {
        return s;
    }

    public static Integer exposed(Integer n) {
        return n * 2;
    }
}
