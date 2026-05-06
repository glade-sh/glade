public class MetadataProbes implements Probe {
    public Object run(String probeId) {
        if (probeId == 'metadata.custom-metadata-query') {
            return runCustomMetadataQuery();
        } else if (probeId == 'metadata.custom-setting-query') {
            return runCustomSettingQuery();
        } else if (probeId == 'metadata.custom-metadata-describe') {
            return runCustomMetadataDescribe();
        }
        throw new IllegalArgumentException('Unknown probe: ' + probeId);
    }

    private Object runCustomMetadataQuery() {
        List<ProbeTestMdt__mdt> recs = [SELECT Value__c FROM ProbeTestMdt__mdt LIMIT 1];
        return recs.size();
    }

    private Object runCustomSettingQuery() {
        List<ProbeTestSetting__c> recs = [SELECT Value__c FROM ProbeTestSetting__c LIMIT 1];
        return recs.size();
    }

    private Object runCustomMetadataDescribe() {
        Schema.DescribeSObjectResult d = ProbeTestMdt__mdt.SObjectType.getDescribe();
        return d.isCustom();
    }
}
