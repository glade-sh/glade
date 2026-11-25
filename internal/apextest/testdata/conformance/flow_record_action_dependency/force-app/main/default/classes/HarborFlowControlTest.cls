@IsTest
private class HarborFlowControlTest {
    @IsTest static void recordActionRuns() {
        insert new Account(Name = 'Synthetic Flow Control');
        System.assertEquals(1, HarborFlowAction.calls);
    }
}
