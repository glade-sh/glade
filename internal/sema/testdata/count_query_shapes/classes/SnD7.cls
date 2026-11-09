public class SnD7 {
    public static void m() {
        Integer n = [SELECT COUNT() FROM Account GROUP BY Name HAVING COUNT(Id) > 1];
    }
}
