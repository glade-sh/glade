@IsTest
private class CedarRouteTest {
    private static Willow__c seed(String mode) {
        Account account = new Account(Name='Cedar account');
        insert as system account;
        Willow__c thing = new Willow__c(Name=mode, Account__c=account.Id);
        insert as system thing;
        return thing;
    }
    private static void copies(Integer expected) {
        System.assertEquals(expected, [SELECT count() FROM Account WHERE Name='Cedar account'], 'child lookup must copy the referenced account');
    }
    private static void interview(String name, Map<String,Object> inputs) {
        Flow.Interview child = Flow.Interview.createInterview(name, inputs);
        child.start();
    }
    @IsTest static void afterLookup() { seed('after'); copies(2); }
    @IsTest static void afterCollection() { seed('afterCollection'); copies(2); }
    @IsTest static void autoLookup() {
        Willow__c thing = seed('auto');
        interview('Cedar_Auto', new Map<String,Object>{'rowId'=>thing.Id}); copies(2);
    }
    @IsTest static void autoRecord() {
        Willow__c thing = seed('record');
        interview('Cedar_Record', new Map<String,Object>{'thingVar'=>thing}); copies(2);
    }
    @IsTest static void autoCollection() {
        Willow__c thing = seed('collection');
        interview('Cedar_CollectionAuto', new Map<String,Object>{'rowId'=>thing.Id}); copies(2);
    }
    @IsTest static void emptyCollection() {
        seed('empty');
        interview('Cedar_CollectionAuto', new Map<String,Object>{'rowId'=>null}); copies(1);
    }
    @IsTest static void directChild() {
        Willow__c thing = seed('direct');
        interview('Cedar_Child', new Map<String,Object>{'thingVar'=>thing}); copies(2);
    }
    @IsTest static void directCollectionChild() {
        Willow__c thing = seed('directCollection');
        interview('Cedar_CollectionChild', new Map<String,Object>{'things'=>new List<Willow__c>{thing}}); copies(2);
    }
    @IsTest static void beforeLookup() {
        Willow__c thing = seed('before'); copies(1);
        System.assertEquals('Cedar account', [SELECT Result__c FROM Willow__c WHERE Id=:thing.Id WITH SYSTEM_MODE].Result__c);
    }
    @IsTest static void autoCollectionVariable() {
        Willow__c thing = seed('collectionVariable');
        interview('Cedar_CollectionVariable', new Map<String,Object>{'things'=>new List<Willow__c>{thing}}); copies(2);
    }
    @IsTest static void autoScalar() {
        Willow__c thing = seed('scalar');
        interview('Cedar_ScalarParent', new Map<String,Object>{'accountId'=>thing.Account__c}); copies(2);
    }
}
