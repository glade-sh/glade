public with sharing class JsonNumberYearProbe {
    static String err(Exception e) { return e.getTypeName() + '|' + e.getMessage(); }
    static String one(String js, Boolean isDate) {
        try {
            NumBox__c o = (NumBox__c) JSON.deserialize(js, NumBox__c.class);
            return JSON.serialize(isDate ? (Object) o.D__c : (Object) o.DT__c);
        } catch (Exception e) { return err(e); }
    }
    public static Map<String,String> observe() {
        Map<String,String> got = new Map<String,String>();
        got.put('d_0', one('{\"D__c\": 0}', true));
        got.put('dt_0', one('{\"DT__c\": 0}', false));
        got.put('d_1', one('{\"D__c\": 1}', true));
        got.put('dt_1', one('{\"DT__c\": 1}', false));
        got.put('d_-1', one('{\"D__c\": -1}', true));
        got.put('dt_-1', one('{\"DT__c\": -1}', false));
        got.put('d_2', one('{\"D__c\": 2}', true));
        got.put('dt_2', one('{\"DT__c\": 2}', false));
        got.put('d_-2', one('{\"D__c\": -2}', true));
        got.put('dt_-2', one('{\"DT__c\": -2}', false));
        got.put('d_12', one('{\"D__c\": 12}', true));
        got.put('dt_12', one('{\"DT__c\": 12}', false));
        got.put('d_1000', one('{\"D__c\": 1000}', true));
        got.put('dt_1000', one('{\"DT__c\": 1000}', false));
        got.put('d_10000', one('{\"D__c\": 10000}', true));
        got.put('dt_10000', one('{\"DT__c\": 10000}', false));
        got.put('d_100000', one('{\"D__c\": 100000}', true));
        got.put('dt_100000', one('{\"DT__c\": 100000}', false));
        got.put('d_100000000', one('{\"D__c\": 100000000}', true));
        got.put('dt_100000000', one('{\"DT__c\": 100000000}', false));
        got.put('d_268435456', one('{\"D__c\": 268435456}', true));
        got.put('dt_268435456', one('{\"DT__c\": 268435456}', false));
        got.put('d_292278993', one('{\"D__c\": 292278993}', true));
        got.put('dt_292278993', one('{\"DT__c\": 292278993}', false));
        got.put('d_292278994', one('{\"D__c\": 292278994}', true));
        got.put('dt_292278994', one('{\"DT__c\": 292278994}', false));
        got.put('d_292278995', one('{\"D__c\": 292278995}', true));
        got.put('dt_292278995', one('{\"DT__c\": 292278995}', false));
        got.put('d_-100000', one('{\"D__c\": -100000}', true));
        got.put('dt_-100000', one('{\"DT__c\": -100000}', false));
        got.put('d_-100000000', one('{\"D__c\": -100000000}', true));
        got.put('dt_-100000000', one('{\"DT__c\": -100000000}', false));
        got.put('d_-292275054', one('{\"D__c\": -292275054}', true));
        got.put('dt_-292275054', one('{\"DT__c\": -292275054}', false));
        got.put('d_-292275056', one('{\"D__c\": -292275056}', true));
        got.put('dt_-292275056', one('{\"DT__c\": -292275056}', false));
        got.put('d_500000000', one('{\"D__c\": 500000000}', true));
        got.put('dt_500000000', one('{\"DT__c\": 500000000}', false));
        got.put('d_-500000000', one('{\"D__c\": -500000000}', true));
        got.put('dt_-500000000', one('{\"DT__c\": -500000000}', false));
        got.put('d_999999999', one('{\"D__c\": 999999999}', true));
        got.put('dt_999999999', one('{\"DT__c\": 999999999}', false));
        got.put('d_1000000000', one('{\"D__c\": 1000000000}', true));
        got.put('dt_1000000000', one('{\"DT__c\": 1000000000}', false));
        got.put('d_1730764800', one('{\"D__c\": 1730764800}', true));
        got.put('dt_1730764800', one('{\"DT__c\": 1730764800}', false));
        got.put('d_2147483648', one('{\"D__c\": 2147483648}', true));
        got.put('dt_2147483648', one('{\"DT__c\": 2147483648}', false));
        got.put('d_4294967296', one('{\"D__c\": 4294967296}', true));
        got.put('dt_4294967296', one('{\"DT__c\": 4294967296}', false));
        got.put('d_100000000000', one('{\"D__c\": 100000000000}', true));
        got.put('dt_100000000000', one('{\"DT__c\": 100000000000}', false));
        got.put('d_253402300799999', one('{\"D__c\": 253402300799999}', true));
        got.put('dt_253402300799999', one('{\"DT__c\": 253402300799999}', false));
        got.put('d_253402300800000', one('{\"D__c\": 253402300800000}', true));
        got.put('dt_253402300800000', one('{\"DT__c\": 253402300800000}', false));
        got.put('d_946684800000', one('{\"D__c\": 946684800000}', true));
        got.put('dt_946684800000', one('{\"DT__c\": 946684800000}', false));
        return got;
    }
}
