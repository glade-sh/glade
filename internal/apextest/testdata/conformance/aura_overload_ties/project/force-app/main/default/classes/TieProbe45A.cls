public class TieProbe45A {
@AuraEnabled public static String same(String value) { return 'string:' + String.valueOf(value); }
@AuraEnabled public static String same(Integer value) { return 'integer:' + String.valueOf(value); }
@AuraEnabled public static String cross(String alpha) { return 'string:' + String.valueOf(alpha); }
@AuraEnabled public static String cross(Integer beta) { return 'integer:' + String.valueOf(beta); }
@AuraEnabled public static String swapped(String beta) { return 'string:' + String.valueOf(beta); }
@AuraEnabled public static String swapped(Integer alpha) { return 'integer:' + String.valueOf(alpha); }
@AuraEnabled public static String control(String value) { return 'control:' + value; }
}
