public class ShapeShortFields {
    // Compile-only: unqualified SObjectType.<object>.fields describe forms.
    // Standard and custom objects. Native basis:
    // conformance/qualified_receivers/cases.tsv:7,8,11,20,21,24 and org.tsv.
    public static Schema.SObjectField nameField = SObjectType.Account.fields.Name.getSObjectField();

    public static Boolean compileRoutes() {
        Schema.DescribeFieldResult direct = SObjectType.Account.fields.Name;
        Map<String, Schema.SObjectField> fieldMap = SObjectType.Account.fields.getMap();
        Schema.DescribeFieldResult qualified = Schema.SObjectType.Account.fields.Name;
        Schema.DescribeSObjectResult described = SObjectType.Account.getDescribe();
        List<Schema.SObjectField> tokens = new List<Schema.SObjectField>{ Account.Name, Account.Industry };
        Map<Schema.SObjectField, String> labels = new Map<Schema.SObjectField, String>{ Account.Name => 'Name' };
        Map<String, Schema.FieldSet> fieldSets = SObjectType.Account.fieldSets.getMap();
        Schema.SObjectField byName = SObjectType.Account.fields.getMap().get('Name');
        String fieldName = SObjectType.Account.fields.Name.getName();
        Map<String, Schema.SObjectField> customMap = SObjectType.ShapeLeaf__c.fields.getMap();
        Schema.DescribeFieldResult customField = SObjectType.ShapeLeaf__c.fields.Grove__c;
        String customName = SObjectType.ShapeLeaf__c.fields.Grove__c.getName();
        Schema.SObjectField customByName = SObjectType.ShapeLeaf__c.fields.getMap().get('Grove__c');
        return direct != null && fieldMap != null && qualified != null && described != null &&
            tokens.size() == 2 && labels.size() == 1 && fieldSets != null && byName != null && fieldName != null &&
            customMap != null && customField != null && customName != null && customByName != null;
    }
}
