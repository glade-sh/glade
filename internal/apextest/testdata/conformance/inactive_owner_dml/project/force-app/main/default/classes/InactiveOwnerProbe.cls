public class InactiveOwnerProbe {
    static Set<String> masked = new Set<String>();
    static String mask(String text) {
        String out = text;
        for (String id : masked) { out = out.replace(id, '<owner>'); out = out.replace(id.substring(0, 15), '<owner>'); }
        return out;
    }
    static Map<String,Object> shape(Database.SaveResult result) {
        Map<String,Object> row = new Map<String,Object>{ 'success' => result.isSuccess() };
        List<String> errors = new List<String>();
        for (Database.Error issue : result.getErrors()) errors.add(mask(String.valueOf(issue.getStatusCode()) + '|' + issue.getMessage() + '|' + issue.getFields()));
        row.put('errors', errors);
        return row;
    }
    static User makeUser(String alias, Boolean active, Id profileId) {
        User record = new User(FirstName = 'Owner', LastName = alias, Email = alias + '@ownerprobe.example', Username = alias + '.' + System.currentTimeMillis() + '@ownerprobe.example',
            Alias = alias, TimeZoneSidKey = 'America/New_York', LocaleSidKey = 'en_US', EmailEncodingKey = 'UTF-8', ProfileId = profileId, LanguageLocaleKey = 'en_US', IsActive = active);
        insert as system record;
        return record;
    }
    public static Map<String,Object> observe() {
        Map<String,Object> result = new Map<String,Object>();
        if (!Test.isRunningTest()) { result.put('context', 'anonymous'); return result; }
        Id profileId = [SELECT Id FROM Profile WHERE Name = 'Standard User' WITH SYSTEM_MODE LIMIT 1].Id;
        User active;
        User inactive;
        User toDeactivate;
        System.runAs(new User(Id = UserInfo.getUserId())) {
            active = makeUser('oact', true, profileId);
            inactive = makeUser('oina', false, profileId);
            toDeactivate = makeUser('odea', true, profileId);
        }
        masked.add(String.valueOf(inactive.Id)); masked.add(String.valueOf(toDeactivate.Id));
        result.put('inactiveFlag', [SELECT IsActive FROM User WHERE Id = :inactive.Id WITH SYSTEM_MODE].IsActive);
        result.put('insertAccountInactiveOwner', shape(Database.insert(new Account(Name = 'Inactive Owner Account', OwnerId = inactive.Id), false)));
        result.put('insertContactInactiveOwner', shape(Database.insert(new Contact(LastName = 'Inactive Owner Contact', OwnerId = inactive.Id), false)));
        result.put('insertOpportunityInactiveOwner', shape(Database.insert(new Opportunity(Name = 'Inactive Owner Opp', StageName = 'Prospecting', CloseDate = Date.newInstance(2030, 1, 1), OwnerId = inactive.Id), false)));
        result.put('insertLeadInactiveOwner', shape(Database.insert(new Lead(LastName = 'Inactive Owner Lead', Company = 'Inactive Co', OwnerId = inactive.Id), false)));
        Account ok = new Account(Name = 'Owner Account', OwnerId = active.Id);
        insert as system ok;
        ok.OwnerId = inactive.Id;
        result.put('updateAccountToInactiveOwner', shape(Database.update(ok, false)));
        ok.OwnerId = active.Id;
        Account pending = new Account(Name = 'Second Account', OwnerId = inactive.Id);
        List<Database.SaveResult> batchResults = Database.insert(new List<Account>{ new Account(Name = 'Bulk Good'), pending }, false);
        result.put('bulkFirst', shape(batchResults[0]));
        result.put('bulkSecond', shape(batchResults[1]));
        Account kept = new Account(Name = 'Kept Owner', OwnerId = toDeactivate.Id);
        insert as system kept;
        System.runAs(new User(Id = UserInfo.getUserId())) {
            toDeactivate.IsActive = false;
            update as system toDeactivate;
        }
        kept.Description = 'changed after owner left';
        result.put('updateOtherFieldInactiveOwner', shape(Database.update(kept, false)));
        kept.OwnerId = toDeactivate.Id;
        result.put('reassignSameInactiveOwner', shape(Database.update(kept, false)));
        try { insert new Account(Name = 'Throwing Inactive', OwnerId = inactive.Id); result.put('throwing', 'no exception'); }
        catch (DmlException issue) { result.put('throwing', mask(issue.getDmlStatusCode(0) + '|' + issue.getMessage())); }
        return result;
    }
}
