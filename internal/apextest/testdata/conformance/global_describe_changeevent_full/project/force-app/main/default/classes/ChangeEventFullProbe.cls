public class ChangeEventFullProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        Map<String, Schema.SObjectType> g = Schema.getGlobalDescribe();
        List<String> keys = new List<String>();
        for (String k : g.keySet()) {
            if (k.endsWith('changeevent')) { Schema.DescribeSObjectResult d = g.get(k).getDescribe(); keys.add(k + '~' + d.getName() + '~' + d.getLabel()); }
        }
        keys.sort();
        m.put('count', keys.size());
        m.put('keys', keys);
        List<String> mine = new List<String>();
        for (String n : new List<String>{ 'v35full__c', 'v35fulllist__c', 'v35fullhier__c', 'v35fulltype__mdt' }) {
            mine.add(n + ':' + g.containsKey(n) + ':' + g.containsKey(n.substringBefore('__') + '__changeevent'));
        }
        m.put('own', mine);
        return m;
    }
    // The named test route carries at most about 4000 characters, so it reports the
    // count, the own keys and a digest of the full sorted key~name list.
    public static Map<String,Object> observeCompact() {
        Map<String,Object> full = observe();
        List<String> keys = (List<String>) full.get('keys');
        Map<String,Object> m = new Map<String,Object>();
        m.put('count', full.get('count'));
        m.put('own', full.get('own'));
        m.put('first', keys.get(0));
        m.put('last', keys.get(keys.size() - 1));
        m.put('sha256', EncodingUtil.convertToHex(Crypto.generateDigest('SHA-256', Blob.valueOf(String.join(keys, ',')))));
        return m;
    }
}
