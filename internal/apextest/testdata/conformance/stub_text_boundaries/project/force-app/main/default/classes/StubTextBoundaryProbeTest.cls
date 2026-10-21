@IsTest public class StubTextBoundaryProbeTest {
    @IsTest static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(StubTextBoundaryProbe.observe()));
    }
}
