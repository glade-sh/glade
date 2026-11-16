Account a = [SELECT Id FROM Account WHERE Name = 'S68-Q4' LIMIT 1];
for (Boolean explicitNs : new List<Boolean>{true, false}) {
    Database.QueryOptions opts = Database.QueryOptions.builder()
        .withExplicitNamespace(explicitNs).build();
    try {
        List<SObject> rows = Database.queryWithBinds(
            'SELECT Age__c, gladetest1__Age__c FROM Account WHERE Id = :aid SET OPTIONS :opts',
            new Map<String,Object>{'aid' => a.Id, 'opts' => opts}, AccessLevel.USER_MODE);
        System.debug('SUB_EXPLICIT=' + explicitNs + ';JSON=' + JSON.serialize(rows[0]));
    } catch (Exception e) {
        System.debug('SUB_EXPLICIT=' + explicitNs + ';ERR=' + e.getTypeName() + ': ' + e.getMessage());
    }
}
