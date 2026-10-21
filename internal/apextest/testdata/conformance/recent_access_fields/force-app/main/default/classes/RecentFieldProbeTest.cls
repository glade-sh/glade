@IsTest private class RecentFieldProbeTest {
    @IsTest static void observeFields() {
        RecentFieldProbe.staticQueries();
        System.assert(false,'RECENT_RESULT|'+JSON.serialize(RecentFieldProbe.observe()));
    }
}
