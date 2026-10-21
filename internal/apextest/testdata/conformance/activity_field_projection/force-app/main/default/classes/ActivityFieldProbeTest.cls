@IsTest private class ActivityFieldProbeTest {
    @IsTest static void observeFields() {
        System.assert(false,'ACTIVITY_RESULT|'+JSON.serialize(ActivityFieldProbe.observe()));
    }
}
