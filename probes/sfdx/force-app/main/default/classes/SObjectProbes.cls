public class SObjectProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'sobject.clone') {
            return runClone();
        } else if (probeId == 'sobject.get-put') {
            return runGetPut();
        } else if (probeId == 'sobject.getSObjectType') {
            return runGetSObjectType();
        } else if (probeId == 'id.validation') {
            return runIdValidation();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runClone() {
        ProbeTestObject__c rec = new ProbeTestObject__c(Name__c='CloneTest', Value__c=42, CurrencyIsoCode='USD');
        ProbeTestObject__c cloned = rec.clone();
        return cloned.Name__c == 'CloneTest';
    }

    private Object runGetPut() {
        ProbeTestObject__c rec = new ProbeTestObject__c();
        rec.put('Name__c', 'PutTest');
        return rec.get('Name__c') == 'PutTest';
    }

    private Object runGetSObjectType() {
        ProbeTestObject__c rec = new ProbeTestObject__c();
        return String.valueOf(rec.getSObjectType());
    }

    private Object runIdValidation() {
        try {
            Id validId = '001000000000001AAA';
            Id invalidId = 'bad-id';
            return validId != null;
        } catch (Exception e) {
            return false;
        }
    }
}
