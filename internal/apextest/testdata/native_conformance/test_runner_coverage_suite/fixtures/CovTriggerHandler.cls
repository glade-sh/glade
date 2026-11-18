public class CovTriggerHandler {
    public static Integer seen = 0;

    public static void onAfterInsert(List<Account> records) {
        for (Account a : records) {
            if (a.Name != null
                && a.Name.startsWith('Cov')) {
                seen = seen + 1;
            }
        }
    }
}
