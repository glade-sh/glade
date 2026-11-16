public class CapQ567 {
    public static void run() {
        Database.QueryOptions options = null;
        try {
            Database.query('SELECT Id FROM Account LIMIT 0 SET OPTIONS :options',
                AccessLevel.SYSTEM_MODE);
            System.debug(LoggingLevel.ERROR, 'NULL_OPTIONS=SUCCESS');
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'NULL_OPTIONS=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
