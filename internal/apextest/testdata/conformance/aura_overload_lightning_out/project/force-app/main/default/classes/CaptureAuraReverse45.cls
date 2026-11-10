public class CaptureAuraReverse45 {
 @AuraEnabled public static String load(String recordId, String groupId) { return 'two:' + String.valueOf(recordId) + ':' + String.valueOf(groupId); }
 @AuraEnabled public static String load(String recordId) { return 'one:' + String.valueOf(recordId); }
 @AuraEnabled public static String choose(String red) { return 'red:' + String.valueOf(red); }
 @AuraEnabled public static String choose(Integer blue) { return 'blue:' + String.valueOf(blue); }
}
