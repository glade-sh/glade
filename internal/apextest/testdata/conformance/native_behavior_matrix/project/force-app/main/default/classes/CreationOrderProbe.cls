public class CreationOrderProbe {
    public static List<String> observe() {
        List<String> observed = new List<String>();
        for (Schema.ChildRelationship relation : Contact.SObjectType.getDescribe().getChildRelationships()) {
            String childName = relation.getChildSObject().getDescribe().getName();
            if (childName == 'Kid__c') {
                observed.add(childName + '.' + relation.getRelationshipName() + '.' + relation.getField().getDescribe().getName());
            }
        }
        return observed;
    }
}
