@IsTest
private class FrozenStandardOrgIsolationTest {
    @IsTest
    static void aWritesStandardRow() {
        Account row = new Account(Name='Glade Frozen Org Isolation Marker');
        insert row;
        System.assertEquals(1, [SELECT COUNT() FROM Account WHERE Name='Glade Frozen Org Isolation Marker']);
    }

    @IsTest
    static void bStartsWithoutPriorRows() {
        System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Name='Glade Frozen Org Isolation Marker']);
    }
}
