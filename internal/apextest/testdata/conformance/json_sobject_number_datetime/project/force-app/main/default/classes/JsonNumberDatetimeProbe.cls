public with sharing class JsonNumberDatetimeProbe {
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
        got.put('dt_ms_lax', obs('{"DT__c": 1730764800000}', false));
        got.put('dt_ms_strict', obs('{"DT__c": 1730764800000}', true));
        got.put('dt_ms_ms_lax', obs('{"DT__c": 1730764812345}', false));
        got.put('dt_sec_lax', obs('{"DT__c": 1730764800}', false));
        got.put('dt_zero_lax', obs('{"DT__c": 0}', false));
        got.put('dt_zero_strict', obs('{"DT__c": 0}', true));
        got.put('dt_neg_lax', obs('{"DT__c": -86400000}', false));
        got.put('dt_neg_strict', obs('{"DT__c": -86400000}', true));
        got.put('dt_neg1_lax', obs('{"DT__c": -1}', false));
        got.put('dt_frac_lax', obs('{"DT__c": 1730764800000.5}', false));
        got.put('dt_frac_strict', obs('{"DT__c": 1730764800000.5}', true));
        got.put('dt_frac15_lax', obs('{"DT__c": 1.5}', false));
        got.put('dt_one_lax', obs('{"DT__c": 1}', false));
        got.put('dt_exp_lax', obs('{"DT__c": 1.7307648E12}', false));
        got.put('dt_negms_lax', obs('{"DT__c": -1730764800000}', false));
        got.put('dt_null_lax', obs('{"DT__c": null}', false));
        return got;
    }
}
