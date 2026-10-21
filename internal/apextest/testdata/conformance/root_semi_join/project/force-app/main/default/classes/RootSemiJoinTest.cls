@IsTest
private class RootSemiJoinTest {
    private static void seed() {
        Account parent = new Account(Name = 'JoinParent');
        insert parent;
        insert new Account(Name = 'JoinEmpty');
        insert new Contact(LastName = 'JoinWitness', AccountId = parent.Id);
    }

    private static void observe(String method, String query) {
        seed();
        Map<String, Object> observation = new Map<String, Object>();
        try {
            observation.put('rows', Database.query(query).size());
        } catch (Exception error) {
            observation.put('type', error.getTypeName());
            observation.put('message', error.getMessage());
        }
        System.assert(false, 'OBSERVED|' + method + '|' + JSON.serialize(observation));
    }

    @IsTest static void rootAntiJoinFirst() {
        observe('rootAntiJoinFirst',
            'SELECT Id FROM Account WHERE (Id NOT IN (SELECT AccountId FROM Contact WHERE LastName = \'JoinWitness\') AND Name = \'JoinEmpty\')');
    }

    @IsTest static void rootReferenceJoinSecond() {
        observe('rootReferenceJoinSecond',
            'SELECT Id FROM Contact WHERE (LastName = \'JoinWitness\' AND AccountId IN (SELECT Id FROM Account WHERE Name = \'JoinParent\'))');
    }

    @IsTest static void rootTwoJoins() {
        observe('rootTwoJoins',
            'SELECT Id FROM Account WHERE (Name = \'JoinParent\' AND Id IN (SELECT AccountId FROM Contact WHERE LastName = \'JoinWitness\') AND Id NOT IN (SELECT AccountId FROM Contact WHERE LastName = \'JoinAbsent\'))');
    }

    @IsTest static void descendantJoinGroup() {
        observe('descendantJoinGroup',
            'SELECT Id FROM Account WHERE (Id IN (SELECT AccountId FROM Contact WHERE LastName = \'JoinWitness\') AND Name = \'JoinParent\') AND Name != \'JoinEmpty\'');
    }
}
