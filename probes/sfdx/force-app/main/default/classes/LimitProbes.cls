public class LimitProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'limits.soql-before-after') {
            return runSoqlBeforeAfter();
        } else if (probeId == 'limits.dml-rows') {
            return runDmlRows();
        } else if (probeId == 'limits.heap-size') {
            return runHeapSize();
        } else if (probeId == 'limits.limit-queries') {
            return runLimitQueries();
        } else if (probeId == 'limits.cpu-time') {
            return runCpuTime();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runSoqlBeforeAfter() {
        Integer before = Limits.getQueries();
        List<ProbeTestObject__c> recs = [SELECT Id FROM ProbeTestObject__c LIMIT 1];
        Integer after = Limits.getQueries();
        return after - before;
    }

    private Object runDmlRows() {
        Integer before = Limits.getDmlRows();
        List<ProbeTestObject__c> recs = new List<ProbeTestObject__c>();
        for (Integer i = 0; i < 3; i++) {
            ProbeTestObject__c r = new ProbeTestObject__c();
            r.Name__c = 'Limit' + i;
            r.CurrencyIsoCode = 'USD';
            recs.add(r);
        }
        insert recs;
        Integer after = Limits.getDmlRows();
        return after - before;
    }

    private Object runHeapSize() {
        Integer before = Limits.getHeapSize();
        List<String> values = new List<String>();
        values.add('abcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyzabcdefghijklmnopqrstuvwxyz');
        Integer after = Limits.getHeapSize();
        return after > before;
    }

    private Object runLimitQueries() {
        return Limits.getLimitQueries();
    }

    private Object runCpuTime() {
        Integer t = Limits.getCpuTime();
        return t >= 0;
    }
}
