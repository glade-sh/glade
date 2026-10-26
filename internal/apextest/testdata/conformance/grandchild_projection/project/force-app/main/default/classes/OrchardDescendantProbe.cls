public class OrchardDescendantProbe {
    public static Map<String, Object> run() {
        List<Account> branches = new List<Account>{
            new Account(Name = 'Orchard North'), new Account(Name = 'Orchard South')
        };
        insert branches;
        List<Opportunity> leaves = new List<Opportunity>{
            new Opportunity(Name = 'Orchard Fruit', AccountId = branches[0].Id, StageName = 'Prospecting', CloseDate = Date.today()),
            new Opportunity(Name = 'Orchard Fruit', AccountId = branches[1].Id, StageName = 'Prospecting', CloseDate = Date.today())
        };
        insert leaves;
        Set<Id> branchIds = new Map<Id, Account>(branches).keySet();
        Set<Id> expectedIds = new Map<Id, Opportunity>(leaves).keySet();
        Set<String> requested = new Set<String>{'Id', 'Name', 'Account.Id', 'Account.Name', 'Account.Owner.Id', 'Account.Owner.Username'};
        List<String> columns = new List<String>(requested);
        List<SObject> queried = Database.query('SELECT ' + String.join(columns, ',') + ' FROM Opportunity WHERE AccountId IN :branchIds');
        Set<Id> found = new Set<Id>();
        Set<Id> roots = new Set<Id>();
        Boolean complete = true;
        Boolean fieldSets = true;
        for (SObject leaf : queried) {
            found.add((Id)leaf.get('Id'));
            SObject branch = leaf.getSObject('Account');
            SObject root = branch.getSObject('Owner');
            roots.add((Id)root.get('Id'));
            complete = complete && root.getSObjectType() == User.SObjectType && root.get('Username') != null;
            fieldSets = fieldSets && branch.getPopulatedFieldsAsMap().keySet().containsAll(new Set<String>{'Id', 'Name', 'Owner'});
            fieldSets = fieldSets && root.getPopulatedFieldsAsMap().keySet().containsAll(new Set<String>{'Id', 'Username'});
        }
        List<SObject> mixed = Database.query('SELECT Id, Account.Owner.Id, account.owner.Username, ACCOUNT.Name FROM Opportunity WHERE AccountId IN :branchIds');
        Boolean dynamicFields = canonicalFields(mixed);
        List<Opportunity> inlineRows = [SELECT Id, Account.Owner.Id, account.owner.Username, ACCOUNT.Name FROM Opportunity WHERE AccountId IN :branchIds];
        return new Map<String, Object>{
            'finished' => complete, 'fields' => fieldSets,
            'allGrandchildren' => found == expectedIds,
            'sharedRoot' => roots == new Set<Id>{UserInfo.getUserId()},
            'dynamicCanonicalFields' => dynamicFields,
            'inlineCanonicalFields' => canonicalFields(inlineRows)
        };
    }

    private static Boolean canonicalFields(List<SObject> leaves) {
        for (SObject leaf : leaves) {
            Set<String> fields = leaf.getPopulatedFieldsAsMap().keySet();
            if (!fields.contains('Account')) return false;
            SObject branch = leaf.getSObject('Account');
            fields = branch.getPopulatedFieldsAsMap().keySet();
            if (!fields.containsAll(new Set<String>{'Id', 'Name', 'Owner'})) return false;
            SObject root = branch.getSObject('Owner');
            fields = root.getPopulatedFieldsAsMap().keySet();
            if (!fields.containsAll(new Set<String>{'Id', 'Username'})) return false;
        }
        return true;
    }
}
