@IsTest public class RtInst62Test {
    @IsTest static void observe() {
        Map<String,Object> m = new Map<String,Object>();
        Schema.DescribeSObjectResult d = Schema.SObjectType.gladens004__V035Env__c;
        Map<String,Schema.RecordTypeInfo> byDev = d.getRecordTypeInfosByDeveloperName();
        List<String> keys = new List<String>(byDev.keySet()); keys.sort();
        m.put('keys', keys);
        for (String n : new List<String>{'Cash', 'Check', 'Extra', 'Master'}) {
          Schema.RecordTypeInfo i = byDev.get(n);
          m.put(n, i == null ? 'null' : 'avail=' + i.isAvailable() + ':default=' + i.isDefaultRecordTypeMapping() + ':active=' + i.isActive());
        }
        System.assert(false, 'OBS|' + JSON.serialize(m));
    }
}
