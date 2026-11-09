public class TfProbe {
    public static Map<String,Object> seen = new Map<String,Object>();
    public static String tag = '';
    static final List<String> FIELDS = new List<String>{'Fx__c', 'F1__c', 'F2__c', 'F3__c', 'Fq__c', 'Fw__c'};
    public static String shape(Object value) {
        if (value == null) return 'null';
        return '[' + String.valueOf(value) + ']';
    }
    public static String read(Chd__c row, String field) {
        Object value = row.get(field);
        Map<String,Object> parsed = (Map<String,Object>)JSON.deserializeUntyped(JSON.serialize(row));
        return 'get=' + shape(value) + ' json=' + (parsed.containsKey(field) ? shape(parsed.get(field)) : 'absent');
    }
    public static Map<String,Object> readAll(Chd__c row) {
        Map<String,Object> values = new Map<String,Object>();
        for (String field : FIELDS) values.put(field, read(row, field));
        return values;
    }
    public static void record(String phase, Chd__c row, Chd__c previous) {
        String prefix = tag + '|' + phase + '|';
        seen.put(prefix + 'new', readAll(row));
        if (previous != null) seen.put(prefix + 'old', readAll(previous));
        if (phase == 'beforeUpdate' && tag == 'updSetInTrigger') {
            row.A__c = 'q';
            seen.put(prefix + 'newAfterSet', readAll(row));
        }
    }
    static Chd__c requery(Id id) {
        return [SELECT Id, Name, A__c, B__c, Par__c, Fx__c, F1__c, F2__c, F3__c, Fq__c, Fw__c FROM Chd__c WHERE Id = :id];
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
        Par__c p1 = new Par__c(Name = 'p1', P__c = 'one');
        Par__c p2 = new Par__c(Name = 'p2', P__c = 'two');
        insert new List<Par__c>{p1, p2};
        Chd__c row = new Chd__c(Name = 'TfProbeRow', Par__c = p1.Id, B__c = 'b');
        tag = 'insert';
        insert row;
        result.put('soqlAfterInsert', readAll(requery(row.Id)));
        tag = 'updSetA';
        row.A__c = 'a';
        update row;
        result.put('soqlAfterSetA', readAll(requery(row.Id)));
        tag = 'updRepoint';
        row.Par__c = p2.Id;
        update row;
        result.put('soqlAfterRepoint', readAll(requery(row.Id)));
        tag = 'updParentValue';
        p2.P__c = 'TWO';
        update p2;
        row.B__c = 'bb';
        update row;
        result.put('soqlAfterParentValue', readAll(requery(row.Id)));
        tag = 'updSetInTrigger';
        row.B__c = 'b3';
        update row;
        result.put('soqlAfterSetInTrigger', readAll(requery(row.Id)));
        for (String key : seen.keySet()) result.put('trigger|' + key, seen.get(key));
        return result;
    }
}
