for (AsyncApexJob j : [SELECT Id, Status, JobType, JobItemsProcessed, TotalJobItems
                       FROM AsyncApexJob WHERE JobType = 'BatchApex'
                       ORDER BY CreatedDate DESC LIMIT 3]) {
    System.debug(LoggingLevel.ERROR, 'JOBROW=' + j.Status + ';' + j.JobItemsProcessed + '/' + j.TotalJobItems);
}
Map<String, System.OrgLimit> m = OrgLimits.getMap();
for (String k : new List<String>{'DailyAsyncApexExecutions',
         'DailyAsyncApexElasticExecutions', 'DailyAsyncApexProcessed'}) {
    System.debug(LoggingLevel.ERROR, 'AFTER;' + k + '=' +
        (m.containsKey(k) ? String.valueOf(m.get(k).getValue()) + '/' + String.valueOf(m.get(k).getLimit()) : 'ABSENT'));
}
