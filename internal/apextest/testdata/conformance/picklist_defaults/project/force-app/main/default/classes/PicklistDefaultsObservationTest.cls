@IsTest
private class PicklistDefaultsObservationTest {
    @IsTest static void typeValues() {
        List<String> activeValues = new List<String>();
        for (Schema.PicklistEntry entry : Account.Type.getDescribe().getPicklistValues()) {
            if (entry.isActive()) activeValues.add(entry.getValue());
        }
        activeValues.sort();
        emit('typeValues',new Map<String,Object>{
            'activeValues'=>activeValues,
            'customerDirect'=>activeValues.contains('Customer - Direct'),
            'technologyPartner'=>activeValues.contains('Technology Partner'),
            'restricted'=>Account.Type.getDescribe().isRestrictedPicklist()
        });
    }

    @IsTest static void insertValues() {
        List<Account> records = new List<Account>{
            new Account(Name='First picklist observation',Type='Customer - Direct'),
            new Account(Name='Second picklist observation',Type='Technology Partner')
        };
        List<Database.SaveResult> results = Database.insert(records,false);
        List<Object> observations = new List<Object>();
        for (Database.SaveResult result : results) {
            Map<String,Object> observation = new Map<String,Object>{'success'=>result.isSuccess()};
            if (!result.isSuccess()) {
                observation.put('status',String.valueOf(result.getErrors()[0].getStatusCode()));
                observation.put('message',result.getErrors()[0].getMessage());
            }
            observations.add(observation);
        }
        emit('insertValues',new Map<String,Object>{'results'=>observations});
    }

    private static void emit(String method,Map<String,Object> observation) {
        System.assert(false,'OBSERVED|'+method+'|'+JSON.serialize(observation));
    }
}
