public class SnD5 {
    public static void m() {
        Integer n = [SELECT COUNT() FROM Account GROUP BY Name][0];
    }
}
