public class StagedParentChildProbe {
    public static List<String> rows() {
        List<String> out = new List<String>();
        for (Schema.SObjectType parentType : new List<Schema.SObjectType>{Lead.SObjectType, Product2.SObjectType}) {
            String parent = String.valueOf(parentType);
            Boolean found = false;
            for (Schema.ChildRelationship cr : parentType.getDescribe().getChildRelationships()) {
                if (cr.getRelationshipName() != 'ParentRecords') { continue; }
                found = true;
                Schema.DescribeFieldResult d = cr.getField().getDescribe();
                List<String> refs = new List<String>();
                for (Schema.SObjectType r : d.getReferenceTo()) { refs.add(String.valueOf(r)); }
                refs.sort();
                out.add(String.join(new List<String>{parent, cr.getRelationshipName(), String.valueOf(cr.getChildSObject()), d.getName(), String.valueOf(d.getType()), String.join(refs, ','), String.valueOf(d.isNillable()), String.valueOf(d.isCreateable()), String.valueOf(d.isUpdateable()), String.valueOf(d.isDefaultedOnCreate()), d.getRelationshipName(), String.valueOf(d.isAccessible())}, '|'));
            }
            if (!found) { out.add(parent + '|ParentRecords|ABSENT'); }
        }
        out.sort();
        return out;
    }
}
