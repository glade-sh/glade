public class IsPersonAccountSoqlProbe {
    public static List<Account> queryRows() {
        return [SELECT IsPersonAccount FROM Account LIMIT 1];
    }
}
