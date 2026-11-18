public class CapOrgLimits67 {
    public static void run() {
        Map<String, System.OrgLimit> m = OrgLimits.getMap();
        System.debug(LoggingLevel.ERROR, 'ORG_LIMIT_KEYS=' + m.size());
        for (String k : new List<String>{'DailyAsyncApexExecutions',
                 'DailyAsyncApexElasticExecutions', 'DailyAsyncApexProcessed',
                 'DailyApiRequests'}) {
            if (m.containsKey(k)) {
                System.debug(LoggingLevel.ERROR, k + '=' + m.get(k).getValue()
                    + '/' + m.get(k).getLimit());
            } else {
                System.debug(LoggingLevel.ERROR, k + '=ABSENT');
            }
        }
    }
}
