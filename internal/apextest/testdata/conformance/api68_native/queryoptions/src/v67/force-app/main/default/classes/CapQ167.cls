public class CapQ167 {
    public static void run() {
        Database.QueryOptions empty = Database.QueryOptions.builder().build();
        Boolean allNull = empty.getDataspace() == null
            && empty.getExplicitNamespace() == null
            && empty.getHonorEmptyStrings() == null;
        Database.QueryOptionsBuilder b = Database.QueryOptions.builder()
            .withHonorEmptyStrings(false);
        Database.QueryOptions oldSnapshot = b.build();
        b.withHonorEmptyStrings(true);
        Database.QueryOptions newSnapshot = b.build();
        System.debug(LoggingLevel.ERROR, 'EMPTY=' + allNull
            + ';SNAPSHOT=' + oldSnapshot.getHonorEmptyStrings()
            + ';CURRENT=' + newSnapshot.getHonorEmptyStrings());
        for (String mode : new List<String>{'NULL_DS','EMPTY_DS','NULL_NS'}) {
            try {
                if (mode == 'NULL_DS') {
                    Database.QueryOptions.builder().withDataspace(null).build();
                }
                if (mode == 'EMPTY_DS') {
                    Database.QueryOptions.builder().withDataspace('').build();
                }
                if (mode == 'NULL_NS') {
                    Database.QueryOptions.builder().withExplicitNamespace(null).build();
                }
                System.debug(LoggingLevel.ERROR, mode + '=SUCCESS');
            } catch (Exception e) {
                System.debug(LoggingLevel.ERROR, mode + '=' + e.getTypeName() + ': ' + e.getMessage());
            }
        }
    }
}
