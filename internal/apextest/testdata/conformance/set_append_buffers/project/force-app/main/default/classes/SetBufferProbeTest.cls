@IsTest public class SetBufferProbeTest {
    @IsTest static void observeRoutes() {
        System.assert(false, 'OBS|' + JSON.serialize(SetBufferProbe.observe()));
    }
}
