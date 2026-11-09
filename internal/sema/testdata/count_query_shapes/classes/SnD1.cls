public class SnD1 {
    public static void m() {
        Integer n = [SELECT COUNT() FROM Account GROUP BY Name];
    }
}
