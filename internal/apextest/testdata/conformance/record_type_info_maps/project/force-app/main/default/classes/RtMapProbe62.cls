public class RtMapProbe62 {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Schema.DescribeSObjectResult d = Schema.SObjectType.V35Rt__c;
        Schema.DescribeSObjectResult dg = Schema.getGlobalDescribe().get('V35Rt__c').getDescribe();
        m.put('user.profile', [SELECT Profile.Name FROM User WHERE Id = :UserInfo.getUserId()].Profile.Name);
        Map<String,Schema.RecordTypeInfo> byName = d.getRecordTypeInfosByName();
        Map<String,Schema.RecordTypeInfo> byDev = d.getRecordTypeInfosByDeveloperName();
        Map<Id,Schema.RecordTypeInfo> byId = d.getRecordTypeInfosById();
        List<Schema.RecordTypeInfo> infos = d.getRecordTypeInfos();
        List<String> k1 = new List<String>(byName.keySet()); k1.sort();
        List<String> k2 = new List<String>(byDev.keySet()); k2.sort();
        List<String> k3 = new List<String>(dg.getRecordTypeInfosByName().keySet()); k3.sort();
        List<String> k4 = new List<String>(dg.getRecordTypeInfosByDeveloperName().keySet()); k4.sort();
        m.put('byName.keys', k1); m.put('byDev.keys', k2); m.put('global.byName.keys', k3); m.put('global.byDev.keys', k4);
        m.put('byId.size', byId.size()); m.put('infos.size', infos.size());
        Map<String,Schema.RecordTypeInfo> probes = new Map<String,Schema.RecordTypeInfo>();
        for (String n : new List<String>{'Alpha', 'Beta', 'Gamma', 'Master', 'alpha', 'Missing'}) {
          probes.put('byName.get.' + n, byName.get(n));
          probes.put('byDev.get.' + n, byDev.get(n));
        }
        for (Schema.RecordTypeInfo i : infos) {
          probes.put('list.' + i.getDeveloperName(), i);
          probes.put('byId.get.' + i.getDeveloperName(), byId.get(i.getRecordTypeId()));
        }
        for (String k : probes.keySet()) {
          Schema.RecordTypeInfo i = probes.get(k);
          m.put(k, i == null ? 'null' : i.getDeveloperName() + ':' + i.getName() + ':avail=' + i.isAvailable() + ':default=' + i.isDefaultRecordTypeMapping() + ':master=' + i.isMaster() + ':active=' + i.isActive() + ':idset=' + (i.getRecordTypeId() != null));
        }
        return m;
    }
}
