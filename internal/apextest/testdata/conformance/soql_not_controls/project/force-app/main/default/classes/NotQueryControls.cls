public class NotQueryControls {
    public static Map<String,Object> observe() {
        Account a = new Account(Name='Aster');
        Account b = new Account(Name='Birch');
        Account c = new Account(Name='Cedar');
        insert new List<Account>{a,b,c};
        insert new List<Contact>{
            new Contact(LastName='LinkedA',AccountId=a.Id),
            new Contact(LastName='LinkedB',AccountId=b.Id)
        };
        Set<Id> ids = new Set<Id>{a.Id,b.Id};
        Set<Id> scope = new Set<Id>{a.Id,b.Id,c.Id};
        Set<Id> contactScope = new Map<Id,Contact>([SELECT Id FROM Contact WHERE AccountId IN :ids]).keySet();
        Map<String,Object> observations = new Map<String,Object>();
        for (String variant : new List<String>{'scalar_group','semi_group','scalar_ungrouped','semi_ungrouped'}) {
            String predicate;
            if (variant == 'scalar_group') {
                predicate = '(NOT (Id IN :ids)) AND Id IN :scope';
            } else if (variant == 'semi_group') {
                predicate = '(NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :scope';
            } else if (variant == 'scalar_ungrouped') {
                predicate = 'NOT Id IN :ids AND Id IN :scope';
            } else {
                predicate = 'NOT Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope) AND Id IN :scope';
            }
            try {
                List<Account> selected = Database.query('SELECT Name FROM Account WHERE '+predicate);
                observations.put(variant, names(selected));
            } catch (Exception e) {
                observations.put(variant, new Map<String,Object>{'type'=>e.getTypeName(),'message'=>e.getMessage()});
            }
        }
        observations.put('scalar_static', names([SELECT Name FROM Account WHERE (NOT (Id IN :ids)) AND Id IN :scope]));
        observations.put('semi_static', names([SELECT Name FROM Account WHERE (NOT (Id IN (SELECT AccountId FROM Contact WHERE Id IN :contactScope))) AND Id IN :scope]));
        return observations;
    }
    private static List<String> names(List<Account> rows) {
        List<String> result = new List<String>();
        for (Account row : rows) result.add(row.Name);
        result.sort();
        return result;
    }
}
