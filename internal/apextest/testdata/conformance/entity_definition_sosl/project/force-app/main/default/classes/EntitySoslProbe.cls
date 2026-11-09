public class EntitySoslProbe {
    static Map<String, Object> shape(String term) {
        Map<String, Object> o = new Map<String, Object>();
        try {
            List<List<SObject>> r = Search.query('FIND \'' + term + '\' IN ALL FIELDS RETURNING EntityDefinition (Label, DeveloperName, QualifiedApiName, NamespacePrefix) LIMIT 15');
            o.put('lists', r.size());
            o.put('rows', r[0].size());
            List<String> names = new List<String>();
            for (SObject s : r[0]) { names.add((String) s.get('QualifiedApiName')); }
            names.sort();
            o.put('names', names);
        } catch (Exception e) {
            o.put('exception', e.getTypeName() + '|' + e.getMessage());
        }
        return o;
    }
    public static Map<String, Object> observe() {
        Map<String, Object> o = new Map<String, Object>();
        o.put('account', shape('account'));
        o.put('accGlob', shape('acc*'));
        o.put('accountDeveloperNameExact', shape('Account'));
        o.put('contactLabel', shape('contact'));
        for (String t : new List<String>{'Work', 'access', 'Feed', 'Account*', '*count', 'Account Feed', 'AccountFeed', 'accountfeed', 'Acc', 'Cont*', 'probe', 'Probe__c', 'pro*', 'Probe Item', 'Quote*'}) { o.put('t:' + t, shape(t)); }
        return o;
    }
}
