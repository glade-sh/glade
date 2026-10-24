public class QualifiedReceiverProbe {
  public static Boolean standardTrigger = false;
  public static Boolean customTrigger = false;
  public static Object standard_schema_static() { return Schema.Account.getSObjectType(); }
  public static Object standard_schema_token() { return Schema.Account.SObjectType; }
  public static Object standard_schema_token_case() { return Schema.Account.sObjectType; }
  public static Object standard_schema_token_field() { return Schema.Account.SObjectType.fields.Name.getDescribe(); }
  public static Object standard_object_token_field() { return Account.SObjectType.fields.Name.getDescribe(); }
  public static Object standard_sobjecttype_field() { return SObjectType.Account.fields.Name.getName(); }
  public static Object standard_fieldset() { return SObjectType.Account.fieldSets.Summary.getFields(); }
  public static Object standard_schema_fieldset() { return Schema.SObjectType.Account.fieldSets.Summary.getFields(); }
  public static Object standard_schema_sobjecttype() { return Schema.SObjectType.Account.getName(); }
  public static Object standard_schema_sobjecttype_fields_map() { return Schema.SObjectType.Account.fields.getMap(); }
  public static Object standard_object_fields() { return Account.fields.Name.getDescribe(); }
  public static Object standard_schema_fields() { return Schema.Account.fields.Name.getDescribe(); }
  public static Object custom_schema_static() { return Schema.Meadow__c.getSObjectType(); }
  public static Object custom_schema_token() { return Schema.Meadow__c.SObjectType; }
  public static Object custom_schema_token_case() { return Schema.Meadow__c.sObjectType; }
  public static Object custom_schema_token_field() { return Schema.Meadow__c.SObjectType.fields.Flag__c.getDescribe(); }
  public static Object custom_object_token_field() { return Meadow__c.SObjectType.fields.Flag__c.getDescribe(); }
  public static Object custom_sobjecttype_field() { return SObjectType.Meadow__c.fields.Flag__c.getName(); }
  public static Object custom_fieldset() { return SObjectType.Meadow__c.fieldSets.Summary.getFields(); }
  public static Object custom_schema_fieldset() { return Schema.SObjectType.Meadow__c.fieldSets.Summary.getFields(); }
  public static Object custom_schema_sobjecttype() { return Schema.SObjectType.Meadow__c.getName(); }
  public static Object custom_schema_sobjecttype_fields_map() { return Schema.SObjectType.Meadow__c.fields.getMap(); }
  public static Object custom_object_fields() { return Meadow__c.fields.Flag__c.getDescribe(); }
  public static Object custom_schema_fields() { return Schema.Meadow__c.fields.Flag__c.getDescribe(); }
  public static Object label_replace() { return Label.MeadowCaption.replace('a', 'b'); }
  public static Object system_label_replace() { return System.Label.MeadowCaption.replace('a', 'b'); }
  public static Object label_length() { return Label.MeadowCaption.length(); }
  public static Object label_argument() { return String.format(Label.MeadowCaption, new List<String>{'x'}); }
  public static Map<String, Boolean> observe() {
    Map<String, Boolean> result = new Map<String, Boolean>();
    result.put('standard_schema_static', standard_schema_static() != null);
    result.put('standard_schema_token', standard_schema_token() != null);
    result.put('standard_schema_token_case', standard_schema_token_case() != null);
    result.put('standard_schema_token_field', standard_schema_token_field() != null);
    result.put('standard_object_token_field', standard_object_token_field() != null);
    result.put('standard_sobjecttype_field', standard_sobjecttype_field() != null);
    result.put('standard_fieldset', standard_fieldset() != null);
    result.put('standard_schema_fieldset', standard_schema_fieldset() != null);
    result.put('standard_schema_sobjecttype', standard_schema_sobjecttype() != null);
    result.put('standard_schema_sobjecttype_fields_map', standard_schema_sobjecttype_fields_map() != null);
    result.put('standard_object_fields', standard_object_fields() != null);
    result.put('standard_schema_fields', standard_schema_fields() != null);
    result.put('custom_schema_static', custom_schema_static() != null);
    result.put('custom_schema_token', custom_schema_token() != null);
    result.put('custom_schema_token_case', custom_schema_token_case() != null);
    result.put('custom_schema_token_field', custom_schema_token_field() != null);
    result.put('custom_object_token_field', custom_object_token_field() != null);
    result.put('custom_sobjecttype_field', custom_sobjecttype_field() != null);
    result.put('custom_fieldset', custom_fieldset() != null);
    result.put('custom_schema_fieldset', custom_schema_fieldset() != null);
    result.put('custom_schema_sobjecttype', custom_schema_sobjecttype() != null);
    result.put('custom_schema_sobjecttype_fields_map', custom_schema_sobjecttype_fields_map() != null);
    result.put('custom_object_fields', custom_object_fields() != null);
    result.put('custom_schema_fields', custom_schema_fields() != null);
    result.put('label_replace', label_replace() != null);
    result.put('system_label_replace', system_label_replace() != null);
    result.put('label_length', label_length() != null);
    result.put('label_argument', label_argument() != null);
    insert new Account(Name = 'Receiver meadow');
    insert new Meadow__c(Name = 'Receiver meadow');
    result.put('standard_trigger', standardTrigger);
    result.put('custom_trigger', customTrigger);
    return result;
  }
}
