public class StandardBehavior {
    private static Map<Id,String> labels = new Map<Id,String>();

    private static String labelMatches(Datacloud.DuplicateResult result) {
        List<String> hits = new List<String>();
        for (Datacloud.MatchResult match : result.getMatchResults()) {
            for (Datacloud.MatchRecord row : match.getMatchRecords()) {
                Id recordId = row.getRecord().Id;
                hits.add(labels.containsKey(recordId) ? labels.get(recordId) : 'unexpected');
            }
        }
        hits.sort();
        return result.getDuplicateRule() + '/' + result.isAllowSave() + '/[' + String.join(hits, ',') + ']';
    }

    public static String find(SObject candidate) {
        List<String> rules = new List<String>();
        try {
            List<Datacloud.FindDuplicatesResult> results = Datacloud.FindDuplicates.findDuplicates(new List<SObject>{candidate});
            for (Datacloud.DuplicateResult rule : results[0].getDuplicateResults()) {
                rules.add(labelMatches(rule));
            }
            rules.sort();
            return 'find=' + String.join(rules, '+');
        } catch (Exception e) {
            return 'find=' + e.getTypeName() + '/' + e.getMessage();
        }
    }

    public static String verdict(Boolean success, Database.Error[] errors) {
        if (success) return 'saved';
        List<String> parts = new List<String>();
        for (Database.Error error : errors) {
            String status = String.valueOf(error.getStatusCode());
            if (error instanceof Database.DuplicateError) {
                status += '/' + labelMatches(((Database.DuplicateError)error).getDuplicateResult());
            }
            parts.add(status);
        }
        return String.join(parts, '+');
    }

    public static String save(Database.SaveResult r) {
        return verdict(r.isSuccess(), r.getErrors());
    }
    private static String ups(Database.UpsertResult r) {
        return verdict(r.isSuccess(), r.getErrors()) + (r.isSuccess() ? '/created=' + r.isCreated() : '');
    }
    public static void remember(Id recordId, String label) {
        labels.put(recordId, label);
    }
    private static Account numbered(Integer i) {
        return new Account(Name='Test Account ' + i, BillingCity='Northport');
    }

    public static String accountSeparate() {
        Account a = numbered(1);
        insert a;
        remember(a.Id, 'first');
        Account b = numbered(2);
        String result = find(b) + '|' + save(Database.insert(b, false));
        return result + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String accountOptions() {
        Account a = numbered(1);
        insert a;
        remember(a.Id, 'first');
        Account b = numbered(2);
        Database.DMLOptions options = new Database.DMLOptions();
        options.DuplicateRuleHeader.allowSave = true;
        b.setOptions(options);
        String result = find(b) + '|perRecord=' + save(Database.insert(b, false));
        Account c = numbered(3);
        return result + '|explicit=' + save(Database.insert(c, options)) + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String accountBulk() {
        Account a = numbered(1), b = numbered(2);
        List<Database.SaveResult> batch = Database.insert(new List<Account>{a,b}, false);
        if (batch[0].isSuccess()) remember(batch[0].getId(), 'first');
        if (batch[1].isSuccess()) remember(batch[1].getId(), 'second');
        Account c = numbered(3);
        return save(batch[0]) + '|' + save(batch[1]) + '|' + find(c) + '|' + save(Database.insert(c, false)) + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String accountNoCity() {
        Account a = numbered(1), b = numbered(2);
        a.BillingCity = null;
        b.BillingCity = null;
        insert a;
        remember(a.Id, 'first');
        return find(b) + '|' + save(Database.insert(b,false));
    }
    public static String accountUpdate() {
        Account a = numbered(1), b = new Account(Name='Cobalt Foundry',BillingCity='Tidemarsh');
        insert new List<Account>{a,b};
        remember(a.Id, 'first');
        b.Name = 'Test Account 2';
        b.BillingCity = 'Northport';
        String result = find(b) + '|' + save(Database.update(b, false));
        return result + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String accountUpsert() {
        Account a = numbered(1);
        insert a;
        remember(a.Id, 'first');
        Account b = numbered(2);
        Database.UpsertResult r = Database.upsert(b, false);
        return find(numbered(3)) + '|' + ups(r) + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String accountUpsertUpdate() {
        Account a = numbered(1), b = new Account(Name='Cobalt Foundry',BillingCity='Tidemarsh');
        insert new List<Account>{a,b};
        remember(a.Id, 'first');
        b.Name = 'Test Account 2';
        b.BillingCity = 'Northport';
        String result = find(b) + '|' + ups(Database.upsert(b, false));
        return result + '|count=' + [SELECT COUNT() FROM Account WHERE BillingCity='Northport'];
    }
    public static String contactBlank() {
        Contact a = new Contact(LastName='Test1', Email='shared@sample.example');
        insert a;
        remember(a.Id, 'first');
        Contact b = new Contact(LastName='Test2', Email=a.Email);
        Contact c = new Contact(LastName='Test3', Email='other@distinct.example');
        return find(b) + '|' + save(Database.insert(b, false)) + '|' + find(c) + '|' + save(Database.insert(c, false));
    }
    public static String leadBlank() {
        Lead a = new Lead(LastName='Test1', Email='shared@sample.example', Company='Beacon');
        insert a;
        remember(a.Id, 'first');
        Lead b = new Lead(LastName='Test2', Email=a.Email, Company='Oxbow');
        return find(b) + '|' + save(Database.insert(b, false));
    }
    private static String samePhone(String phone) {
        Contact a = new Contact(FirstName='Helen', LastName='Quill', Phone=phone);
        insert a;
        remember(a.Id, 'first');
        Contact b = new Contact(FirstName=a.FirstName, LastName=a.LastName, Phone=phone);
        return find(b) + '|' + save(Database.insert(b, false));
    }
    public static String phoneInternational() { return samePhone('+44 20 7946 0958'); }
    public static String phoneExtension() { return samePhone('(415) 555-0100 x5'); }
}
