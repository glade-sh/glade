public class ShapeRelationships {
    // Compile-only: standard and custom parents, typed SOQL and dynamic children.
    public static void compileRoutes() {
        Account standardParent = [SELECT Id, (SELECT Id FROM Contacts) FROM Account LIMIT 1];
        List<SObject> standardChildren = standardParent.getSObjects('Contacts');
        ShapeGrove__c customParent = [SELECT Id, (SELECT Id FROM Leaves__r) FROM ShapeGrove__c LIMIT 1];
        List<SObject> customChildren = customParent.getSObjects('Leaves__r');
        List<Schema.ChildRelationship> standardNames = Account.SObjectType.getDescribe().getChildRelationships();
        List<Schema.ChildRelationship> customNames = ShapeGrove__c.SObjectType.getDescribe().getChildRelationships();
        for (Schema.ChildRelationship child : standardNames) {
            String relationship = child.getRelationshipName();
        }
        for (Schema.ChildRelationship child : customNames) {
            String relationship = child.getRelationshipName();
        }
    }
}
