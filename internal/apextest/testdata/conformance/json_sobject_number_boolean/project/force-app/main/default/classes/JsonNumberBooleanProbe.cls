public with sharing class JsonNumberBooleanProbe {
    static String err(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static String s(Object v) { return v == null ? '-' : JSON.serialize(v); }
    static String obs(String js, Boolean strict) {
        try {
            NumBox__c o = strict ? (NumBox__c) JSON.deserializeStrict(js, NumBox__c.class) : (NumBox__c) JSON.deserialize(js, NumBox__c.class);
            return 'ok|' + s(o.D__c) + '|' + s(o.DT__c) + '|' + s(o.B__c);
        } catch (Exception e) { return err(e); }
    }
    public static Map<String,String> observe() {
        Map<String,String> got = new Map<String,String>();
        got.put('tz', UserInfo.getTimeZone().getID());
        got.put('all_ms_lax', obs('{"D__c": 1730764800000, "DT__c": 1730764800000, "B__c": 1}', false));
        got.put('all_ms_strict', obs('{"D__c": 1730764800000, "DT__c": 1730764800000, "B__c": 1}', true));
        got.put('b_1_lax', obs('{"B__c": 1}', false));
        got.put('b_1_strict', obs('{"B__c": 1}', true));
        got.put('b_0_lax', obs('{"B__c": 0}', false));
        got.put('b_0_strict', obs('{"B__c": 0}', true));
        got.put('b_2_lax', obs('{"B__c": 2}', false));
        got.put('b_2_strict', obs('{"B__c": 2}', true));
        got.put('b_neg_lax', obs('{"B__c": -1}', false));
        got.put('b_frac_lax', obs('{"B__c": 1.5}', false));
        got.put('b_frac_strict', obs('{"B__c": 1.5}', true));
        got.put('b_fraczero_lax', obs('{"B__c": 0.0}', false));
        got.put('b_small_lax', obs('{"B__c": 0.1}', false));
        got.put('b_str_true_lax', obs('{"B__c": "true"}', false));
        got.put('b_str_1_lax', obs('{"B__c": "1"}', false));
        got.put('b_null_lax', obs('{"B__c": null}', false));
        got.put('mixed_zero_lax', obs('{"D__c": 0, "DT__c": 0, "B__c": 0}', false));
        got.put('mixed_zero_strict', obs('{"D__c": 0, "DT__c": 0, "B__c": 0}', true));
        got.put('mixed_neg_lax', obs('{"D__c": -1730764800000, "DT__c": -1730764800000, "B__c": -1}', false));
        got.put('mixed_frac_lax', obs('{"D__c": 1730764800000.7, "DT__c": 1730764800000.7, "B__c": 0.5}', false));
        got.put('mixed_frac_strict', obs('{"D__c": 1730764800000.7, "DT__c": 1730764800000.7, "B__c": 0.5}', true));
        return got;
    }
}
