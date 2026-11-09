public class EmptyTriggerProbe {
    public static Map<String,Object> seen = new Map<String,Object>();
    public static String shape(Object value) {
        if (value == null) return 'null';
        return '[' + String.valueOf(value) + ']';
    }
    public static void record(String phase, SObject row) {
        Map<String,Object> values = new Map<String,Object>();
        for (String field : new List<String>{'Description', 'BillingStreet', 'Phone', 'Website', 'Type', 'AccountNumber', 'Industry', 'Name'}) {
            values.put(field, shape(row.get(field)));
        }
        seen.put(phase, values);
    }
    public static Map<String,Object> observe() {
        seen = new Map<String,Object>();
        Account first = new Account(Name = 'EmptyTrigger Probe', Description = '', BillingStreet = '', Phone = '', Website = '', Type = '', AccountNumber = '', Industry = '');
        insert as system first;
        Map<String,Object> result = new Map<String,Object>();
        result.put('memoryAfterInsertDescription', shape(first.Description));
        result.put('memoryAfterInsertType', shape(first.Type));
        result.put('memoryAfterInsertPhone', shape(first.Phone));
        first.Description = 'D';
        first.Type = 'Customer';
        first.Phone = '1';
        update as system first;
        first.Description = '';
        first.Type = '';
        first.Phone = '';
        update as system first;
        result.put('memoryAfterUpdateDescription', shape(first.Description));
        result.put('triggerPhases', seen);
        return result;
    }
}
