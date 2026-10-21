public class GovernorEdgeProbe {
    public static void observe(String shape, Integer count) {
        for (Integer i = 0; i < count; i++) {
            if (shape == 'children') {
                List<Account> rows = [SELECT Id, (SELECT Id FROM Contacts), (SELECT Id FROM Opportunities), (SELECT Id FROM Cases), (SELECT Id FROM Tasks) FROM Account WHERE Id = null];
            } else if (shape == 'queries') {
                List<Account> rows = [SELECT Id FROM Account WHERE Id = null];
            } else if (shape == 'emptyDml') {
                insert new List<Account>();
            } else if (shape == 'dml') {
                insert new Account(Name = 'Governor edge');
            }
        }
        System.debug('EDGE|' + Limits.getQueries() + '|' + Limits.getAggregateQueries() + '|' + Limits.getDmlStatements() + '|' + Limits.getDmlRows());
    }
}
