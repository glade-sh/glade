@IsTest public class LogAccessProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(LogAccessProbe.observe()));
    }
}
