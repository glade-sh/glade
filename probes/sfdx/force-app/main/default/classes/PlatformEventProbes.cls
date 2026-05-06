public class PlatformEventProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'platform-event.publish') {
            return runPublish();
        } else if (probeId == 'platform-event.describe') {
            return runDescribe();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runPublish() {
        ProbeTestEvent__e evt = new ProbeTestEvent__e();
        evt.Message__c = 'probe-test';
        Database.SaveResult sr = EventBus.publish(evt);
        return sr.isSuccess();
    }

    private Object runDescribe() {
        Schema.DescribeSObjectResult d = ProbeTestEvent__e.SObjectType.getDescribe();
        return d.getName();
    }
}
