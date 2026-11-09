public class SnD2 {
    public static void m() {
        List<AggregateResult> r = [SELECT COUNT() FROM Account GROUP BY Name];
    }
}
