public class RelationshipWriteProbe {
    public static String phase;
    public static Map<String, Object> observed = new Map<String, Object>();

    public static Map<String, Object> run() {
        observed = new Map<String, Object>();
        Contact direct = new Contact(LastName = 'Direct');
        attempt(direct, 'direct');
        phase = 'before';
        insert new Contact(LastName = 'Before');
        phase = 'after';
        try {
            insert new Contact(LastName = 'After');
        } catch (DmlException ex) {
            observed.put('after', 'dml');
            observed.put('afterType', ex.getTypeName());
            observed.put('afterReadOnly', ex.getMessage().contains('System.FinalException: Record is read-only'));
        }
        phase = null;
        return observed;
    }

    public static void attempt(Contact record, String key) {
        try {
            record.putSObject('Account', new Account(Name = 'Related'));
            observed.put(key, 'accepted');
            observed.put(key + 'Name', record.getSObject('Account').get('Name'));
        } catch (SObjectException ex) {
            observed.put(key, 'sobject');
            observed.put(key + 'Type', ex.getTypeName());
            observed.put(key + 'Message', ex.getMessage());
        } catch (Exception ex) {
            observed.put(key, 'other');
            observed.put(key + 'Type', ex.getTypeName());
            observed.put(key + 'Message', ex.getMessage());
        }
    }
}
