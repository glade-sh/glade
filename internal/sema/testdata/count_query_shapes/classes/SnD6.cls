public class SnD6 {
    public static void m() {
        AggregateResult r = [SELECT COUNT() FROM Account GROUP BY Name][0];
    }
}
