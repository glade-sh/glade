public class UnsavedMapProbe {
    public static Map<String, Object> run() {
        Map<String, Object> observed = new Map<String, Object>();
        Contact first = new Contact(LastName = 'First');
        Contact second = new Contact(LastName = 'Second');
        Contact identified = new Contact(Id = '003000000000001AAA', LastName = 'Identified');
        List<SObject> rows = new List<SObject>{first};
        try {
            Map<Id, SObject> constructed = new Map<Id, SObject>(rows);
            observed.put('constructorRejected', false);
        } catch (Exception ex) {
            observed.put('constructorRejected', true);
        }
        try {
            Map<Id, SObject> target = new Map<Id, SObject>();
            target.putAll(rows);
            observed.put('single', 'accepted');
            describe(target, 'single', observed);
        } catch (Exception ex) {
            observed.put('single', ex.getTypeName());
            observed.put('singleMessage', ex.getMessage());
        }
        try {
            Map<Id, SObject> target = new Map<Id, SObject>();
            target.putAll(new List<SObject>{first, second});
            observed.put('duplicate', 'accepted');
            describe(target, 'duplicate', observed);
        } catch (Exception ex) {
            observed.put('duplicate', ex.getTypeName());
            observed.put('duplicateMessage', ex.getMessage());
        }
        try {
            Map<Id, SObject> target = new Map<Id, SObject>();
            target.putAll(new List<SObject>{identified, first});
            observed.put('mixed', 'accepted');
            observed.put('mixedIdentified', target.containsKey(identified.Id));
            describe(target, 'mixed', observed);
        } catch (Exception ex) {
            observed.put('mixed', ex.getTypeName());
            observed.put('mixedMessage', ex.getMessage());
        }
        Map<Id, SObject> direct = new Map<Id, SObject>();
        direct.put(null, first);
        describe(direct, 'direct', observed);
        return observed;
    }

    private static void describe(Map<Id, SObject> target, String prefix, Map<String, Object> observed) {
        observed.put(prefix + 'Size', target.size());
        observed.put(prefix + 'ContainsNull', target.containsKey(null));
        observed.put(prefix + 'KeysString', String.valueOf(target.keySet()));
        List<String> values = new List<String>();
        for (SObject row : target.values()) {
            values.add((String)row.get('LastName'));
        }
        values.sort();
        observed.put(prefix + 'Values', values);
        List<String> keys = new List<String>();
        for (Id key : target.keySet()) {
            SObject row = target.get(key);
            keys.add(String.valueOf(key) + '|' + String.valueOf(key == null) + '|' + (row == null ? 'missing' : (String)row.get('LastName')));
        }
        keys.sort();
        observed.put(prefix + 'Keys', keys);
        try {
            observed.put(prefix + 'Json', JSON.serialize(target));
        } catch (Exception ex) {
            observed.put(prefix + 'Json', ex.getTypeName() + '|' + ex.getMessage());
        }
    }
}
