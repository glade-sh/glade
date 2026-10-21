public class GroveParentProbe {
    public static String observe() {
        Account parent = new Account(Name='Grove parent', Description='Grove \'text\' & value');
        insert parent;
        Account emptyParent = new Account(Name='Grove empty');
        insert emptyParent;
        List<Contact> children = new List<Contact>{
            new Contact(LastName='R01', AccountId=parent.Id),
            new Contact(LastName='R02', AccountId=emptyParent.Id),
            new Contact(LastName='R03'),
            new Contact(LastName='R04', AccountId=parent.Id, Description='initial')
        };
        Map<String,Object> result = new Map<String,Object>();
        for (Contact child : children) {
            Map<String,Object> cell = new Map<String,Object>{'description'=>null, 'exception'=>null};
            try {
                insert child;
                Contact stored = [SELECT Description FROM Contact WHERE Id=:child.Id];
                cell.put('description', stored.Description);
            } catch (Exception error) {
                cell.put('exception', error.getTypeName());
            }
            result.put(child.LastName,cell);
        }
        return JSON.serialize(result);
    }
}
