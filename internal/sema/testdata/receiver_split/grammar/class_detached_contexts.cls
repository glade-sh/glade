public class CrRouteBefore {
    public static String F = CrProbe
        .value();
    public static String helper() { return 'h'; }
    public static String run() {
        CrProbe /* hi */ .value();
        String x = CrProbe
            .value();
        CrRouteBefore
            .helper();
        return CrProbe
            .value();
    }
}
