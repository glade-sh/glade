public class SnC1 {
    public static void m() {
        for (Account a : [SELECT COUNT() FROM Account]) {
        }
    }
}
