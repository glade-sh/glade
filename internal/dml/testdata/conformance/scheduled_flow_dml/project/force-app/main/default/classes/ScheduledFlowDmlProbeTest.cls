@IsTest
private class ScheduledFlowDmlProbeTest {
    private static Account createProbe(String name) {
        Account probe = new Account(Name = name);
        insert probe;
        insert new Contact(LastName = 'seed', AccountId = probe.Id);
        return probe;
    }

    private static void assertUnchanged(Id probeId, String description) {
        Account stored = [SELECT Description FROM Account WHERE Id = :probeId];
        System.assertEquals(description, stored.Description, 'Description changed during DML');
        System.assertEquals(0, [SELECT COUNT() FROM Contact WHERE LastName LIKE 'flow-run-%'], 'A flow counter was created during DML');
    }

    @IsTest
    static void scheduledUnconditionalInsert() {
        Account probe = createProbe('unconditional-insert');
        assertUnchanged(probe.Id, null);
    }

    @IsTest
    static void scheduledUnconditionalUpdate() {
        Account probe = createProbe('unconditional-update');
        probe.Description = 'seed';
        update probe;
        assertUnchanged(probe.Id, 'seed');
    }

    @IsTest
    static void scheduledUnconditionalDelete() {
        Account probe = createProbe('unconditional-delete');
        delete probe;
        System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Id = :probe.Id], 'The source record was not deleted');
        System.assertEquals(0, [SELECT COUNT() FROM Contact WHERE LastName LIKE 'flow-run-%'], 'A flow counter was created during delete');
    }

    @IsTest
    static void scheduledDecisionInsert() {
        Account probe = createProbe('decision-insert');
        assertUnchanged(probe.Id, null);
    }

    @IsTest
    static void scheduledDecisionUpdate() {
        Account probe = createProbe('decision-update');
        probe.Description = 'seed';
        update probe;
        assertUnchanged(probe.Id, 'seed');
    }

    @IsTest
    static void scheduledDecisionDelete() {
        Account probe = createProbe('decision-delete');
        delete probe;
        System.assertEquals(0, [SELECT COUNT() FROM Account WHERE Id = :probe.Id], 'The source record was not deleted');
        System.assertEquals(0, [SELECT COUNT() FROM Contact WHERE LastName LIKE 'flow-run-%'], 'A flow counter was created during delete');
    }

    @IsTest
    static void scheduledStartStopTest() {
        Test.startTest();
        Account probe = new Account(Name = 'start-stop-insert');
        insert probe;
        Test.stopTest();
        assertUnchanged(probe.Id, null);
    }

    @IsTest
    static void scheduledFilteredInsert() {
        Account probe = createProbe('filtered-insert');
        assertUnchanged(probe.Id, null);
    }

    @IsTest
    static void deleteBeforeInsertControl() {
        Account probe = createProbe('delete-control');
        assertUnchanged(probe.Id, null);
    }

    @IsTest
    static void deleteBeforeUpdateControl() {
        Account probe = createProbe('delete-control');
        probe.Description = 'seed';
        update probe;
        assertUnchanged(probe.Id, 'seed');
    }
}
