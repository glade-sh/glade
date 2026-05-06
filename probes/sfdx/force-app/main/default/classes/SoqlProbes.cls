public class SoqlProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'soql.select-all') {
            return runSelectAll();
        } else if (probeId == 'soql.aggregate-count') {
            return runAggregateCount();
        } else if (probeId == 'soql.where-like') {
            return runWhereLike();
        } else if (probeId == 'soql.order-desc') {
            return runOrderDesc();
        } else if (probeId == 'soql.dynamic') {
            return runDynamic();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runSelectAll() {
        List<ProbeTestObject__c> recs = [SELECT Name__c FROM ProbeTestObject__c ORDER BY Name__c LIMIT 2];
        return recs.size();
    }

    private Object runAggregateCount() {
        Integer count = [SELECT COUNT() FROM ProbeTestObject__c];
        return count;
    }

    private Object runWhereLike() {
        List<ProbeTestObject__c> recs = [SELECT Name__c FROM ProbeTestObject__c WHERE Name__c LIKE 'Record%'];
        return recs.size();
    }

    private Object runOrderDesc() {
        List<ProbeTestObject__c> recs = [SELECT Name__c FROM ProbeTestObject__c ORDER BY Value__c DESC LIMIT 1];
        if (recs.isEmpty()) {
            return null;
        }
        return recs[0].Name__c;
    }

    private Object runDynamic() {
        String q = 'SELECT Name__c FROM ProbeTestObject__c WHERE Value__c = 10 LIMIT 1';
        List<ProbeTestObject__c> recs = Database.query(q);
        if (recs.isEmpty()) {
            return null;
        }
        return recs[0].Name__c;
    }
}
