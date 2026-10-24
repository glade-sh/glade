public class CustomPlatformDescribeProbe {
    public static List<String> describeRows() {
        List<String> rows = new List<String>();
        for (Schema.ChildRelationship child : LinkProbe__c.SObjectType.getDescribe().getChildRelationships()) {
            String name = child.getRelationshipName();
            if (name != null) {
                rows.add(name + '|' + String.valueOf(child.getChildSObject()) + '|' + String.valueOf(child.getField()) + '|' + child.isCascadeDelete() + '|' + child.isRestrictedDelete());
            }
        }
        rows.sort();
        return rows;
    }

    public static List<String> queryRows() {
        LinkProbe__c parent = [SELECT Id FROM LinkProbe__c LIMIT 1];
        List<String> rows = new List<String>();
        for (Schema.ChildRelationship child : LinkProbe__c.SObjectType.getDescribe().getChildRelationships()) {
            String name = child.getRelationshipName();
            if (name != null) {
                try {
                    List<SObject> parents = Database.query('SELECT Id,(SELECT Id FROM ' + name + ') FROM LinkProbe__c WHERE Id = \'' + parent.Id + '\'');
                    rows.add(name + '|' + parents[0].getSObjects(name).size());
                } catch (Exception e) {
                    rows.add(name + '|reject|' + e.getTypeName());
                }
            }
        }
        rows.sort();
        return rows;
    }
}
