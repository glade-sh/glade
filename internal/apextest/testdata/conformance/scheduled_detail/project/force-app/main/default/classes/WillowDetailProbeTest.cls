@IsTest
private class WillowDetailProbeTest {
    @IsTest static void queuedSchedule() { System.assert(false, 'OBS|' + JSON.serialize(WillowDetailProbe.observe(false, false))); }
    @IsTest static void queuedBatch() { System.assert(false, 'OBS|' + JSON.serialize(WillowDetailProbe.observe(true, false))); }
    @IsTest static void recordedSchedule() { System.assert(false, 'OBS|' + JSON.serialize(WillowDetailProbe.observe(false, true))); }
    @IsTest static void recordedBatch() { System.assert(false, 'OBS|' + JSON.serialize(WillowDetailProbe.observe(true, true))); }
}
