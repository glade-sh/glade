public class GroveLookupSourceProbe {
    public static String observe() {
        Map<String, Object> results = new Map<String, Object>();
        for (String key : new List<String>{'P01', 'P02', 'P03'}) {
            Account parent = new Account(Name='Grove ' + key, Description=key == 'P03' ? null : 'original', Phone=key == 'P03' ? null : 'old');
            insert parent;
            Contact child = new Contact(AccountId=parent.Id, LastName=key, Phone=key == 'P02' ? null : '5550100');
            insert child;
            parent = [SELECT Description, Phone FROM Account WHERE Id = :parent.Id];
            results.put(key, new Map<String, Object>{'description'=>parent.Description, 'phone'=>parent.Phone});
        }
        return JSON.serialize(results);
    }
}
