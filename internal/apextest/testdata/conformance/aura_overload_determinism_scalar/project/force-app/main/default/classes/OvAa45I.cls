public class OvAa45I {
@AuraEnabled public static String pick(Integer value) { return 'integer:' + String.valueOf(value); }
@AuraEnabled public static String pick(String value) { return 'string:' + String.valueOf(value); }
}
