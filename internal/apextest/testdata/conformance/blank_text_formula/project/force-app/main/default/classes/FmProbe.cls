public class FmProbe {
    public static Map<String,Object> seen = new Map<String,Object>();
    public static String tag = '';
    static final List<String> FIELDS = new List<String>{'Fb__c', 'Fs__c', 'Fc__c'};
    public static String shape(Object value) {
        if (value == null) return 'null';
        return '[' + String.valueOf(value) + ']';
    }
    static Object staticValue(Fm__c row, String field) {
        if (field == 'Fb__c') return row.Fb__c;
        if (field == 'Fs__c') return row.Fs__c;
        return row.Fc__c;
    }
    public static String read(Fm__c row, String field) {
        Object value = row.get(field);
        Object direct = staticValue(row, field);
        String text = 'getNull=' + (value == null) + ' staticNull=' + (direct == null);
        text += ' valueOf=' + shape(String.valueOf(value));
        text += ' concat=' + shape('x' + value);
        Map<String,Object> parsed = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(row));
        text += ' json=' + (parsed.containsKey(field) ? shape(parsed.get(field)) : 'absent');
        return text;
    }
    public static Map<String,Object> readAll(Fm__c row) {
        Map<String,Object> values = new Map<String,Object>();
        for (String field : FIELDS) values.put(field, read(row, field));
        return values;
    }
    public static void record(String phase, Fm__c row, Fm__c previous) {
        String prefix = tag + '|' + phase + '|';
        seen.put(prefix + 'new', readAll(row));
        if (previous != null) {
            seen.put(prefix + 'old', readAll(previous));
            Map<String,Object> changed = new Map<String,Object>();
            for (String field : FIELDS) {
                changed.put(field, 'getNe=' + (row.get(field) != previous.get(field)) + ' staticNe=' + (staticValue(row, field) != staticValue(previous, field)));
            }
            seen.put(prefix + 'ne', changed);
        }
    }
    static Fm__c requery(Id id) {
        return [SELECT Id, Name, A__c, B__c, C__c, N__c, M__c, Fb__c, Fs__c, Fc__c FROM Fm__c WHERE Id = :id];
    }
    public static Map<String,Object> chunk(Integer index, Integer count) {
        Map<String,Object> all = observe();
        List<String> keys = new List<String>(all.keySet());
        keys.sort();
        Map<String,Object> part = new Map<String,Object>();
        for (Integer i = 0; i < keys.size(); i++) if (Math.mod(i, count) == index) part.put(keys[i], all.get(keys[i]));
        return part;
    }
    public static Map<String,Object> observe() {
        seen = new Map<String,Object>();
        Map<String,Object> result = new Map<String,Object>();
        Fm__c row = new Fm__c(Name = 'FmProbeRow');
        result.put('memoryNew', readAll(row));
        result.put('memoryNewWithInput', readAll(new Fm__c(Name = 'FmProbeRow', A__c = 'a')));
        tag = 'insert';
        insert row;
        result.put('memoryAfterInsert', readAll(row));
        result.put('soqlAfterInsert', readAll(requery(row.Id)));
        tag = 'updUnrelated';
        row.M__c = 'm1';
        update row;
        result.put('memoryAfterUnrelated', readAll(row));
        result.put('soqlAfterUnrelated', readAll(requery(row.Id)));
        tag = 'updFlipToValue';
        row.A__c = 'a';
        update row;
        result.put('memoryAfterFlipToValue', readAll(row));
        result.put('soqlAfterFlipToValue', readAll(requery(row.Id)));
        tag = 'updFlipToBlank';
        row.A__c = '';
        update row;
        result.put('memoryAfterFlipToBlank', readAll(row));
        result.put('soqlAfterFlipToBlank', readAll(requery(row.Id)));
        tag = 'updUnrelatedBlank';
        row.M__c = 'm2';
        update row;
        result.put('soqlAfterUnrelatedBlank', readAll(requery(row.Id)));
        for (String key : seen.keySet()) result.put('trigger|' + key, seen.get(key));
        return result;
    }
}
