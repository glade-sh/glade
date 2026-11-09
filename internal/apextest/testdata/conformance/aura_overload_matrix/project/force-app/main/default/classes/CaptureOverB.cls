public class CaptureOverB {
@AuraEnabled public static String arity(String first, String second, String third) { return 'three:' + String.valueOf(first) + ':' + String.valueOf(second) + ':' + String.valueOf(third); }
@AuraEnabled public static String arity(String first, String second) { return 'two:' + String.valueOf(first) + ':' + String.valueOf(second); }
@AuraEnabled public static String arity(String first) { return 'one:' + String.valueOf(first); }
@AuraEnabled public static String arity() { return 'zero'; }
@AuraEnabled public static String scalar(Integer value) { return 'integer:' + String.valueOf(value); }
@AuraEnabled public static String scalar(String value) { return 'string:' + String.valueOf(value); }
@AuraEnabled public static String names(Integer blue) { return 'blue:' + String.valueOf(blue); }
@AuraEnabled public static String names(String red) { return 'red:' + String.valueOf(red); }
@AuraEnabled public static String overlap(String red, Boolean green) { return 'green:' + String.valueOf(red) + ':' + String.valueOf(green); }
@AuraEnabled public static String overlap(String red, Integer blue) { return 'blue:' + String.valueOf(red) + ':' + String.valueOf(blue); }
@AuraEnabled public static String control(String item) { return 'control:' + String.valueOf(item); }
}
