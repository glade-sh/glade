@IsTest
private class BirchFlowDecisionTest {
    private static Map<String,Object> observe(String name, String kind) {
        try {
            Account a = new Account(Name = name, Type = kind);
            insert a;
            Account r = [SELECT Description, Site FROM Account WHERE Id = :a.Id];
            return new Map<String,Object>{'description' => r.Description, 'site' => r.Site};
        } catch (Exception e) {
            return new Map<String,Object>{'error' => e.getTypeName() + ':' + e.getMessage()};
        }
    }
    @IsTest static void observeDecision() {
        insert new Contact(LastName = 'Birch hit', Title = 'Guide');
        Map<String,Object> result = new Map<String,Object>();
        for (String kind : new List<String>{'Prospect','Other','Customer - Direct','Partner'}) {
            result.put(kind, new Map<String,Object>{'matched' => observe('Birch hit', kind), 'missing' => observe('Birch miss', kind)});
        }
        System.assert(false, 'OBSERVED|lookupDecision|' + JSON.serialize(result));
    }
}
