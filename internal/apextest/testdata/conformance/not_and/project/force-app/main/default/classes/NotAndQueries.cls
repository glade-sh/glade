public class NotAndQueries {
    public static Map<String,Object> observe() {
        Account a = new Account(Name='Aster');
        Account b = new Account(Name='Birch');
        Account c = new Account(Name='Cedar');
        insert new List<Account>{a,b,c};
        Contact ca = new Contact(LastName='LinkedA',AccountId=a.Id);
        Contact cb = new Contact(LastName='LinkedB',AccountId=b.Id);
        insert new List<Contact>{ca,cb};
        Set<Id> ids = new Set<Id>{a.Id,b.Id};
        Set<Id> scope = new Set<Id>{a.Id,b.Id,c.Id};
        Set<Id> emptyIds = new Set<Id>();
        Set<Id> contactScope = new Set<Id>{ca.Id,cb.Id};
        Set<Id> emptyContacts = new Set<Id>();
        Map<String,String> predicates = new Map<String,String>{
            'scalar_ungrouped'=>'NOT Id IN :ids AND Id IN :scope',
            'semi_ungrouped'=>'NOT Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope) AND Id IN :scope',
            'scalar_inner_group'=>'NOT (Id IN :ids) AND Id IN :scope',
            'semi_inner_group'=>'NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope)) AND Id IN :scope',
            'scalar_group'=>'(NOT (Id IN :ids)) AND Id IN :scope',
            'semi_group'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :scope',
            'scalar_lower'=>'(not (id in :ids)) and id in :scope',
            'semi_lower'=>'(not (id in (select accountid from contact where id in :contactScope))) and id in :scope',
            'scalar_ungrouped_lower'=>'not id in :ids and id in :scope',
            'semi_ungrouped_lower'=>'not id in (select accountid from contact where id in :contactScope) and id in :scope',
            'scalar_empty'=>'(NOT (Id IN :emptyIds)) AND Id IN :scope',
            'semi_empty'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :emptyContacts))) AND Id IN :scope',
            'scalar_empty_scope'=>'(NOT (Id IN :ids)) AND Id IN :emptyIds',
            'semi_empty_scope'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :emptyIds',
            'scalar_and_false'=>'(NOT (Id IN :ids)) AND Name=\'Absent\' AND Id IN :scope',
            'semi_and_false'=>'(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Name=\'Absent\' AND Id IN :scope',
            'scalar_standalone'=>'NOT (Id IN :ids)',
            'semi_standalone'=>'NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))'
        };
        Map<String,Object> observations = new Map<String,Object>();
        for (String variant : predicates.keySet()) {
            try {
                List<Account> selected = Database.query('SELECT Id,Name FROM Account WHERE '+predicates.get(variant));
                observations.put(variant, names(selected,scope));
            } catch (Exception e) {
                observations.put(variant, new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()});
            }
        }
        observations.put('scalar_inline', names([SELECT Id,Name FROM Account WHERE (NOT (Id IN :ids)) AND Id IN :scope],scope));
        observations.put('semi_inline', names([SELECT Id,Name FROM Account WHERE (NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :scope],scope));
        return observations;
    }
    // Default access is private. Calls within the class exercise that twin.
    static List<String> names(List<Account> rows, Set<Id> scope) {
        List<String> result = new List<String>();
        for (Account row : rows) {
            if (scope.contains(row.Id)) result.add(row.Name);
        }
        result.sort();
        return result;
    }
}
