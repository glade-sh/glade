public class Cv62 {
static String d(Object o) { return o == null ? 'NULL' : String.valueOf(o); }
static String ty(Object o) { if (o == null) return 'null'; if (o instanceof String) return 'String'; if (o instanceof Integer) return 'Integer'; if (o instanceof Long) return 'Long'; if (o instanceof Double) return 'Double'; if (o instanceof Decimal) return 'Decimal'; if (o instanceof Boolean) return 'Boolean'; if (o instanceof List<Object>) return 'List'; if (o instanceof Map<String,Object>) return 'Map'; return 'Other'; }
@AuraEnabled public static String tString(String value) { return 'String:' + d(value); }
@AuraEnabled public static String tInteger(Integer value) { return 'Integer:' + d(value); }
@AuraEnabled public static String tLong(Long value) { return 'Long:' + d(value); }
@AuraEnabled public static String tDecimal(Decimal value) { return 'Decimal:' + d(value); }
@AuraEnabled public static String tDouble(Double value) { return 'Double:' + d(value); }
@AuraEnabled public static String tBoolean(Boolean value) { return 'Boolean:' + d(value); }
@AuraEnabled public static String tDate(Date value) { return 'Date:' + d(value); }
@AuraEnabled public static String tDatetime(Datetime value) { return 'Datetime:' + d(value); }
@AuraEnabled public static String tId(Id value) { return 'Id:' + d(value); }
@AuraEnabled public static String tListString(List<String> value) { return 'List<String>:' + (value==null ? 'NULL' : 'n=' + value.size() + ':' + String.join(value, '|')); }
@AuraEnabled public static String tObject(Object value) { return 'Object:' + d(value) + '#' + ty(value) + '#' + (value==null ? 'NULL' : JSON.serialize(value)); }
@AuraEnabled public static String tAccount(Account value) { return 'Account:' + d(value); }
@AuraEnabled public static String tSObject(SObject value) { return 'SObject:' + d(value); }
@AuraEnabled(cacheable=true) public static String wString(String value) { return 'String:' + d(value); }
@AuraEnabled(cacheable=true) public static String wInteger(Integer value) { return 'Integer:' + d(value); }
@AuraEnabled(cacheable=true) public static String wLong(Long value) { return 'Long:' + d(value); }
@AuraEnabled(cacheable=true) public static String wDecimal(Decimal value) { return 'Decimal:' + d(value); }
@AuraEnabled(cacheable=true) public static String wDouble(Double value) { return 'Double:' + d(value); }
@AuraEnabled(cacheable=true) public static String wBoolean(Boolean value) { return 'Boolean:' + d(value); }
@AuraEnabled(cacheable=true) public static String wDate(Date value) { return 'Date:' + d(value); }
@AuraEnabled(cacheable=true) public static String wDatetime(Datetime value) { return 'Datetime:' + d(value); }
@AuraEnabled(cacheable=true) public static String wId(Id value) { return 'Id:' + d(value); }
@AuraEnabled(cacheable=true) public static String wListString(List<String> value) { return 'List<String>:' + (value==null ? 'NULL' : 'n=' + value.size() + ':' + String.join(value, '|')); }
@AuraEnabled(cacheable=true) public static String wObject(Object value) { return 'Object:' + d(value) + '#' + ty(value) + '#' + (value==null ? 'NULL' : JSON.serialize(value)); }
@AuraEnabled(cacheable=true) public static String wAccount(Account value) { return 'Account:' + d(value); }
@AuraEnabled(cacheable=true) public static String wSObject(SObject value) { return 'SObject:' + d(value); }
}
