@IsTest
private class GroveFormulaObservation {
    @IsTest static void dateSubtractionBeforeInsert() {
        List<Account> rows = new List<Account>{
            new Account(Name = 'Blank'),
            new Account(Name = 'Past', GroveDate__c = Date.today().addDays(-5)),
            new Account(Name = 'Future', GroveDate__c = Date.today().addDays(2))
        };
        Database.insert(rows, AccessLevel.SYSTEM_MODE);
        Set<Id> ids = new Map<Id, Account>(rows).keySet();
        Map<String, Object> result = new Map<String, Object>();
        for (Account row : [SELECT Name, Description, GroveAge__c FROM Account WHERE Id IN :ids WITH SYSTEM_MODE]) {
            result.put(row.Name, new Map<String, Object>{
                'before' => JSON.deserializeUntyped(row.Description),
                'stored' => row.GroveAge__c
            });
        }
        System.assert(false, 'OBSERVED|dateSubtractionBeforeInsert|' + JSON.serialize(result));
    }
}
