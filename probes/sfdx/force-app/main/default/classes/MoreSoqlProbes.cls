public class MoreSoqlProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'soql.count-distinct') {
            return runCountDistinct();
        } else if (probeId == 'soql.group-by') {
            return runGroupBy();
        } else if (probeId == 'soql.subquery') {
            return runSubquery();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runCountDistinct() {
        List<AggregateResult> results = [SELECT COUNT_DISTINCT(Name__c) dist FROM ProbeTestObject__c WHERE Name__c LIKE 'Record%'];
        return (Integer)results[0].get('dist');
    }

    private Object runGroupBy() {
        List<AggregateResult> results = [SELECT Name__c, COUNT(Id) cnt FROM ProbeTestObject__c WHERE Name__c LIKE 'Record%' GROUP BY Name__c];
        return results.size();
    }

    private Object runSubquery() {
        // Account subquery requires Contact child relationship
        List<Account> accounts = [SELECT Name, (SELECT Id FROM Contacts) FROM Account LIMIT 1];
        return accounts != null;
    }
}
