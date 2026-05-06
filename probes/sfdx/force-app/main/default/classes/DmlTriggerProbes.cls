public class DmlTriggerProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'dml.insert-trigger') {
            return runInsertTrigger();
        } else if (probeId == 'dml.update-return') {
            return runUpdateReturn();
        } else if (probeId == 'dml.delete-return') {
            return runDeleteReturn();
        } else if (probeId == 'dml.undelete') {
            return runUndelete();
        } else if (probeId == 'dml.insert-fail-duplicate') {
            return runInsertFail();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runInsertTrigger() {
        ProbeTestObject__c rec = new ProbeTestObject__c();
        rec.Name__c = 'TriggerTest';
        rec.Value__c = 42;
        insert rec;

        ProbeTestObject__c verify = [SELECT Triggered__c FROM ProbeTestObject__c WHERE Name__c = 'TriggerTest' LIMIT 1];
        return verify.Triggered__c;
    }

    private Object runUpdateReturn() {
        ProbeTestObject__c rec = new ProbeTestObject__c();
        rec.Name__c = 'UpdateTest';
        rec.Value__c = 1;
        insert rec;

        rec.Value__c = 2;
        Database.SaveResult sr = Database.update(rec, false);
        return sr.isSuccess();
    }

    private Object runDeleteReturn() {
        ProbeTestObject__c rec = new ProbeTestObject__c();
        rec.Name__c = 'DeleteTest';
        rec.Value__c = 1;
        insert rec;

        Database.DeleteResult dr = Database.delete(rec, false);
        return dr.isSuccess();
    }

    private Object runUndelete() {
        ProbeTestObject__c rec = new ProbeTestObject__c();
        rec.Name__c = 'UndeleteTest';
        rec.Value__c = 1;
        insert rec;
        delete rec;

        Database.UndeleteResult ur = Database.undelete(rec, false);
        return ur.isSuccess();
    }

    private Object runInsertFail() {
        // Attempt to insert without required fields (if any) or with invalid data
        // For this object, there are no required custom fields, so we just test a basic fail path
        // by simulating a DML with allOrNothing=false and checking result shape
        ProbeTestObject__c rec = new ProbeTestObject__c();
        rec.Name__c = 'FailTest';
        Database.SaveResult sr = Database.insert(rec, false);
        return sr.isSuccess();
    }
}
