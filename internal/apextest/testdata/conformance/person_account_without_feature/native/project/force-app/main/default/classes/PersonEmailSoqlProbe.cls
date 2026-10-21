public class PersonEmailSoqlProbe {
    public static List<Account> queryRows() {
        return [SELECT PersonEmail FROM Account LIMIT 1];
    }
}
