public class CaptureConvert45 {
 @AuraEnabled public static String load(String first) { return 'one:' + String.valueOf(first); }
 @AuraEnabled public static String load(String first,Integer count) { return 'two:' + String.valueOf(first) + ':' + String.valueOf(count); }
 @AuraEnabled public static String control(String first) { return 'control:' + String.valueOf(first); }
}
