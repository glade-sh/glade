public class NotQueryEdgeControls {
    public static Map<String,Object> observe() {
        Account a = new Account(Name='Aster');
        Account b = new Account(Name='Birch');
        Account c = new Account(Name='Cedar');
        insert new List<Account>{a,b,c};
        Contact ca = new Contact(LastName='LinkedA',AccountId=a.Id);
        Contact cb = new Contact(LastName='LinkedB',AccountId=b.Id);
        insert new List<Contact>{ca,cb};
        Set<Id> scope = new Set<Id>{a.Id,b.Id,c.Id};
        Set<Id> allMatches = new Set<Id>{a.Id,b.Id};
        Set<Id> noMatches = new Set<Id>{c.Id};
        Set<Id> contactScope = new Set<Id>{ca.Id,cb.Id};
        Set<Id> emptyContacts = new Set<Id>();
        Map<String,String> predicates = new Map<String,String>{
            'standalone_not_semi'=>'NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))',
            'semi_empty'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :emptyContacts))) AND Id IN :scope',
            'semi_scope_no_matches'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :noMatches',
            'semi_scope_all_matches'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :allMatches',
            'semi_not_in'=>'(NOT (Id NOT IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :scope',
            'semi_and_false'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Name=\'Absent\' AND Id IN :scope'
        };
        Map<String,Object> observations = new Map<String,Object>();
        for (String variant : predicates.keySet()) {
            try {
                List<Account> selected = Database.query('SELECT Id,Name FROM Account WHERE '+predicates.get(variant));
                List<String> names = new List<String>();
                for (Account row : selected) {
                    if (scope.contains(row.Id)) names.add(row.Name);
                }
                names.sort();
                observations.put(variant, names);
            } catch (Exception e) {
                observations.put(variant, new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()});
            }
        }
        return observations;
    }
}
