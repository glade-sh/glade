public class CmdtDescProbe {
    // Flag order: create update nillable defOnCreate access filterable groupable sortable unique permissionable calculated custom auto extId idLookup nameField restrictedDelete restrictedPicklist cascade writeReq
    static String b(Boolean v) { return v ? '1' : '0'; }
    static Object fieldFlags(Schema.DescribeFieldResult x) {
        String flags = b(x.isCreateable()) + b(x.isUpdateable()) + b(x.isNillable()) + b(x.isDefaultedOnCreate()) + b(x.isAccessible()) + b(x.isFilterable()) +
            b(x.isGroupable()) + b(x.isSortable()) + b(x.isUnique()) + b(x.isPermissionable()) + b(x.isCalculated()) + b(x.isCustom()) + b(x.isAutoNumber()) +
            b(x.isExternalId()) + b(x.isIdLookup()) + b(x.isNameField()) + b(x.isRestrictedDelete()) + b(x.isRestrictedPicklist()) + b(x.isCascadeDelete()) +
            b(x.isWriteRequiresMasterRead());
        return String.valueOf(x.getType()) + ' ' + flags + ' ' + x.getLength() + ' ' + String.valueOf(x.getReferenceTo()) + ' ' + x.getRelationshipName();
    }
    public static Map<String,Object> observe() {
        Map<String,Object> m = new Map<String,Object>();
        for (Schema.SObjectType t : new List<Schema.SObjectType>{ V35Child__mdt.SObjectType, V35Parent__mdt.SObjectType }) {
            Schema.DescribeSObjectResult d = t.getDescribe();
            m.put(d.getName(), b(d.isCreateable()) + b(d.isUpdateable()) + b(d.isDeletable()) + b(d.isQueryable()) + b(d.isSearchable()) + b(d.isAccessible()) +
                b(d.isCustom()) + b(d.isCustomSetting()) + b(d.isMergeable()) + b(d.isUndeletable()) + b(d.isFeedEnabled()));
        }
        Map<String,Schema.SObjectField> fm = V35Child__mdt.SObjectType.getDescribe().fields.getMap();
        List<String> names = new List<String>(fm.keySet()); names.sort();
        Map<String,Object> fields = new Map<String,Object>();
        for (String n : names) { fields.put(n, fieldFlags(fm.get(n).getDescribe())); }
        m.put('childFields', fields);
        Map<String,Object> pf = new Map<String,Object>();
        Map<String,Schema.SObjectField> pm = V35Parent__mdt.SObjectType.getDescribe().fields.getMap();
        List<String> pn = new List<String>(pm.keySet()); pn.sort();
        for (String n : pn) { pf.put(n, fieldFlags(pm.get(n).getDescribe())); }
        m.put('parentFields', pf);
        return m;
    }
}
