public class DefaultProbe62 {
 public static String selectValue(Object value) { return 'object:' + String.valueOf(value); }
 public static String selectValue(String value) { return 'string:' + String.valueOf(value); }
 public static String run() { return selectValue(DefaultProbe__c.Blank__c.getDescribe().getDefaultValue()) + '|' + selectValue(DefaultProbe__c.Number__c.getDescribe().getDefaultValue()) + '|' + selectValue(DefaultProbe__c.Flag__c.getDescribe().getDefaultValue()) + '|' + String.valueOf(Integer.valueOf(DefaultProbe__c.Blank__c.getDescribe().getDefaultValue())); }
}
