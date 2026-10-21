public class AccountNameProbe {
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        Map<String,Schema.SObjectField> fields = Schema.SObjectType.Account.fields.getMap();
        List<String> names = new List<String>();
        for (Schema.SObjectField field : Account.SObjectType.getDescribe().fields.getMap().values()) {
            if (field.getDescribe().isNameField()) names.add(field.getDescribe().getName());
        }
        names.sort();
        result.put('describe_names', String.join(names, ','));
        result.put('direct_name', Account.Name.getDescribe().isNameField());
        for (String name : new List<String>{'Name','FirstName','LastName'}) {
            Schema.SObjectField field = fields.get(name);
            result.put(name, field == null ? null : field.getDescribe().isNameField());
        }
        Account row = new Account(Name='Owned meadow');
        insert row;
        Account loaded = [SELECT Name FROM Account WHERE Id = :row.Id];
        String selected;
        for (Schema.SObjectField field : fields.values()) {
            if (field.getDescribe().isNameField()) { selected = field.getDescribe().getName(); break; }
        }
        result.put('selected_name', selected);
        try { result.put('selected_value', String.valueOf(loaded.get(selected))); }
        catch (Exception error) { result.put('selected_value', error.getTypeName() + '|' + error.getMessage()); }
        return result;
    }
}
