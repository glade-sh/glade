@IsTest
public class CallbackProjectionProbeTest {
    @IsTest
    static void observe() {
        System.assert(false, 'OBS|' + JSON.serialize(CallbackProjectionProbe.observe()));
    }
}
