@IsTest public class EventForwardingProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(EventForwardingProbe.observe()));
    }
}
