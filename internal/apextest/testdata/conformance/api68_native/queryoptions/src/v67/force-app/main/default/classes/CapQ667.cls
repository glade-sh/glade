public class CapQ667 {
    public static void run() {
        String stage;
        for (Schema.PicklistEntry e : Opportunity.StageName.getDescribe().getPicklistValues()) {
            if (e.isActive()) {
                stage = e.getValue();
                break;
            }
        }
        Opportunity a = new Opportunity(Name = 'S68-HIGH', StageName = stage,
            CloseDate = Date.today(), Amount = 1000, Probability = 50);
        Opportunity b = new Opportunity(Name = 'S68-LOW', StageName = stage,
            CloseDate = Date.today(), Amount = 100, Probability = 50);
        insert new List<Opportunity>{a, b};
        try {
            List<SObject> rows = Database.queryWithBinds(
                'SELECT Name FROM Opportunity WHERE Id IN :ids'
                + ' AND FORMULA(\'Amount - ExpectedRevenue\') > 100 ORDER BY Name',
                new Map<String,Object>{'ids'=>new Set<Id>{a.Id, b.Id}},
                AccessLevel.SYSTEM_MODE);
            System.debug(LoggingLevel.ERROR, 'ROWS=' + rows.size()
                + (rows.isEmpty() ? '' : ';NAME=' + rows[0].get('Name')));
        } catch (Exception e) {
            System.debug(LoggingLevel.ERROR, 'FORMULA_ERR=' + e.getTypeName() + ': ' + e.getMessage());
        }
    }
}
