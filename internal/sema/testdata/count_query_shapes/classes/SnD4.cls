public class SnD4 {
    public static void m() {
        List<AggregateResult> r = [SELECT Name, COUNT() FROM Account GROUP BY Name];
    }
}
