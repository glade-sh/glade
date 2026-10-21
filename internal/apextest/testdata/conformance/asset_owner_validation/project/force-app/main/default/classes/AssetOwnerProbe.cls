public class AssetOwnerProbe {
    public static Map<String, Object> observe() {
        Account parent = new Account(Name = 'Meadow Parent');
        insert parent;
        Contact person = new Contact(LastName = 'Willow', AccountId = parent.Id);
        insert person;

        Database.SaveResult[] assetResults = Database.insert(new List<Asset>{
            new Asset(Name = 'Unrelated Pebble'),
            new Asset(Name = 'Account Pebble', AccountId = parent.Id),
            new Asset(Name = 'Contact Pebble', ContactId = person.Id)
        }, false);
        Database.SaveResult[] ownerResults = Database.insert(new List<Account>{
            new Account(Name = 'Missing Owner', OwnerId = '005000000000000AAA'),
            new Account(Name = 'Current Owner', OwnerId = UserInfo.getUserId())
        }, false);

        return new Map<String, Object>{
            'asset_unrelated' => summarize(assetResults[0]),
            'asset_account' => summarize(assetResults[1]),
            'asset_contact' => summarize(assetResults[2]),
            'account_missing_owner' => summarize(ownerResults[0]),
            'account_current_owner' => summarize(ownerResults[1])
        };
    }

    private static Map<String, Object> summarize(Database.SaveResult result) {
        List<Object> errors = new List<Object>();
        for (Database.Error problem : result.getErrors()) {
            List<String> fields = problem.getFields();
            fields.sort();
            errors.add(new Map<String, Object>{
                'code' => String.valueOf(problem.getStatusCode()),
                'fields' => fields
            });
        }
        return new Map<String, Object>{
            'success' => result.isSuccess(),
            'errors' => errors
        };
    }
}
