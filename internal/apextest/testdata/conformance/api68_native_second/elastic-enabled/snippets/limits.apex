Map<String, System.OrgLimit> m = OrgLimits.getMap();
for (String k : new List<String>{'DailyAsyncApexExecutions',
         'DailyAsyncApexElasticExecutions', 'DailyAsyncApexProcessed'}) {
    System.debug(LoggingLevel.ERROR, k + '=' +
        (m.containsKey(k) ? String.valueOf(m.get(k).getValue()) + '/' + String.valueOf(m.get(k).getLimit()) : 'ABSENT'));
}
