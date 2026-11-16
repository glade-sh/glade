public class CapQ2368 {
    public static void run() {
        Database.QueryOptions opt = Database.QueryOptions.builder()
            .withDataspace('default').withHonorEmptyStrings(true).build();
        System.debug(LoggingLevel.ERROR, 'DATASPACE=' + opt.getDataspace()
            + ';HONOR=' + opt.getHonorEmptyStrings());
        try {
            List<SObject> rows = Database.queryWithBinds(
                'SELECT Id FROM Account LIMIT 1 SET OPTIONS :opt',
                new Map<String,Object>{'opt'=>opt}, AccessLevel.SYSTEM_MODE);
            System.debug(LoggingLevel.ERROR, 'PLATFORM_ROWS=' + rows.size());
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'PLATFORM_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
