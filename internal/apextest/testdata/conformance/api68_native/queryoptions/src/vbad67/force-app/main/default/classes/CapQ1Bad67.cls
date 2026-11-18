public class CapQ1Bad67 {
    public static void run() {
        String options = 'bad';
        List<Account> rows = [SELECT Id FROM Account LIMIT 0 SET OPTIONS :options];
        System.debug(LoggingLevel.ERROR, 'ROWS=' + rows.size());
    }
}
