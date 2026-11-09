public class SubscriberDescribeProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
        List<String> keys = new List<String>();
        for (String k : g.keySet()) { if (k.contains('v035') && !k.endsWith('changeevent')) { keys.add(k); } }
        keys.sort();
        m.put('v035Keys', keys);
        m.put('containsFull', g.containsKey('gladens004__v035env__c'));
        m.put('containsBare', g.containsKey('v035env__c'));
        m.put('containsBareMixed', g.containsKey('V035Env__c'));
        m.put('getBareNull', g.get('v035env__c') == null);
        m.put('getFullName', g.get('gladens004__V035Env__c') == null ? 'null' : g.get('gladens004__V035Env__c').getDescribe().getName());
        Integer mismatches = 0;
        Set<String> names = new Set<String>();
        for (String k : g.keySet()) {
            String nm = g.get(k).getDescribe().getName().toLowerCase();
            names.add(nm);
            if (nm != k) { mismatches++; }
        }
        m.put('keyNameMismatches', mismatches);
        m.put('sizeEqualsRekeyedSize', g.keySet().size() == names.size());
        m.put('sizeEqualsMapSize', g.size() == g.keySet().size());
        m.put('valuesSizeEqualsKeys', g.values().size() == g.keySet().size());
        m.put('namespaceOfKeys', new List<Object>{ g.get('gladens004__v035env__c') != null });
        return m;
    }
}
