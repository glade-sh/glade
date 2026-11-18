public class CapPeSchema67 {
    public static void run() {
        List<String> names = new List<String>{
            'AgentQltyEvalEvent','ObjectMilestoneCreatedEvent','ObjectMilestoneCreatedDtlEvent',
            'UniversalAnomalyEvent','WebCartAbandonedEvent',
            'QuoteDocumentCreationEvent','FlowOrchestrationWorkItemEvent'};
        Map<String, Schema.SObjectType> gd = Schema.getGlobalDescribe();
        for (String n : names) {
            if (!gd.containsKey(n)) {
                System.debug(LoggingLevel.ERROR, n + '|ABSENT');
                continue;
            }
            Schema.DescribeSObjectResult d = gd.get(n).getDescribe();
            System.debug(LoggingLevel.ERROR, n + '|CREATE=' + d.isCreateable()
                + '|QUERY=' + d.isQueryable() + '|FIELDS=' + d.fields.getMap().size());
            for (String k : d.fields.getMap().keySet()) {
                Schema.DescribeFieldResult f = d.fields.getMap().get(k).getDescribe();
                if (f.isCustom() || k == 'Id' || k == 'Name' || k == 'EventUuid'
                    || k == 'ReplayId' || k == 'CreatedDate' || k == 'CreatedById') {
                    System.debug(LoggingLevel.ERROR, n + '.' + f.getName() + '|' + f.getType()
                        + '|NULL=' + f.isNillable() + '|CREATE=' + f.isCreateable());
                }
            }
        }
    }
}
