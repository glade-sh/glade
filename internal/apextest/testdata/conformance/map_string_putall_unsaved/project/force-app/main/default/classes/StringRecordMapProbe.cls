public class StringRecordMapProbe {
    public static Map<String, Object> run() {
        Map<String, Object> observed = new Map<String, Object>();
        Account first = new Account(Name = 'First');
        Account second = new Account(Name = 'Second');
        Account stored = new Account(Name = 'Stored');
        insert stored;
        putRows('single', new List<SObject>{first}, stored.Id, observed);
        putRows('duplicate', new List<SObject>{first, second}, stored.Id, observed);
        putRows('mixed', new List<SObject>{stored, first}, stored.Id, observed);
        try {
            Map<String, SObject> constructed = new Map<String, SObject>(new List<SObject>{first});
            observed.put('constructorRejected', false);
            observed.put('constructorSize', constructed.size());
        } catch (Exception ex) {
            observed.put('constructorRejected', true);
        }
        try {
            Map<String, SObject> constructed = new Map<String, SObject>(new List<SObject>{stored});
            observed.put('savedConstructor', 'accepted');
            observed.put('savedConstructorSize', constructed.size());
            observed.put('savedConstructorId', constructed.containsKey(String.valueOf(stored.Id)));
        } catch (Exception ex) {
            observed.put('savedConstructor', ex.getTypeName());
        }
        ContactPointEmail channel = new ContactPointEmail(
            EmailDomain = 'Owned domain',
            BestTimeToContactEndTime = Time.newInstance(11, 12, 13, 14)
        );
        putRows('time', new List<SObject>{channel}, stored.Id, observed);
        Map<Id, SObject> idControl = new Map<Id, SObject>();
        try {
            idControl.putAll(new List<SObject>{first});
            observed.put('idControl', 'accepted');
            observed.put('idControlSize', idControl.size());
            observed.put('idControlLiteralNull', idControl.containsKey(null));
            List<String> keys = new List<String>();
            for (Id key : idControl.keySet()) {
                SObject row = idControl.get(key);
                keys.add(String.valueOf(key == null) + '|' + label(row));
            }
            keys.sort();
            observed.put('idControlKeys', keys);
        } catch (Exception ex) {
            observed.put('idControl', ex.getTypeName());
        }
        Map<String, SObject> literal = new Map<String, SObject>();
        literal.put(null, first);
        observed.put('literalNull', literal.containsKey(null));
        observed.put('literalValue', label(literal.get(null)));
        return observed;
    }

    private static void putRows(String prefix, List<SObject> rows, Id savedId, Map<String, Object> observed) {
        Map<String, SObject> target = new Map<String, SObject>();
        try {
            target.putAll(rows);
            observed.put(prefix + 'Outcome', 'accepted');
        } catch (Exception ex) {
            observed.put(prefix + 'Outcome', ex.getTypeName());
        }
        observed.put(prefix + 'Size', target.size());
        observed.put(prefix + 'LiteralNull', target.containsKey(null));
        observed.put(prefix + 'SavedId', target.containsKey(String.valueOf(savedId)));
        List<String> values = new List<String>();
        for (SObject row : target.values()) {
            values.add(label(row));
        }
        values.sort();
        observed.put(prefix + 'Values', values);
        List<String> keys = new List<String>();
        for (String key : target.keySet()) {
            String keyKind = key == null ? 'null' : key == String.valueOf(savedId) ? 'saved-id' : 'other';
            keys.add(keyKind + '|' + label(target.get(key)));
        }
        keys.sort();
        observed.put(prefix + 'Keys', keys);
    }

    private static String label(SObject row) {
        if (row == null) {
            return 'missing';
        }
        if (row.getSObjectType() == Account.SObjectType) {
            return (String)row.get('Name');
        }
        if (row.getSObjectType() == ContactPointEmail.SObjectType) {
            return (String)row.get('EmailDomain') + '|' + String.valueOf(row.get('BestTimeToContactEndTime') instanceof Time);
        }
        return 'unexpected-type';
    }
}
