public class GroveParentPicklistProbe {
    public static String observe() {
        Account parent = new Account(Name='Grove picklist parent', Industry='Banking');
        insert parent;
        Account emptyParent = new Account(Name='Grove empty picklist');
        insert emptyParent;
        Map<String, Object> result = new Map<String, Object>();
        for (Contact child : new List<Contact>{
            new Contact(LastName='K01', AccountId=parent.Id, Description='initial'),
            new Contact(LastName='K02', AccountId=emptyParent.Id, Description='initial')
        }) {
            Map<String, Object> cell = new Map<String, Object>{'description'=>null, 'exception'=>null};
            try {
                insert child;
                Contact stored = [SELECT Description FROM Contact WHERE Id=:child.Id];
                cell.put('description', stored.Description);
            } catch (Exception error) {
                cell.put('exception', error.getTypeName());
            }
            result.put(child.LastName, cell);
        }
        return JSON.serialize(result);
    }
}
