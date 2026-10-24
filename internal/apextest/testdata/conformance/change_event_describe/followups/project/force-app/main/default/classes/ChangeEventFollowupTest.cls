@IsTest private class ChangeEventFollowupTest {
    @IsTest static void administratorDescribe() { ChangeEventFollowup.run(); System.assert(false, 'OBSERVED|' + JSON.serialize(ChangeEventFollowup.rows)); }
    @IsTest static void noFieldAccessDescribe() {
        User actor = [SELECT Id FROM User WHERE Alias = 'eventlo' LIMIT 1];
        System.runAs(actor) { ChangeEventFollowup.run(); System.assert(false, 'OBSERVED|' + JSON.serialize(ChangeEventFollowup.rows)); }
    }
}
