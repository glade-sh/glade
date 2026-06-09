public class PerfRiskQueryTests {
    public static void noWhereUnfilteredQuery() {
        List<Account> accounts = [SELECT Id, Name FROM Account];
        accounts.size();
    }
}
