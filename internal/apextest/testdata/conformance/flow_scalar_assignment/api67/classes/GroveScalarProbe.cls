public class GroveScalarProbe {
    public static String observe() {
        Account parent = new Account(Name='Grove scalar parent', Description='parent & punctuation');
        insert parent;
        Map<String,Object> result = new Map<String,Object>();
        for (String key : new List<String>{'A01','A02','A03','A04','A05','A06','A07','A08','A09','A10','A11'}) {
            Contact child = new Contact(LastName=key);
            if (key != 'A10') child.AccountId=parent.Id;
            if (key != 'A09' && key != 'A07') child.Description='leaf & punctuation';
            if (key != 'A11') child.LeadSource='Web';
            child.Birthdate=Date.newInstance(2001,2,3);
            Map<String,Object> cell=new Map<String,Object>{'description'=>null,'datePreserved'=>false,'exception'=>null};
            try {
                insert child;
                Contact stored=[SELECT Description,Birthdate FROM Contact WHERE Id=:child.Id];
                cell.put('description', stored.Description);
                cell.put('datePreserved', stored.Birthdate==child.Birthdate);
            } catch (Exception error) { cell.put('exception', error.getTypeName()); }
            result.put(key,cell);
        }
        return JSON.serialize(result);
    }
}
