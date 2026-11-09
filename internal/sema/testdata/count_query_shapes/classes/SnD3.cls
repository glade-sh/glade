public class SnD3 {
    public static void m() {
        for (AggregateResult r : [SELECT COUNT() FROM Account GROUP BY Name]) {
        }
    }
}
