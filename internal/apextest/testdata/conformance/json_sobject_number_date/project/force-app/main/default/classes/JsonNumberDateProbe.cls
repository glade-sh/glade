public with sharing class JsonNumberDateProbe {
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
        got.put('d_ms_lax', obs('{"D__c": 1730764800000}', false));
        got.put('d_ms_strict', obs('{"D__c": 1730764800000}', true));
        got.put('d_ms_midday_lax', obs('{"D__c": 1730808000000}', false));
        got.put('d_ms_late_lax', obs('{"D__c": 1730851199999}', false));
        got.put('d_sec_lax', obs('{"D__c": 1730764800}', false));
        got.put('d_zero_lax', obs('{"D__c": 0}', false));
        got.put('d_zero_strict', obs('{"D__c": 0}', true));
        got.put('d_neg_lax', obs('{"D__c": -86400000}', false));
        got.put('d_neg_strict', obs('{"D__c": -86400000}', true));
        got.put('d_neg1_lax', obs('{"D__c": -1}', false));
        got.put('d_frac_lax', obs('{"D__c": 1730764800000.5}', false));
        got.put('d_frac_strict', obs('{"D__c": 1730764800000.5}', true));
        got.put('d_frac_small_lax', obs('{"D__c": 1.5}', false));
        got.put('d_one_lax', obs('{"D__c": 1}', false));
        got.put('d_exp_lax', obs('{"D__c": 1.7307648E12}', false));
        got.put('d_negms_lax', obs('{"D__c": -1730764800000}', false));
        got.put('d_null_lax', obs('{"D__c": null}', false));
        return got;
    }
}
