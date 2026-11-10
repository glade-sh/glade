public class CaptureAura62 {
 @AuraEnabled public static String load(String recordId) { return 'one:' + String.valueOf(recordId); }
 @AuraEnabled public static String load(String recordId, String groupId) { return 'two:' + String.valueOf(recordId) + ':' + String.valueOf(groupId); }
 @AuraEnabled public static String control(String item) { return 'control:' + String.valueOf(item); }
}
