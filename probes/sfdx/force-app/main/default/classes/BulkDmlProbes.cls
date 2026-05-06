public class BulkDmlProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'bulkdml.partial-success') {
            return runPartialSuccess();
        } else if (probeId == 'bulkdml.errors-shape') {
            return runErrorsShape();
        } else if (probeId == 'bulkdml.id-assignment') {
            return runIdAssignment();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runPartialSuccess() {
        List<ProbeTestObject__c> recs = new List<ProbeTestObject__c>{
            new ProbeTestObject__c(Name__c='Bulk1', Value__c=1, CurrencyIsoCode='USD'),
            new ProbeTestObject__c(Name__c='Bulk1', Value__c=2, CurrencyIsoCode='USD') // same name to test duplicate behavior if any
        };
        Database.SaveResult[] results = Database.insert(recs, false);
        Integer successCount = 0;
        for (Database.SaveResult sr : results) {
            if (sr.isSuccess()) successCount++;
        }
        return successCount >= 1;
    }

    private Object runErrorsShape() {
        List<ProbeTestObject__c> recs = new List<ProbeTestObject__c>{
            new ProbeTestObject__c(Name__c='BulkErr1', Value__c=1, CurrencyIsoCode='USD')
        };
        Database.SaveResult[] results = Database.insert(recs, false);
        Database.SaveResult sr = results[0];
        if (sr.isSuccess()) {
            return sr.getId() != null;
        }
        return sr.getErrors().size() > 0;
    }

    private Object runIdAssignment() {
        List<ProbeTestObject__c> recs = new List<ProbeTestObject__c>{
            new ProbeTestObject__c(Name__c='BulkId1', Value__c=1, CurrencyIsoCode='USD'),
            new ProbeTestObject__c(Name__c='BulkId2', Value__c=2, CurrencyIsoCode='USD')
        };
        Database.SaveResult[] results = Database.insert(recs, false);
        Integer idCount = 0;
        for (Database.SaveResult sr : results) {
            if (sr.isSuccess() && sr.getId() != null) idCount++;
        }
        return idCount;
    }
}
